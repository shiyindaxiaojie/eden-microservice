package service

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/shiyindaxiaojie/eden-go-logger"
	pb "github.com/shiyindaxiaojie/eden-go-registry/api/proto/registry/v1"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/cluster"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/cluster/cp"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/store"
	"google.golang.org/grpc"
)

type subscription struct {
	consumerService string
	ch              chan WatchEvent
}

type catalogService struct {
	store  *store.Registry
	cpNode CPNode
	apNode APNode
	mu     sync.RWMutex
	// namespace -> serviceName -> subscriptions
	subs map[string]map[string][]subscription
	// namespace -> subscriberService -> providerService set
	deps map[string]map[string]map[string]bool
}

func NewCatalogService(s *store.Registry, cp CPNode, ap APNode) CatalogService {
	svc := &catalogService{
		store:  s,
		cpNode: cp,
		apNode: ap,
		subs:   make(map[string]map[string][]subscription),
		deps:   make(map[string]map[string]map[string]bool),
	}
	s.Services.NotifyFunc = svc.handleNotify
	return svc
}

func (s *catalogService) Register(inst *model.Instance) error {
	if !s.store.Namespaces.Exists(inst.Namespace) {
		return fmt.Errorf("namespace not found: %s", inst.EffectiveNamespace())
	}

	mode := s.store.GetMode()
	if mode == "cp" && s.cpNode != nil {
		if !s.cpNode.IsLeader() {
			return s.forwardToLeader("register", inst, "", "")
		}
		cmd := cp.Command{Type: cp.CmdRegister, Instance: inst}
		return s.cpNode.Apply(cmd, 5*time.Second)
	}
	if s.apNode != nil {
		return s.apNode.Apply("register", inst, false)
	}
	s.store.Register(inst)
	s.store.AppendEvent("Client Registration", inst.ServiceName, fmt.Sprintf("%s:%d", inst.Host, inst.Port), "Instance registered")
	return nil
}

func (s *catalogService) namespaceKey(namespace string) string {
	if namespace == "" {
		return model.DefaultNamespace
	}
	return namespace
}

func (s *catalogService) SetInstanceStatus(namespace, serviceName, instanceID string, status string) error {
	var healthStatus model.HealthStatus
	switch status {
	case "online":
		healthStatus = model.HealthPassing
	case "offline":
		healthStatus = model.HealthCritical
	default:
		return fmt.Errorf("invalid status: %s, must be 'online' or 'offline'", status)
	}

	inst, ok := s.store.SetInstanceStatus(namespace, serviceName, instanceID, healthStatus)
	if !ok {
		return fmt.Errorf("instance not found: %s/%s", serviceName, instanceID)
	}
	s.store.AppendEvent("Client Registration", serviceName, fmt.Sprintf("%s:%d", inst.Host, inst.Port), fmt.Sprintf("Status changed to %s", status))
	return nil
}

func (s *catalogService) Heartbeat(namespace, serviceName, instanceID string) error {
	mode := s.store.GetMode()
	if mode == "cp" && s.cpNode != nil {
		if !s.cpNode.IsLeader() {
			return s.forwardToLeader("heartbeat", &model.Instance{Namespace: namespace}, serviceName, instanceID)
		}
		cmd := cp.Command{Type: cp.CmdHeartbeat, Namespace: namespace, ServiceName: serviceName, InstanceID: instanceID}
		return s.cpNode.Apply(cmd, 5*time.Second)
	}
	data := map[string]string{"namespace": namespace, "service_name": serviceName, "instance_id": instanceID}
	if s.apNode != nil {
		return s.apNode.Apply("heartbeat", data, false)
	}
	inst, _ := s.store.Services.HeartbeatNS(namespace, serviceName, instanceID)
	if inst == nil {
		return fmt.Errorf("instance not found")
	}
	s.store.AppendEvent("Heartbeat", serviceName, fmt.Sprintf("%s:%d", inst.Host, inst.Port), "Heartbeat received")
	return nil
}

func (s *catalogService) ListServices(namespace string) ([]interface{}, error) {
	ns := s.namespaceKey(namespace)
	if !s.store.Namespaces.Exists(ns) {
		return nil, fmt.Errorf("namespace not found: %s", ns)
	}

	services := s.store.Services.ListServicesNS(ns)

	result := make([]interface{}, len(services))
	for i, svc := range services {
		subCount := len(s.GetSubscribers(ns, svc.Name))
		result[i] = map[string]interface{}{
			"name":             svc.Name,
			"instance_count":   svc.InstanceCount,
			"healthy_count":    svc.HealthyCount,
			"subscriber_count": subCount,
		}
	}
	return result, nil
}

func (s *catalogService) GetService(namespace, name string, healthyOnly bool) ([]*model.Instance, error) {
	ns := s.namespaceKey(namespace)
	if !s.store.Namespaces.Exists(ns) {
		return nil, fmt.Errorf("namespace not found: %s", ns)
	}

	if healthyOnly {
		return s.store.Services.GetServiceHealthyNS(ns, name), nil
	}
	return s.store.Services.GetServiceNS(ns, name), nil
}

func (s *catalogService) Subscribe(namespace, serviceName, consumerService string, ch chan WatchEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns := s.namespaceKey(namespace)
	if s.subs[ns] == nil {
		s.subs[ns] = make(map[string][]subscription)
	}
	s.subs[ns][serviceName] = append(s.subs[ns][serviceName], subscription{
		consumerService: consumerService,
		ch:              ch,
	})
	if consumerService != "" {
		s.recordDependencyNoLock(ns, consumerService, serviceName)
	}
}

func (s *catalogService) Unsubscribe(namespace, serviceName string, ch chan WatchEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns := s.namespaceKey(namespace)
	subs := s.subs[ns][serviceName]
	for i, v := range subs {
		if v.ch == ch {
			s.subs[ns][serviceName] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
}

func (s *catalogService) GetSubscribers(namespace, serviceName string) []string {
	ns := s.namespaceKey(namespace)
	seen := make(map[string]bool)
	combined := make([]string, 0)

	for _, consumer := range s.getTopologySubscribers(ns, serviceName) {
		if !seen[consumer] {
			seen[consumer] = true
			combined = append(combined, consumer)
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, consumer := range s.getSubscribersNoLock(ns, serviceName) {
		if !seen[consumer] {
			seen[consumer] = true
			combined = append(combined, consumer)
		}
	}
	sort.Strings(combined)
	return combined
}

func (s *catalogService) getSubscribersNoLock(namespace, serviceName string) []string {
	consumers := make([]string, 0)
	seen := make(map[string]bool)
	for consumer, providers := range s.deps[namespace] {
		if providers[serviceName] {
			if !seen[consumer] {
				seen[consumer] = true
				consumers = append(consumers, consumer)
			}
		}
	}
	sort.Strings(consumers)
	return consumers
}

func (s *catalogService) getTopologySubscribers(namespace, serviceName string) []string {
	reports := s.store.Topology.Reports(namespace)
	consumers := make([]string, 0)
	for consumer, report := range reports {
		for _, provider := range report.Providers {
			if provider == serviceName {
				consumers = append(consumers, consumer)
				break
			}
		}
	}
	sort.Strings(consumers)
	return consumers
}

func (s *catalogService) GetDependencyGraph(namespace string) map[string]interface{} {
	ns := s.namespaceKey(namespace)
	services := s.store.Services.ListServicesNS(ns)
	nodeSet := make(map[string]bool)
	for _, svc := range services {
		nodeSet[svc.Name] = true
	}

	// Build nodes
	nodes := make([]map[string]interface{}, 0)
	for _, svc := range services {
		nodes = append(nodes, map[string]interface{}{
			"id":             svc.Name,
			"name":           svc.Name,
			"instance_count": svc.InstanceCount,
			"healthy_count":  svc.HealthyCount,
		})
	}

	// Build edges from dependency tracking
	edges := make([]map[string]string, 0)
	reports := s.store.Topology.Reports(ns)
	for consumer, report := range reports {
		for _, provider := range report.Providers {
			edges = append(edges, map[string]string{
				"source": consumer,
				"target": provider,
			})
			if !nodeSet[consumer] {
				nodes = append(nodes, map[string]interface{}{
					"id":   consumer,
					"name": consumer,
				})
				nodeSet[consumer] = true
			}
			if !nodeSet[provider] {
				nodes = append(nodes, map[string]interface{}{
					"id":   provider,
					"name": provider,
				})
				nodeSet[provider] = true
			}
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	for consumer, providers := range s.deps[ns] {
		for provider := range providers {
			edges = append(edges, map[string]string{
				"source": consumer,
				"target": provider,
			})
			// Ensure nodes exist
			if !nodeSet[consumer] {
				nodes = append(nodes, map[string]interface{}{
					"id":   consumer,
					"name": consumer,
				})
				nodeSet[consumer] = true
			}
		}
	}

	return map[string]interface{}{
		"namespace": ns,
		"nodes":     nodes,
		"edges":     edges,
	}
}

// RecordDependency records that consumerService depends on providerService.
func (s *catalogService) RecordDependency(namespace, consumerService, providerService string) {
	if consumerService == "" || providerService == "" || consumerService == providerService {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.recordDependencyNoLock(s.namespaceKey(namespace), consumerService, providerService)
}

func (s *catalogService) recordDependencyNoLock(namespace, consumerService, providerService string) {
	if s.deps[namespace] == nil {
		s.deps[namespace] = make(map[string]map[string]bool)
	}
	if s.deps[namespace][consumerService] == nil {
		s.deps[namespace][consumerService] = make(map[string]bool)
	}
	s.deps[namespace][consumerService][providerService] = true
}

func (s *catalogService) replaceDependenciesNoLock(namespace, consumerService string, providers []string) {
	if s.deps[namespace] == nil {
		s.deps[namespace] = make(map[string]map[string]bool)
	}
	s.deps[namespace][consumerService] = make(map[string]bool, len(providers))
	for _, provider := range providers {
		if provider == "" || provider == consumerService {
			continue
		}
		s.deps[namespace][consumerService][provider] = true
	}
}

func (s *catalogService) ReportTopology(namespace, consumerService string, providers []string, checksum string) bool {
	ns := s.namespaceKey(namespace)
	changed := s.store.Topology.Report(ns, consumerService, providers, checksum)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.replaceDependenciesNoLock(ns, consumerService, providers)
	return changed
}

func (s *catalogService) GetTopology(namespace string) *model.TopologyGraph {
	ns := s.namespaceKey(namespace)
	serviceNames := make(map[string]bool)
	nodes := make([]model.TopologyNode, 0)

	for _, summary := range s.store.Services.ListServicesNS(ns) {
		serviceNames[summary.Name] = true
	}
	reports := s.store.Topology.Reports(ns)
	for consumer, report := range reports {
		serviceNames[consumer] = true
		for _, provider := range report.Providers {
			serviceNames[provider] = true
		}
	}

	names := make([]string, 0, len(serviceNames))
	for name := range serviceNames {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		instances := s.store.Services.GetServiceNS(ns, name)
		topologyInstances := make([]model.TopologyInstance, 0, len(instances))
		healthyCount := 0
		for _, instance := range instances {
			if instance.Status == model.HealthPassing {
				healthyCount++
			}
			topologyInstances = append(topologyInstances, model.TopologyInstance{
				ID:         instance.ID,
				Host:       instance.Host,
				Port:       instance.Port,
				Status:     instance.Status,
				Datacenter: instance.Datacenter,
			})
		}
		nodes = append(nodes, model.TopologyNode{
			ID:            name,
			Name:          name,
			Namespace:     ns,
			InstanceCount: len(instances),
			HealthyCount:  healthyCount,
			Instances:     topologyInstances,
		})
	}

	edges := make([]model.TopologyEdge, 0)
	for consumer, report := range reports {
		for _, provider := range report.Providers {
			edges = append(edges, model.TopologyEdge{
				Source:    consumer,
				Target:    provider,
				Checksum:  report.Checksum,
				UpdatedAt: report.UpdatedAt,
			})
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Source == edges[j].Source {
			return edges[i].Target < edges[j].Target
		}
		return edges[i].Source < edges[j].Source
	})

	return &model.TopologyGraph{
		Namespace: ns,
		Nodes:     nodes,
		Edges:     edges,
	}
}

// Namespace management
func (s *catalogService) ListNamespaces() []*model.Namespace {
	return s.store.ListNamespaces()
}

func (s *catalogService) CreateNamespace(ns *model.Namespace) bool {
	return s.store.CreateNamespace(ns)
}

func (s *catalogService) UpdateNamespace(ns *model.Namespace) bool {
	return s.store.UpdateNamespace(ns)
}

func (s *catalogService) DeleteNamespace(name string) bool {
	return s.store.DeleteNamespace(name)
}

func (s *catalogService) handleNotify(namespace, serviceName, action string, instances []*model.Instance) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if subs, ok := s.subs[s.namespaceKey(namespace)][serviceName]; ok {
		for _, ch := range subs {
			select {
			case ch.ch <- WatchEvent{Action: action, Instances: instances}:
			default:
				// Skip if slow consumer
			}
		}
	}
}

func (s *catalogService) forwardToLeader(action string, inst *model.Instance, serviceName, instanceID string) error {
	leaderID := s.cpNode.LeaderID()
	if leaderID == "" {
		return fmt.Errorf("no leader available")
	}

	targetSvc := serviceName
	if inst != nil {
		targetSvc = inst.ServiceName
	}
	logger.Info("[Proxy] Forwarding '%s' request for service '%s' to Leader: %s", action, targetSvc, leaderID)

	pm, ok := s.apNode.GetPM().(*cluster.PeerManager)
	if !ok {
		return fmt.Errorf("peer manager unavailable")
	}

	var conn *grpc.ClientConn
	var err error
	pm.Range(func(p *cluster.Peer) bool {
		if p.ID == leaderID {
			conn, err = p.GetConn()
			return false
		}
		return true
	})

	if conn == nil {
		if err != nil {
			return fmt.Errorf("failed to proxy to leader: %w", err)
		}
		return fmt.Errorf("leader connection not found in peer manager for %s", leaderID)
	}

	client := pb.NewRegistryServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch action {
	case "register":
		_, err = client.Register(ctx, &pb.RegisterRequest{
			Instance: &pb.ServiceInstance{
				Id:          inst.ID,
				ServiceName: inst.ServiceName,
				Namespace:   inst.Namespace,
				Host:        inst.Host,
				Port:        int32(inst.Port),
				Weight:      int32(inst.Weight),
				Metadata:    inst.Metadata,
				Datacenter:  inst.Datacenter,
			},
		})
	case "heartbeat":
		_, err = client.Heartbeat(ctx, &pb.HeartbeatRequest{
			Namespace:   inst.Namespace,
			ServiceName: serviceName,
			InstanceId:  instanceID,
		})
	}
	return err
}

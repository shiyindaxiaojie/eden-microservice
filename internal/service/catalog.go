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

	_, ok := s.store.SetInstanceStatus(namespace, serviceName, instanceID, healthStatus)
	if !ok {
		return fmt.Errorf("instance not found: %s/%s", serviceName, instanceID)
	}
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
	s.store.Services.HeartbeatNS(namespace, serviceName, instanceID)
	return nil
}

func (s *catalogService) ListServices(namespace string) ([]interface{}, error) {
	ns := s.namespaceKey(namespace)
	if !s.store.Namespaces.Exists(ns) {
		return nil, fmt.Errorf("namespace not found: %s", ns)
	}

	services := s.store.Services.ListServicesNS(ns)

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]interface{}, len(services))
	for i, svc := range services {
		subCount := len(s.getSubscribersNoLock(ns, svc.Name))
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
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getSubscribersNoLock(s.namespaceKey(namespace), serviceName)
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

func (s *catalogService) GetDependencyGraph(namespace string) map[string]interface{} {
	ns := s.namespaceKey(namespace)

	s.mu.RLock()
	defer s.mu.RUnlock()

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

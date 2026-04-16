package catalog

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/shiyindaxiaojie/eden-go-logger"
	grpc_registry "github.com/shiyindaxiaojie/eden-go-registry/api/proto/registry/v1"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/cluster/replication"
	"google.golang.org/grpc"
)

type WatchEvent struct {
	Action    string
	Instances []*Instance
}

type Registry interface {
	Register(inst *Instance) error
	Deregister(namespace, serviceName, instanceID string) error
	SetInstanceStatus(namespace, serviceName, instanceID string, status string) error
	Heartbeat(namespace, serviceName, instanceID string) error
	ListServices(namespace string) ([]interface{}, error)
	GetService(namespace, name string, healthyOnly bool) ([]*Instance, error)
	Subscribe(namespace, serviceName, consumerService string, ch chan WatchEvent)
	Unsubscribe(namespace, serviceName string, ch chan WatchEvent)
	GetSubscribers(namespace, serviceName string) []string
	RecordDependency(namespace, consumerService, providerService string)
	GetDependencyGraph(namespace string) map[string]interface{}
	ReportTopology(namespace, consumerService string, providers []string, checksum string) bool
	GetTopology(namespace string) *TopologyGraph
	ListNamespaces() []*Namespace
	CreateNamespace(ns *Namespace) bool
	UpdateNamespace(ns *Namespace) bool
	DeleteNamespace(name string) bool
	Metrics() *MetricsStore
}

type CPNode interface {
	Apply(cmd interface{}, timeout time.Duration) error
	IsLeader() bool
	LeaderID() string
	Join(nodeID, addr string) error
}

type APNode interface {
	Apply(cmdType string, data interface{}, isReplicate bool) error
	GetLeaderConn(leaderID string) (*grpc.ClientConn, error)
}

type ModeReader interface {
	GetMode() string
}

type subscription struct {
	consumerService string
	ch              chan WatchEvent
}

type registry struct {
	state      *State
	modeReader ModeReader
	cpNode     CPNode
	apNode     APNode
	mu         sync.RWMutex
	subs       map[string]map[string][]subscription
	deps       map[string]map[string]map[string]bool
}

func NewRegistry(state *State, modeReader ModeReader, cp CPNode, ap APNode) Registry {
	svc := &registry{
		state:      state,
		modeReader: modeReader,
		cpNode:     cp,
		apNode:     ap,
		subs:       make(map[string]map[string][]subscription),
		deps:       make(map[string]map[string]map[string]bool),
	}
	svc.state.Instances.NotifyFunc = svc.handleNotify
	return svc
}

func (s *registry) mode() string {
	if s.modeReader == nil {
		return ""
	}
	return s.modeReader.GetMode()
}

func namespaceKey(namespace string) string {
	if namespace == "" {
		return DefaultNamespace
	}
	return namespace
}

func (s *registry) Register(inst *Instance) error {
	if !s.state.Namespaces.Exists(inst.Namespace) {
		return fmt.Errorf("namespace not found: %s", inst.EffectiveNamespace())
	}

	mode := s.mode()
	if mode == "cp" && s.cpNode != nil {
		if !s.cpNode.IsLeader() {
			return s.forwardToLeader("register", inst, "", "", "")
		}
		cmd := replication.Command{Type: replication.CmdRegister, Instance: toReplicatedInstance(inst)}
		return s.cpNode.Apply(cmd, 5*time.Second)
	}
	if s.apNode != nil {
		return s.apNode.Apply("register", inst, false)
	}
	s.state.Register(inst)
	s.state.AppendEvent(EventTypeServiceRegister, inst.ServiceName, fmt.Sprintf("%s:%d", inst.Host, inst.Port), "Instance registered")
	return nil
}

func (s *registry) Deregister(namespace, serviceName, instanceID string) error {
	mode := s.mode()
	if mode == "cp" && s.cpNode != nil {
		if !s.cpNode.IsLeader() {
			return s.forwardToLeader("deregister", &Instance{Namespace: namespace}, serviceName, instanceID, "")
		}
		cmd := replication.Command{Type: replication.CmdDeregister, Namespace: namespace, ServiceName: serviceName, InstanceID: instanceID}
		return s.cpNode.Apply(cmd, 5*time.Second)
	}

	data := map[string]string{"namespace": namespace, "service_name": serviceName, "instance_id": instanceID}
	if s.apNode != nil {
		return s.apNode.Apply("deregister", data, false)
	}

	inst, ok := s.state.Instances.DeregisterNS(namespace, serviceName, instanceID)
	if !ok {
		return fmt.Errorf("instance not found: %s/%s", serviceName, instanceID)
	}

	s.state.AppendEvent(EventTypeServiceOffline, inst.ServiceName, fmt.Sprintf("%s:%d", inst.Host, inst.Port), "Instance deregistered")
	return nil
}

func toReplicatedInstance(inst *Instance) *replication.Instance {
	if inst == nil {
		return nil
	}
	return &replication.Instance{
		ID:            inst.ID,
		ServiceName:   inst.ServiceName,
		Namespace:     inst.Namespace,
		Host:          inst.Host,
		Port:          inst.Port,
		Weight:        inst.Weight,
		Datacenter:    inst.Datacenter,
		Metadata:      inst.Metadata,
		Status:        string(inst.Status),
		ManualOffline: inst.ManualOffline,
		LastHeartbeat: inst.LastHeartbeat,
		RegisteredAt:  inst.RegisteredAt,
	}
}

func (s *registry) SetInstanceStatus(namespace, serviceName, instanceID string, status string) error {
	var healthStatus HealthStatus
	switch status {
	case "online":
		healthStatus = HealthPassing
	case "offline":
		healthStatus = HealthCritical
	default:
		return fmt.Errorf("invalid status: %s, must be 'online' or 'offline'", status)
	}

	mode := s.mode()
	if mode == "cp" && s.cpNode != nil {
		if !s.cpNode.IsLeader() {
			return s.forwardToLeader("set_instance_status", &Instance{Namespace: namespace}, serviceName, instanceID, status)
		}
		cmd := replication.Command{
			Type:        replication.CmdSetInstanceStatus,
			Namespace:   namespace,
			ServiceName: serviceName,
			InstanceID:  instanceID,
			Status:      status,
		}
		return s.cpNode.Apply(cmd, 5*time.Second)
	}

	data := map[string]string{
		"namespace":    namespace,
		"service_name": serviceName,
		"instance_id":  instanceID,
		"status":       status,
	}
	if s.apNode != nil {
		return s.apNode.Apply("set_instance_status", data, false)
	}

	inst, ok := s.state.SetInstanceStatus(namespace, serviceName, instanceID, healthStatus)
	if !ok {
		return fmt.Errorf("instance not found: %s/%s", serviceName, instanceID)
	}
	resolvedService := inst.ServiceName
	eventType := EventTypeServiceOffline
	message := "Instance taken offline"
	if status == "online" {
		eventType = EventTypeServiceOnline
		message = "Instance restored online"
	}
	s.state.AppendEvent(eventType, resolvedService, fmt.Sprintf("%s:%d", inst.Host, inst.Port), message)
	return nil
}

func (s *registry) Heartbeat(namespace, serviceName, instanceID string) error {
	mode := s.mode()
	if mode == "cp" && s.cpNode != nil {
		if !s.cpNode.IsLeader() {
			return s.forwardToLeader("heartbeat", &Instance{Namespace: namespace}, serviceName, instanceID, "")
		}
		cmd := replication.Command{Type: replication.CmdHeartbeat, Namespace: namespace, ServiceName: serviceName, InstanceID: instanceID}
		return s.cpNode.Apply(cmd, 5*time.Second)
	}
	data := map[string]string{"namespace": namespace, "service_name": serviceName, "instance_id": instanceID}
	if s.apNode != nil {
		return s.apNode.Apply("heartbeat", data, false)
	}
	inst, recovered := s.state.HeartbeatNS(namespace, serviceName, instanceID)
	if inst == nil {
		return fmt.Errorf("instance not found")
	}
	resolvedService := inst.ServiceName
	s.state.AppendEvent(EventTypeServiceHeartbeat, resolvedService, fmt.Sprintf("%s:%d", inst.Host, inst.Port), "Heartbeat received")
	if recovered {
		s.state.AppendEvent(EventTypeServiceOnline, resolvedService, fmt.Sprintf("%s:%d", inst.Host, inst.Port), "Heartbeat recovered instance to online")
	}
	return nil
}

func (s *registry) ListServices(namespace string) ([]interface{}, error) {
	ns := namespaceKey(namespace)
	if !s.state.Namespaces.Exists(ns) {
		return nil, fmt.Errorf("namespace not found: %s", ns)
	}

	services := s.state.Instances.ListServicesNS(ns)
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

func (s *registry) GetService(namespace, name string, healthyOnly bool) ([]*Instance, error) {
	ns := namespaceKey(namespace)
	if !s.state.Namespaces.Exists(ns) {
		return nil, fmt.Errorf("namespace not found: %s", ns)
	}

	if healthyOnly {
		return s.state.Instances.GetServiceHealthyNS(ns, name), nil
	}
	return s.state.Instances.GetServiceNS(ns, name), nil
}

func (s *registry) Subscribe(namespace, serviceName, consumerService string, ch chan WatchEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns := namespaceKey(namespace)
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

func (s *registry) Unsubscribe(namespace, serviceName string, ch chan WatchEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns := namespaceKey(namespace)
	subs := s.subs[ns][serviceName]
	for i, subscriber := range subs {
		if subscriber.ch == ch {
			s.subs[ns][serviceName] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
}

func (s *registry) GetSubscribers(namespace, serviceName string) []string {
	ns := namespaceKey(namespace)
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

func (s *registry) getSubscribersNoLock(namespace, serviceName string) []string {
	consumers := make([]string, 0)
	seen := make(map[string]bool)
	for consumer, providers := range s.deps[namespace] {
		if providers[serviceName] && !seen[consumer] {
			seen[consumer] = true
			consumers = append(consumers, consumer)
		}
	}
	sort.Strings(consumers)
	return consumers
}

func (s *registry) getTopologySubscribers(namespace, serviceName string) []string {
	activeServices := make(map[string]bool)
	for _, summary := range s.state.Instances.ListServicesNS(namespace) {
		activeServices[summary.Name] = true
	}
	if len(activeServices) == 0 || !activeServices[serviceName] {
		return []string{}
	}

	reports := s.state.Topology.Reports(namespace)
	consumers := make([]string, 0)
	for consumer, report := range reports {
		if !activeServices[consumer] {
			continue
		}
		for _, provider := range report.Providers {
			if provider == serviceName && activeServices[provider] {
				consumers = append(consumers, consumer)
				break
			}
		}
	}
	sort.Strings(consumers)
	return consumers
}

func (s *registry) GetDependencyGraph(namespace string) map[string]interface{} {
	ns := namespaceKey(namespace)
	services := s.state.Instances.ListServicesNS(ns)
	nodeSet := make(map[string]bool)
	for _, svc := range services {
		nodeSet[svc.Name] = true
	}
	s.state.Topology.Prune(ns, nodeSet)

	nodes := make([]map[string]interface{}, 0)
	for _, svc := range services {
		nodes = append(nodes, map[string]interface{}{
			"id":             svc.Name,
			"name":           svc.Name,
			"instance_count": svc.InstanceCount,
			"healthy_count":  svc.HealthyCount,
		})
	}

	dependencyEdges := s.dependencyEdges(ns, nodeSet)
	edges := make([]map[string]string, 0, len(dependencyEdges))
	for _, edge := range dependencyEdges {
		edges = append(edges, map[string]string{
			"source": edge.Source,
			"target": edge.Target,
		})
	}

	return map[string]interface{}{
		"namespace": ns,
		"nodes":     nodes,
		"edges":     edges,
	}
}

func (s *registry) RecordDependency(namespace, consumerService, providerService string) {
	if consumerService == "" || providerService == "" || consumerService == providerService {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.recordDependencyNoLock(namespaceKey(namespace), consumerService, providerService)
}

func (s *registry) recordDependencyNoLock(namespace, consumerService, providerService string) {
	if s.deps[namespace] == nil {
		s.deps[namespace] = make(map[string]map[string]bool)
	}
	if s.deps[namespace][consumerService] == nil {
		s.deps[namespace][consumerService] = make(map[string]bool)
	}
	s.deps[namespace][consumerService][providerService] = true
	s.syncTopologyReportNoLock(namespace, consumerService)
}

func (s *registry) replaceDependenciesNoLock(namespace, consumerService string, providers []string) {
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

func (s *registry) syncTopologyReportNoLock(namespace, consumerService string) {
	providers := sortedProviders(s.deps[namespace][consumerService], consumerService)
	checksum := ""
	if existing := s.state.Topology.Reports(namespace)[consumerService]; existing != nil && sameStringSlices(existing.Providers, providers) {
		checksum = existing.Checksum
	}
	s.state.Topology.Report(namespace, consumerService, providers, checksum)
}

func (s *registry) syncTopologyReports(namespace string) {
	s.mu.RLock()
	providersByConsumer := make(map[string][]string, len(s.deps[namespace]))
	for consumerService, providerSet := range s.deps[namespace] {
		providersByConsumer[consumerService] = sortedProviders(providerSet, consumerService)
	}
	s.mu.RUnlock()

	if len(providersByConsumer) == 0 {
		return
	}

	reports := s.state.Topology.Reports(namespace)
	for consumerService, providers := range providersByConsumer {
		checksum := ""
		if existing := reports[consumerService]; existing != nil && sameStringSlices(existing.Providers, providers) {
			checksum = existing.Checksum
		}
		s.state.Topology.Report(namespace, consumerService, providers, checksum)
	}
}

func sortedProviders(providerSet map[string]bool, consumerService string) []string {
	providers := make([]string, 0, len(providerSet))
	for provider := range providerSet {
		if provider == "" || provider == consumerService {
			continue
		}
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	return providers
}

func sameStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func (s *registry) ReportTopology(namespace, consumerService string, providers []string, checksum string) bool {
	ns := namespaceKey(namespace)
	changed := s.state.Topology.Report(ns, consumerService, providers, checksum)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.replaceDependenciesNoLock(ns, consumerService, providers)
	return changed
}

func (s *registry) GetTopology(namespace string) *TopologyGraph {
	ns := namespaceKey(namespace)
	serviceNames := make(map[string]bool)
	serviceSummaries := s.state.Instances.ListServicesNS(ns)
	nodes := make([]TopologyNode, 0)

	for _, summary := range serviceSummaries {
		serviceNames[summary.Name] = true
	}
	s.state.Topology.Prune(ns, serviceNames)
	names := make([]string, 0, len(serviceNames))
	for name := range serviceNames {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		instances := s.state.Instances.GetServiceNS(ns, name)
		topologyInstances := make([]TopologyInstance, 0, len(instances))
		healthyCount := 0
		for _, instance := range instances {
			if instance.Status == HealthPassing {
				healthyCount++
			}
			topologyInstances = append(topologyInstances, TopologyInstance{
				ID:         instance.ID,
				Host:       instance.Host,
				Port:       instance.Port,
				Status:     instance.Status,
				Datacenter: instance.Datacenter,
			})
		}
		nodes = append(nodes, TopologyNode{
			ID:            name,
			Name:          name,
			Namespace:     ns,
			InstanceCount: len(instances),
			HealthyCount:  healthyCount,
			Instances:     topologyInstances,
		})
	}

	edges := s.dependencyEdges(ns, serviceNames)

	return &TopologyGraph{
		Namespace: ns,
		Nodes:     nodes,
		Edges:     edges,
	}
}

func (s *registry) dependencyEdges(namespace string, activeServices map[string]bool) []TopologyEdge {
	s.syncTopologyReports(namespace)
	reports := s.state.Topology.Reports(namespace)
	edgeMap := make(map[string]TopologyEdge)

	addEdge := func(edge TopologyEdge) {
		if !activeServices[edge.Source] || !activeServices[edge.Target] || edge.Source == edge.Target {
			return
		}

		key := edge.Source + "\x00" + edge.Target
		if existing, ok := edgeMap[key]; ok {
			if existing.Checksum == "" && edge.Checksum != "" {
				edgeMap[key] = edge
			}
			return
		}
		edgeMap[key] = edge
	}

	for consumer, report := range reports {
		for _, provider := range report.Providers {
			addEdge(TopologyEdge{
				Source:    consumer,
				Target:    provider,
				Checksum:  report.Checksum,
				UpdatedAt: report.UpdatedAt,
			})
		}
	}

	s.mu.RLock()
	for consumer, providers := range s.deps[namespace] {
		for provider := range providers {
			addEdge(TopologyEdge{
				Source: consumer,
				Target: provider,
			})
		}
	}
	s.mu.RUnlock()

	edges := make([]TopologyEdge, 0, len(edgeMap))
	for _, edge := range edgeMap {
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Source == edges[j].Source {
			return edges[i].Target < edges[j].Target
		}
		return edges[i].Source < edges[j].Source
	})
	return edges
}

func (s *registry) ListNamespaces() []*Namespace {
	return s.state.ListNamespaces()
}

func (s *registry) CreateNamespace(ns *Namespace) bool {
	return s.state.CreateNamespace(ns)
}

func (s *registry) UpdateNamespace(ns *Namespace) bool {
	return s.state.UpdateNamespace(ns)
}

func (s *registry) DeleteNamespace(name string) bool {
	return s.state.DeleteNamespace(name)
}

func (s *registry) Metrics() *MetricsStore {
	return s.state.Metrics
}

func (s *registry) handleNotify(namespace, serviceName, action string, instances []*Instance) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if subs, ok := s.subs[namespaceKey(namespace)][serviceName]; ok {
		for _, subscriber := range subs {
			select {
			case subscriber.ch <- WatchEvent{Action: action, Instances: instances}:
			default:
			}
		}
	}
}

func (s *registry) forwardToLeader(action string, inst *Instance, serviceName, instanceID, status string) error {
	leaderID := s.cpNode.LeaderID()
	if leaderID == "" {
		return fmt.Errorf("no leader available")
	}

	targetSvc := serviceName
	if inst != nil {
		targetSvc = inst.ServiceName
	}
	logger.Info("[Proxy] Forwarding '%s' request for service '%s' to Leader: %s", action, targetSvc, leaderID)

	conn, err := s.apNode.GetLeaderConn(leaderID)
	if err != nil {
		return err
	}

	client := grpc_registry.NewRegistryServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch action {
	case "register":
		_, err = client.Register(ctx, &grpc_registry.RegisterRequest{
			Instance: &grpc_registry.ServiceInstance{
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
	case "deregister":
		_, err = client.Deregister(ctx, &grpc_registry.DeregisterRequest{
			Namespace:   inst.Namespace,
			ServiceName: serviceName,
			InstanceId:  instanceID,
		})
	case "set_instance_status":
		_, err = client.SetInstanceStatus(ctx, &grpc_registry.SetInstanceStatusRequest{
			Namespace:   inst.Namespace,
			ServiceName: serviceName,
			InstanceId:  instanceID,
			Status:      status,
		})
	case "heartbeat":
		_, err = client.Heartbeat(ctx, &grpc_registry.HeartbeatRequest{
			Namespace:   inst.Namespace,
			ServiceName: serviceName,
			InstanceId:  instanceID,
		})
	}
	return err
}

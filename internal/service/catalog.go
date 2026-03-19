package service

import (
	"context"
	"fmt"
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

type catalogService struct {
	store  *store.Registry
	cpNode CPNode
	apNode APNode
	mu     sync.RWMutex
	subs   map[string][]chan []*model.Instance
	// Track dependencies: subscriberService -> set of providerServices
	deps   map[string]map[string]bool
}

func NewCatalogService(s *store.Registry, cp CPNode, ap APNode) CatalogService {
	svc := &catalogService{
		store:  s,
		cpNode: cp,
		apNode: ap,
		subs:   make(map[string][]chan []*model.Instance),
		deps:   make(map[string]map[string]bool),
	}
	s.Services.NotifyFunc = svc.handleNotify
	return svc
}

func (s *catalogService) Register(inst *model.Instance) error {
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

func (s *catalogService) Heartbeat(serviceName, instanceID string) error {
	mode := s.store.GetMode()
	if mode == "cp" && s.cpNode != nil {
		if !s.cpNode.IsLeader() {
			return s.forwardToLeader("heartbeat", nil, serviceName, instanceID)
		}
		cmd := cp.Command{Type: cp.CmdHeartbeat, ServiceName: serviceName, InstanceID: instanceID}
		return s.cpNode.Apply(cmd, 5*time.Second)
	}
	data := map[string]string{"service_name": serviceName, "instance_id": instanceID}
	if s.apNode != nil {
		return s.apNode.Apply("heartbeat", data, false)
	}
	s.store.Heartbeat(serviceName, instanceID)
	return nil
}

func (s *catalogService) ListServices() ([]interface{}, error) {
	services := s.store.ListServices()

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]interface{}, len(services))
	for i, svc := range services {
		subCount := len(s.subs[svc.Name])
		result[i] = map[string]interface{}{
			"name":             svc.Name,
			"instance_count":   svc.InstanceCount,
			"healthy_count":    svc.HealthyCount,
			"subscriber_count": subCount,
		}
	}
	return result, nil
}

func (s *catalogService) GetService(name string, healthyOnly bool) ([]*model.Instance, error) {
	if healthyOnly {
		return s.store.GetServiceHealthy(name), nil
	}
	return s.store.GetService(name), nil
}

func (s *catalogService) Subscribe(serviceName string, ch chan []*model.Instance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subs[serviceName] = append(s.subs[serviceName], ch)
}

func (s *catalogService) Unsubscribe(serviceName string, ch chan []*model.Instance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	subs := s.subs[serviceName]
	for i, v := range subs {
		if v == ch {
			s.subs[serviceName] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
}

func (s *catalogService) GetSubscribers(serviceName string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return consumer service names that depend on this service
	var consumers []string
	for consumer, providers := range s.deps {
		if providers[serviceName] {
			consumers = append(consumers, consumer)
		}
	}
	return consumers
}

func (s *catalogService) GetDependencyGraph(namespace string) map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect all service names from store
	services := s.store.ListServices()
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
	for consumer, providers := range s.deps {
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
		"nodes": nodes,
		"edges": edges,
	}
}

// RecordDependency records that consumerService depends on providerService.
func (s *catalogService) RecordDependency(consumerService, providerService string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deps[consumerService] == nil {
		s.deps[consumerService] = make(map[string]bool)
	}
	s.deps[consumerService][providerService] = true
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

func (s *catalogService) handleNotify(serviceName string, instances []*model.Instance) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if subs, ok := s.subs[serviceName]; ok {
		for _, ch := range subs {
			select {
			case ch <- instances:
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
				Host:        inst.Host,
				Port:        int32(inst.Port),
				Weight:      int32(inst.Weight),
				Metadata:    inst.Metadata,
				Datacenter:  inst.Datacenter,
			},
		})
	case "heartbeat":
		_, err = client.Heartbeat(ctx, &pb.HeartbeatRequest{ServiceName: serviceName, InstanceId: instanceID})
	}
	return err
}

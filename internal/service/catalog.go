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
}

func NewCatalogService(s *store.Registry, cp CPNode, ap APNode) CatalogService {
	svc := &catalogService{
		store:  s,
		cpNode: cp,
		apNode: ap,
		subs:   make(map[string][]chan []*model.Instance),
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

func (s *catalogService) Deregister(serviceName, instanceID string) error {
	mode := s.store.GetMode()
	if mode == "cp" && s.cpNode != nil {
		if !s.cpNode.IsLeader() {
			return s.forwardToLeader("deregister", nil, serviceName, instanceID)
		}
		cmd := cp.Command{Type: cp.CmdDeregister, ServiceName: serviceName, InstanceID: instanceID}
		return s.cpNode.Apply(cmd, 5*time.Second)
	}
	data := map[string]string{"service_name": serviceName, "instance_id": instanceID}
	if s.apNode != nil {
		return s.apNode.Apply("deregister", data, false)
	}
	s.store.Deregister(serviceName, instanceID)
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
	result := make([]interface{}, len(services))
	for i, svc := range services {
		result[i] = svc
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
	case "deregister":
		_, err = client.Deregister(ctx, &pb.DeregisterRequest{ServiceName: serviceName, InstanceId: instanceID})
	case "heartbeat":
		_, err = client.Heartbeat(ctx, &pb.HeartbeatRequest{ServiceName: serviceName, InstanceId: instanceID})
	}
	return err
}

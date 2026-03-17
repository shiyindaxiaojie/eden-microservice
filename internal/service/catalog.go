package service

import (
	"sync"
	"time"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/cluster/cp"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/store"
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

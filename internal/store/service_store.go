package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
)

// ServiceStore handles service instance management and health.
type ServiceStore struct {
	mu         sync.RWMutex
	services   map[string]map[string]*model.Instance // serviceName -> instanceID -> Instance
	dataPath   string
	NotifyFunc func(serviceName string, instances []*model.Instance)
}

func NewServiceStore(dataPath string) *ServiceStore {
	s := &ServiceStore{
		services: make(map[string]map[string]*model.Instance),
		dataPath: dataPath,
	}
	s.Load()
	return s
}

// Register adds or updates an instance.
func (s *ServiceStore) Register(inst *model.Instance) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.services[inst.ServiceName]; !ok {
		s.services[inst.ServiceName] = make(map[string]*model.Instance)
	}

	inst.Status = model.HealthPassing
	inst.LastHeartbeat = time.Now()
	if inst.RegisteredAt.IsZero() {
		inst.RegisteredAt = time.Now()
	}
	if inst.Weight <= 0 {
		inst.Weight = 1
	}

	s.services[inst.ServiceName][inst.ID] = inst
	
	if s.NotifyFunc != nil {
		instances := s.getServiceNoLock(inst.ServiceName)
		go s.NotifyFunc(inst.ServiceName, instances)
	}
}

// Deregister removes an instance.
func (s *ServiceStore) Deregister(serviceName, instanceID string) (*model.Instance, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	instances, ok := s.services[serviceName]
	if !ok {
		return nil, false
	}

	inst, exists := instances[instanceID]
	if !exists {
		return nil, false
	}

	delete(instances, instanceID)
	if len(instances) == 0 {
		delete(s.services, serviceName)
	}

	if s.NotifyFunc != nil {
		instances := s.getServiceNoLock(serviceName)
		go s.NotifyFunc(serviceName, instances)
	}

	return inst, true
}

// Heartbeat updates the last heartbeat time for an instance.
func (s *ServiceStore) Heartbeat(serviceName, instanceID string) (*model.Instance, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	instances, ok := s.services[serviceName]
	if !ok {
		return nil, false
	}
	inst, exists := instances[instanceID]
	if !exists {
		return nil, false
	}
	inst.LastHeartbeat = time.Now()
	recovered := false
	if inst.Status == model.HealthCritical {
		inst.Status = model.HealthPassing
		recovered = true
	}
	
	if recovered && s.NotifyFunc != nil {
		instances := s.getServiceNoLock(serviceName)
		go s.NotifyFunc(serviceName, instances)
	}
	
	return inst, recovered
}

// GetService returns all instances for a given service name.
func (s *ServiceStore) GetService(name string) []*model.Instance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getServiceNoLock(name)
}

func (s *ServiceStore) getServiceNoLock(name string) []*model.Instance {
	instances, ok := s.services[name]
	if !ok {
		return nil
	}
	result := make([]*model.Instance, 0, len(instances))
	for _, inst := range instances {
		cp := *inst
		result = append(result, &cp)
	}
	return result
}

// GetServiceHealthy returns only healthy instances.
func (s *ServiceStore) GetServiceHealthy(name string) []*model.Instance {
	s.mu.RLock()
	defer s.mu.RUnlock()

	instances, ok := s.services[name]
	if !ok {
		return nil
	}
	result := make([]*model.Instance, 0, len(instances))
	for _, inst := range instances {
		if inst.Status == model.HealthPassing {
			cp := *inst
			result = append(result, &cp)
		}
	}
	return result
}

// ListServices returns a summary of all registered services.
func (s *ServiceStore) ListServices() []ServiceSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ServiceSummary, 0, len(s.services))
	for name, instances := range s.services {
		healthy := 0
		for _, inst := range instances {
			if inst.Status == model.HealthPassing {
				healthy++
			}
		}
		result = append(result, ServiceSummary{
			Name:          name,
			InstanceCount: len(instances),
			HealthyCount:  healthy,
		})
	}
	return result
}

// Stats returns aggregate statistics.
func (s *ServiceStore) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var st Stats
	st.ServiceCount = len(s.services)
	for _, instances := range s.services {
		st.InstanceCount += len(instances)
		for _, inst := range instances {
			if inst.Status == model.HealthPassing {
				st.HealthyCount++
			}
		}
	}
	if st.InstanceCount > 0 {
		st.HealthRate = float64(st.HealthyCount) / float64(st.InstanceCount) * 100
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	st.MemoryUsage = ms.Alloc
	return st
}

// MarkCritical marks instances that have not sent heartbeat within ttl as critical.
func (s *ServiceStore) MarkCritical(ttl time.Duration) ([]*model.Instance, []*model.Instance) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var markedCritical []*model.Instance
	var removed []*model.Instance

	for svcName, instances := range s.services {
		changed := false
		for id, inst := range instances {
			if now.Sub(inst.LastHeartbeat) > ttl {
				if inst.Status == model.HealthPassing {
					inst.Status = model.HealthCritical
					markedCritical = append(markedCritical, inst)
					changed = true
				}
				// If critical for more than 2x TTL, remove entirely
				if now.Sub(inst.LastHeartbeat) > ttl*2 {
					delete(instances, id)
					removed = append(removed, inst)
					changed = true
				}
			}
		}
		if len(instances) == 0 {
			delete(s.services, svcName)
			changed = true
		}
		
		if changed && s.NotifyFunc != nil {
			instances := s.getServiceNoLock(svcName)
			go s.NotifyFunc(svcName, instances)
		}
	}
	return markedCritical, removed
}

// GetAll returns a snapshot of all services.
func (s *ServiceStore) GetAll() map[string]map[string]*model.Instance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	copy := make(map[string]map[string]*model.Instance, len(s.services))
	for name, instances := range s.services {
		instCopy := make(map[string]*model.Instance, len(instances))
		for id, inst := range instances {
			instCopy[id] = inst
		}
		copy[name] = instCopy
	}
	return copy
}

// Merge merges remote state into local state.
func (s *ServiceStore) Merge(remote map[string]map[string]*model.Instance) {
	s.mu.Lock()
	defer s.mu.Unlock()

	changed := false
	for svcName, remoteInsts := range remote {
		if _, ok := s.services[svcName]; !ok {
			s.services[svcName] = make(map[string]*model.Instance)
		}
		localInsts := s.services[svcName]
		for id, rInst := range remoteInsts {
			if lInst, ok := localInsts[id]; ok {
				// Keep the one with later heartbeat
				if rInst.LastHeartbeat.After(lInst.LastHeartbeat) {
					localInsts[id] = rInst
					changed = true
				}
			} else {
				localInsts[id] = rInst
				changed = true
			}
		}
	}
	
	if changed {
		s.Save()
	}
}

// Restore replaces total state.
func (s *ServiceStore) Restore(services map[string]map[string]*model.Instance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.services = services
	s.Save()
}

func (s *ServiceStore) Load() {
	if s.dataPath == "" {
		return
	}
	file := filepath.Join(s.dataPath, "services.json")
	data, err := os.ReadFile(file)
	if err == nil {
		var services map[string][]*model.Instance
		if err := json.Unmarshal(data, &services); err == nil {
			m := make(map[string]map[string]*model.Instance, len(services))
			for name, list := range services {
				mm := make(map[string]*model.Instance, len(list))
				for _, inst := range list {
					mm[inst.ID] = inst
				}
				m[name] = mm
			}
			s.services = m
		}
	}
}

func (s *ServiceStore) Save() {
	if s.dataPath == "" {
		return
	}
	os.MkdirAll(s.dataPath, 0755)
	file := filepath.Join(s.dataPath, "services.json")
	all := s.GetAll()
	output := make(map[string][]*model.Instance, len(all))
	for name, m := range all {
		list := make([]*model.Instance, 0, len(m))
		for _, inst := range m {
			list = append(list, inst)
		}
		output[name] = list
	}
	data, _ := json.MarshalIndent(output, "", "  ")
	_ = os.WriteFile(file, data, 0644)
}

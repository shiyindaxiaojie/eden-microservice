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

// SubscriberInfo describes a subscription channel and its source.
type SubscriberInfo struct {
	ServiceName  string `json:"service_name"`
	ConsumerName string `json:"consumer_name,omitempty"`
}

// ServiceStore handles service instance management and health.
type ServiceStore struct {
	mu sync.RWMutex
	// namespace -> serviceName -> instanceID -> Instance
	services   map[string]map[string]map[string]*model.Instance
	dataPath   string
	NotifyFunc func(namespace, serviceName, action string, instances []*model.Instance)
}

func NewServiceStore(dataPath string) *ServiceStore {
	s := &ServiceStore{
		services: make(map[string]map[string]map[string]*model.Instance),
		dataPath: dataPath,
	}
	s.Load()
	return s
}

// effectiveNS returns the effective namespace, defaulting to "default".
func effectiveNS(ns string) string {
	if ns == "" {
		return model.DefaultNamespace
	}
	return ns
}

// Register adds or updates an instance.
func (s *ServiceStore) Register(inst *model.Instance) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns := effectiveNS(inst.Namespace)
	if _, ok := s.services[ns]; !ok {
		s.services[ns] = make(map[string]map[string]*model.Instance)
	}
	if _, ok := s.services[ns][inst.ServiceName]; !ok {
		s.services[ns][inst.ServiceName] = make(map[string]*model.Instance)
	}

	inst.Status = model.HealthPassing
	inst.LastHeartbeat = time.Now()
	if inst.RegisteredAt.IsZero() {
		inst.RegisteredAt = time.Now()
	}
	if inst.Weight <= 0 {
		inst.Weight = 1
	}

	s.services[ns][inst.ServiceName][inst.ID] = inst

	s.notifyNoLock(ns, inst.ServiceName, "online")
}

// Deregister removes an instance.
func (s *ServiceStore) Deregister(serviceName, instanceID string) (*model.Instance, bool) {
	return s.DeregisterNS("", serviceName, instanceID)
}

// DeregisterNS removes an instance from a specific namespace.
func (s *ServiceStore) DeregisterNS(namespace, serviceName, instanceID string) (*model.Instance, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns := effectiveNS(namespace)
	nsSvcs, ok := s.services[ns]
	if !ok {
		return nil, false
	}
	instances, ok := nsSvcs[serviceName]
	if !ok {
		return nil, false
	}

	inst, exists := instances[instanceID]
	if !exists {
		return nil, false
	}

	delete(instances, instanceID)
	if len(instances) == 0 {
		delete(nsSvcs, serviceName)
	}
	if len(nsSvcs) == 0 {
		delete(s.services, ns)
	}

	s.notifyNoLock(ns, serviceName, "offline")

	return inst, true
}

// SetInstanceStatus sets an instance to online or offline.
func (s *ServiceStore) SetInstanceStatus(namespace, serviceName, instanceID string, status model.HealthStatus) (*model.Instance, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns := effectiveNS(namespace)
	nsSvcs, ok := s.services[ns]
	if !ok {
		return nil, false
	}
	instances, ok := nsSvcs[serviceName]
	if !ok {
		return nil, false
	}
	inst, exists := instances[instanceID]
	if !exists {
		return nil, false
	}

	inst.Status = status
	if status == model.HealthPassing {
		inst.LastHeartbeat = time.Now()
	}

	action := "update"
	if status == model.HealthPassing {
		action = "online"
	} else if status == model.HealthCritical {
		action = "offline"
	}
	s.notifyNoLock(ns, serviceName, action)

	return inst, true
}

// Heartbeat updates the last heartbeat time for an instance.
func (s *ServiceStore) Heartbeat(serviceName, instanceID string) (*model.Instance, bool) {
	return s.HeartbeatNS("", serviceName, instanceID)
}

// HeartbeatNS updates heartbeat for an instance in a specific namespace.
func (s *ServiceStore) HeartbeatNS(namespace, serviceName, instanceID string) (*model.Instance, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns := effectiveNS(namespace)
	nsSvcs, ok := s.services[ns]
	if !ok {
		return nil, false
	}
	instances, ok := nsSvcs[serviceName]
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

	if recovered {
		s.notifyNoLock(ns, serviceName, "online")
	}

	return inst, recovered
}

// GetService returns all instances for a given service name (default namespace).
func (s *ServiceStore) GetService(name string) []*model.Instance {
	return s.GetServiceNS("", name)
}

// GetServiceNS returns all instances for a given service in a namespace.
func (s *ServiceStore) GetServiceNS(namespace, name string) []*model.Instance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getServiceNoLock(effectiveNS(namespace), name)
}

func (s *ServiceStore) getServiceNoLock(namespace, name string) []*model.Instance {
	nsSvcs, ok := s.services[effectiveNS(namespace)]
	if !ok {
		return nil
	}
	instances, ok := nsSvcs[name]
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

// GetServiceHealthy returns only healthy instances (default namespace).
func (s *ServiceStore) GetServiceHealthy(name string) []*model.Instance {
	return s.GetServiceHealthyNS("", name)
}

// GetServiceHealthyNS returns only healthy instances in a namespace.
func (s *ServiceStore) GetServiceHealthyNS(namespace, name string) []*model.Instance {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ns := effectiveNS(namespace)
	nsSvcs, ok := s.services[ns]
	if !ok {
		return nil
	}
	instances, ok := nsSvcs[name]
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

// ListServices returns a summary of all registered services (default namespace).
func (s *ServiceStore) ListServices() []ServiceSummary {
	return s.ListServicesNS("")
}

// ListServicesNS returns service summaries for a specific namespace.
func (s *ServiceStore) ListServicesNS(namespace string) []ServiceSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ns := effectiveNS(namespace)
	nsSvcs, ok := s.services[ns]
	if !ok {
		return nil
	}

	result := make([]ServiceSummary, 0, len(nsSvcs))
	for name, instances := range nsSvcs {
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

// Stats returns aggregate statistics across all namespaces.
func (s *ServiceStore) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var st Stats
	for _, nsSvcs := range s.services {
		st.ServiceCount += len(nsSvcs)
		for _, instances := range nsSvcs {
			st.InstanceCount += len(instances)
			for _, inst := range instances {
				if inst.Status == model.HealthPassing {
					st.HealthyCount++
				}
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

	for ns, nsSvcs := range s.services {
		for svcName, instances := range nsSvcs {
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
				delete(nsSvcs, svcName)
				changed = true
			}

			if changed {
				s.notifyNoLock(ns, svcName, "offline")
			}
		}
		if len(nsSvcs) == 0 {
			delete(s.services, ns)
		}
	}
	return markedCritical, removed
}

func (s *ServiceStore) notifyNoLock(namespace, serviceName, action string) {
	if s.NotifyFunc == nil {
		return
	}

	insts := s.getServiceNoLock(namespace, serviceName)
	go s.NotifyFunc(namespace, serviceName, action, insts)
}

// GetAll returns a snapshot of all services (flat, for backward-compat sync).
func (s *ServiceStore) GetAll() map[string]map[string]*model.Instance {
	s.mu.RLock()
	defer s.mu.RUnlock()

	copy := make(map[string]map[string]*model.Instance)
	for _, nsSvcs := range s.services {
		for name, instances := range nsSvcs {
			instCopy := make(map[string]*model.Instance, len(instances))
			for id, inst := range instances {
				instCopy[id] = inst
			}
			copy[name] = instCopy
		}
	}
	return copy
}

// GetAllNS returns a snapshot of all services keyed by namespace.
func (s *ServiceStore) GetAllNS() map[string]map[string]map[string]*model.Instance {
	s.mu.RLock()
	defer s.mu.RUnlock()

	copy := make(map[string]map[string]map[string]*model.Instance, len(s.services))
	for ns, nsSvcs := range s.services {
		nsCopy := make(map[string]map[string]*model.Instance, len(nsSvcs))
		for name, instances := range nsSvcs {
			instCopy := make(map[string]*model.Instance, len(instances))
			for id, inst := range instances {
				instCopy[id] = inst
			}
			nsCopy[name] = instCopy
		}
		copy[ns] = nsCopy
	}
	return copy
}

// Merge merges remote state into local state (default namespace for backward compat).
func (s *ServiceStore) Merge(remote map[string]map[string]*model.Instance) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns := model.DefaultNamespace
	changed := false
	if _, ok := s.services[ns]; !ok {
		s.services[ns] = make(map[string]map[string]*model.Instance)
	}
	for svcName, remoteInsts := range remote {
		if _, ok := s.services[ns][svcName]; !ok {
			s.services[ns][svcName] = make(map[string]*model.Instance)
		}
		localInsts := s.services[ns][svcName]
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
		s.saveNoLock()
	}
}

// MergeNS merges remote state into local state with namespace support.
func (s *ServiceStore) MergeNS(remote map[string]map[string]map[string]*model.Instance) {
	s.mu.Lock()
	defer s.mu.Unlock()

	changed := false
	for ns, remoteServices := range remote {
		if _, ok := s.services[ns]; !ok {
			s.services[ns] = make(map[string]map[string]*model.Instance)
		}
		for svcName, remoteInsts := range remoteServices {
			if _, ok := s.services[ns][svcName]; !ok {
				s.services[ns][svcName] = make(map[string]*model.Instance)
			}
			localInsts := s.services[ns][svcName]
			for id, rInst := range remoteInsts {
				if lInst, ok := localInsts[id]; ok {
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
	}

	if changed {
		s.saveNoLock()
	}
}

// Restore replaces total state (backward compat: flat map -> default namespace).
func (s *ServiceStore) Restore(services map[string]map[string]*model.Instance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.services = map[string]map[string]map[string]*model.Instance{
		model.DefaultNamespace: services,
	}
	s.saveNoLock()
}

// RestoreNS replaces total state with namespace support.
func (s *ServiceStore) RestoreNS(services map[string]map[string]map[string]*model.Instance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.services = services
	s.saveNoLock()
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
			// Migrate flat format to namespace-scoped format
			m := make(map[string]map[string]map[string]*model.Instance)
			for name, list := range services {
				for _, inst := range list {
					ns := effectiveNS(inst.Namespace)
					if _, ok := m[ns]; !ok {
						m[ns] = make(map[string]map[string]*model.Instance)
					}
					if _, ok := m[ns][name]; !ok {
						m[ns][name] = make(map[string]*model.Instance)
					}
					m[ns][name][inst.ID] = inst
				}
			}
			s.services = m
		}
	}
}

func (s *ServiceStore) Save() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.saveNoLock()
}

// saveNoLock persists services to disk. Caller must hold s.mu (read or write).
// Saves in flat format (serviceName -> instances) for backward compatibility.
func (s *ServiceStore) saveNoLock() {
	if s.dataPath == "" {
		return
	}
	os.MkdirAll(s.dataPath, 0755)
	file := filepath.Join(s.dataPath, "services.json")
	output := make(map[string][]*model.Instance)
	for _, nsSvcs := range s.services {
		for name, instances := range nsSvcs {
			list := make([]*model.Instance, 0, len(instances))
			for _, inst := range instances {
				cp := *inst
				list = append(list, &cp)
			}
			output[name] = append(output[name], list...)
		}
	}
	data, _ := json.MarshalIndent(output, "", "  ")
	_ = os.WriteFile(file, data, 0644)
}

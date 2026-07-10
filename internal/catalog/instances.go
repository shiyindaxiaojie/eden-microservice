package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shiyindaxiaojie/eden-registry/internal/metrics/process"
)

// SubscriberInfo describes a subscription channel and its source.
type SubscriberInfo struct {
	ServiceName  string `json:"service_name"`
	ConsumerName string `json:"consumer_name,omitempty"`
}

// InstanceRegistry tracks service instances and their health state.
type InstanceRegistry struct {
	mu sync.RWMutex
	// namespace -> qualified service name -> instanceID -> Instance
	services        map[string]map[string]map[string]*Instance
	dataPath        string
	persistenceMode string
	flushInterval   time.Duration
	dirty           bool
	saveTimer       *time.Timer
	NotifyFunc      func(namespace, serviceName, action string, instances []*Instance)
}

const defaultRegistryFlushInterval = time.Second

func NewInstanceRegistry(dataPath string) *InstanceRegistry {
	s := &InstanceRegistry{
		services:        make(map[string]map[string]map[string]*Instance),
		dataPath:        dataPath,
		persistenceMode: "async",
		flushInterval:   defaultRegistryFlushInterval,
	}
	s.Load()
	return s
}

func normalizeRegistryFlushMode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), "sync") {
		return "sync"
	}
	return "async"
}

func normalizeRegistryFlushInterval(interval time.Duration) time.Duration {
	if interval <= 0 {
		return defaultRegistryFlushInterval
	}
	return interval
}

// effectiveNS returns the effective namespace, defaulting to "default".
func effectiveNS(ns string) string {
	ns = strings.TrimSpace(ns)
	if ns == "" {
		return DefaultNamespace
	}
	return ns
}

// Register adds or updates an instance.
func (s *InstanceRegistry) Register(inst *Instance) {
	s.mu.Lock()
	defer s.mu.Unlock()

	inst.NormalizeIdentity()
	ns := effectiveNS(inst.Namespace)
	qualifiedName := inst.QualifiedServiceName()
	if _, ok := s.services[ns]; !ok {
		s.services[ns] = make(map[string]map[string]*Instance)
	}
	if _, ok := s.services[ns][qualifiedName]; !ok {
		s.services[ns][qualifiedName] = make(map[string]*Instance)
	}

	inst.Status = HealthPassing
	inst.ManualOffline = false
	inst.LastHeartbeat = time.Now()
	if inst.RegisteredAt.IsZero() {
		inst.RegisteredAt = time.Now()
	}
	if inst.Weight <= 0 {
		inst.Weight = 1
	}

	s.services[ns][qualifiedName][inst.ID] = inst

	s.schedulePersistNoLock()
	s.notifyNoLock(ns, qualifiedName, "online")
}

// Deregister removes an instance.
func (s *InstanceRegistry) Deregister(serviceName, instanceID string) (*Instance, bool) {
	return s.DeregisterNS("", serviceName, instanceID)
}

// DeregisterNS removes an instance from a specific namespace.
func (s *InstanceRegistry) DeregisterNS(namespace, serviceName, instanceID string) (*Instance, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns := effectiveNS(namespace)
	nsSvcs, ok := s.services[ns]
	if !ok {
		return nil, false
	}

	resolvedService, instances, inst, ok := s.resolveInstanceNoLock(ns, serviceName, instanceID)
	if !ok {
		return nil, false
	}

	delete(instances, instanceID)
	if len(instances) == 0 {
		delete(nsSvcs, resolvedService)
	}
	if len(nsSvcs) == 0 {
		delete(s.services, ns)
	}

	s.schedulePersistNoLock()
	s.notifyNoLock(ns, resolvedService, "offline")

	return inst, true
}

// SetInstanceStatus sets an instance to online or offline.
func (s *InstanceRegistry) SetInstanceStatus(namespace, serviceName, instanceID string, status HealthStatus) (*Instance, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns := effectiveNS(namespace)
	resolvedService, _, inst, ok := s.resolveInstanceNoLock(ns, serviceName, instanceID)
	if !ok {
		return nil, false
	}

	inst.Status = status
	inst.ManualOffline = status == HealthOffline

	action := "update"
	if status == HealthPassing {
		action = "online"
	} else if status == HealthOffline {
		action = "offline"
	}
	s.schedulePersistNoLock()
	s.notifyNoLock(ns, resolvedService, action)

	return inst, true
}

// Heartbeat updates the last heartbeat time for an instance.
func (s *InstanceRegistry) Heartbeat(serviceName, instanceID string) (*Instance, bool) {
	return s.HeartbeatNS("", serviceName, instanceID)
}

// HeartbeatNS updates heartbeat for an instance in a specific namespace.
func (s *InstanceRegistry) HeartbeatNS(namespace, serviceName, instanceID string) (*Instance, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns := effectiveNS(namespace)
	resolvedService, _, inst, ok := s.resolveInstanceNoLock(ns, serviceName, instanceID)
	if !ok {
		return nil, false
	}
	inst.LastHeartbeat = time.Now()
	recovered := false
	if inst.Status == HealthCritical && !inst.ManualOffline {
		inst.Status = HealthPassing
		recovered = true
	}

	if recovered {
		s.schedulePersistNoLock()
		s.notifyNoLock(ns, resolvedService, "online")
	}

	return inst, recovered
}

func (s *InstanceRegistry) resolveInstanceNoLock(namespace, serviceName, instanceID string) (string, map[string]*Instance, *Instance, bool) {
	nsSvcs, ok := s.services[effectiveNS(namespace)]
	if !ok || instanceID == "" {
		return "", nil, nil, false
	}

	if serviceName != "" {
		qualifiedName := QualifiedServiceName("", serviceName)
		if instances, ok := nsSvcs[qualifiedName]; ok {
			if inst, exists := instances[instanceID]; exists {
				return qualifiedName, instances, inst, true
			}
		}
	}

	for resolvedService, instances := range nsSvcs {
		if inst, exists := instances[instanceID]; exists {
			return resolvedService, instances, inst, true
		}
	}

	return "", nil, nil, false
}

// GetService returns all instances for a given service name (default namespace).
func (s *InstanceRegistry) GetService(name string) []*Instance {
	return s.GetServiceNS("", name)
}

// GetServiceNS returns all instances for a given service in a namespace.
func (s *InstanceRegistry) GetServiceNS(namespace, name string) []*Instance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getServiceNoLock(effectiveNS(namespace), name)
}

func (s *InstanceRegistry) getServiceNoLock(namespace, name string) []*Instance {
	nsSvcs, ok := s.services[effectiveNS(namespace)]
	if !ok {
		return nil
	}
	instances, ok := nsSvcs[QualifiedServiceName("", name)]
	if !ok {
		return nil
	}
	result := make([]*Instance, 0, len(instances))
	for _, inst := range instances {
		cp := *inst
		result = append(result, &cp)
	}
	return result
}

// GetServiceHealthy returns only healthy instances (default namespace).
func (s *InstanceRegistry) GetServiceHealthy(name string) []*Instance {
	return s.GetServiceHealthyNS("", name)
}

// GetServiceHealthyNS returns only healthy instances in a namespace.
func (s *InstanceRegistry) GetServiceHealthyNS(namespace, name string) []*Instance {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ns := effectiveNS(namespace)
	nsSvcs, ok := s.services[ns]
	if !ok {
		return nil
	}
	instances, ok := nsSvcs[QualifiedServiceName("", name)]
	if !ok {
		return nil
	}
	result := make([]*Instance, 0, len(instances))
	for _, inst := range instances {
		if inst.Status == HealthPassing {
			cp := *inst
			result = append(result, &cp)
		}
	}
	return result
}

// ListServices returns a summary of all registered services (default namespace).
func (s *InstanceRegistry) ListServices() []ServiceSummary {
	return s.ListServicesNS("")
}

// ListServicesNS returns service summaries for a specific namespace.
func (s *InstanceRegistry) ListServicesNS(namespace string) []ServiceSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ns := effectiveNS(namespace)
	nsSvcs, ok := s.services[ns]
	if !ok {
		return nil
	}

	result := make([]ServiceSummary, 0, len(nsSvcs))
	for qualifiedName, instances := range nsSvcs {
		healthy := 0
		name, group := NormalizeServiceIdentity(qualifiedName, "")
		for _, inst := range instances {
			name = inst.ServiceName
			group = inst.EffectiveGroup()
			if inst.Status == HealthPassing {
				healthy++
			}
		}
		result = append(result, ServiceSummary{
			Name:          name,
			Group:         group,
			QualifiedName: qualifiedName,
			InstanceCount: len(instances),
			HealthyCount:  healthy,
		})
	}
	return result
}

// Stats returns aggregate statistics across all namespaces.
func (s *InstanceRegistry) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var st Stats
	for _, nsSvcs := range s.services {
		st.ServiceCount += len(nsSvcs)
		for _, instances := range nsSvcs {
			st.InstanceCount += len(instances)
			for _, inst := range instances {
				if inst.Status == HealthPassing {
					st.HealthyCount++
				}
			}
		}
	}
	if st.InstanceCount > 0 {
		st.HealthRate = float64(st.HealthyCount) / float64(st.InstanceCount) * 100
	}

	st.MemoryUsage = process.CurrentUsage()
	return st
}

// MarkCritical marks instances that have not sent heartbeat within ttl as critical.
func (s *InstanceRegistry) MarkCritical(ttl time.Duration, maxFailures int, removalDelay time.Duration) ([]*Instance, []*Instance) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var markedCritical []*Instance
	var removed []*Instance

	criticalTTL := ttl * time.Duration(maxFailures)
	removalTTL := criticalTTL + removalDelay

	for ns, nsSvcs := range s.services {
		for svcName, instances := range nsSvcs {
			changed := false
			for id, inst := range instances {
				elapsed := now.Sub(inst.LastHeartbeat)
				if elapsed > criticalTTL {
					if inst.Status == HealthPassing {
						inst.Status = HealthCritical
						markedCritical = append(markedCritical, inst)
						changed = true
					}
					// If critical for more than removal threshold, remove entirely
					if elapsed > removalTTL {
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
				s.schedulePersistNoLock()
				s.notifyNoLock(ns, svcName, "offline")
			}
		}
		if len(nsSvcs) == 0 {
			delete(s.services, ns)
		}
	}
	return markedCritical, removed
}

func (s *InstanceRegistry) notifyNoLock(namespace, serviceName, action string) {
	if s.NotifyFunc == nil {
		return
	}

	insts := s.getServiceNoLock(namespace, serviceName)
	go s.NotifyFunc(namespace, serviceName, action, insts)
}

// GetAll returns a snapshot of all services (flat, for backward-compat sync).
func (s *InstanceRegistry) GetAll() map[string]map[string]*Instance {
	s.mu.RLock()
	defer s.mu.RUnlock()

	copy := make(map[string]map[string]*Instance)
	for _, nsSvcs := range s.services {
		for name, instances := range nsSvcs {
			instCopy := make(map[string]*Instance, len(instances))
			for id, inst := range instances {
				instCopy[id] = inst
			}
			copy[name] = instCopy
		}
	}
	return copy
}

// GetAllNS returns a snapshot of all services keyed by namespace.
func (s *InstanceRegistry) GetAllNS() map[string]map[string]map[string]*Instance {
	s.mu.RLock()
	defer s.mu.RUnlock()

	copy := make(map[string]map[string]map[string]*Instance, len(s.services))
	for ns, nsSvcs := range s.services {
		nsCopy := make(map[string]map[string]*Instance, len(nsSvcs))
		for name, instances := range nsSvcs {
			instCopy := make(map[string]*Instance, len(instances))
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
func (s *InstanceRegistry) Merge(remote map[string]map[string]*Instance) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns := DefaultNamespace
	changed := false
	if _, ok := s.services[ns]; !ok {
		s.services[ns] = make(map[string]map[string]*Instance)
	}
	for svcName, remoteInsts := range remote {
		for id, rInst := range remoteInsts {
			normalizeStoredInstance(rInst, svcName, ns)
			qualifiedName := rInst.QualifiedServiceName()
			if _, ok := s.services[ns][qualifiedName]; !ok {
				s.services[ns][qualifiedName] = make(map[string]*Instance)
			}
			localInsts := s.services[ns][qualifiedName]
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
		s.schedulePersistNoLock()
	}
}

// MergeNS merges remote state into local state with namespace support.
func (s *InstanceRegistry) MergeNS(remote map[string]map[string]map[string]*Instance) {
	s.mu.Lock()
	defer s.mu.Unlock()

	changed := false
	for ns, remoteServices := range remote {
		ns = effectiveNS(ns)
		if _, ok := s.services[ns]; !ok {
			s.services[ns] = make(map[string]map[string]*Instance)
		}
		for svcName, remoteInsts := range remoteServices {
			for id, rInst := range remoteInsts {
				normalizeStoredInstance(rInst, svcName, ns)
				qualifiedName := rInst.QualifiedServiceName()
				if _, ok := s.services[ns][qualifiedName]; !ok {
					s.services[ns][qualifiedName] = make(map[string]*Instance)
				}
				localInsts := s.services[ns][qualifiedName]
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
		s.schedulePersistNoLock()
	}
}

// Restore replaces total state (backward compat: flat map -> default namespace).
func (s *InstanceRegistry) Restore(services map[string]map[string]*Instance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.services = make(map[string]map[string]map[string]*Instance)
	s.restoreNamespaceNoLock(DefaultNamespace, services)
	s.saveNoLock()
}

// RestoreNS replaces total state with namespace support.
func (s *InstanceRegistry) RestoreNS(services map[string]map[string]map[string]*Instance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.services = make(map[string]map[string]map[string]*Instance)
	for namespace, namespaceServices := range services {
		s.restoreNamespaceNoLock(namespace, namespaceServices)
	}
	s.saveNoLock()
}

func (s *InstanceRegistry) restoreNamespaceNoLock(namespace string, services map[string]map[string]*Instance) {
	ns := effectiveNS(namespace)
	if s.services[ns] == nil {
		s.services[ns] = make(map[string]map[string]*Instance)
	}
	for storedName, instances := range services {
		for id, inst := range instances {
			normalizeStoredInstance(inst, storedName, ns)
			qualifiedName := inst.QualifiedServiceName()
			if s.services[ns][qualifiedName] == nil {
				s.services[ns][qualifiedName] = make(map[string]*Instance)
			}
			s.services[ns][qualifiedName][id] = inst
		}
	}
}

func (s *InstanceRegistry) Load() {
	if s.dataPath == "" {
		return
	}
	file := filepath.Join(s.dataPath, "services.json")
	data, err := os.ReadFile(file)
	if err == nil {
		var services map[string][]*Instance
		if err := json.Unmarshal(data, &services); err == nil {
			// Migrate flat format to namespace-scoped format
			m := make(map[string]map[string]map[string]*Instance)
			for name, list := range services {
				for _, inst := range list {
					normalizeStoredInstance(inst, name, inst.Namespace)
					ns := effectiveNS(inst.Namespace)
					qualifiedName := inst.QualifiedServiceName()
					if _, ok := m[ns]; !ok {
						m[ns] = make(map[string]map[string]*Instance)
					}
					if _, ok := m[ns][qualifiedName]; !ok {
						m[ns][qualifiedName] = make(map[string]*Instance)
					}
					m[ns][qualifiedName][inst.ID] = inst
				}
			}
			s.services = m
		}
	}
}

func normalizeStoredInstance(inst *Instance, storedName, namespace string) {
	if inst == nil {
		return
	}
	if strings.TrimSpace(inst.ServiceName) == "" {
		inst.ServiceName = storedName
	} else if strings.TrimSpace(inst.Group) == "" && strings.Contains(storedName, ServiceGroupSeparator) {
		inst.ServiceName = storedName
	}
	if strings.TrimSpace(inst.Namespace) == "" {
		inst.Namespace = namespace
	}
	inst.NormalizeIdentity()
}

func (s *InstanceRegistry) Save() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveTimer != nil {
		s.saveTimer.Stop()
		s.saveTimer = nil
	}
	s.saveNoLock()
	s.dirty = false
}

func (s *InstanceRegistry) SetFlushMode(mode string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.persistenceMode = normalizeRegistryFlushMode(mode)
	if s.persistenceMode == "sync" {
		if s.saveTimer != nil {
			s.saveTimer.Stop()
			s.saveTimer = nil
		}
		if s.dirty {
			s.saveNoLock()
			s.dirty = false
		}
		return
	}
	if s.dirty {
		s.scheduleAsyncFlushNoLock(true)
	}
}

func (s *InstanceRegistry) SetFlushInterval(interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.flushInterval = normalizeRegistryFlushInterval(interval)
	if s.persistenceMode == "async" && s.dirty {
		s.scheduleAsyncFlushNoLock(true)
	}
}

func (s *InstanceRegistry) GetFlushMode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.persistenceMode
}

func (s *InstanceRegistry) GetFlushInterval() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.flushInterval
}

func (s *InstanceRegistry) schedulePersistNoLock() {
	if s.dataPath == "" {
		return
	}
	if s.persistenceMode == "sync" {
		if s.saveTimer != nil {
			s.saveTimer.Stop()
			s.saveTimer = nil
		}
		s.saveNoLock()
		s.dirty = false
		return
	}
	s.dirty = true
	s.scheduleAsyncFlushNoLock(false)
}

func (s *InstanceRegistry) scheduleAsyncFlushNoLock(reset bool) {
	if s.persistenceMode != "async" || !s.dirty {
		return
	}
	if s.saveTimer != nil {
		if !reset {
			return
		}
		s.saveTimer.Stop()
	}
	interval := normalizeRegistryFlushInterval(s.flushInterval)
	s.saveTimer = time.AfterFunc(interval, s.flushDirtyAsync)
}

func (s *InstanceRegistry) flushDirtyAsync() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.saveTimer = nil
	if s.persistenceMode != "async" || !s.dirty {
		return
	}
	s.saveNoLock()
	s.dirty = false
}

// saveNoLock persists services to disk. Caller must hold s.mu (read or write).
// Saves in flat format (serviceName -> instances) for backward compatibility.
func (s *InstanceRegistry) saveNoLock() {
	if s.dataPath == "" {
		return
	}
	os.MkdirAll(s.dataPath, 0755)
	file := filepath.Join(s.dataPath, "services.json")
	output := make(map[string][]*Instance)
	for _, nsSvcs := range s.services {
		for name, instances := range nsSvcs {
			list := make([]*Instance, 0, len(instances))
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

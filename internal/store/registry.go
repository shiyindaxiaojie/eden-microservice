package store

import (
	"fmt"
	"sync"
	"time"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
)

// Registry is the in-memory, concurrency-safe service registry.
type Registry struct {
	mu        sync.RWMutex
	services  map[string]map[string]*model.Instance // serviceName -> instanceID -> Instance
	events    []*model.Event
	eventSeq  uint64
	maxEvents int
}

// NewRegistry creates a new empty registry.
func NewRegistry() *Registry {
	return &Registry{
		services:  make(map[string]map[string]*model.Instance),
		events:    make([]*model.Event, 0, 1024),
		maxEvents: 10000,
	}
}

// Register adds or updates an instance.
func (r *Registry) Register(inst *model.Instance) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.services[inst.ServiceName]; !ok {
		r.services[inst.ServiceName] = make(map[string]*model.Instance)
	}

	inst.Status = model.HealthPassing
	inst.LastHeartbeat = time.Now()
	if inst.RegisteredAt.IsZero() {
		inst.RegisteredAt = time.Now()
	}
	if inst.Weight <= 0 {
		inst.Weight = 1
	}

	r.services[inst.ServiceName][inst.ID] = inst
	r.appendEventLocked("register", inst.ServiceName, fmt.Sprintf("%s:%d", inst.Host, inst.Port), "instance registered")
}

// Deregister removes an instance.
func (r *Registry) Deregister(serviceName, instanceID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	instances, ok := r.services[serviceName]
	if !ok {
		return false
	}

	inst, exists := instances[instanceID]
	if !exists {
		return false
	}

	addr := fmt.Sprintf("%s:%d", inst.Host, inst.Port)
	delete(instances, instanceID)
	if len(instances) == 0 {
		delete(r.services, serviceName)
	}

	r.appendEventLocked("deregister", serviceName, addr, "instance deregistered")
	return true
}

// Heartbeat updates the last heartbeat time for an instance.
func (r *Registry) Heartbeat(serviceName, instanceID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	instances, ok := r.services[serviceName]
	if !ok {
		return false
	}
	inst, exists := instances[instanceID]
	if !exists {
		return false
	}
	inst.LastHeartbeat = time.Now()
	if inst.Status == model.HealthCritical {
		inst.Status = model.HealthPassing
		r.appendEventLocked("health_change", serviceName, fmt.Sprintf("%s:%d", inst.Host, inst.Port), "instance recovered")
	}
	return true
}

// GetService returns all instances for a given service name.
func (r *Registry) GetService(name string) []*model.Instance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances, ok := r.services[name]
	if !ok {
		return nil
	}
	result := make([]*model.Instance, 0, len(instances))
	for _, inst := range instances {
		result = append(result, inst)
	}
	return result
}

// GetServiceHealthy returns only healthy instances.
func (r *Registry) GetServiceHealthy(name string) []*model.Instance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances, ok := r.services[name]
	if !ok {
		return nil
	}
	result := make([]*model.Instance, 0, len(instances))
	for _, inst := range instances {
		if inst.Status == model.HealthPassing {
			result = append(result, inst)
		}
	}
	return result
}

// ServiceSummary holds counts for a single service.
type ServiceSummary struct {
	Name           string `json:"name"`
	InstanceCount  int    `json:"instance_count"`
	HealthyCount   int    `json:"healthy_count"`
}

// ListServices returns a summary of all registered services.
func (r *Registry) ListServices() []ServiceSummary {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ServiceSummary, 0, len(r.services))
	for name, instances := range r.services {
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
type Stats struct {
	ServiceCount  int     `json:"service_count"`
	InstanceCount int     `json:"instance_count"`
	HealthyCount  int     `json:"healthy_count"`
	HealthRate    float64 `json:"health_rate"`
}

func (r *Registry) Stats() Stats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var s Stats
	s.ServiceCount = len(r.services)
	for _, instances := range r.services {
		s.InstanceCount += len(instances)
		for _, inst := range instances {
			if inst.Status == model.HealthPassing {
				s.HealthyCount++
			}
		}
	}
	if s.InstanceCount > 0 {
		s.HealthRate = float64(s.HealthyCount) / float64(s.InstanceCount) * 100
	}
	return s
}

// MarkCritical marks instances that have not sent heartbeat within ttl as critical.
// Returns list of removed instance IDs.
func (r *Registry) MarkCritical(ttl time.Duration) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	var removed []string
	for svcName, instances := range r.services {
		for id, inst := range instances {
			if now.Sub(inst.LastHeartbeat) > ttl {
				if inst.Status == model.HealthPassing {
					inst.Status = model.HealthCritical
					r.appendEventLocked("health_change", svcName, fmt.Sprintf("%s:%d", inst.Host, inst.Port), "instance marked critical (TTL expired)")
				}
				// If critical for more than 2x TTL, remove entirely
				if now.Sub(inst.LastHeartbeat) > ttl*2 {
					addr := fmt.Sprintf("%s:%d", inst.Host, inst.Port)
					delete(instances, id)
					removed = append(removed, id)
					r.appendEventLocked("deregister", svcName, addr, "instance removed (TTL expired)")
				}
			}
		}
		if len(instances) == 0 {
			delete(r.services, svcName)
		}
	}
	return removed
}

// GetEvents returns recent events, newest first, up to limit.
func (r *Registry) GetEvents(limit int) []*model.Event {
	r.mu.RLock()
	defer r.mu.RUnlock()

	total := len(r.events)
	if limit <= 0 || limit > total {
		limit = total
	}
	result := make([]*model.Event, limit)
	for i := 0; i < limit; i++ {
		result[i] = r.events[total-1-i]
	}
	return result
}

// Snapshot returns a deep copy of all instances (used by Raft snapshots).
func (r *Registry) Snapshot() map[string][]*model.Instance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snap := make(map[string][]*model.Instance, len(r.services))
	for name, instances := range r.services {
		list := make([]*model.Instance, 0, len(instances))
		for _, inst := range instances {
			cp := *inst
			list = append(list, &cp)
		}
		snap[name] = list
	}
	return snap
}

// Restore replaces the registry from snapshot data.
func (r *Registry) Restore(data map[string][]*model.Instance) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.services = make(map[string]map[string]*model.Instance, len(data))
	for name, instances := range data {
		m := make(map[string]*model.Instance, len(instances))
		for _, inst := range instances {
			m[inst.ID] = inst
		}
		r.services[name] = m
	}
}

func (r *Registry) appendEventLocked(eventType, service, instance, message string) {
	r.eventSeq++
	evt := &model.Event{
		ID:        r.eventSeq,
		Type:      eventType,
		Service:   service,
		Instance:  instance,
		Message:   message,
		Timestamp: time.Now(),
	}
	r.events = append(r.events, evt)
	// Trim old events if exceeding max
	if len(r.events) > r.maxEvents {
		r.events = r.events[len(r.events)-r.maxEvents:]
	}
}

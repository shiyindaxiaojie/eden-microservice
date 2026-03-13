package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

	// Metadata for Auth/RBAC and Settings
	apiKeys  map[string]*model.APIKey
	users    map[string]*model.User
	mode     string // "ap" or "cp"
	dataPath string
}

// NewRegistry creates a new registry with persistence path.
func NewRegistry(dataPath string) *Registry {
	r := &Registry{
		services:  make(map[string]map[string]*model.Instance),
		events:    make([]*model.Event, 0, 1024),
		maxEvents: 1000,
		apiKeys:   make(map[string]*model.APIKey),
		users:     make(map[string]*model.User),
		mode:      "ap", // default
		dataPath:  dataPath,
	}
	r.loadAll()
	return r
}

func (r *Registry) loadAll() {
	if r.dataPath == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.loadServicesLocked()
	r.loadEventsLocked()
	r.loadUsersLocked()
	r.loadSettingsLocked()
}

func (r *Registry) loadServicesLocked() {
	file := filepath.Join(r.dataPath, "services.json")
	data, err := os.ReadFile(file)
	if err != nil {
		return
	}
	var services map[string][]*model.Instance
	if err := json.Unmarshal(data, &services); err == nil {
		r.services = make(map[string]map[string]*model.Instance, len(services))
		for name, list := range services {
			m := make(map[string]*model.Instance, len(list))
			for _, inst := range list {
				m[inst.ID] = inst
			}
			r.services[name] = m
		}
	}
}

func (r *Registry) loadEventsLocked() {
	file := filepath.Join(r.dataPath, "events.json")
	data, err := os.ReadFile(file)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, &r.events); err == nil {
		for _, evt := range r.events {
			if evt.ID > r.eventSeq {
				r.eventSeq = evt.ID
			}
		}
	}
}

func (r *Registry) loadUsersLocked() {
	file := filepath.Join(r.dataPath, "users.json")
	data, err := os.ReadFile(file)
	if err != nil {
		return
	}
	var meta struct {
		Users map[string]*model.User `json:"users"`
	}
	if err := json.Unmarshal(data, &meta); err == nil {
		if meta.Users != nil {
			r.users = meta.Users
		}
	}
}

// SeedBuiltInUsers injects users from the config into the registry.
// If a user doesn't exist, it adds them. It also updates IsBuiltIn flags.
func (r *Registry) SeedBuiltInUsers(users []model.User) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	changed := false
	for _, u := range users {
		u := u // copy
		u.IsBuiltIn = true
		
		existing, ok := r.users[u.Username]
		if !ok {
			r.users[u.Username] = &u
			changed = true
		} else {
			// Update existing built-in users if password or role changed
			if existing.Password != u.Password || existing.Role != u.Role || !existing.IsBuiltIn {
				existing.Password = u.Password
				existing.Role = u.Role
				existing.IsBuiltIn = true
				changed = true
			}
		}
	}
	
	if changed {
		r.saveUsersLocked()
	}
}

func (r *Registry) loadSettingsLocked() {
	file := filepath.Join(r.dataPath, "settings.json")
	data, err := os.ReadFile(file)
	if err != nil {
		return
	}
	var meta struct {
		APIKeys map[string]*model.APIKey `json:"api_keys"`
		Mode    string                   `json:"mode"`
	}
	if err := json.Unmarshal(data, &meta); err == nil {
		if meta.APIKeys != nil {
			r.apiKeys = meta.APIKeys
		}
		if meta.Mode != "" {
			r.mode = meta.Mode
		}
	}
}

func (r *Registry) saveServicesLocked() {
	if r.dataPath == "" {
		return
	}
	os.MkdirAll(r.dataPath, 0755)
	file := filepath.Join(r.dataPath, "services.json")
	// Convert to slice format for snapshot-like json
	services := make(map[string][]*model.Instance, len(r.services))
	for name, m := range r.services {
		list := make([]*model.Instance, 0, len(m))
		for _, inst := range m {
			list = append(list, inst)
		}
		services[name] = list
	}
	data, _ := json.MarshalIndent(services, "", "  ")
	_ = os.WriteFile(file, data, 0644)
}

func (r *Registry) saveEventsLocked() {
	if r.dataPath == "" {
		return
	}
	os.MkdirAll(r.dataPath, 0755)
	file := filepath.Join(r.dataPath, "events.json")
	data, _ := json.MarshalIndent(r.events, "", "  ")
	_ = os.WriteFile(file, data, 0644)
}

func (r *Registry) saveUsersLocked() {
	if r.dataPath == "" {
		return
	}
	os.MkdirAll(r.dataPath, 0755)
	file := filepath.Join(r.dataPath, "users.json")
	meta := struct {
		Users map[string]*model.User `json:"users"`
	}{
		Users: r.users,
	}
	data, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(file, data, 0644)
}

func (r *Registry) saveSettingsLocked() {
	if r.dataPath == "" {
		return
	}
	os.MkdirAll(r.dataPath, 0755)
	file := filepath.Join(r.dataPath, "settings.json")
	meta := struct {
		APIKeys map[string]*model.APIKey `json:"api_keys"`
		Mode    string                   `json:"mode"`
	}{
		APIKeys: r.apiKeys,
		Mode:    r.mode,
	}
	data, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(file, data, 0644)
}

// ---------- API Key Management ----------

func (r *Registry) AddAPIKey(key *model.APIKey) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.apiKeys[key.Key] = key
	r.saveSettingsLocked()
}

func (r *Registry) DeleteAPIKey(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.apiKeys, key)
	r.saveSettingsLocked()
}

func (r *Registry) ListAPIKeys() []*model.APIKey {
	r.mu.RLock()
	defer r.mu.RUnlock()
	now := time.Now().Unix()
	keys := make([]*model.APIKey, 0, len(r.apiKeys))
	for _, k := range r.apiKeys {
		cp := *k
		if cp.ExpiresAt > 0 && now > cp.ExpiresAt {
			cp.Status = "expired"
		} else {
			cp.Status = "active"
		}
		keys = append(keys, &cp)
	}
	return keys
}

func (r *Registry) GetAPIKey(key string) (*model.APIKey, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	k, ok := r.apiKeys[key]
	return k, ok
}

// ---------- User Management ----------

func (r *Registry) AddUser(u *model.User) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users[u.Username] = u
	r.saveUsersLocked()
}

func (r *Registry) DeleteUser(username string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.users, username)
	r.saveUsersLocked()
}

func (r *Registry) ListUsers() []*model.User {
	r.mu.RLock()
	defer r.mu.RUnlock()
	users := make([]*model.User, 0, len(r.users))
	for _, u := range r.users {
		users = append(users, u)
	}
	return users
}

func (r *Registry) GetUser(username string) (*model.User, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[username]
	return u, ok
}

// ---------- Mode Management ----------

func (r *Registry) SetMode(mode string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if mode != "ap" && mode != "cp" {
		return
	}
	r.mode = mode
	r.saveSettingsLocked()
}

func (r *Registry) GetMode() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.mode
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
	r.saveServicesLocked()
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

	r.saveServicesLocked()
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
	MemoryUsage   uint64  `json:"memory_usage"`
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

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	s.MemoryUsage = ms.Alloc
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

// SnapshotData represents the full state for Raft/Persistence.
type SnapshotData struct {
	Services map[string][]*model.Instance `json:"services"`
	APIKeys  map[string]*model.APIKey     `json:"api_keys"`
	Users    map[string]*model.User       `json:"users"`
	Mode     string                       `json:"mode"`
}

// Snapshot returns a deep copy of all data.
func (r *Registry) Snapshot() *SnapshotData {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snap := &SnapshotData{
		Services: make(map[string][]*model.Instance, len(r.services)),
		APIKeys:  make(map[string]*model.APIKey, len(r.apiKeys)),
		Users:    make(map[string]*model.User, len(r.users)),
		Mode:     r.mode,
	}

	for name, instances := range r.services {
		list := make([]*model.Instance, 0, len(instances))
		for _, inst := range instances {
			cp := *inst
			list = append(list, &cp)
		}
		snap.Services[name] = list
	}

	for k, v := range r.apiKeys {
		cp := *v
		snap.APIKeys[k] = &cp
	}
	for k, v := range r.users {
		cp := *v
		snap.Users[k] = &cp
	}

	return snap
}

// Restore replaces the registry from snapshot data.
func (r *Registry) Restore(data *SnapshotData) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.services = make(map[string]map[string]*model.Instance, len(data.Services))
	for name, instances := range data.Services {
		m := make(map[string]*model.Instance, len(instances))
		for _, inst := range instances {
			m[inst.ID] = inst
		}
		r.services[name] = m
	}

	if data.APIKeys != nil {
		r.apiKeys = data.APIKeys
	} else {
		r.apiKeys = make(map[string]*model.APIKey)
	}

	if data.Users != nil {
		r.users = data.Users
	} else {
		r.users = make(map[string]*model.User)
	}

	if data.Mode != "" {
		r.mode = data.Mode
	}

	r.saveServicesLocked()
	r.saveEventsLocked()
	r.saveUsersLocked()
	r.saveSettingsLocked()
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
	r.saveEventsLocked()
}

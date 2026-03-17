package store

import (
	"time"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
)

// ServiceSummary holds counts for a single service.
type ServiceSummary struct {
	Name          string `json:"name"`
	InstanceCount int    `json:"instance_count"`
	HealthyCount  int    `json:"healthy_count"`
}

// Stats represents aggregate registry statistics.
type Stats struct {
	ServiceCount  int     `json:"service_count"`
	InstanceCount int     `json:"instance_count"`
	HealthyCount  int     `json:"healthy_count"`
	HealthRate    float64 `json:"health_rate"`
	MemoryUsage   uint64  `json:"memory_usage"`
}

// Registry is the unified entry point for data access, aggregating specialized stores.
type Registry struct {
	Services *ServiceStore
	Auth     *AuthStore
	Config   *ConfigStore
	Events   *EventStore
	dataPath string
}

// NewRegistry creates a new registry with persistence path.
func NewRegistry(dataPath string) *Registry {
	r := &Registry{
		Services: NewServiceStore(dataPath),
		Auth:     NewAuthStore(dataPath),
		Config:   NewConfigStore(dataPath),
		Events:   NewEventStore(1000, dataPath),
		dataPath: dataPath,
	}
	return r
}

// Methods for backward compatibility or orchestration
func (r *Registry) Register(inst *model.Instance) {
	r.Services.Register(inst)
}

func (r *Registry) Deregister(serviceName, instanceID string) (*model.Instance, bool) {
	return r.Services.Deregister(serviceName, instanceID)
}

func (r *Registry) Heartbeat(serviceName, instanceID string) (*model.Instance, bool) {
	return r.Services.Heartbeat(serviceName, instanceID)
}

func (r *Registry) GetMode() string { return r.Config.GetMode() }
func (r *Registry) GetEnvironment() string { return r.Config.GetEnvironment() }
func (r *Registry) GetSeeds() []string { return r.Config.GetSeeds() }
func (r *Registry) GetLogLevel() string { return r.Config.GetLogLevel() }
func (r *Registry) Stats() Stats { return r.Services.Stats() }
func (r *Registry) GetUser(u string) (*model.User, bool) { return r.Auth.GetUser(u) }
func (r *Registry) GetAPIKey(k string) (*model.APIKey, bool) { return r.Auth.GetAPIKey(k) }
func (r *Registry) ListServices() []ServiceSummary { return r.Services.ListServices() }
func (r *Registry) GetServiceHealthy(n string) []*model.Instance { return r.Services.GetServiceHealthy(n) }
func (r *Registry) GetService(n string) []*model.Instance { return r.Services.GetService(n) }
func (r *Registry) ListUsers() []*model.User { return r.Auth.ListUsers() }
func (r *Registry) AddUser(u *model.User) { r.Auth.AddUser(u); r.Auth.Save() }
func (r *Registry) DeleteUser(u string) { r.Auth.DeleteUser(u); r.Auth.Save() }
func (r *Registry) ListAPIKeys() []*model.APIKey { return r.Auth.ListAPIKeys() }
func (r *Registry) AddAPIKey(k *model.APIKey) { r.Auth.AddAPIKey(k); r.Auth.Save() }
func (r *Registry) DeleteAPIKey(k string) { r.Auth.DeleteAPIKey(k); r.Auth.Save() }
func (r *Registry) SetMode(m string) { r.Config.SetMode(m); r.Config.Save() }
func (r *Registry) SetEnvironment(e string) { r.Config.SetEnvironment(e); r.Config.Save() }
func (r *Registry) SetSeeds(s []string) { r.Config.SetSeeds(s); r.Config.Save() }
func (r *Registry) SetLogLevel(l string) { r.Config.SetLogLevel(l); r.Config.Save() }
func (r *Registry) ListEvents() []*model.Event { return r.Events.List() }
func (r *Registry) MarkCritical(ttl time.Duration) ([]*model.Instance, []*model.Instance) {
	return r.Services.MarkCritical(ttl)
}

// SnapshotData represents the full state for Raft/Persistence.
type SnapshotData struct {
	Services map[string][]*model.Instance `json:"services"`
	APIKeys     map[string]*model.APIKey     `json:"api_keys"`
	Users       map[string]*model.User       `json:"users"`
	Mode        string                       `json:"mode"`
	Environment string                       `json:"environment"`
	Seeds       []string                     `json:"seeds"`
}

// Snapshot returns a deep copy of all data.
func (r *Registry) Snapshot() *SnapshotData {
	allServices := r.Services.GetAll()
	services := make(map[string][]*model.Instance, len(allServices))
	for name, m := range allServices {
		list := make([]*model.Instance, 0, len(m))
		for _, inst := range m {
			cp := *inst
			list = append(list, &cp)
		}
		services[name] = list
	}

	snap := &SnapshotData{
		Services:    services,
		APIKeys:     r.Auth.GetAllAPIKeys(),
		Users:       r.Auth.GetAllUsers(),
		Mode:        r.Config.GetMode(),
		Environment: r.Config.GetEnvironment(),
		Seeds:       r.Config.GetSeeds(),
	}
	return snap
}

// Restore replaces the registry from snapshot data.
func (r *Registry) Restore(data *SnapshotData) {
	services := make(map[string]map[string]*model.Instance, len(data.Services))
	for name, list := range data.Services {
		m := make(map[string]*model.Instance, len(list))
		for _, inst := range list {
			m[inst.ID] = inst
		}
		services[name] = m
	}

	r.Services.Restore(services)
	r.Auth.Restore(data.Users, data.APIKeys)
	r.Config.Restore(data.Mode, data.Environment, "", data.Seeds)

	r.Services.Save()
	r.Auth.Save()
	r.Config.Save()
}

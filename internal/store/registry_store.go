package store

import (
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	"time"
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
	Services   *ServiceStore
	Auth       *AuthStore
	Config     *ConfigStore
	Events     *EventStore
	Namespaces *NamespaceStore
	Topology   *TopologyStore
	dataPath   string
}

// NewRegistry creates a new registry with persistence path.
func NewRegistry(dataPath string) *Registry {
	r := &Registry{
		Services:   NewServiceStore(dataPath),
		Auth:       NewAuthStore(dataPath),
		Config:     NewConfigStore(dataPath),
		Events:     NewEventStore(1000, dataPath),
		Namespaces: NewNamespaceStore(dataPath),
		Topology:   NewTopologyStore(dataPath),
		dataPath:   dataPath,
	}
	r.Events.SetRetentionDaysProvider(func() int {
		return r.Config.GetEventRetentionDays()
	})
	r.Events.Cleanup(r.Config.GetEventRetentionDays())
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

func (r *Registry) HeartbeatNS(namespace, serviceName, instanceID string) (*model.Instance, bool) {
	return r.Services.HeartbeatNS(namespace, serviceName, instanceID)
}

func (r *Registry) GetMode() string                          { return r.Config.GetMode() }
func (r *Registry) GetEnvironment() string                   { return r.Config.GetEnvironment() }
func (r *Registry) GetSeeds() []string                       { return r.Config.GetSeeds() }
func (r *Registry) GetLogLevel() string                      { return r.Config.GetLogLevel() }
func (r *Registry) Stats() Stats                             { return r.Services.Stats() }
func (r *Registry) GetUser(u string) (*model.User, bool)     { return r.Auth.GetUser(u) }
func (r *Registry) GetAPIKey(k string) (*model.APIKey, bool) { return r.Auth.GetAPIKey(k) }
func (r *Registry) ListServices() []ServiceSummary           { return r.Services.ListServices() }
func (r *Registry) ListServicesNS(namespace string) []ServiceSummary {
	return r.Services.ListServicesNS(namespace)
}
func (r *Registry) GetServiceHealthy(n string) []*model.Instance {
	return r.Services.GetServiceHealthy(n)
}
func (r *Registry) GetServiceHealthyNS(namespace, name string) []*model.Instance {
	return r.Services.GetServiceHealthyNS(namespace, name)
}
func (r *Registry) GetService(n string) []*model.Instance { return r.Services.GetService(n) }
func (r *Registry) GetServiceNS(namespace, name string) []*model.Instance {
	return r.Services.GetServiceNS(namespace, name)
}
func (r *Registry) ListUsers() []*model.User     { return r.Auth.ListUsers() }
func (r *Registry) AddUser(u *model.User)        { r.Auth.AddUser(u); r.Auth.Save() }
func (r *Registry) DeleteUser(u string)          { r.Auth.DeleteUser(u); r.Auth.Save() }
func (r *Registry) ListAPIKeys() []*model.APIKey { return r.Auth.ListAPIKeys() }
func (r *Registry) AddAPIKey(k *model.APIKey)    { r.Auth.AddAPIKey(k); r.Auth.Save() }
func (r *Registry) DeleteAPIKey(k string)        { r.Auth.DeleteAPIKey(k); r.Auth.Save() }
func (r *Registry) SetMode(m string)             { r.Config.SetMode(m); r.Config.Save() }
func (r *Registry) SetEnvironment(e string)      { r.Config.SetEnvironment(e); r.Config.Save() }
func (r *Registry) SetSeeds(s []string)          { r.Config.SetSeeds(s); r.Config.Save() }
func (r *Registry) SetLogLevel(l string) {
	r.Config.SetLogLevel(model.NormalizeLogLevel(l))
	r.Config.Save()
}
func (r *Registry) GetEventRetentionDays() int { return r.Config.GetEventRetentionDays() }
func (r *Registry) SetEventRetentionDays(d int) {
	r.Config.SetEventRetentionDays(d)
	r.Config.Save()
	r.Events.Cleanup(d)
}
func (r *Registry) GetLogRetentionDays() int     { return r.Config.GetLogRetentionDays() }
func (r *Registry) SetLogRetentionDays(d int)    { r.Config.SetLogRetentionDays(d); r.Config.Save() }
func (r *Registry) GetEventTypes() []string      { return r.Config.GetEventTypes() }
func (r *Registry) SetEventTypes(t []string)     { r.Config.SetEventTypes(t); r.Config.Save() }
func (r *Registry) GetHeartbeatMaxFailures() int { return r.Config.GetHeartbeatMaxFailures() }
func (r *Registry) GetAPIKeyAuthEnabled() bool   { return r.Config.GetAPIKeyAuthEnabled() }
func (r *Registry) HasAPIKeyAuthSetting() bool   { return r.Config.HasAPIKeyAuthEnabled() }
func (r *Registry) SetHeartbeatMaxFailures(n int) {
	r.Config.SetHeartbeatMaxFailures(n)
	r.Config.Save()
}
func (r *Registry) GetInstanceRemovalDelaySeconds() int {
	return r.Config.GetInstanceRemovalDelaySeconds()
}
func (r *Registry) SetInstanceRemovalDelaySeconds(n int) {
	r.Config.SetInstanceRemovalDelaySeconds(n)
	r.Config.Save()
}
func (r *Registry) SetAPIKeyAuthEnabled(enabled bool) {
	r.Config.SetAPIKeyAuthEnabled(enabled)
	r.Config.Save()
}
func (r *Registry) ListEvents() []*model.Event { return r.Events.List() }
func (r *Registry) MarkCritical(ttl time.Duration) ([]*model.Instance, []*model.Instance) {
	// Dynamically calculate based on settings if needed, or pass fixed ttl
	// For now, let's update ServiceStore.MarkCritical to take both.
	maxFail := r.GetHeartbeatMaxFailures()
	if maxFail <= 0 {
		maxFail = 3
	}
	removalDelay := time.Duration(r.GetInstanceRemovalDelaySeconds()) * time.Second
	return r.Services.MarkCritical(ttl, maxFail, removalDelay)
}

// Namespace convenience methods
func (r *Registry) ListNamespaces() []*model.Namespace       { return r.Namespaces.List() }
func (r *Registry) CreateNamespace(ns *model.Namespace) bool { return r.Namespaces.Create(ns) }

func (r *Registry) AppendEvent(etype, service, instance, message string) {
	normalized := model.NormalizeEventTypes([]string{etype})
	if len(normalized) == 0 {
		return
	}
	if !r.shouldRecordEvent(normalized[0]) {
		return
	}
	r.Events.Append(&model.Event{
		Type:      normalized[0],
		Service:   service,
		Instance:  instance,
		Message:   message,
		Timestamp: time.Now(),
	})
}

func (r *Registry) shouldRecordEvent(etype string) bool {
	allowed := r.GetEventTypes()
	if len(allowed) == 0 {
		return false
	}
	for _, t := range allowed {
		if t == etype {
			return true
		}
	}
	return false
}
func (r *Registry) UpdateNamespace(ns *model.Namespace) bool { return r.Namespaces.Update(ns) }
func (r *Registry) DeleteNamespace(name string) bool         { return r.Namespaces.Delete(name) }
func (r *Registry) SetInstanceStatus(ns, svc, id string, status model.HealthStatus) (*model.Instance, bool) {
	return r.Services.SetInstanceStatus(ns, svc, id, status)
}

// SnapshotData represents the full state for Raft/Persistence.
type SnapshotData struct {
	Services           map[string][]*model.Instance                `json:"services,omitempty"`
	ServicesByNS       map[string]map[string][]*model.Instance     `json:"services_by_namespace,omitempty"`
	Namespaces         []*model.Namespace                          `json:"namespaces,omitempty"`
	TopologyReports    map[string]map[string]*model.TopologyReport `json:"topology_reports,omitempty"`
	APIKeys            map[string]*model.APIKey                    `json:"api_keys"`
	Users              map[string]*model.User                      `json:"users"`
	Mode               string                                      `json:"mode"`
	Environment        string                                      `json:"environment"`
	Seeds              []string                                    `json:"seeds"`
	LogLevel           string                                      `json:"log_level"`
	EventRetentionDays int                                         `json:"event_retention_days"`
	LogRetentionDays   int                                         `json:"log_retention_days"`
	EventTypes         []string                                    `json:"event_types"`
	HBMaxFail          int                                         `json:"heartbeat_max_failures"`
	RemovalDelay       int                                         `json:"instance_removal_delay_seconds"`
	APIKeyAuthEnabled  bool                                        `json:"api_key_auth_enabled"`
}

// Snapshot returns a deep copy of all data.
func (r *Registry) Snapshot() *SnapshotData {
	allServices := r.Services.GetAllNS()
	servicesByNS := make(map[string]map[string][]*model.Instance, len(allServices))
	for ns, services := range allServices {
		nsCopy := make(map[string][]*model.Instance, len(services))
		for name, m := range services {
			list := make([]*model.Instance, 0, len(m))
			for _, inst := range m {
				cp := *inst
				list = append(list, &cp)
			}
			nsCopy[name] = list
		}
		servicesByNS[ns] = nsCopy
	}

	snap := &SnapshotData{
		ServicesByNS:       servicesByNS,
		Namespaces:         r.Namespaces.List(),
		TopologyReports:    r.Topology.Snapshot(),
		APIKeys:            r.Auth.GetAllAPIKeys(),
		Users:              r.Auth.GetAllUsers(),
		Mode:               r.Config.GetMode(),
		Environment:        r.Config.GetEnvironment(),
		Seeds:              r.Config.GetSeeds(),
		LogLevel:           r.Config.GetLogLevel(),
		EventRetentionDays: r.Config.GetEventRetentionDays(),
		LogRetentionDays:   r.Config.GetLogRetentionDays(),
		EventTypes:         r.Config.GetEventTypes(),
		HBMaxFail:          r.Config.GetHeartbeatMaxFailures(),
		RemovalDelay:       r.Config.GetInstanceRemovalDelaySeconds(),
		APIKeyAuthEnabled:  r.Config.GetAPIKeyAuthEnabled(),
	}
	return snap
}

// Restore replaces the registry from snapshot data.
func (r *Registry) Restore(data *SnapshotData) {
	if len(data.ServicesByNS) > 0 {
		services := make(map[string]map[string]map[string]*model.Instance, len(data.ServicesByNS))
		for ns, nsServices := range data.ServicesByNS {
			services[ns] = make(map[string]map[string]*model.Instance, len(nsServices))
			for name, list := range nsServices {
				m := make(map[string]*model.Instance, len(list))
				for _, inst := range list {
					m[inst.ID] = inst
				}
				services[ns][name] = m
			}
		}
		r.Services.RestoreNS(services)
	} else {
		services := make(map[string]map[string]*model.Instance, len(data.Services))
		for name, list := range data.Services {
			m := make(map[string]*model.Instance, len(list))
			for _, inst := range list {
				m[inst.ID] = inst
			}
			services[name] = m
		}
		r.Services.Restore(services)
	}

	if len(data.Namespaces) > 0 {
		r.Namespaces.Restore(data.Namespaces)
	}
	if len(data.TopologyReports) > 0 {
		r.Topology.Restore(data.TopologyReports)
	}
	r.Auth.Restore(data.Users, data.APIKeys)
	r.Config.Restore(data.Mode, data.Environment, data.LogLevel, data.Seeds, data.EventRetentionDays, data.LogRetentionDays, data.EventTypes, data.HBMaxFail, data.RemovalDelay, data.APIKeyAuthEnabled)

	r.Services.Save()
	r.Auth.Save()
	r.Config.Save()
}

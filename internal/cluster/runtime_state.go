package cluster

import (
	"time"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/auth"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/catalog"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/settings"
)

// SnapshotData represents the full state for Raft and anti-entropy sync.
type SnapshotData struct {
	Services           map[string][]*catalog.Instance                `json:"services,omitempty"`
	ServicesByNS       map[string]map[string][]*catalog.Instance     `json:"services_by_namespace,omitempty"`
	Namespaces         []*catalog.Namespace                          `json:"namespaces,omitempty"`
	TopologyReports    map[string]map[string]*catalog.TopologyReport `json:"topology_reports,omitempty"`
	APIKeys            map[string]*auth.APIKey                       `json:"api_keys"`
	Users              map[string]*auth.User                         `json:"users"`
	Mode               string                                        `json:"mode"`
	Environment        string                                        `json:"environment"`
	Seeds              []string                                      `json:"seeds"`
	LogLevel           string                                        `json:"log_level"`
	EventRetentionDays int                                           `json:"event_retention_days"`
	LogRetentionDays   int                                           `json:"log_retention_days"`
	EventTypes         []string                                      `json:"event_types"`
	HBMaxFail          int                                           `json:"heartbeat_max_failures"`
	RemovalDelay       int                                           `json:"instance_removal_delay_seconds"`
	APIKeyAuthEnabled  bool                                          `json:"api_key_auth_enabled"`
	NotifyAlertNodeID  string                                        `json:"notify_alert_node_id,omitempty"`
	EventStorageMode   string                                        `json:"event_storage_mode"`
	MetricsStorageMode string                                        `json:"metrics_storage_mode"`
	MetricsRetentionDays int                                         `json:"metrics_retention_days"`
}

// RuntimeState composes the domain modules that participate in node runtime and cluster replication.
type RuntimeState struct {
	Catalog  *catalog.State
	Auth     *auth.Directory
	Settings *settings.Profile
}

func NewRuntimeState(dataPath string) *RuntimeState {
	state := &RuntimeState{
		Catalog:  catalog.NewState(dataPath),
		Auth:     auth.NewDirectory(dataPath),
		Settings: settings.NewProfile(dataPath),
	}
	state.Catalog.Events.SetRetentionDaysProvider(func() int {
		return state.Settings.GetEventRetentionDays()
	})
	state.Catalog.SetEventTypesProvider(state.Settings.GetEventTypes)
	state.Catalog.Events.Cleanup(state.Settings.GetEventRetentionDays())
	
	state.Catalog.Metrics.SetRetentionDaysProvider(state.Settings.GetMetricsRetentionDays)
	state.Catalog.Metrics.Cleanup()

	return state
}

func (s *RuntimeState) StartMetricsRecorder() {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			if s.Settings.GetMetricsStorageMode() == "persistent" {
				stats := s.Stats()
				s.Catalog.Metrics.RecordMemory(stats.MemoryUsage)
			}
		}
	}()
}

func (s *RuntimeState) Register(inst *catalog.Instance) {
	s.Catalog.Register(inst)
}

func (s *RuntimeState) Deregister(serviceName, instanceID string) (*catalog.Instance, bool) {
	return s.Catalog.Deregister(serviceName, instanceID)
}

func (s *RuntimeState) Heartbeat(serviceName, instanceID string) (*catalog.Instance, bool) {
	return s.Catalog.Heartbeat(serviceName, instanceID)
}

func (s *RuntimeState) HeartbeatNS(namespace, serviceName, instanceID string) (*catalog.Instance, bool) {
	return s.Catalog.HeartbeatNS(namespace, serviceName, instanceID)
}

func (s *RuntimeState) GetMode() string {
	return s.Settings.GetMode()
}

func (s *RuntimeState) GetEnvironment() string {
	return s.Settings.GetEnvironment()
}

func (s *RuntimeState) GetSeeds() []string {
	return s.Settings.GetSeeds()
}

func (s *RuntimeState) GetLogLevel() string {
	return s.Settings.GetLogLevel()
}

func (s *RuntimeState) Stats() catalog.Stats {
	return s.Catalog.Stats()
}

func (s *RuntimeState) GetUser(username string) (*auth.User, bool) {
	return s.Auth.GetUser(username)
}

func (s *RuntimeState) GetAPIKey(key string) (*auth.APIKey, bool) {
	return s.Auth.GetAPIKey(key)
}

func (s *RuntimeState) ListServices() []catalog.ServiceSummary {
	return s.Catalog.ListServices()
}

func (s *RuntimeState) ListServicesNS(namespace string) []catalog.ServiceSummary {
	return s.Catalog.ListServicesNS(namespace)
}

func (s *RuntimeState) GetServiceHealthy(name string) []*catalog.Instance {
	return s.Catalog.GetServiceHealthy(name)
}

func (s *RuntimeState) GetServiceHealthyNS(namespace, name string) []*catalog.Instance {
	return s.Catalog.GetServiceHealthyNS(namespace, name)
}

func (s *RuntimeState) GetService(name string) []*catalog.Instance {
	return s.Catalog.GetService(name)
}

func (s *RuntimeState) GetServiceNS(namespace, name string) []*catalog.Instance {
	return s.Catalog.GetServiceNS(namespace, name)
}

func (s *RuntimeState) ListUsers() []*auth.User {
	return s.Auth.ListUsers()
}

func (s *RuntimeState) AddUser(user *auth.User) {
	s.Auth.AddUser(user)
	s.Auth.Save()
}

func (s *RuntimeState) DeleteUser(username string) {
	s.Auth.DeleteUser(username)
	s.Auth.Save()
}

func (s *RuntimeState) ListAPIKeys() []*auth.APIKey {
	return s.Auth.ListAPIKeys()
}

func (s *RuntimeState) AddAPIKey(key *auth.APIKey) {
	s.Auth.AddAPIKey(key)
	s.Auth.Save()
}

func (s *RuntimeState) DeleteAPIKey(key string) {
	s.Auth.DeleteAPIKey(key)
	s.Auth.Save()
}

func (s *RuntimeState) SetMode(mode string) {
	s.Settings.SetMode(mode)
	s.Settings.Save()
}

func (s *RuntimeState) SetEnvironment(environment string) {
	s.Settings.SetEnvironment(environment)
	s.Settings.Save()
}

func (s *RuntimeState) SetSeeds(seeds []string) {
	s.Settings.SetSeeds(seeds)
	s.Settings.Save()
}

func (s *RuntimeState) SetLogLevel(level string) {
	s.Settings.SetLogLevel(level)
	s.Settings.Save()
}

func (s *RuntimeState) GetEventRetentionDays() int {
	return s.Settings.GetEventRetentionDays()
}

func (s *RuntimeState) SetEventRetentionDays(days int) {
	s.Settings.SetEventRetentionDays(days)
	s.Settings.Save()
	s.Catalog.Events.Cleanup(days)
}

func (s *RuntimeState) GetLogRetentionDays() int {
	return s.Settings.GetLogRetentionDays()
}

func (s *RuntimeState) SetLogRetentionDays(days int) {
	s.Settings.SetLogRetentionDays(days)
	s.Settings.Save()
}

func (s *RuntimeState) GetEventTypes() []string {
	return s.Settings.GetEventTypes()
}

func (s *RuntimeState) SetEventTypes(eventTypes []string) {
	s.Settings.SetEventTypes(eventTypes)
	s.Settings.Save()
}

func (s *RuntimeState) GetHeartbeatMaxFailures() int {
	return s.Settings.GetHeartbeatMaxFailures()
}

func (s *RuntimeState) GetAPIKeyAuthEnabled() bool {
	return s.Settings.GetAPIKeyAuthEnabled()
}

func (s *RuntimeState) HasAPIKeyAuthSetting() bool {
	return s.Settings.HasAPIKeyAuthEnabled()
}

func (s *RuntimeState) SetHeartbeatMaxFailures(n int) {
	s.Settings.SetHeartbeatMaxFailures(n)
	s.Settings.Save()
}

func (s *RuntimeState) GetInstanceRemovalDelaySeconds() int {
	return s.Settings.GetInstanceRemovalDelaySeconds()
}

func (s *RuntimeState) SetInstanceRemovalDelaySeconds(n int) {
	s.Settings.SetInstanceRemovalDelaySeconds(n)
	s.Settings.Save()
}

func (s *RuntimeState) SetAPIKeyAuthEnabled(enabled bool) {
	s.Settings.SetAPIKeyAuthEnabled(enabled)
	s.Settings.Save()
}

func (s *RuntimeState) GetNotifyAlertNodeID() string {
	return s.Settings.GetNotifyAlertNodeID()
}

func (s *RuntimeState) SetNotifyAlertNodeID(nodeID string) {
	s.Settings.SetNotifyAlertNodeID(nodeID)
	s.Settings.Save()
}

func (s *RuntimeState) GetEventStorageMode() string {
	return s.Settings.GetEventStorageMode()
}

func (s *RuntimeState) SetEventStorageMode(mode string) {
	s.Settings.SetEventStorageMode(mode)
	s.Settings.Save()
	s.Catalog.Events.SetStorageMode(mode)
}

func (s *RuntimeState) GetMetricsStorageMode() string {
	return s.Settings.GetMetricsStorageMode()
}

func (s *RuntimeState) SetMetricsStorageMode(mode string) {
	s.Settings.SetMetricsStorageMode(mode)
	s.Settings.Save()
	s.Catalog.Metrics.SetStorageMode(mode)
}

func (s *RuntimeState) GetMetricsRetentionDays() int {
	return s.Settings.GetMetricsRetentionDays()
}

func (s *RuntimeState) SetMetricsRetentionDays(days int) {
	s.Settings.SetMetricsRetentionDays(days)
	s.Settings.Save()
}

func (s *RuntimeState) ListEvents() []*catalog.Event {
	return s.Catalog.ListEvents()
}

func (s *RuntimeState) MarkCritical(ttl time.Duration) ([]*catalog.Instance, []*catalog.Instance) {
	maxFail := s.GetHeartbeatMaxFailures()
	if maxFail <= 0 {
		maxFail = 3
	}
	removalDelay := time.Duration(s.GetInstanceRemovalDelaySeconds()) * time.Second
	return s.Catalog.Instances.MarkCritical(ttl, maxFail, removalDelay)
}

func (s *RuntimeState) ListNamespaces() []*catalog.Namespace {
	return s.Catalog.ListNamespaces()
}

func (s *RuntimeState) CreateNamespace(ns *catalog.Namespace) bool {
	return s.Catalog.CreateNamespace(ns)
}

func (s *RuntimeState) AppendEvent(eventType, service, instance, message string) {
	s.Catalog.AppendEvent(eventType, service, instance, message)
}

func (s *RuntimeState) UpdateNamespace(ns *catalog.Namespace) bool {
	return s.Catalog.UpdateNamespace(ns)
}

func (s *RuntimeState) DeleteNamespace(name string) bool {
	return s.Catalog.DeleteNamespace(name)
}

func (s *RuntimeState) SetInstanceStatus(namespace, serviceName, instanceID string, status catalog.HealthStatus) (*catalog.Instance, bool) {
	return s.Catalog.SetInstanceStatus(namespace, serviceName, instanceID, status)
}

func (s *RuntimeState) Snapshot() *SnapshotData {
	allServices := s.Catalog.Instances.GetAllNS()
	servicesByNS := make(map[string]map[string][]*catalog.Instance, len(allServices))
	for namespace, services := range allServices {
		namespaceCopy := make(map[string][]*catalog.Instance, len(services))
		for name, instances := range services {
			list := make([]*catalog.Instance, 0, len(instances))
			for _, inst := range instances {
				cp := *inst
				list = append(list, &cp)
			}
			namespaceCopy[name] = list
		}
		servicesByNS[namespace] = namespaceCopy
	}

	return &SnapshotData{
		ServicesByNS:       servicesByNS,
		Namespaces:         s.Catalog.Namespaces.List(),
		TopologyReports:    s.Catalog.Topology.Snapshot(),
		APIKeys:            s.Auth.GetAllAPIKeys(),
		Users:              s.Auth.GetAllUsers(),
		Mode:               s.Settings.GetMode(),
		Environment:        s.Settings.GetEnvironment(),
		Seeds:              s.Settings.GetSeeds(),
		LogLevel:           s.Settings.GetLogLevel(),
		EventRetentionDays: s.Settings.GetEventRetentionDays(),
		LogRetentionDays:   s.Settings.GetLogRetentionDays(),
		EventTypes:         s.Settings.GetEventTypes(),
		HBMaxFail:          s.Settings.GetHeartbeatMaxFailures(),
		RemovalDelay:       s.Settings.GetInstanceRemovalDelaySeconds(),
		APIKeyAuthEnabled:  s.Settings.GetAPIKeyAuthEnabled(),
		NotifyAlertNodeID:  s.Settings.GetNotifyAlertNodeID(),
		EventStorageMode:   s.Settings.GetEventStorageMode(),
		MetricsStorageMode: s.Settings.GetMetricsStorageMode(),
		MetricsRetentionDays: s.Settings.GetMetricsRetentionDays(),
	}
}

func (s *RuntimeState) Restore(data *SnapshotData) {
	if len(data.ServicesByNS) > 0 {
		services := make(map[string]map[string]map[string]*catalog.Instance, len(data.ServicesByNS))
		for namespace, namespaceServices := range data.ServicesByNS {
			services[namespace] = make(map[string]map[string]*catalog.Instance, len(namespaceServices))
			for name, list := range namespaceServices {
				instances := make(map[string]*catalog.Instance, len(list))
				for _, inst := range list {
					instances[inst.ID] = inst
				}
				services[namespace][name] = instances
			}
		}
		s.Catalog.Instances.RestoreNS(services)
	} else {
		services := make(map[string]map[string]*catalog.Instance, len(data.Services))
		for name, list := range data.Services {
			instances := make(map[string]*catalog.Instance, len(list))
			for _, inst := range list {
				instances[inst.ID] = inst
			}
			services[name] = instances
		}
		s.Catalog.Instances.Restore(services)
	}

	if len(data.Namespaces) > 0 {
		s.Catalog.Namespaces.Restore(data.Namespaces)
	}
	if len(data.TopologyReports) > 0 {
		s.Catalog.Topology.Restore(data.TopologyReports)
	}
	s.Auth.Restore(data.Users, data.APIKeys)
	s.Settings.Restore(data.Mode, data.Environment, data.LogLevel, data.Seeds, data.EventRetentionDays, data.LogRetentionDays, data.EventTypes, data.HBMaxFail, data.RemovalDelay, data.APIKeyAuthEnabled, data.NotifyAlertNodeID, data.EventStorageMode, data.MetricsStorageMode, data.MetricsRetentionDays)

	s.Catalog.Instances.Save()
	s.Auth.Save()
	s.Settings.Save()
	s.Catalog.Events.Cleanup(s.Settings.GetEventRetentionDays())
}

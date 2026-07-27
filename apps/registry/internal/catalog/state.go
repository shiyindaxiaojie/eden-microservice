package catalog

import (
	"time"
)

// ServiceSummary holds counts for a single service.
type ServiceSummary struct {
	Name          string `json:"name"`
	Group         string `json:"group"`
	QualifiedName string `json:"qualified_name"`
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

// State owns the service-catalog domain state.
type State struct {
	Instances          *InstanceRegistry
	Events             *EventLog
	Namespaces         *NamespaceRegistry
	Topology           *TopologyIndex
	Metrics            *MetricsStore
	eventTypesProvider func() []string
	onEventCallback    func(*Event)
}

func NewState(dataPath string) *State {
	return &State{
		Instances:  NewInstanceRegistry(dataPath),
		Events:     NewEventLog(1000, dataPath),
		Namespaces: NewNamespaceRegistry(dataPath),
		Topology:   NewTopologyIndex(dataPath),
		Metrics:    NewMetricsStore(dataPath, nil), // Retention days will be set via provider
	}
}

func (s *State) SetEventTypesProvider(fn func() []string) {
	s.eventTypesProvider = fn
}

func (s *State) SetOnEventCallback(fn func(*Event)) {
	s.onEventCallback = fn
}

func (s *State) Register(inst *Instance) {
	s.Instances.Register(inst)
}

func (s *State) Deregister(serviceName, instanceID string) (*Instance, bool) {
	return s.Instances.Deregister(serviceName, instanceID)
}

func (s *State) Heartbeat(serviceName, instanceID string) (*Instance, bool) {
	return s.Instances.Heartbeat(serviceName, instanceID)
}

func (s *State) HeartbeatNS(namespace, serviceName, instanceID string) (*Instance, bool) {
	return s.Instances.HeartbeatNS(namespace, serviceName, instanceID)
}

func (s *State) Stats() Stats {
	return s.Instances.Stats()
}

func (s *State) ListServices() []ServiceSummary {
	return s.Instances.ListServices()
}

func (s *State) ListServicesNS(namespace string) []ServiceSummary {
	return s.Instances.ListServicesNS(namespace)
}

func (s *State) GetServiceHealthy(name string) []*Instance {
	return s.Instances.GetServiceHealthy(name)
}

func (s *State) GetServiceHealthyNS(namespace, name string) []*Instance {
	return s.Instances.GetServiceHealthyNS(namespace, name)
}

func (s *State) GetService(name string) []*Instance {
	return s.Instances.GetService(name)
}

func (s *State) GetServiceNS(namespace, name string) []*Instance {
	return s.Instances.GetServiceNS(namespace, name)
}

func (s *State) ListEvents() []*Event {
	return s.Events.List()
}

func (s *State) QueryEvents(count, offset int, query, date, startTime, endTime, eventType, service string) ([]*Event, int) {
	return s.Events.QueryEvents(count, offset, query, date, startTime, endTime, eventType, service)
}

func (s *State) AppendEvent(eventType, service, instance, message string) {
	normalized := NormalizeEventTypes([]string{eventType})
	if len(normalized) == 0 {
		return
	}
	if !s.shouldRecordEvent(normalized[0]) {
		return
	}
	event := &Event{
		Type:      normalized[0],
		Service:   service,
		Instance:  instance,
		Message:   message,
		Timestamp: time.Now(),
	}
	s.Events.Append(event)
	if s.onEventCallback != nil {
		go s.onEventCallback(event)
	}
}

func (s *State) shouldRecordEvent(eventType string) bool {
	if s.eventTypesProvider == nil {
		return false
	}
	allowed := s.eventTypesProvider()
	if len(allowed) == 0 {
		return false
	}
	for _, candidate := range allowed {
		if candidate == eventType {
			return true
		}
	}
	return false
}

func (s *State) ListNamespaces() []*Namespace {
	return s.Namespaces.List()
}

func (s *State) CreateNamespace(ns *Namespace) bool {
	return s.Namespaces.Create(ns)
}

func (s *State) UpdateNamespace(ns *Namespace) bool {
	return s.Namespaces.Update(ns)
}

func (s *State) DeleteNamespace(name string) bool {
	return s.Namespaces.Delete(name)
}

func (s *State) SetInstanceStatus(namespace, serviceName, instanceID string, status HealthStatus) (*Instance, bool) {
	return s.Instances.SetInstanceStatus(namespace, serviceName, instanceID, status)
}

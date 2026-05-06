package catalog

import "time"

// HealthStatus represents the health state of a service instance.
type HealthStatus string

const (
	HealthPassing  HealthStatus = "passing"
	HealthCritical HealthStatus = "critical"
)

// DefaultNamespace is the namespace used when none is specified.
const DefaultNamespace = "default"

// Instance represents a single instance of a service.
type Instance struct {
	ID            string            `json:"id"`
	ServiceName   string            `json:"service_name"`
	Namespace     string            `json:"namespace,omitempty"`
	Host          string            `json:"host"`
	Port          int               `json:"port"`
	Weight        int               `json:"weight"`
	Datacenter    string            `json:"dc"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Status        HealthStatus      `json:"status"`
	ManualOffline bool              `json:"manual_offline,omitempty"`
	LastHeartbeat time.Time         `json:"last_heartbeat"`
	RegisteredAt  time.Time         `json:"registered_at"`
}

// EffectiveNamespace returns the namespace, defaulting to the default namespace when empty.
func (inst *Instance) EffectiveNamespace() string {
	if inst.Namespace == "" {
		return DefaultNamespace
	}
	return inst.Namespace
}

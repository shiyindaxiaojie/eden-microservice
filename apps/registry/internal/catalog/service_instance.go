package catalog

import (
	"strings"
	"time"
)

// HealthStatus represents the health state of a service instance.
type HealthStatus string

const (
	HealthPassing  HealthStatus = "passing"
	HealthCritical HealthStatus = "critical"
	HealthOffline  HealthStatus = "offline"
)

// DefaultNamespace is the namespace used when none is specified.
const DefaultNamespace = "default"

const (
	// DefaultServiceGroup is used by registry clients that do not support service groups.
	DefaultServiceGroup = "default"
	// ServiceGroupSeparator is accepted for backward compatibility with qualified Nacos service names.
	ServiceGroupSeparator = "@@"
)

// Instance represents a single instance of a service.
type Instance struct {
	ID            string            `json:"id"`
	ServiceName   string            `json:"service_name"`
	Group         string            `json:"group"`
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
	if strings.TrimSpace(inst.Namespace) == "" {
		return DefaultNamespace
	}
	return strings.TrimSpace(inst.Namespace)
}

// EffectiveGroup returns the normalized service group.
func (inst *Instance) EffectiveGroup() string {
	_, group := NormalizeServiceIdentity(inst.ServiceName, inst.Group)
	return group
}

// QualifiedServiceName returns the internal lookup key for this service identity.
func (inst *Instance) QualifiedServiceName() string {
	return QualifiedServiceName(inst.Group, inst.ServiceName)
}

// NormalizeIdentity separates legacy group@@service names and fills storage defaults.
func (inst *Instance) NormalizeIdentity() {
	inst.ServiceName, inst.Group = NormalizeServiceIdentity(inst.ServiceName, inst.Group)
	inst.Namespace = inst.EffectiveNamespace()
}

// NormalizeServiceIdentity returns a clean service name and a non-empty group.
func NormalizeServiceIdentity(serviceName, group string) (string, string) {
	name := strings.TrimSpace(serviceName)
	normalizedGroup := strings.TrimSpace(group)
	if index := strings.Index(name, ServiceGroupSeparator); index > 0 {
		legacyGroup := strings.TrimSpace(name[:index])
		legacyName := strings.TrimSpace(name[index+len(ServiceGroupSeparator):])
		if legacyName != "" {
			name = legacyName
			if normalizedGroup == "" || (normalizedGroup == DefaultServiceGroup && legacyGroup != DefaultServiceGroup) {
				normalizedGroup = legacyGroup
			}
		}
	}
	if normalizedGroup == "" {
		normalizedGroup = DefaultServiceGroup
	}
	return name, normalizedGroup
}

// QualifiedServiceName builds the collision-free internal lookup key. The native
// default group keeps the historical unqualified key for compatibility.
func QualifiedServiceName(group, serviceName string) string {
	name, normalizedGroup := NormalizeServiceIdentity(serviceName, group)
	if name == "" || normalizedGroup == DefaultServiceGroup {
		return name
	}
	return normalizedGroup + ServiceGroupSeparator + name
}

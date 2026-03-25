package registry

// ServiceInstance represents a service instance in the registry.
type ServiceInstance struct {
	ID          string            `json:"id"`
	ServiceName string            `json:"service_name"`
	Host        string            `json:"host"`
	Port        int               `json:"port"`
	Weight      int               `json:"weight"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Healthy     bool              `json:"healthy"`
	Datacenter  string            `json:"datacenter,omitempty"`
}

// Registry is the core abstraction for service registration and discovery.
// Implementations include Eden (native), Consul adapter, and Nacos adapter.
type Registry interface {
	// Register registers this service instance with the registry.
	Register(instance *ServiceInstance) error

	// Deregister removes this service instance from the registry.
	Deregister(instance *ServiceInstance) error

	// Discovery returns all healthy instances of the given service.
	Discovery(serviceName string) ([]*ServiceInstance, error)

	// Subscribe watches for changes in the given service's instance list.
	// The callback is invoked whenever the instance list changes.
	Subscribe(serviceName string, callback func([]*ServiceInstance)) error

	// Heartbeat sends a heartbeat for the given instance to keep it alive.
	Heartbeat(instance *ServiceInstance) error

	// Close shuts down the registry client and releases resources.
	Close() error
}

// Config is the common configuration for all registry implementations.
type Config struct {
	Type          string   `yaml:"type" json:"type"`                     // "eden", "consul", "nacos"
	Addresses     []string `yaml:"addresses" json:"addresses"`           // registry server addresses
	APIKey        string   `yaml:"api_key" json:"api_key"`               // authentication key
	Namespace     string   `yaml:"namespace" json:"namespace"`           // nacos namespace
	Datacenter    string   `yaml:"datacenter" json:"datacenter"`         // consul datacenter
	Transport     string   `yaml:"transport" json:"transport"`           // eden only: "grpc", "quic", "http"
	DiscoveryMode string   `yaml:"discovery_mode" json:"discovery_mode"` // eden only: "auto" or "static"
}

package sdk

const DefaultServiceGroup = "default"

// ServiceInstance represents a service instance in the registry.
type ServiceInstance struct {
	ID          string            `json:"id"`
	ServiceName string            `json:"service_name"`
	Group       string            `json:"group,omitempty"`
	Host        string            `json:"host"`
	Port        int               `json:"port"`
	Weight      int               `json:"weight"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Healthy     bool              `json:"healthy"`
	Datacenter  string            `json:"datacenter,omitempty"`
}

// Registry is the shared abstraction for service registration and discovery.
type Registry interface {
	Register(instance *ServiceInstance) error
	Deregister(instance *ServiceInstance) error
	Discovery(serviceName string) ([]*ServiceInstance, error)
	DiscoveryGroup(serviceName, group string) ([]*ServiceInstance, error)
	Subscribe(serviceName string, callback func([]*ServiceInstance)) error
	SubscribeGroup(serviceName, group string, callback func([]*ServiceInstance)) error
	Heartbeat(instance *ServiceInstance) error
	Close() error
}

// Config defines the native SDK client configuration.
type Config struct {
	Addresses     []string
	APIKey        string
	Datacenter    string
	Namespace     string
	CacheDir      string
	Transport     string // "http", "grpc" (default), "quic"
	DiscoveryMode string // "auto" (default) or "static"
}

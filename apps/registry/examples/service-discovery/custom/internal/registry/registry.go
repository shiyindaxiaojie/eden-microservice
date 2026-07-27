package registry

// ServiceInstance is the local instance model used by the custom integration example.
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

// Registry is the local abstraction used by the custom example.
type Registry interface {
	Register(instance *ServiceInstance) error
	Deregister(instance *ServiceInstance) error
	Discovery(serviceName string) ([]*ServiceInstance, error)
	Subscribe(serviceName string, callback func([]*ServiceInstance)) error
	Heartbeat(instance *ServiceInstance) error
	Close() error
}

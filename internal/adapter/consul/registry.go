package consul

import (
	"time"

	"github.com/shiyindaxiaojie/eden-registry/internal/adapter/consul/api"
	"github.com/shiyindaxiaojie/eden-registry/pkg/sdk"
)

type consulRegistry struct {
	client *api.Client
}

// NewRegistry creates a new Consul-based registry implementation.
func NewRegistry(cfg *sdk.Config) (sdk.Registry, error) {
	config := api.DefaultConfig()
	if len(cfg.Addresses) > 0 {
		config.Address = cfg.Addresses[0]
	}
	config.Datacenter = cfg.Datacenter
	config.Token = cfg.APIKey

	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}
	return &consulRegistry{client: client}, nil
}

func (r *consulRegistry) Register(inst *sdk.ServiceInstance) error {
	reg := &api.AgentServiceRegistration{
		ID:      inst.ID,
		Name:    inst.ServiceName,
		Address: inst.Host,
		Port:    inst.Port,
		Meta:    inst.Metadata,
		Weights: &api.AgentWeights{Passing: inst.Weight},
	}
	return r.client.Agent().ServiceRegister(reg)
}

func (r *consulRegistry) Deregister(inst *sdk.ServiceInstance) error {
	return r.client.Agent().ServiceDeregister(inst.ID)
}

func (r *consulRegistry) Discovery(serviceName string) ([]*sdk.ServiceInstance, error) {
	entries, _, err := r.client.Health().Service(serviceName, "", true, nil)
	if err != nil {
		return nil, err
	}

	instances := make([]*sdk.ServiceInstance, 0, len(entries))
	for _, entry := range entries {
		instances = append(instances, &sdk.ServiceInstance{
			ID:          entry.Service.ID,
			ServiceName: entry.Service.Service,
			Host:        entry.Service.Address,
			Port:        entry.Service.Port,
			Metadata:    entry.Service.Meta,
			Healthy:     true,
			Datacenter:  entry.Node.Datacenter,
		})
	}
	return instances, nil
}

func (r *consulRegistry) Subscribe(serviceName string, callback func([]*sdk.ServiceInstance)) error {
	// Simple polling for now
	go func() {
		lastInstances := []*sdk.ServiceInstance{}
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for {
			<-ticker.C

			instances, err := r.Discovery(serviceName)
			if err == nil {
				// Simple check
				if len(instances) != len(lastInstances) {
					lastInstances = instances
					callback(instances)
				}
			}
		}
	}()
	return nil
}

func (r *consulRegistry) Heartbeat(inst *sdk.ServiceInstance) error {
	return r.client.Agent().PassTTL("service:"+inst.ID, "heartbeat")
}

func (r *consulRegistry) Close() error {
	return nil
}

package consulapi

import (
	"encoding/json"
	"fmt"
	"time"

	consulapi "github.com/hashicorp/consul/api"
)

// ServiceInstance is the local demo shape used by the Consul example.
type ServiceInstance struct {
	ID          string
	ServiceName string
	Host        string
	Port        int
	Weight      int
	Metadata    map[string]string
}

// Client wraps the official Consul client with a tiny demo-oriented helper surface.
type Client struct {
	raw *consulapi.Client
}

func New(raw *consulapi.Client) *Client {
	return &Client{raw: raw}
}

func (c *Client) Register(inst *ServiceInstance) error {
	return c.raw.Agent().ServiceRegister(&consulapi.AgentServiceRegistration{
		ID:      inst.ID,
		Name:    inst.ServiceName,
		Address: inst.Host,
		Port:    inst.Port,
		Meta:    inst.Metadata,
		Weights: &consulapi.AgentWeights{Passing: inst.Weight},
		Check:   &consulapi.AgentServiceCheck{TTL: "30s"},
	})
}

func (c *Client) Deregister(inst *ServiceInstance) error {
	return c.raw.Agent().ServiceDeregister(inst.ID)
}

func (c *Client) Heartbeat(inst *ServiceInstance) error {
	return c.raw.Agent().PassTTL("service:"+inst.ID, "heartbeat")
}

func (c *Client) Discovery(serviceName string) ([]*ServiceInstance, error) {
	entries, _, err := c.raw.Health().Service(serviceName, "", true, nil)
	if err != nil {
		return nil, err
	}

	result := make([]*ServiceInstance, 0, len(entries))
	for _, entry := range entries {
		weight := entry.Service.Weights.Passing
		if weight <= 0 {
			weight = 1
		}
		result = append(result, &ServiceInstance{
			ID:          entry.Service.ID,
			ServiceName: entry.Service.Service,
			Host:        entry.Service.Address,
			Port:        entry.Service.Port,
			Weight:      weight,
			Metadata:    entry.Service.Meta,
		})
	}
	return result, nil
}

func (c *Client) Subscribe(serviceName string, callback func([]*ServiceInstance)) error {
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		lastHash := ""
		for range ticker.C {
			instances, err := c.Discovery(serviceName)
			if err != nil {
				continue
			}
			hash := instancesHash(instances)
			if hash != lastHash {
				lastHash = hash
				callback(instances)
			}
		}
	}()
	return nil
}

func (c *Client) Close() error {
	return nil
}

func instancesHash(instances []*ServiceInstance) string {
	data, _ := json.Marshal(instances)
	return fmt.Sprintf("%x", data)
}

package nacos

import (
	"net"
	"strconv"
	"strings"

	"github.com/shiyindaxiaojie/eden-go-registry/pkg/nacos/clients/naming_client"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/nacos/common/constant"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/nacos/model"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/nacos/vo"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/registry"
)

type nacosRegistry struct {
	client naming_client.INamingClient
}

// NewRegistry creates a new Nacos-based registry implementation.
func NewRegistry(cfg *registry.Config) (registry.Registry, error) {
	// Map registry.Config to Nacos ServerConfig
	serverConfigs := make([]constant.ServerConfig, 0, len(cfg.Addresses))
	for _, addr := range cfg.Addresses {
		host := "127.0.0.1"
		port := 8848

		addr = strings.TrimSpace(addr)
		if addr != "" {
			if strings.Contains(addr, "://") {
				addr = strings.TrimPrefix(addr, "http://")
				addr = strings.TrimPrefix(addr, "https://")
			}

			if h, p, err := net.SplitHostPort(addr); err == nil {
				host = h
				if value, convErr := strconv.Atoi(p); convErr == nil {
					port = value
				}
			} else if h, p, found := strings.Cut(addr, ":"); found {
				if h != "" {
					host = h
				}
				if value, convErr := strconv.Atoi(p); convErr == nil {
					port = value
				}
			} else if addr != "" {
				host = addr
			}
		}

		serverConfigs = append(serverConfigs, constant.ServerConfig{
			IpAddr: host,
			Port:   uint64(port),
		})
	}

	client, err := naming_client.NewNamingClient(vo.NacosClientParam{
		ServerConfigs: serverConfigs,
		ClientConfig:  &constant.ClientConfig{NamespaceId: cfg.Namespace},
	})
	if err != nil {
		return nil, err
	}
	return &nacosRegistry{client: client}, nil
}

func (r *nacosRegistry) Register(inst *registry.ServiceInstance) error {
	_, err := r.client.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          inst.Host,
		Port:        uint64(inst.Port),
		ServiceName: inst.ServiceName,
		Weight:      float64(inst.Weight),
		Metadata:    inst.Metadata,
	})
	return err
}

func (r *nacosRegistry) Deregister(inst *registry.ServiceInstance) error {
	_, err := r.client.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          inst.Host,
		Port:        uint64(inst.Port),
		ServiceName: inst.ServiceName,
	})
	return err
}

func (r *nacosRegistry) Discovery(serviceName string) ([]*registry.ServiceInstance, error) {
	hosts, err := r.client.SelectInstances(vo.SelectInstancesParam{
		ServiceName: serviceName,
		HealthyOnly: true,
	})
	if err != nil {
		return nil, err
	}

	instances := make([]*registry.ServiceInstance, 0, len(hosts))
	for _, h := range hosts {
		instances = append(instances, &registry.ServiceInstance{
			ID:          h.InstanceId,
			ServiceName: h.ServiceName,
			Host:        h.Ip,
			Port:        int(h.Port),
			Weight:      int(h.Weight),
			Metadata:    h.Metadata,
			Healthy:     h.Healthy,
		})
	}
	return instances, nil
}

func (r *nacosRegistry) Subscribe(serviceName string, callback func([]*registry.ServiceInstance)) error {
	return r.client.Subscribe(&vo.SubscribeParam{
		ServiceName: serviceName,
		SubscribeCallback: func(services []model.Instance, err error) {
			if err == nil {
				instances := make([]*registry.ServiceInstance, 0, len(services))
				for _, h := range services {
					instances = append(instances, &registry.ServiceInstance{
						ID:          h.InstanceId,
						ServiceName: h.ServiceName,
						Host:        h.Ip,
						Port:        int(h.Port),
						Weight:      int(h.Weight),
						Metadata:    h.Metadata,
						Healthy:     h.Healthy,
					})
				}
				callback(instances)
			}
		},
	})
}

func (r *nacosRegistry) Heartbeat(inst *registry.ServiceInstance) error {
	// Nacos v1 SDK uses client-side heartbeats automatically;
	// our compat layer might need manual calls if it points to Eden.
	// But Eden's Register/Heartbeat are distinct.
	// For compat, we just re-register or use the same heartbeat logic.
	return r.Register(inst)
}

func (r *nacosRegistry) Close() error {
	return nil
}

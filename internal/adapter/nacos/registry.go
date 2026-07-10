package nacos

import (
	"net"
	"strconv"
	"strings"

	"github.com/shiyindaxiaojie/eden-registry/internal/adapter/nacos/clients/naming_client"
	"github.com/shiyindaxiaojie/eden-registry/internal/adapter/nacos/common/constant"
	"github.com/shiyindaxiaojie/eden-registry/internal/adapter/nacos/model"
	"github.com/shiyindaxiaojie/eden-registry/internal/adapter/nacos/vo"
	"github.com/shiyindaxiaojie/eden-registry/pkg/sdk"
)

type nacosRegistry struct {
	client naming_client.INamingClient
}

// NewRegistry creates a new Nacos-based registry implementation.
func NewRegistry(cfg *sdk.Config) (sdk.Registry, error) {
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

func (r *nacosRegistry) Register(inst *sdk.ServiceInstance) error {
	_, err := r.client.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          inst.Host,
		Port:        uint64(inst.Port),
		ServiceName: inst.ServiceName,
		GroupName:   inst.Group,
		Weight:      float64(inst.Weight),
		Metadata:    inst.Metadata,
	})
	return err
}

func (r *nacosRegistry) Deregister(inst *sdk.ServiceInstance) error {
	_, err := r.client.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          inst.Host,
		Port:        uint64(inst.Port),
		ServiceName: inst.ServiceName,
		GroupName:   inst.Group,
	})
	return err
}

func (r *nacosRegistry) Discovery(serviceName string) ([]*sdk.ServiceInstance, error) {
	return r.DiscoveryGroup(serviceName, "")
}

func (r *nacosRegistry) DiscoveryGroup(serviceName, group string) ([]*sdk.ServiceInstance, error) {
	hosts, err := r.client.SelectInstances(vo.SelectInstancesParam{
		ServiceName: serviceName,
		GroupName:   group,
		HealthyOnly: true,
	})
	if err != nil {
		return nil, err
	}

	instances := make([]*sdk.ServiceInstance, 0, len(hosts))
	for _, h := range hosts {
		instances = append(instances, &sdk.ServiceInstance{
			ID:          h.InstanceId,
			ServiceName: h.ServiceName,
			Group:       group,
			Host:        h.Ip,
			Port:        int(h.Port),
			Weight:      int(h.Weight),
			Metadata:    h.Metadata,
			Healthy:     h.Healthy,
		})
	}
	return instances, nil
}

func (r *nacosRegistry) Subscribe(serviceName string, callback func([]*sdk.ServiceInstance)) error {
	return r.SubscribeGroup(serviceName, "", callback)
}

func (r *nacosRegistry) SubscribeGroup(serviceName, group string, callback func([]*sdk.ServiceInstance)) error {
	return r.client.Subscribe(&vo.SubscribeParam{
		ServiceName: serviceName,
		GroupName:   group,
		SubscribeCallback: func(services []model.Instance, err error) {
			if err == nil {
				instances := make([]*sdk.ServiceInstance, 0, len(services))
				for _, h := range services {
					instances = append(instances, &sdk.ServiceInstance{
						ID:          h.InstanceId,
						ServiceName: h.ServiceName,
						Group:       group,
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

func (r *nacosRegistry) Heartbeat(inst *sdk.ServiceInstance) error {
	// Nacos v1 SDK uses client-side heartbeats automatically;
	// our compat layer might need manual calls if it points to the local registry.
	// But the local registry keeps Register and Heartbeat separate.
	// For compat, we just re-register or use the same heartbeat logic.
	return r.Register(inst)
}

func (r *nacosRegistry) Close() error {
	return nil
}

package nacosapi

import (
	"net"
	"strconv"
	"strings"

	nacosclients "github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	nacosmodel "github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

type ServiceInstance struct {
	ID          string
	ServiceName string
	Host        string
	Port        int
	Weight      int
	Metadata    map[string]string
}

type Client struct {
	raw       nacosclients.INamingClient
	groupName string
}

func New(raw nacosclients.INamingClient) *Client {
	return &Client{
		raw:       raw,
		groupName: constant.DEFAULT_GROUP,
	}
}

func ParseServerConfig(addr string) constant.ServerConfig {
	value := strings.TrimSpace(addr)
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimPrefix(value, "https://")

	host := "127.0.0.1"
	port := 8848
	if parsedHost, parsedPort, err := net.SplitHostPort(value); err == nil {
		host = parsedHost
		if number, convErr := strconv.Atoi(parsedPort); convErr == nil {
			port = number
		}
	} else if left, right, ok := strings.Cut(value, ":"); ok {
		if left != "" {
			host = left
		}
		if number, convErr := strconv.Atoi(right); convErr == nil {
			port = number
		}
	} else if value != "" {
		host = value
	}

	return constant.ServerConfig{
		Scheme: "http",
		IpAddr: host,
		Port:   uint64(port),
	}
}

func DefaultClientConfig(namespace string) *constant.ClientConfig {
	ns := strings.TrimSpace(namespace)
	if ns == "" || strings.EqualFold(ns, constant.DEFAULT_NAMESPACE_ID) {
		ns = ""
	}

	return &constant.ClientConfig{
		NamespaceId:         ns,
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		CacheDir:            ".tmp/nacos/cache",
		LogDir:              ".tmp/nacos/log",
		LogLevel:            "warn",
	}
}

func (c *Client) Register(inst *ServiceInstance) error {
	weight := inst.Weight
	if weight <= 0 {
		weight = 1
	}

	_, err := c.raw.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          inst.Host,
		Port:        uint64(inst.Port),
		Weight:      float64(weight),
		Enable:      true,
		Healthy:     true,
		Metadata:    inst.Metadata,
		ClusterName: "DEFAULT",
		ServiceName: inst.ServiceName,
		GroupName:   c.groupName,
		Ephemeral:   false,
	})
	return err
}

func (c *Client) Deregister(inst *ServiceInstance) error {
	_, err := c.raw.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          inst.Host,
		Port:        uint64(inst.Port),
		Cluster:     "DEFAULT",
		ServiceName: inst.ServiceName,
		GroupName:   c.groupName,
		Ephemeral:   false,
	})
	return err
}

func (c *Client) Heartbeat(inst *ServiceInstance) error {
	return nil
}

func (c *Client) Discovery(serviceName string) ([]*ServiceInstance, error) {
	items, err := c.raw.SelectInstances(vo.SelectInstancesParam{
		ServiceName: serviceName,
		GroupName:   c.groupName,
		HealthyOnly: true,
	})
	if err != nil {
		return nil, err
	}

	result := make([]*ServiceInstance, 0, len(items))
	for _, item := range items {
		weight := int(item.Weight)
		if weight <= 0 {
			weight = 1
		}
		result = append(result, &ServiceInstance{
			ID:          item.InstanceId,
			ServiceName: serviceName,
			Host:        item.Ip,
			Port:        int(item.Port),
			Weight:      weight,
			Metadata:    item.Metadata,
		})
	}
	return result, nil
}

func (c *Client) Subscribe(serviceName string, callback func([]*ServiceInstance)) error {
	return c.raw.Subscribe(&vo.SubscribeParam{
		ServiceName: serviceName,
		GroupName:   c.groupName,
		SubscribeCallback: func(services []nacosmodel.Instance, err error) {
			if err != nil {
				return
			}

			items := make([]*ServiceInstance, 0, len(services))
			for _, service := range services {
				weight := int(service.Weight)
				if weight <= 0 {
					weight = 1
				}
				items = append(items, &ServiceInstance{
					ID:          service.InstanceId,
					ServiceName: serviceName,
					Host:        service.Ip,
					Port:        int(service.Port),
					Weight:      weight,
					Metadata:    service.Metadata,
				})
			}
			callback(items)
		},
	})
}

func (c *Client) Close() error {
	c.raw.CloseClient()
	return nil
}

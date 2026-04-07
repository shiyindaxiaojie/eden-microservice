package nacosapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

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
	raw             nacosclients.INamingClient
	groupName       string
	namespace       string
	apiKey          string
	reportTargets   []string
	httpClient      *http.Client

	mu               sync.RWMutex
	consumerService  string
	providers        map[string]struct{}
	topologyChecksum string
}

func New(raw nacosclients.INamingClient) *Client {
	return &Client{
		raw:           raw,
		groupName:     constant.DEFAULT_GROUP,
		namespace:     normalizeNamespace(envOr("NACOS_NAMESPACE", "public")),
		apiKey:        envOr("NACOS_API_KEY", ""),
		reportTargets: normalizeTargets(splitAddrs(envOr("NACOS_ADDR", "127.0.0.1:8500"))),
		httpClient:    &http.Client{Timeout: 5 * time.Second},
		providers:     make(map[string]struct{}),
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

	c.mu.Lock()
	c.consumerService = inst.ServiceName
	c.mu.Unlock()

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
	c.rememberProvider(serviceName)
	c.reportTopology()

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
	c.rememberProvider(serviceName)
	c.reportTopology()

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

func (c *Client) rememberProvider(serviceName string) {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.consumerService == "" || serviceName == c.consumerService {
		return
	}
	c.providers[serviceName] = struct{}{}
}

func (c *Client) reportTopology() {
	consumerService, providers, checksum := c.topologySnapshot()
	if consumerService == "" || checksum == "" {
		return
	}

	c.mu.RLock()
	lastChecksum := c.topologyChecksum
	c.mu.RUnlock()
	if checksum == lastChecksum {
		return
	}

	payload := map[string]interface{}{
		"namespace":        c.namespace,
		"consumer_service": consumerService,
		"providers":        providers,
		"checksum":         checksum,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	for _, target := range c.reportTargets {
		req, err := http.NewRequest(http.MethodPost, target+"/v1/catalog/topology/report", bytes.NewReader(data))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			req.Header.Set("X-API-Key", c.apiKey)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			continue
		}
		_, _ = bytes.NewBuffer(nil).ReadFrom(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			c.mu.Lock()
			c.topologyChecksum = checksum
			c.mu.Unlock()
			return
		}
	}
}

func (c *Client) topologySnapshot() (string, []string, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.consumerService == "" || len(c.providers) == 0 {
		return "", nil, ""
	}

	providers := make([]string, 0, len(c.providers))
	for serviceName := range c.providers {
		if serviceName == "" || serviceName == c.consumerService {
			continue
		}
		providers = append(providers, serviceName)
	}
	if len(providers) == 0 {
		return c.consumerService, nil, ""
	}

	sort.Strings(providers)
	payload, _ := json.Marshal(map[string]interface{}{
		"namespace": c.namespace,
		"consumer":  c.consumerService,
		"providers": providers,
	})
	sum := sha256.Sum256(payload)
	return c.consumerService, providers, fmt.Sprintf("%x", sum[:])
}

func envOr(key, def string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return def
}

func splitAddrs(raw string) []string {
	parts := strings.Split(raw, ",")
	addrs := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			addrs = append(addrs, part)
		}
	}
	return addrs
}

func normalizeTargets(addrs []string) []string {
	targets := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
			targets = append(targets, addr)
			continue
		}
		if strings.HasPrefix(addr, ":") {
			targets = append(targets, "http://127.0.0.1"+addr)
			continue
		}
		targets = append(targets, "http://"+addr)
	}
	return targets
}

func normalizeNamespace(namespace string) string {
	ns := strings.TrimSpace(namespace)
	if ns == "" || strings.EqualFold(ns, constant.DEFAULT_NAMESPACE_ID) {
		return ""
	}
	return ns
}

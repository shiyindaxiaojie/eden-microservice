package consulapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
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
	raw             *consulapi.Client
	registryTargets []string
	apiKey          string
	namespace       string
	httpClient      *http.Client

	mu               sync.RWMutex
	consumerService  string
	providers        map[string]struct{}
	topologyChecksum string
}

func New(raw *consulapi.Client) *Client {
	return &Client{
		raw:             raw,
		registryTargets: normalizeTargets(splitAddrs(envOr("CONSUL_ADDR", "127.0.0.1:8500"))),
		apiKey:          envOr("CONSUL_API_KEY", ""),
		namespace:       envOr("CONSUL_NAMESPACE", "default"),
		httpClient:      &http.Client{Timeout: 5 * time.Second},
		providers:       make(map[string]struct{}),
	}
}

func (c *Client) Register(inst *ServiceInstance) error {
	c.mu.Lock()
	c.consumerService = inst.ServiceName
	c.mu.Unlock()

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
	c.rememberProvider(serviceName)
	c.reportTopology()

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
	c.rememberProvider(serviceName)
	c.reportTopology()

	go func() {
		lastInstances := []*ServiceInstance{}
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			instances, err := c.Discovery(serviceName)
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

func (c *Client) Close() error {
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

	for _, target := range c.registryTargets {
		req, err := http.NewRequest(http.MethodPost, target+"/v1/catalog/topology/report", bytes.NewReader(data))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			req.Header.Set("X-API-Key", c.apiKey)
			req.Header.Set("X-Consul-Token", c.apiKey)
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

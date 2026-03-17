// Package api is a drop-in replacement for github.com/hashicorp/consul/api.
// It provides the same API surface but connects to Eden Go Registry instead of Consul.
//
// Usage in existing Consul-based code:
//
//	// Before:
//	// import consulapi "github.com/hashicorp/consul/api"
//
//	// After (only change the import):
//	import consulapi "github.com/shiyindaxiaojie/eden-go-registry/pkg/consul/api"
//
// No other code changes are required.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ---------- Config & Client ----------

// Config is used to configure the creation of a client.
type Config struct {
	Address    string
	Datacenter string
	Token      string
	HttpClient *http.Client
}

// DefaultConfig returns a default configuration for the client.
func DefaultConfig() *Config {
	return &Config{
		Address: "127.0.0.1:8500",
	}
}

// Client provides a client to the Eden API (Consul-compatible surface).
type Client struct {
	config *Config
	client *http.Client
}

// NewClient returns a new client.
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		config = DefaultConfig()
	}
	httpClient := config.HttpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{config: config, client: httpClient}, nil
}

// Agent returns a handle to the agent-like endpoints.
func (c *Client) Agent() *Agent {
	return &Agent{client: c}
}

// Catalog returns a handle to the catalog-like endpoints.
func (c *Client) Catalog() *Catalog {
	return &Catalog{client: c}
}

// Health returns a handle to the health-like endpoints.
func (c *Client) Health() *Health {
	return &Health{client: c}
}

func (c *Client) baseURL() string {
	addr := c.config.Address
	if addr != "" && addr[0] == ':' {
		addr = "127.0.0.1" + addr
	}
	return "http://" + addr
}

func (c *Client) doPost(path string, body interface{}) error {
	data, _ := json.Marshal(body)
	url := c.baseURL() + path

	req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	if c.config.Token != "" {
		req.Header.Set("X-API-Key", c.config.Token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("consul-compat: status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *Client) doGet(path string) ([]byte, error) {
	url := c.baseURL() + path
	req, _ := http.NewRequest("GET", url, nil)
	if c.config.Token != "" {
		req.Header.Set("X-API-Key", c.config.Token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// ---------- Agent ----------

// Agent can be used to query the Agent-like endpoints.
type Agent struct {
	client *Client
}

// AgentServiceRegistration is used to register a new service.
type AgentServiceRegistration struct {
	ID                string
	Name              string
	Tags              []string
	Port              int
	Address           string
	Meta              map[string]string
	Check             *AgentServiceCheck
	Checks            AgentServiceChecks
	Weights           *AgentWeights
	EnableTagOverride bool
}

// AgentServiceCheck is used to define a node or service level check.
type AgentServiceCheck struct {
	CheckID                        string
	Name                           string
	TTL                            string
	HTTP                           string
	Interval                       string
	Timeout                        string
	DeregisterCriticalServiceAfter string
}

// AgentServiceChecks is a list of checks.
type AgentServiceChecks []*AgentServiceCheck

// AgentWeights represent optional weights for a service.
type AgentWeights struct {
	Passing int
	Warning int
}

// ServiceRegister registers a new service with the local agent.
func (a *Agent) ServiceRegister(service *AgentServiceRegistration) error {
	body := map[string]interface{}{
		"id":           service.ID,
		"service_name": service.Name,
		"host":         service.Address,
		"port":         service.Port,
		"metadata":     service.Meta,
	}
	if service.Weights != nil && service.Weights.Passing > 0 {
		body["weight"] = service.Weights.Passing
	}
	if a.client.config.Datacenter != "" {
		body["datacenter"] = a.client.config.Datacenter
	}
	return a.client.doPost("/v1/catalog/register", body)
}

// ServiceDeregister deregisters a service with the local agent.
func (a *Agent) ServiceDeregister(serviceID string) error {
	// We need the service name to deregister. In Eden, we use the serviceID
	// which is the instance ID. Query services to find the service_name first.
	data, err := a.client.doGet("/v1/catalog/services")
	if err != nil {
		return err
	}

	var services []struct {
		Name string `json:"name"`
	}
	json.Unmarshal(data, &services)

	// Try each service to find the instance
	for _, svc := range services {
		body := map[string]string{
			"service_name": svc.Name,
			"instance_id":  serviceID,
		}
		err := a.client.doPost("/v1/catalog/deregister", body)
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("service instance %s not found", serviceID)
}

// UpdateTTL sends a TTL check update (maps to heartbeat in Eden).
func (a *Agent) UpdateTTL(checkID, output, status string) error {
	// checkID format in Consul is typically "service:<serviceID>"
	// We need to find the service_name for this instance
	instanceID := checkID
	if len(checkID) > 8 && checkID[:8] == "service:" {
		instanceID = checkID[8:]
	}

	data, err := a.client.doGet("/v1/catalog/services")
	if err != nil {
		return err
	}

	var services []struct {
		Name string `json:"name"`
	}
	json.Unmarshal(data, &services)

	for _, svc := range services {
		body := map[string]string{
			"service_name": svc.Name,
			"instance_id":  instanceID,
		}
		err := a.client.doPost("/v1/catalog/heartbeat", body)
		if err == nil {
			return nil
		}
	}
	return nil
}

// PassTTL is a helper for updating a TTL check to passing.
func (a *Agent) PassTTL(checkID, note string) error {
	return a.UpdateTTL(checkID, note, "passing")
}

// ---------- Catalog ----------

// Catalog can be used to query the Catalog endpoints.
type Catalog struct {
	client *Client
}

// CatalogService information.
type CatalogService struct {
	ID          string
	Node        string
	ServiceID   string
	ServiceName string
	Address     string
	ServicePort int
	ServiceMeta map[string]string
	ServiceTags []string
	Datacenter  string
}

// QueryOptions are used to parameterize a query.
type QueryOptions struct {
	Datacenter string
	WaitIndex  uint64
	WaitTime   time.Duration
}

// QueryMeta is used to return meta data about a query.
type QueryMeta struct {
	LastIndex uint64
}

// Service lists instances of a service in the catalog.
func (c *Catalog) Service(service, tag string, q *QueryOptions) ([]*CatalogService, *QueryMeta, error) {
	path := fmt.Sprintf("/v1/catalog/service/%s", service)
	data, err := c.client.doGet(path)
	if err != nil {
		return nil, nil, err
	}

	var edenInstances []struct {
		ID          string            `json:"id"`
		ServiceName string            `json:"service_name"`
		Host        string            `json:"host"`
		Port        int               `json:"port"`
		Status      string            `json:"status"`
		Metadata    map[string]string `json:"metadata"`
		Datacenter  string            `json:"datacenter"`
	}
	json.Unmarshal(data, &edenInstances)

	result := make([]*CatalogService, 0, len(edenInstances))
	for _, ei := range edenInstances {
		if tag != "" {
			// Filter by tag if specified
			found := false
			for _, v := range ei.Metadata {
				if v == tag {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		result = append(result, &CatalogService{
			ID:          ei.ID,
			ServiceID:   ei.ID,
			ServiceName: ei.ServiceName,
			Address:     ei.Host,
			ServicePort: ei.Port,
			ServiceMeta: ei.Metadata,
			Datacenter:  ei.Datacenter,
		})
	}

	return result, &QueryMeta{LastIndex: 1}, nil
}

// ---------- Health ----------

// Health can be used to query the Health endpoints.
type Health struct {
	client *Client
	mu     sync.Mutex
	index  uint64
}

// ServiceEntry represents a complete service entry with node and checks.
type ServiceEntry struct {
	Node    *Node
	Service *AgentService
	Checks  HealthChecks
}

// Node represents a Consul node.
type Node struct {
	ID         string
	Node       string
	Address    string
	Datacenter string
}

// AgentService represents an active service.
type AgentService struct {
	ID      string
	Service string
	Address string
	Port    int
	Tags    []string
	Meta    map[string]string
	Weights AgentWeights
}

// HealthCheck represents a single health check.
type HealthCheck struct {
	CheckID   string
	Status    string
	ServiceID string
}

// HealthChecks is a list of health checks.
type HealthChecks []*HealthCheck

// Service returns the healthy instances of a service.
func (h *Health) Service(service, tag string, passingOnly bool, q *QueryOptions) ([]*ServiceEntry, *QueryMeta, error) {
	path := fmt.Sprintf("/v1/catalog/service/%s", service)
	if passingOnly {
		path += "?passing=true"
	}
	data, err := h.client.doGet(path)
	if err != nil {
		return nil, nil, err
	}

	var edenInstances []struct {
		ID          string            `json:"id"`
		ServiceName string            `json:"service_name"`
		Host        string            `json:"host"`
		Port        int               `json:"port"`
		Weight      int               `json:"weight"`
		Status      string            `json:"status"`
		Metadata    map[string]string `json:"metadata"`
		Datacenter  string            `json:"datacenter"`
	}
	json.Unmarshal(data, &edenInstances)

	h.mu.Lock()
	h.index++
	currentIndex := h.index
	h.mu.Unlock()

	entries := make([]*ServiceEntry, 0, len(edenInstances))
	for _, ei := range edenInstances {
		weight := ei.Weight
		if weight <= 0 {
			weight = 1
		}
		entries = append(entries, &ServiceEntry{
			Node: &Node{
				ID:         ei.ID,
				Node:       ei.ID,
				Address:    ei.Host,
				Datacenter: ei.Datacenter,
			},
			Service: &AgentService{
				ID:      ei.ID,
				Service: ei.ServiceName,
				Address: ei.Host,
				Port:    ei.Port,
				Meta:    ei.Metadata,
				Weights: AgentWeights{Passing: weight},
			},
			Checks: HealthChecks{
				{
					CheckID:   "service:" + ei.ID,
					Status:    ei.Status,
					ServiceID: ei.ID,
				},
			},
		})
	}

	return entries, &QueryMeta{LastIndex: currentIndex}, nil
}

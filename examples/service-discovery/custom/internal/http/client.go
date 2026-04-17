package httpclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"strings"
	"time"

	"github.com/shiyindaxiaojie/eden-registry/examples/service-discovery/custom/internal/registry"
)

type Client struct {
	targets []string

	namespace       string
	datacenter      string
	apiKey          string
	consumerService string

	httpClient *http.Client
}

func NewFromEnv() (registry.Registry, error) {
	targets := normalizeTargets(splitAddrs(envOr("CUSTOM_HTTP_ADDRS", "http://127.0.0.1:8500")))
	if len(targets) == 0 {
		return nil, fmt.Errorf("CUSTOM_HTTP_ADDRS is empty")
	}

	return &Client{
		targets:    targets,
		namespace:  envOr("CUSTOM_NAMESPACE", "default"),
		datacenter: envOr("CUSTOM_DATACENTER", "dc1"),
		apiKey:     envOr("CUSTOM_API_KEY", ""),
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}, nil
}

func (c *Client) Register(instance *registry.ServiceInstance) error {
	c.consumerService = instance.ServiceName
	body := map[string]interface{}{
		"id":           instance.ID,
		"service_name": instance.ServiceName,
		"namespace":    c.namespace,
		"host":         instance.Host,
		"port":         instance.Port,
		"weight":       instance.Weight,
		"datacenter":   c.datacenter,
		"metadata":     instance.Metadata,
	}
	return c.postJSON("/v1/catalog/register", body)
}

func (c *Client) Deregister(instance *registry.ServiceInstance) error {
	body := map[string]string{
		"namespace":    c.namespace,
		"service_name": instance.ServiceName,
		"instance_id":  instance.ID,
		"status":       "offline",
	}
	return c.postJSON("/v1/catalog/instance/status", body)
}

func (c *Client) Discovery(serviceName string) ([]*registry.ServiceInstance, error) {
	query := neturl.Values{}
	query.Set("passing", "true")
	if c.namespace != "" {
		query.Set("namespace", c.namespace)
	}
	if c.datacenter != "" {
		query.Set("dc", c.datacenter)
	}
	if c.consumerService != "" {
		query.Set("consumer_service", c.consumerService)
	}

	body, err := c.get("/v1/catalog/service/" + neturl.PathEscape(serviceName) + "?" + query.Encode())
	if err != nil {
		return nil, err
	}

	var items []*registry.ServiceInstance
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *Client) Subscribe(serviceName string, callback func([]*registry.ServiceInstance)) error {
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		lastHash := ""
		for range ticker.C {
			items, err := c.Discovery(serviceName)
			if err != nil {
				continue
			}
			hash := snapshot(items)
			if hash != lastHash {
				lastHash = hash
				callback(items)
			}
		}
	}()
	return nil
}

func (c *Client) Heartbeat(instance *registry.ServiceInstance) error {
	body := map[string]string{
		"namespace":    c.namespace,
		"service_name": instance.ServiceName,
		"instance_id":  instance.ID,
	}
	err := c.postJSON("/v1/catalog/heartbeat", body)
	if err != nil && isInstanceNotFound(err) {
		return c.Register(instance)
	}
	return err
}

func (c *Client) Close() error {
	return nil
}

func (c *Client) postJSON(path string, payload interface{}) error {
	data, _ := json.Marshal(payload)

	var lastErr error
	for _, target := range c.targets {
		req, err := http.NewRequest(http.MethodPost, target+path, bytes.NewReader(data))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			req.Header.Set("X-API-Key", c.apiKey)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
			continue
		}
		return nil
	}
	return lastErr
}

func (c *Client) get(path string) ([]byte, error) {
	var lastErr error
	for _, target := range c.targets {
		req, err := http.NewRequest(http.MethodGet, target+path, nil)
		if err != nil {
			lastErr = err
			continue
		}
		if c.apiKey != "" {
			req.Header.Set("X-API-Key", c.apiKey)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
			continue
		}
		return body, nil
	}
	return nil, lastErr
}

func snapshot(items []*registry.ServiceInstance) string {
	data, _ := json.Marshal(items)
	return string(data)
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

func isInstanceNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "not found")
}

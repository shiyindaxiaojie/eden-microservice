package registryhttp

import (
	"os"
	"strings"

	"github.com/shiyindaxiaojie/eden-go-registry/pkg/eden"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/registry"
)

func NewFromEnv() (registry.Registry, error) {
	return eden.NewWithConfig(&eden.Config{
		Addresses:     splitAddrs(envOr("EDEN_HTTP_ADDRS", "http://127.0.0.1:8500")),
		APIKey:        envOr("EDEN_API_KEY", ""),
		Namespace:     envOr("EDEN_NAMESPACE", "default"),
		Datacenter:    envOr("EDEN_DATACENTER", "dc1"),
		Transport:     "http",
		DiscoveryMode: strings.ToLower(strings.TrimSpace(envOr("EDEN_DISCOVERY_MODE", "auto"))),
	})
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

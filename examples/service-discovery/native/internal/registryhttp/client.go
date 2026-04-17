package registryhttp

import (
	"os"
	"strings"

	"github.com/shiyindaxiaojie/eden-registry/pkg/sdk"
)

func NewFromEnv() (sdk.Registry, error) {
	return sdk.NewWithConfig(&sdk.Config{
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

package registrygrpc

import (
	"os"
	"strings"

	"github.com/shiyindaxiaojie/eden-registry/pkg/sdk"
)

func NewFromEnv() (sdk.Registry, error) {
	discoveryMode := strings.ToLower(strings.TrimSpace(envOr("EDEN_DISCOVERY_MODE", "auto")))
	if discoveryMode == "" {
		discoveryMode = "auto"
	}

	return sdk.NewWithConfig(&sdk.Config{
		Addresses:     splitAddrs(envOr("EDEN_GRPC_ADDRS", "127.0.0.1:9000")),
		APIKey:        envOr("EDEN_API_KEY", ""),
		Namespace:     envOr("EDEN_NAMESPACE", "default"),
		Datacenter:    envOr("EDEN_DATACENTER", "dc1"),
		Transport:     "grpc",
		DiscoveryMode: discoveryMode,
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

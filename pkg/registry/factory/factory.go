package factory

import (
	"fmt"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/consul"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/eden"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/nacos"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/registry"
)

// NewRegistry creates a new registry client based on the configuration.
func NewRegistry(cfg *registry.Config) (registry.Registry, error) {
	switch cfg.Type {
	case "eden":
		return eden.NewWithConfig(&eden.Config{
			Addresses:  cfg.Addresses,
			APIKey:     cfg.APIKey,
			Datacenter: cfg.Datacenter,
			Transport:  "grpc", // default to grpc for eden
		})
	case "consul":
		return consul.NewRegistry(cfg)
	case "nacos":
		return nacos.NewRegistry(cfg)
	default:
		return nil, fmt.Errorf("registry: unknown type: %s", cfg.Type)
	}
}

// NewEdenClient is a convenience function to create an Eden native client.
func NewEdenClient(addresses []string, apiKey, datacenter string) (*eden.Client, error) {
	return eden.New(addresses, apiKey, datacenter)
}

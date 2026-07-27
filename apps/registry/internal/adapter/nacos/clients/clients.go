// Package clients is a drop-in replacement for
// github.com/nacos-group/nacos-sdk-go/v2/clients.
// It creates naming clients that talk to the local registry instead of Nacos.
//
// Usage in existing Nacos-based code:
//
//	// Before:
//	// import "github.com/nacos-group/nacos-sdk-go/v2/clients"
//
//	// After (only change the import):
//	import "eden-microservice/apps/registry/internal/adapter/nacos/clients"
package clients

import (
	"eden-microservice/apps/registry/internal/adapter/nacos/clients/naming_client"
	"eden-microservice/apps/registry/internal/adapter/nacos/vo"
)

// NewNamingClient creates a new naming client backed by the local registry.
func NewNamingClient(param vo.NacosClientParam) (naming_client.INamingClient, error) {
	return naming_client.NewNamingClient(param)
}

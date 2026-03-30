// Package clients is a drop-in replacement for
// github.com/nacos-group/nacos-sdk-go/v2/clients.
// It creates naming clients that talk to Eden Registry instead of Nacos.
//
// Usage in existing Nacos-based code:
//
//	// Before:
//	// import "github.com/nacos-group/nacos-sdk-go/v2/clients"
//
//	// After (only change the import):
//	import "github.com/shiyindaxiaojie/eden-go-registry/internal/adapter/nacos/clients"
package clients

import (
	"github.com/shiyindaxiaojie/eden-go-registry/internal/adapter/nacos/clients/naming_client"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/adapter/nacos/vo"
)

// NewNamingClient creates a new naming client backed by Eden Registry.
func NewNamingClient(param vo.NacosClientParam) (naming_client.INamingClient, error) {
	return naming_client.NewNamingClient(param)
}

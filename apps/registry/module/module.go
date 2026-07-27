// Package module is the public composition surface for the registry module.
package module

import (
	"eden-microservice/apps/registry/api/catalog"
	consuladapter "eden-microservice/apps/registry/internal/adapter/consul"
	nacosadapter "eden-microservice/apps/registry/internal/adapter/nacos"
)

type (
	ConsulHTTPAdapter = consuladapter.HTTPAdapter
	NacosHTTPAdapter  = nacosadapter.HTTPAdapter
	NacosNamingServer = nacosadapter.NacosNamingServer
)

type Container struct {
	State   *catalog.State
	Service catalog.Registry
}

func NewContainer(dataPath string, mode catalog.ModeReader, cp catalog.CPNode, ap catalog.APNode) *Container {
	state := catalog.NewState(dataPath)
	return &Container{State: state, Service: catalog.NewRegistry(state, mode, cp, ap)}
}

var (
	NewConsulHTTPAdapter = consuladapter.NewHTTPAdapter
	NewNacosHTTPAdapter  = nacosadapter.NewHTTPAdapter
	NewNacosNamingServer = nacosadapter.NewNacosNamingServer
)

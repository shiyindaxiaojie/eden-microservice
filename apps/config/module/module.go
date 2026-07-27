// Package module is the public composition surface for the configuration center.
package module

import (
	api "eden-microservice/apps/config/api"
	nacosadapter "eden-microservice/apps/config/internal/adapter/nacos"
)

type NacosHTTPAdapter = nacosadapter.ConfigHTTPAdapter

type Container struct {
	Service api.Service
}

func NewContainer(dataPath string) (*Container, error) {
	service, err := api.Open(dataPath)
	if err != nil {
		return nil, err
	}
	return &Container{Service: service}, nil
}

var NewNacosHTTPAdapter = nacosadapter.NewConfigHTTPAdapter

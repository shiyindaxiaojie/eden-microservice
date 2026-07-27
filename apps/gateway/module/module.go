// Package module is the public composition surface for gateway routing.
package module

import api "eden-microservice/apps/gateway/api"

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

var NewRuntime = api.NewRuntime

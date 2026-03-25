package main

import (
	"github.com/shiyindaxiaojie/eden-go-registry/examples/service-discovery/consul/internal/app"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/consul"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/registry"
)

func main() {
	reg, err := consul.NewRegistry(&registry.Config{
		Addresses:  []string{app.EnvOr("CONSUL_ADDR", "127.0.0.1:8500")},
		APIKey:     app.EnvOr("CONSUL_API_KEY", ""),
		Datacenter: app.EnvOr("CONSUL_DATACENTER", "dc1"),
	})
	if err != nil {
		panic(err)
	}

	if err := app.RunAuthCenter(app.ServiceConfig{
		Title:           "Consul Auth Center",
		Integration:     "pkg/consul",
		Transport:       "http-compat",
		ServiceName:     "consul-auth-center",
		ServiceID:       app.EnvOr("SERVICE_ID", "consul-auth-center-1"),
		Host:            app.EnvOr("SERVICE_HOST", "127.0.0.1"),
		Port:            app.Atoi(app.EnvOr("SERVICE_PORT", "22002")),
		Registry:        reg,
		UserServiceName: "consul-user-center",
	}); err != nil {
		panic(err)
	}
}

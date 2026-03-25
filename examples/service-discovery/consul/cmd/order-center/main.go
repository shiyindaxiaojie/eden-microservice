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

	if err := app.RunOrderCenter(app.ServiceConfig{
		Title:           "Consul Order Center",
		Integration:     "pkg/consul",
		Transport:       "http-compat",
		ServiceName:     "consul-order-center",
		ServiceID:       app.EnvOr("SERVICE_ID", "consul-order-center-1"),
		Host:            app.EnvOr("SERVICE_HOST", "127.0.0.1"),
		Port:            app.Atoi(app.EnvOr("SERVICE_PORT", "22003")),
		Registry:        reg,
		UserServiceName: "consul-user-center",
		AuthServiceName: "consul-auth-center",
	}); err != nil {
		panic(err)
	}
}

package main

import (
	"github.com/shiyindaxiaojie/eden-go-registry/examples/service-discovery/eden/internal/app"
	"github.com/shiyindaxiaojie/eden-go-registry/examples/service-discovery/eden/internal/registryhttp"
)

func main() {
	reg, err := registryhttp.NewFromEnv()
	if err != nil {
		panic(err)
	}

	if err := app.RunAuthCenter(app.ServiceConfig{
		Title:           "Eden HTTP Auth Center",
		Integration:     "pkg/eden",
		Transport:       "http",
		ServiceName:     "eden-http-auth-center",
		ServiceID:       app.EnvOr("SERVICE_ID", "eden-http-auth-center-1"),
		Host:            app.EnvOr("SERVICE_HOST", "127.0.0.1"),
		Port:            app.Atoi(app.EnvOr("SERVICE_PORT", "21102")),
		Registry:        reg,
		UserServiceName: "eden-http-user-center",
	}); err != nil {
		panic(err)
	}
}

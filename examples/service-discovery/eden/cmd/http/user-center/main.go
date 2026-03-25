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

	if err := app.RunUserCenter(app.ServiceConfig{
		Title:           "Eden HTTP User Center",
		Integration:     "pkg/eden",
		Transport:       "http",
		ServiceName:     "eden-http-user-center",
		ServiceID:       app.EnvOr("SERVICE_ID", "eden-http-user-center-1"),
		Host:            app.EnvOr("SERVICE_HOST", "127.0.0.1"),
		Port:            app.Atoi(app.EnvOr("SERVICE_PORT", "21101")),
		Registry:        reg,
		AuthServiceName: "eden-http-auth-center",
	}); err != nil {
		panic(err)
	}
}

package main

import (
	"github.com/shiyindaxiaojie/eden-go-registry/examples/service-discovery/custom/internal/app"
	"github.com/shiyindaxiaojie/eden-go-registry/examples/service-discovery/custom/internal/httpclient"
)

func main() {
	reg, err := httpclient.NewFromEnv()
	if err != nil {
		panic(err)
	}

	if err := app.RunAuthCenter(app.ServiceConfig{
		Title:           "Custom HTTP Auth Center",
		Integration:     "custom",
		Transport:       "http",
		ServiceName:     "custom-http-auth-center",
		ServiceID:       app.EnvOr("SERVICE_ID", "custom-http-auth-center-1"),
		Host:            app.EnvOr("SERVICE_HOST", "127.0.0.1"),
		Port:            app.Atoi(app.EnvOr("SERVICE_PORT", "24102")),
		Registry:        reg,
		UserServiceName: "custom-http-user-center",
	}); err != nil {
		panic(err)
	}
}

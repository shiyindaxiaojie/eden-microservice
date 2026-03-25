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

	if err := app.RunUserCenter(app.ServiceConfig{
		Title:           "Custom HTTP User Center",
		Integration:     "custom",
		Transport:       "http",
		ServiceName:     "custom-http-user-center",
		ServiceID:       app.EnvOr("SERVICE_ID", "custom-http-user-center-1"),
		Host:            app.EnvOr("SERVICE_HOST", "127.0.0.1"),
		Port:            app.Atoi(app.EnvOr("SERVICE_PORT", "24101")),
		Registry:        reg,
		AuthServiceName: "custom-http-auth-center",
	}); err != nil {
		panic(err)
	}
}

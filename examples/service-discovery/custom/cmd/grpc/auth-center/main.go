package main

import (
	"github.com/shiyindaxiaojie/eden-go-registry/examples/service-discovery/custom/internal/app"
	"github.com/shiyindaxiaojie/eden-go-registry/examples/service-discovery/custom/internal/grpcclient"
)

func main() {
	reg, err := grpcclient.NewFromEnv()
	if err != nil {
		panic(err)
	}

	if err := app.RunAuthCenter(app.ServiceConfig{
		Title:           "Custom gRPC Auth Center",
		Integration:     "custom",
		Transport:       "grpc",
		ServiceName:     "custom-grpc-auth-center",
		ServiceID:       app.EnvOr("SERVICE_ID", "custom-grpc-auth-center-1"),
		Host:            app.EnvOr("SERVICE_HOST", "127.0.0.1"),
		Port:            app.Atoi(app.EnvOr("SERVICE_PORT", "24002")),
		Registry:        reg,
		UserServiceName: "custom-grpc-user-center",
	}); err != nil {
		panic(err)
	}
}

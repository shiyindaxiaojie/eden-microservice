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

	if err := app.RunOrderCenter(app.ServiceConfig{
		Title:           "Custom gRPC Order Center",
		Integration:     "custom",
		Transport:       "grpc",
		ServiceName:     "custom-grpc-order-center",
		ServiceID:       app.EnvOr("SERVICE_ID", "custom-grpc-order-center-1"),
		Host:            app.EnvOr("SERVICE_HOST", "127.0.0.1"),
		Port:            app.Atoi(app.EnvOr("SERVICE_PORT", "24003")),
		Registry:        reg,
		UserServiceName: "custom-grpc-user-center",
		AuthServiceName: "custom-grpc-auth-center",
	}); err != nil {
		panic(err)
	}
}

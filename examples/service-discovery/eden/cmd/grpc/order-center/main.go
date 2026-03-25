package main

import (
	"github.com/shiyindaxiaojie/eden-go-registry/examples/service-discovery/eden/internal/app"
	"github.com/shiyindaxiaojie/eden-go-registry/examples/service-discovery/eden/internal/registrygrpc"
)

func main() {
	reg, err := registrygrpc.NewFromEnv()
	if err != nil {
		panic(err)
	}

	if err := app.RunOrderCenter(app.ServiceConfig{
		Title:           "Eden gRPC Order Center",
		Integration:     "pkg/eden",
		Transport:       "grpc",
		ServiceName:     "eden-grpc-order-center",
		ServiceID:       app.EnvOr("SERVICE_ID", "eden-grpc-order-center-1"),
		Host:            app.EnvOr("SERVICE_HOST", "127.0.0.1"),
		Port:            app.Atoi(app.EnvOr("SERVICE_PORT", "21003")),
		Registry:        reg,
		UserServiceName: "eden-grpc-user-center",
		AuthServiceName: "eden-grpc-auth-center",
	}); err != nil {
		panic(err)
	}
}

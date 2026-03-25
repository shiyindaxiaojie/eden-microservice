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

	if err := app.RunUserCenter(app.ServiceConfig{
		Title:           "Eden gRPC User Center",
		Integration:     "pkg/eden",
		Transport:       "grpc",
		ServiceName:     "eden-grpc-user-center",
		ServiceID:       app.EnvOr("SERVICE_ID", "eden-grpc-user-center-1"),
		Host:            app.EnvOr("SERVICE_HOST", "127.0.0.1"),
		Port:            app.Atoi(app.EnvOr("SERVICE_PORT", "21001")),
		Registry:        reg,
		AuthServiceName: "eden-grpc-auth-center",
	}); err != nil {
		panic(err)
	}
}

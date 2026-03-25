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

	if err := app.RunUserCenter(app.ServiceConfig{
		Title:           "Custom gRPC User Center",
		Integration:     "custom",
		Transport:       "grpc",
		ServiceName:     "custom-grpc-user-center",
		ServiceID:       app.EnvOr("SERVICE_ID", "custom-grpc-user-center-1"),
		Host:            app.EnvOr("SERVICE_HOST", "127.0.0.1"),
		Port:            app.Atoi(app.EnvOr("SERVICE_PORT", "24001")),
		Registry:        reg,
		AuthServiceName: "custom-grpc-auth-center",
	}); err != nil {
		panic(err)
	}
}

module eden-microservice/apps/gateway

go 1.25.8

require (
	eden-microservice/apps/registry v0.0.0
	github.com/shiyindaxiaojie/eden-go-logger v1.0.2
	go.etcd.io/bbolt v1.3.5
)

require (
	eden-microservice/packages v0.0.0 // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/grpc v1.79.2 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace (
	eden-microservice/apps/registry => ../registry
	eden-microservice/packages => ../../packages
)

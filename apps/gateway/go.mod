module eden-microservice/apps/gateway

go 1.25.8

require (
	eden-microservice/apps/registry v0.0.0
	github.com/shiyindaxiaojie/eden-go-logger v1.0.2
	go.etcd.io/bbolt v1.3.5
)

require (
	eden-microservice/packages v0.0.0 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260414002931-afd174a4e478 // indirect
	google.golang.org/grpc v1.82.1 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace (
	eden-microservice/apps/registry => ../registry
	eden-microservice/packages => ../../packages
)

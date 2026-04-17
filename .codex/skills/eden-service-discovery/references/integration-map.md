# Integration Map

## Main example families

- `examples/service-discovery/eden`: native `pkg/eden` integration
- `examples/service-discovery/consul`: official Consul SDK against Eden compatibility endpoints
- `examples/service-discovery/nacos`: official Nacos SDK against Eden compatibility endpoints
- `examples/service-discovery/custom`: protocol-level integration with local proto/assets

## Core code paths

- `pkg/eden/client.go`: native client behavior
- `internal/catalog/*`: registration, discovery, heartbeat, events, topology
- `internal/transport/http/*`: HTTP endpoints
- `internal/transport/rpc/registry_server.go`: gRPC registry surface
- `internal/adapter/consul/*`: Consul-compatible behavior
- `internal/adapter/nacos/*`: Nacos-compatible behavior
- `api/proto/registry/v1/registry.proto`: native gRPC contract

## Useful expectations from repo docs

- HTTP registration path: `POST /v1/catalog/register`
- HTTP heartbeat path: `POST /v1/catalog/heartbeat`
- HTTP discovery path: `GET /v1/catalog/service/{name}`
- gRPC service: `eden.registry.v1.RegistryService`

## Example selection

- Choose `eden` for native SDK work.
- Choose `consul` or `nacos` when validating compatibility promises.
- Choose `custom` when checking the public protocol surface seen by external projects.

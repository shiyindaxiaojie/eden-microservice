# Focalors Integration Guide

## Integration Policy

For Go services, the recommended entry point is:

- [`pkg/sdk`](../pkg/sdk)

Other supported paths remain available for specific constraints:

- native HTTP API
- native gRPC API
- Consul-compatible access
- Nacos-compatible access

For new projects, start with `pkg/sdk` instead of the compatibility adapters.

## Go SDK

### Recommended Mode: gRPC

```go
client, err := sdk.NewWithConfig(&sdk.Config{
    Addresses:     []string{"127.0.0.1:9000", "127.0.0.1:9001"},
    Namespace:     "default",
    Datacenter:    "dc1",
    Transport:     "grpc",
    DiscoveryMode: "auto",
})
if err != nil {
    panic(err)
}
defer client.Close()
```

### HTTP Mode

```go
client, err := sdk.NewWithConfig(&sdk.Config{
    Addresses:  []string{"http://127.0.0.1:8500"},
    Namespace:  "default",
    Datacenter: "dc1",
    Transport:  "http",
})
```

Use this when the environment cannot use gRPC and only basic registry operations are required.

### QUIC Mode

```go
client, err := sdk.NewWithConfig(&sdk.Config{
    Addresses:     []string{"127.0.0.1:10000"},
    Namespace:     "default",
    Datacenter:    "dc1",
    Transport:     "quic",
    DiscoveryMode: "static",
})
```

Use this when gRPC semantics are still desired but the network path benefits from QUIC transport.

## Native HTTP API

Common endpoints:

- `POST /v1/catalog/register`
- `POST /v1/catalog/heartbeat`
- `GET /v1/catalog/service/{name}`
- `POST /v1/catalog/instance/status`
- `POST /v1/catalog/topology/report`

## Native gRPC API

- Proto: [`api/proto/registry/v1/registry.proto`](../api/proto/registry/v1/registry.proto)
- Service: `eden.registry.v1.RegistryService`

Core RPCs:

- `Register`
- `Heartbeat`
- `Discover`
- `Watch`
- `GetMembers`
- `ReportTopology`

## Compatibility Adapters

### Consul Compatibility

Use this when existing services already depend on `github.com/hashicorp/consul/api` and the short-term goal is to switch the registry backend with minimal code changes.

Constraints:

- this covers registry and discovery migration, not the full Consul product surface
- KV, service mesh, ACL, and other broader Consul capabilities must be evaluated separately

References:

- [Consul example](../examples/service-discovery/consul/README.md)
- [Service discovery example index](../examples/service-discovery/README.md)

### Nacos Compatibility

Use this when existing services already depend on `github.com/nacos-group/nacos-sdk-go/v2` naming APIs and only the registry backend should change.

Constraints:

- this covers naming migration, not Nacos configuration management
- Nacos Config migration should be treated as a separate workstream

References:

- [Nacos example](../examples/service-discovery/nacos/README.md)
- [Service discovery example index](../examples/service-discovery/README.md)

### Custom Protocol Access

Use this when an external project integrates only through the published HTTP or gRPC contracts.

Recommended reading order:

1. [Service discovery examples](../examples/service-discovery/README.md)
2. [Custom protocol example](../examples/service-discovery/custom/README.md)
3. [`api/proto/registry/v1/registry.proto`](../api/proto/registry/v1/registry.proto)

## Selection Guidance

| Scenario | Recommended path |
| --- | --- |
| new Go services | `pkg/sdk + grpc` |
| Go services with network constraints | `pkg/sdk + http` or `pkg/sdk + quic` |
| non-Go services | gRPC first, HTTP second |
| stock system migration | Consul or Nacos compatibility |

## Related Reading

- [Deployment](./deployment.md)
- [Architecture](./architecture.md)
- [Simplified Chinese integration](./integration_zh-CN.md)

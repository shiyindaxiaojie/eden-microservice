# Service Discovery Examples

`examples/service-discovery` contains four end-to-end example sets that all model the same microservice topology:

- `auth-center`
- `user-center`
- `order-center`

Each example set starts 2-3 instances per service and exposes HTTP endpoints so that registration, discovery, subscription, and service-to-service calls can be verified directly.

## Example Sets

- `native`
  Uses the native `pkg/sdk` client and demonstrates direct registry service discovery integration.
- `consul`
  Uses the official `github.com/hashicorp/consul/api` client against the registry's Consul-compatible HTTP endpoints. The intended switch path is to change only the registry address.
- `nacos`
  Uses the official `github.com/nacos-group/nacos-sdk-go/v2` naming client against the registry's Nacos-compatible HTTP and gRPC endpoints. The intended switch path is to change only the registry address.
- `custom`
  Treats the demo as an external project. It does not depend on the repository's `pkg/registry` or root proto package; instead, it integrates only through the published HTTP/gRPC protocols and keeps its own local protocol assets under `custom/internal`.

## Common Layout

- Every example set is self-contained.
- Each service keeps its integration flow in its own `main.go`.
- Startup scripts live inside each example directory.
- There is no shared helper under `examples/service-discovery/internal`.

## Which Example To Read

- Start with `native` if you want the native `pkg/sdk` integration path.
- Read `consul` if you want the "official Consul SDK, only change address" integration path.
- Read `nacos` if you want the "official Nacos SDK, only change address" integration path.
- Read `custom` if you want to see what an external project would look like when integrating only at the protocol level.

## Example Index

- [native](./native/README.md)
- [consul](./consul/README.md)
- [nacos](./nacos/README.md)
- [custom](./custom/README.md)
- [Simplified Chinese](./README-zh-CN.md)


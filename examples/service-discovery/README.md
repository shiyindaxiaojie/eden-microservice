# Service Discovery Examples

`examples/service-discovery` contains four end-to-end example sets that all model the same microservice topology:

- `auth-center`
- `user-center`
- `order-center`

Each example set starts 2-3 instances per service and exposes HTTP endpoints so that registration, discovery, subscription, and service-to-service calls can be verified directly.

## Example Sets

- `eden`
  Uses the native Eden client and demonstrates direct Eden service discovery integration.
- `consul`
  Uses the Consul-compatible adapter and demonstrates integration through the unified registry interface.
- `nacos`
  Uses the Nacos-compatible adapter and demonstrates integration through the unified registry interface.
- `custom`
  Treats the demo as an external project. It does not depend on the repository's `pkg/registry` or root proto package; instead, it integrates only through the published HTTP/gRPC protocols and keeps its own local protocol assets under `custom/internal`.

## Common Layout

- Every example set is self-contained.
- Each service keeps its integration flow in its own `main.go`.
- Startup scripts live inside each example directory.
- There is no shared helper under `examples/service-discovery/internal`.

## Which Example To Read

- Start with `eden` if you want the native Eden client integration path.
- Read `consul` or `nacos` if you want adapter-style integration examples.
- Read `custom` if you want to see what an external project would look like when integrating only at the protocol level.

## Example Index

- [eden](./eden/README.md)
- [consul](./consul/README.md)
- [nacos](./nacos/README.md)
- [custom](./custom/README.md)

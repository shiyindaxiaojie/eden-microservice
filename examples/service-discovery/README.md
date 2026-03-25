# Service Discovery Examples

`examples/service-discovery` now contains four fully independent example sets:

- `eden`
- `consul`
- `nacos`
- `custom`

Each directory contains its own service code, startup script, and README. There is no shared example helper under `examples/service-discovery/internal`.

Each example set runs the same three services:

- `auth-center`
- `user-center`
- `order-center`

Each set starts 2-3 instances per service and exposes HTTP endpoints so that service-to-service invocation can be tested directly.

## Example Index

- [eden](D:/Workspaces/Git/eden-go-registry/examples/service-discovery/eden/README.md)
- [consul](D:/Workspaces/Git/eden-go-registry/examples/service-discovery/consul/README.md)
- [nacos](D:/Workspaces/Git/eden-go-registry/examples/service-discovery/nacos/README.md)
- [custom](D:/Workspaces/Git/eden-go-registry/examples/service-discovery/custom/README.md)

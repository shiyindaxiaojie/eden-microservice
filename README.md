# Focalors

English | [中文](README-zh-CN.md)

Focalors is a lightweight service registry for production environments. It focuses on registration, discovery, health checks, topology, and governance, and is intended for systems that only need registry capabilities, do not want to introduce a configuration center or service mesh, or run in memory-constrained environments.

## Demo Screenshots

Services
![](https://cdn.jsdelivr.net/gh/shiyindaxiaojie/cdn/eden-registry/services.png)

Topology
![](https://cdn.jsdelivr.net/gh/shiyindaxiaojie/cdn/eden-registry/topology.png)

Clusters
![](https://cdn.jsdelivr.net/gh/shiyindaxiaojie/cdn/eden-registry/clusters.png)

Settings
![](https://cdn.jsdelivr.net/gh/shiyindaxiaojie/cdn/eden-registry/settings.png)

## Product Positioning

Focalors targets registry-only scenarios. It covers registration, discovery, health control, topology, governance, and AP / CP cluster coordination, while providing compatible access for Nacos and Consul APIs.

<table align="right" width="38%">
  <tr>
    <td>

**Applicable Scenarios**

- Registry-only deployments that do not require a separate configuration center or service mesh.
- Environments that require both `AP` high availability and `CP` consistency management within one runtime model.
- Memory-constrained environments where the available runtime budget remains below `100MB`.

    </td>
  </tr>
</table>

| Capability | Focalors | Nacos | Consul |
| --- | --- | --- | --- |
| Service registration and discovery | ✓ | ✓ | ✓ |
| Service health checks | ✓ | ✓ | ✓ |
| Instance online / offline control | ✓ | ✓ | ✓ |
| Service dependency topology | ✓ | ✗ | ✗ |
| AP consistency | ✓ | ✓ | ✗ |
| CP consistency | ✓ | ✗ | ✓ |
| AP / CP switching | ✓ | ✗ | ✗ |
| RBAC control | ✓ | ✓ | ✓ |
| Namespace isolation | ✓ | ✓ | ✓ (paid) |
| Weak-network transport | ✓ | ✗ | ✗ |
| Event storage | ✓ | ✗ | ✗ |
| Memory footprint | Low | High | Medium |

## Architecture / Runtime Flow

### Architecture

```mermaid
---
title: Focalors Service Registry Architecture
---
flowchart TB
    subgraph ActiveClients["Clients"]
        direction LR
        SDK["Focalors Client"]
        NACOS["Nacos Client"]
        CONSUL["Consul Client"]
        CUSTOM["Custom Client"]
        CONSOLE["Focalors Console"]
    end

    subgraph REGISTRY["Registry"]
        direction LR
        ACCESS["Access Layer"]
        CONTROL["Control Plane"]
        CATALOG["Registry / Discovery"]
        CLUSTER["Cluster Management"]
    end

    subgraph PeerNodes["Cluster Nodes"]
        direction TB
        N1["Node 1"]
        N2["Node 2"]
        N3["Node 3"]
        N1 <-->|gRPC / Raft| N2
        N1 <-->|gRPC / Raft| N3
        N2 <-->|gRPC / Raft| N3
    end

    subgraph PersistentStore["Storage"]
        direction LR
        RBAC["RBAC"]
        NS["Namespaces"]
        EVENT["Events"]
        LOG["Logs"]
        ALERT["Alerts"]
        NOTICE["Notifications"]
        RAFT["Raft Log / Snapshot"]
    end

    SDK -->|gRPC| ACCESS
    NACOS -->|Nacos gRPC / HTTP| ACCESS
    CONSUL -->|Consul HTTP| ACCESS
    CUSTOM -->|gRPC / HTTP| ACCESS
    CONSOLE -->|HTTP| CONTROL

    ACCESS --> CATALOG
    ACCESS --> CLUSTER

    CLUSTER ---|gRPC / Raft| N1

    CONTROL --> RBAC
    CONTROL --> NS
    CONTROL --> EVENT
    CONTROL --> LOG
    CONTROL --> ALERT
    CONTROL --> NOTICE
    CLUSTER -->|CP persistence| RAFT
```

- `Access Layer`: protocol adaptation and request routing for gRPC, HTTP, QUIC, and Nacos / Consul compatibility.
- `Registry / Discovery`: service registration, discovery, health state, namespaces, and topology.
- `Cluster Management`: gRPC replication in AP mode and Raft consensus in CP mode.
- `Control Plane`: authentication, authorization, settings, alerts, and notifications.
- `Storage`: events, logs, and Raft log / snapshot persistence in CP mode.

### Runtime Flow

```mermaid
sequenceDiagram
    autonumber

    participant C as Client
    participant T as Access Layer
    participant F as Node Domain
    participant R as Cluster Sync
    participant D as Persistent Storage

    Note over C,D: Scenario 1: Register / Heartbeat
    C->>T: Send register request
    alt Focalors SDK
        T->>F: gRPC by default, QUIC / HTTP fallback when needed
    else Nacos / Consul client
        T->>F: Compatibility protocol access
    else Custom client
        T->>F: Direct gRPC / HTTP call
    end
    F->>R: Choose standalone / AP / CP synchronization path
    alt AP mode
        R->>R: Sync instance catalog / seeds / settings through gRPC
    else CP mode
        R->>R: Sync critical metadata through Raft TCP
    end
    F->>D: Persist config / events / logs
    R->>D: Persist Raft log / snapshot
    D-->>F: Return write result
    F-->>T: Return register result
    T-->>C: Success response

    Note over C,D: Scenario 2: Service Discovery
    C->>T: Query service
    T->>F: Route to registry / discovery logic
    F->>D: Read instances / config / event summary
    D-->>F: Return query result
    F-->>T: Assemble protocol response
    T-->>C: Return discovery result
```

## Quick Start

Start the server:

```bash
go run ./cmd/server/main.go
```

Default API address:

```text
http://127.0.0.1:8500
```

Specify a configuration file explicitly:

```bash
go run ./cmd/server/main.go -config config/config.yaml.example
```

Run tests:

```bash
go test ./...
```

## Deployment Modes

| Mode | Key configuration | Best fit |
| --- | --- | --- |
| `standalone` | `mode: "standalone"` | local development, testing, fast validation |
| `cluster + ap` | `mode: "cluster"` + `consistency: "ap"` | availability-first production environments |
| `cluster + cp` | `mode: "cluster"` + `consistency: "cp"` | consistency-first production environments with leader-based writes |

Standalone example:

```yaml
mode: "standalone"
consistency: "ap"
server:
  http: ":8500"
  grpc: "auto"
  quic: "off"
  raft: "off"
```

`grpc: "auto"` selects the first available port in the `9000-9999` range, so a local single-node startup uses `127.0.0.1:9000` unless that port is already occupied.

AP cluster example:

```yaml
mode: "cluster"
consistency: "ap"
server:
  http: ":8500"
  grpc: "auto"
  quic: "off"
  raft: "off"
```

When multiple AP nodes run on the same host, `grpc: "auto"` continues through `9000-9999`, typically producing `:9000`, `:9001`, `:9002`, and so on.

CP cluster example:

```yaml
mode: "cluster"
consistency: "cp"
bootstrap: true
server:
  http: ":8500"
  grpc: ":9000"
  raft: "127.0.0.1:7000"
```

For full deployment details, see [Deployment Guide](./docs/deployment.md).

## Client Integration

| Integration path | Best fit | Example |
| --- | --- | --- |
| Focalors SDK | Go services, primary integration path | [Native integration example](./examples/service-discovery/native/README.md) |
| Nacos compatibility | Existing Nacos Naming systems with minimal business code changes | [Nacos migration example](./examples/service-discovery/nacos/README.md) |
| Consul compatibility | Existing Consul HTTP / SDK systems while keeping the original call model | [Consul migration example](./examples/service-discovery/consul/README.md) |
| Custom gRPC / HTTP | External systems that integrate directly through public protocols | [Custom protocol example](./examples/service-discovery/custom/README.md) |

## Development Guide

When developing, first identify which layer you need to change. The main path is: `cmd/server` for bootstrap and composition, `internal` for server implementation, `pkg/sdk` for the public Go SDK, and `examples` for integration and migration validation.

| Task | Entry directory |
| --- | --- |
| Server bootstrap and runtime composition | `cmd/server` |
| Registration, discovery, health, topology core logic | `internal/catalog` |
| AP / CP cluster runtime | `internal/cluster` |
| HTTP / gRPC / QUIC interfaces | `internal/transport` |
| Nacos / Consul compatibility adapters | `internal/adapter` |
| Authentication, authorization, system settings | `internal/auth`, `internal/settings` |
| Alerts and notifications | `internal/alert`, `internal/notify` |
| Public Go SDK | `pkg/sdk` |
| Protocol definition | `api/proto` |
| Integration and migration validation | `examples` |

Repository structure:

```text
eden-registry
├─ cmd
│  └─ server                 # server bootstrap and runtime composition
├─ api
│  └─ proto                  # gRPC / protobuf contracts
├─ internal
│  ├─ catalog                # registration, discovery, health, topology core logic
│  ├─ cluster                # AP / CP cluster runtime
│  ├─ transport
│  │  ├─ http                # native HTTP interfaces
│  │  ├─ rpc                 # gRPC interfaces
│  │  └─ quic                # QUIC transport entry
│  ├─ adapter                # Nacos / Consul compatibility adapters
│  ├─ auth                   # authentication, users, API keys
│  ├─ settings               # system settings and runtime control
│  ├─ alert                  # alert rules and event evaluation
│  └─ notify                 # notification delivery
├─ pkg
│  └─ sdk                    # public Go SDK
├─ examples                  # integration and migration examples
└─ docs                      # architecture, deployment, and integration docs
```

Common development commands:

```bash
go run ./cmd/server/main.go
go run ./cmd/server/main.go -config config/config.yaml.example
go test ./...
```

Development notes:

- For registry or discovery behavior, start from `internal/catalog` instead of the compatibility layer.
- For AP / CP consistency and node coordination, start from `internal/cluster`.
- For protocol changes, use `internal/transport` for native interfaces and `internal/adapter` for compatibility interfaces.
- For SDK or developer experience changes, update `pkg/sdk` and `examples` together.

## Documentation

- [Architecture](./docs/architecture.md)
- [Deployment](./docs/deployment.md)
- [Integration](./docs/integration.md)

## 📄 License

This project is licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.

# AGENTS.md

This file guides Codex and other AI coding agents when working in the
`eden-microservice` repository. Human-facing product docs live in `README.md`
and `docs/`; behavior contracts for agents live in `specs/`.

DO NOT send optional commentary.

## AI Contribution Guidelines

- Spec-first coding is mandatory. Before changing behavior, APIs, SDKs,
  storage, runtime flow, compatibility adapters, or console semantics, read the
  related files under [`specs/`](./specs/README.md).
- If code and specs disagree, do not silently make the code the source of
  truth. Update the spec in the same change or call out the mismatch before
  implementation.
- Keep changes scoped to the touched domain. Avoid broad package renames,
  unrelated UI restyling, or storage rewrites unless the requested work needs
  them.
- Respect existing user changes in the working tree. Do not revert files you did
  not intentionally change.
- Prefer tests before implementation for behavior changes. For Go, add focused
  tests near the package under change; for the web UI, run type checking and
  build when views or API contracts change.

## Product Boundary

`eden-microservice` is a lightweight microservice control plane. The target
scope is to replace these common dependencies in small and medium deployments:

- Nacos Naming registry
- Nacos Config configuration center
- Spring Cloud Gateway style API gateway
- ZooKeeper registry usage
- Consul registry usage

The project currently contains historical `eden-registry` / `Focalors` naming in
module paths, docs, and UI text. Do not mass-rename these as a drive-by change.
Treat product renaming as a separate migration unless the current task is the
rename itself.

## Current Architecture

Existing production code is centered on service registration and control-plane
management:

| Area | Current location | Responsibility |
| --- | --- | --- |
| Server bootstrap | `cmd/server` | process startup and runtime wiring |
| Runtime config | `internal/config` | YAML/env process configuration |
| Registry domain | `internal/catalog` | service registry, discovery, health, topology, events |
| AP/CP cluster | `internal/cluster` | AP replication, CP Raft, shared runtime state |
| HTTP API | `internal/transport/http` | console API, native HTTP API, compatibility routes |
| gRPC API | `internal/transport/rpc` | native registry RPC and cluster RPC |
| Compatibility | `internal/adapter/nacos`, `internal/adapter/consul` | Nacos Naming and Consul HTTP compatibility |
| Auth/settings | `internal/auth`, `internal/settings` | login, RBAC, API keys, runtime settings |
| Console | `web/src` | Vue 3 + Element Plus admin console |
| SDK | `pkg/sdk` | Go client API |

Target new domains:

| Area | Recommended location | Responsibility |
| --- | --- | --- |
| Config center | `internal/configcenter` | Config resource model, storage, history, watch, Nacos Config compatibility |
| Gateway control | `internal/gateway` | route definitions, validation, publication, admin APIs |
| Gateway data plane | `internal/gateway/proxy` or `internal/transport/gateway` | HTTP reverse proxy, matching, load balancing, filters |
| Config UI | `web/src/views/configs.vue` | configuration management |
| Route UI | `web/src/views/routes.vue` | gateway route management |

Use `internal/configcenter` rather than `internal/config` for the config center;
`internal/config` already owns process configuration.

## Build And Test Commands

```bash
go test ./...
go run ./cmd/server/main.go
go run ./cmd/server/main.go -config config/config.yaml.example
```

```bash
cd web
npm run check:i18n
npm run build
```

For targeted work, run the smallest relevant Go test package first, then run
`go test ./...` before completion when the change touches shared behavior.

## API Standards

Native APIs keep the current `/v1/*` style. Compatibility APIs preserve the
external product shape they replace.

| API type | Base path | Rule |
| --- | --- | --- |
| Native registry | `/v1/catalog/*` | existing registry API |
| Native config | `/v1/configs`, `/v1/config/*` | console and custom clients |
| Native gateway | `/v1/gateway/*` | route/admin/control operations |
| Nacos Naming compatibility | `/nacos/v1/ns/*`, `/v1/ns/*` | preserve existing behavior |
| Nacos Config compatibility | `/nacos/v1/cs/*` | match Nacos Config client expectations |
| Consul compatibility | `/v1/agent/*`, `/v1/health/*`, `/v1/catalog/*` | preserve Consul HTTP shape |
| Internal sync | `/internal/sync/*` | node-to-node only |

Console/native APIs may use the local `jsonOK` and `httpError` helpers.
Compatibility endpoints must return the payload shape expected by the upstream
client ecosystem, even when that differs from native APIs.

## Auth And RBAC

- Admin users can mutate registry settings, users, API keys, config resources,
  and gateway routes.
- Developer users can manage service-facing resources where explicitly allowed,
  including configuration content in non-system namespaces.
- Viewer users are read-only when viewer routes are implemented.
- Client-facing registry/config/gateway runtime APIs may use API keys or
  compatibility auth according to the selected adapter.

## Console Menu Direction

The control plane should expose configuration and gateway routing as first-class
modules:

1. `概览` / Overview
2. `服务列表` / Services
3. `配置管理` / Configs
4. `路由管理` / Routes
5. `命名空间` / Namespaces
6. `节点管理` / Nodes
7. `访问控制` / RBAC
8. `系统设置` / Settings

`配置管理` and `路由管理` should not be nested under Services. They are separate
control-plane domains.

## Authoritative Specs

Read the relevant spec before changing a domain:

- Top-level design: [`specs/zh-CN/design/eden-microservice-design-spec.md`](./specs/zh-CN/design/eden-microservice-design-spec.md)
- Resource model: [`specs/zh-CN/design/resource-model-spec.md`](./specs/zh-CN/design/resource-model-spec.md)
- Config center: [`specs/zh-CN/config/README.md`](./specs/zh-CN/config/README.md)
- API gateway: [`specs/zh-CN/gateway/README.md`](./specs/zh-CN/gateway/README.md)
- HTTP APIs: [`specs/zh-CN/http-api/api-spec.md`](./specs/zh-CN/http-api/api-spec.md)
- Console: [`specs/zh-CN/console/console-spec.md`](./specs/zh-CN/console/console-spec.md)


# AGENTS.md

This file guides Codex, Claude Code, and compatible agents working in the
`eden-microservice` repository. Human-facing product docs live in `README.md` and
`docs/`; behavior contracts for agents live in `specs/`.
`CLAUDE.md` references this file; do not create conflicting root-level instructions.
Do not send optional process narration. Keep this file under 200 lines.

## Start Every Task

1. Read this file, the applicable specification, `skills/eden-microservice/SKILL.md`, and relevant
   entries in `specs/zh-CN/engineering/agent-pitfalls.md`. Read `CONTEXT.md` only when
   terminology is relevant or ambiguous.
2. Run `git status --short`; preserve user changes outside the task.
3. Read affected implementation and tests before proposing or editing.
4. Keep explanation, review, and diagnosis read-only by default; start behavior changes with a
   focused test.
5. For failures, establish a stable reproduction and test one falsifiable hypothesis at a time.

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
- Put temporary artifacts, screenshots, logs, and drafts in `.tmp/`; never commit secrets or
  local runtime configuration.

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

The repository is a `go.work` monorepo. Business modules own implementation under
`apps/<domain>/internal`, expose stable contracts from `api` and composition from `module`, and
must not import another module's `internal` packages.

| Area | Current location | Responsibility |
| --- | --- | --- |
| Workspace | `go.work` | local module composition |
| Shared foundations | `packages/{config,crypto,metrics,replication,transport}` | repository-local process, replication, and transport support |
| Registry | `apps/registry` | service registry, discovery, health, topology, compatibility adapters, Go SDK |
| Config center | `apps/config` | config resources, storage, history, watch, Nacos Config compatibility |
| Gateway | `apps/gateway` | route control plane, validation, publication, proxy runtime |
| Permission control | `apps/auth` | login, users, RBAC roles, API keys |
| Cluster management | `apps/cluster` | AP replication, CP Raft, membership and runtime settings |
| Aggregate server | `apps/server/cmd/server`, `apps/server/module` | runtime wiring and unified HTTP/gRPC transports |
| Console | `apps/ui/src` | Vue 3 + Element Plus admin console |

Use `apps/config/internal/configcenter` for control-plane configuration resources and
`packages/config` for process YAML/environment configuration.

## Build And Test Commands

```bash
go work sync
go test ./packages/...
go test ./apps/auth/... ./apps/cluster/... ./apps/config/... ./apps/gateway/... ./apps/registry/... ./apps/server/...
go run ./apps/server/cmd/server
go run ./apps/server/cmd/server -config configs/eden-microservice.yaml.example
```

```bash
cd apps/ui
npm run check:i18n
npm run build
```

For targeted work, enter the owning module and run the smallest relevant package first, then
`go test ./...` for that module. Verify every affected module when shared behavior changes.

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
- Module boundaries: [`specs/zh-CN/modules/README.md`](./specs/zh-CN/modules/README.md)

## Focused Skills

- [`eden-microservice-diagnose`](./skills/eden-microservice-diagnose/SKILL.md): reproduce and isolate failures before fixes.
- [`eden-microservice-tdd`](./skills/eden-microservice-tdd/SKILL.md): implement behavior at a tested public seam.
- [`eden-microservice-code-review`](./skills/eden-microservice-code-review/SKILL.md): review contract fidelity and engineering risk separately.
- [`eden-microservice-domain-modeling`](./skills/eden-microservice-domain-modeling/SKILL.md): resolve ownership and terminology before cross-domain changes.

## Specifications / 规范

- Choose the specification language requested by the task; never infer it from the operating system.
- [English index](./specs/en/README.md) · [简体中文索引](./specs/zh-CN/README.md)
- Keep parallel contracts aligned when both exist. Current product contracts are Chinese-canonical; do
  not introduce partial English translations in unrelated changes.

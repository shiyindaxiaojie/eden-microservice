# AGENTS.md

Shared instructions for Codex, Claude Code, and compatible agents. `CLAUDE.md` references
this file; keep it tool-neutral, under 200 lines, without duplicate root instructions.

## Start Every Task

1. Run `git status --short`; preserve unrelated user changes.
2. Follow `skills/eden-microservice/SKILL.md`. Read the applicable contract and affected code/tests;
   search relevant entries in `specs/zh-CN/engineering/agent-pitfalls.md`. Read `CONTEXT.md`
   only for relevant or ambiguous terminology. Reuse already-read, unchanged context.
3. Explanation, review, and diagnosis are read-only unless changes are requested. Complete
   authorized work with routine in-scope decisions; ask only when a missing choice materially
   changes behavior or authority.

## Engineering Loop

- Specifications govern behavior, APIs, storage, runtime, and console semantics. Resolve
  code/spec mismatches explicitly; update the contract in the same change when it changes.
- Keep changes in the affected domain; no unrelated renames, restyling, or rewrites.
- Reproduce failures before source edits; test one falsifiable hypothesis at a time.
- Behavior changes start with a focused failing test. Run required verification once;
  broaden or repeat only for changes, failures, or unresolved risk. Documentation-only edits
  use document checks, not application builds.
- Use scoped searches and relevant sections; do not load entire spec trees or pitfall registers.
- Skip optional process narration. Report the scoped result and verification commands/results
  concisely, with evidence for success and any remaining gaps.
- Record pitfalls only for reusable causes/prevention confirmed by user correction, test,
  or review; exclude exploratory failures, transient issues, and personal settings.
- Put temporary artifacts, screenshots, logs, and drafts in `.tmp/`; never commit secrets
  or local runtime configuration.

## Product Boundary

`eden-microservice` is a lightweight control plane replacing Nacos Naming/Config,
Spring Cloud Gateway-style routing, and ZooKeeper/Consul registry usage in small/medium deployments.
Historical `eden-registry` / `Focalors` names require a separate migration; no drive-by mass renames.

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
| Examples | `examples` | runnable integration and migration examples |

Use `apps/config/internal/configcenter` for control-plane configuration resources and
`packages/config` for process YAML/environment configuration.

## Build And Test Commands

Run the owning Go package first, then `go test ./...` from that module; shared changes require
every affected module. Workspace-wide checks, when needed: `go test ./packages/...`,
`go test ./apps/auth/... ./apps/cluster/... ./apps/config/... ./apps/gateway/... ./apps/registry/... ./apps/server/...`,
and `go test ./examples/...`. Use `go work sync` for workspace dependency synchronization.
Startup: `go run ./apps/server/cmd/server` (optional
`-config apps/server/config/eden-microservice.yaml.example`).
For UI views/API changes, run `npm run check:i18n` and `npm run build` in `apps/ui`.

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

Keep first-level Overview, Services, Configs, Routes, Namespaces, Nodes, RBAC, and Settings.
Configs and Routes are separate control-plane domains; do not nest them under Services.

## Specifications and Skills

Use [specs](./specs/README.md) to locate the governing document only when unknown.
Follow the requested language, never the OS: [English](./specs/en/README.md),
[简体中文](./specs/zh-CN/README.md). Product contracts are Chinese-canonical;
align existing parallel contracts, without partial translations in unrelated changes.
The repository router selects diagnosis, TDD, review, or domain modeling as needed.

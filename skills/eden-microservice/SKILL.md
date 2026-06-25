---
name: eden-microservice
description: Use when designing or implementing eden-microservice registry, config center, gateway, compatibility adapters, console, or control-plane storage behavior.
---

# Eden Microservice

Use this skill for work that changes control-plane behavior in
`eden-microservice`.

## Required Reading

Before coding, read:

1. `AGENTS.md`
2. `specs/README.md`
3. The domain README under `specs/zh-CN/`
4. The specific spec for the API, storage, UI, or compatibility behavior being
   changed

## Domain Routing

| Task | Primary files to inspect first |
| --- | --- |
| Registry behavior | `internal/catalog`, `internal/transport/http/catalog.go`, `internal/transport/rpc/registry_server.go` |
| Nacos Naming compatibility | `internal/adapter/nacos` |
| Consul compatibility | `internal/adapter/consul` |
| Config center | `specs/zh-CN/config`, then planned package `internal/configcenter` |
| API gateway | `specs/zh-CN/gateway`, then planned package `internal/gateway` |
| Console navigation | `web/src/App.vue`, `web/src/router/index.ts`, `web/src/utils/i18n.ts` |
| Cluster consistency | `internal/cluster`, `internal/cluster/replication` |
| Runtime process config | `internal/config`, `config/config.yaml.example` |

## Implementation Principles

- Do not place configuration-center code in `internal/config`; that package is
  for process configuration.
- Native APIs should remain simple `/v1/*` JSON APIs.
- Nacos and Consul compatibility endpoints should preserve upstream client
  behavior, including response bodies and status conventions.
- Config content is a black box. Store, version, publish, query, and notify it;
  do not parse business keys inside it.
- Gateway data-plane traffic should be separated from console/admin APIs when
  possible, preferably with a dedicated listener.
- For CP mode, writes to durable control-plane resources should go through Raft.
  For AP mode, writes may fan out and converge.


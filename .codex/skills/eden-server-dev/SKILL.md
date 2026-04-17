---
name: eden-server-dev
description: Use this skill when working on the Eden Go server runtime, startup flow, config handling, AP/CP mode selection, transport listeners, auth bootstrap, or local backend debugging. Trigger for requests like run the Eden server, adjust config.yaml, debug startup, change AP/CP behavior, inspect HTTP/gRPC/QUIC/Raft wiring, or trace backend initialization in cmd/server and internal packages.
---

# Eden Server Dev

Use this skill for backend work centered on starting, configuring, and debugging the Eden registry server.

## What to inspect first

- Read [`references/runtime-map.md`](./references/runtime-map.md).
- If the task changes runtime behavior, also read [`configs/config.yaml.example`](../../../configs/config.yaml.example).
- For deeper architecture context, open [`docs/architecture_zh.md`](../../../docs/architecture_zh.md) only for the relevant section.

## Core workflow

1. Start from [`cmd/server/main.go`](../../../cmd/server/main.go) and identify whether the change belongs to boot flags, config loading, runtime state, transport wiring, or service initialization.
2. Trace the affected package under `internal/` before editing. Common areas are `internal/config`, `internal/catalog`, `internal/cluster`, `internal/transport`, `internal/auth`, `internal/settings`.
3. Prefer changing the narrowest layer that owns the behavior instead of patching startup glue.
4. When the task mentions AP/CP, confirm whether the behavior is runtime-state driven, config driven, or both.
5. When the task mentions ports or protocols, verify whether the code path uses HTTP, gRPC, QUIC, or Raft, and whether `auto` / `off` / explicit addresses are supported.

## Validation

- For backend changes, prefer `go test ./...` when the affected scope is broad.
- For startup/config changes, also run `go run ./cmd/server/main.go` or the smallest relevant command when feasible.
- Call out any validation you could not run.

## Repo-specific guardrails

- Default config path in code is `configs/config.yaml`, but the repo currently exposes `configs/config.yaml.example`; do not assume a real config file exists.
- The server can fall back to defaults when config loading fails, so confirm whether behavior comes from config or fallback values.
- Console auth and API-key behavior can be seeded from config and persisted runtime state; check both before concluding a bug.

---
name: eden-service-discovery
description: Use this skill when working on service registration/discovery flows, pkg/eden client usage, heartbeat/watch behavior, topology reporting, or the example integrations under examples/service-discovery, including Eden native, Consul-compatible, Nacos-compatible, and custom protocol clients.
---

# Eden Service Discovery

Use this skill for changes or investigations around client integration and example-based verification.

## What to inspect first

- Read [`references/integration-map.md`](./references/integration-map.md).
- If the task is about examples, open the matching README under `examples/service-discovery/*/README.md`.
- If the task is about the SDK, inspect `pkg/eden` first.
- If the task is about external compatibility, inspect `internal/adapter/consul` or `internal/adapter/nacos`.

## Core workflow

1. Identify the integration style: native Eden, Consul compatibility, Nacos compatibility, or custom protocol.
2. Trace the request path end to end: client/example code -> transport/API contract -> backend catalog/adapter implementation.
3. Keep example sets self-contained. Do not introduce cross-example helpers unless the repo already does so.
4. Preserve the repo's separation between HTTP and gRPC example entrypoints.
5. When changing wire contracts, verify both code and README/proto expectations.

## Validation

- Run focused tests in the touched package first.
- If examples are affected, verify the relevant example README, env vars, and startup scripts still match the code.
- If protocol definitions changed, check generated code locations under `api/proto` or example-local proto outputs.

## Repo-specific guardrails

- `examples/service-discovery/custom` intentionally models an external project and keeps its own protocol assets.
- The native `eden` examples split HTTP and gRPC entrypoints and use different env vars.
- The repo advertises compatibility with official Consul and Nacos clients, so avoid changes that force repo-specific client code into those paths.

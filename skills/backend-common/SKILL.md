---
name: backend-common
description: Detailed Go backend development guidelines, project structure, design principles, handler-splitting rules, configuration rules, consistency-mode constraints, and coding standards for eden-go-registry. Use when working in cmd/, configs/, internal/, pkg/, or api/proto/ to implement or review handlers, services, stores, cluster logic, SDK behavior, config fields, or protobuf changes.
---

# Backend Common Skill

This skill defines the baseline backend rules for `eden-go-registry`.

## 1. Project Structure

- `cmd/`: entry points such as `cmd/server`.
- `configs/`: configuration files, including `config.yaml` and `config.yaml.example`.
- `internal/`: private application code.
  - `cluster/`: AP and CP cluster implementations.
  - `config/`: Viper-based configuration loading.
  - `handler/`: HTTP handlers, middleware, routing, and response helpers.
  - `health/`: TTL-based health checks.
  - `model/`: backend models.
  - `service/`: business orchestration and coordination.
  - `store/`: registry, namespace, and persistence logic.
  - `pkg/crypto/`: internal crypto helpers.
- `pkg/`: public SDKs and adapters.
- `api/proto/`: protobuf contracts and generated gRPC files.
- `web/`: frontend console.

## 2. Design Pattern Requirements

> Every backend change must follow these design rules.

### 2.1 Single Responsibility Principle (SRP)

- One file should handle one category of responsibility.
- Split handler files by business domain; do not mix unrelated domains in one file.
- Reassess a file once it grows beyond roughly 200 lines.
- Keep cross-cutting helpers such as HTTP response utilities in dedicated files like `response.go`.

### 2.2 Handler File Split Rules

Keep the handler layer split by business domain:

| File | Responsibility |
|------|----------------|
| `handler.go` | Handler struct, constructor, route registration, CORS |
| `response.go` | shared HTTP response helpers such as `httpError` and `jsonOK` |
| `auth_handler.go` | JWT, RBAC, API key middleware, login, token generation |
| `catalog_handler.go` | service registration, heartbeat, discovery, instance operations |
| `cluster_handler.go` | cluster join, member list, stats, events |
| `namespace_handler.go` | namespace CRUD |
| `settings_handler.go` | user management, API key management, consistency mode |

When adding a new business area, create a new `xxx_handler.go`. Do not keep appending unrelated handlers to an existing file.

### 2.3 Interface-Based Design

- Define interfaces for core components such as stores and cluster nodes when it improves testability and substitution.
- Pass dependencies through constructors.
- Do not introduce global mutable state.

### 2.4 Package Naming

- Prefer singular package names such as `config` and `model`.
- Name packages by capability, such as `crypto`, instead of generic buckets.
- Do not create packages named `common`, `util`, or `helper`.

## 3. Configuration Rules

- Keep runtime config in `configs/config.yaml`.
- Keep safe example config in `configs/config.yaml.example` and use placeholders for secrets.
- Use Viper for YAML plus environment variable overrides with the `REGISTRY_` prefix.
- When adding a config field, update all of the following together:
  1. `internal/config/config.go`
  2. `configs/config.yaml`
  3. `configs/config.yaml.example`

## 4. Error Handling and Responses

- Use `httpError(w, code, msg)` for JSON error responses.
- Use `jsonOK(w, data)` for JSON success responses.
- Validate the HTTP method at the top of each handler unless the route dispatcher already owns that branching.
- Keep error messages useful for developers without leaking sensitive internals.
- Preserve leader redirect handling for CP-mode write operations by routing leader errors through `handleLeaderRedirect` where appropriate.

## 5. Consistency Mode Rules

- AP mode is eventually consistent and uses peer broadcast behavior.
- CP mode is strongly consistent and uses Raft log replication.
- Both node types are initialized during startup and the system supports online mode switching.
- In request handling, check the active mode first and route behavior to the corresponding AP or CP implementation.
- Do not introduce changes that accidentally break one mode while only testing the other.

## 6. SDK and Proto Changes

- Treat `pkg/eden` as a compatibility-sensitive surface.
- Prefer optional fields and transparent defaults over public API signature changes.
- When changing protobuf contracts, update the `.proto`, generated files, and backend callers together.
- Use the local Go toolchain that is already installed. Do not download or install Go.

## 7. Code Style and Validation

- Run `go fmt ./...` after non-trivial Go edits.
- Run `go vet ./...` for static analysis when logic changes.
- Run `go build ./...` at minimum before wrapping up backend work.
- Write concise English comments.
- Add godoc comments for exported functions and types.
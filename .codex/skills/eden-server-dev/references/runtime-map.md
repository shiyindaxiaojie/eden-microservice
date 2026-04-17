# Runtime Map

## Main entry

- `cmd/server/main.go` parses flags, loads config, normalizes mode/consistency, restores runtime state, initializes auth/logging/storage, and wires HTTP/gRPC/QUIC/Raft listeners.

## Important flags

- `-config`
- `-data-dir`
- `-node-id`
- `-http-addr`
- `-mode`
- `-consistency`
- `-grpc`
- `-quic`
- `-raft`

## Common ownership map

- `internal/config`: config structs and file loading
- `internal/catalog`: registration, discovery, heartbeat, topology
- `internal/cluster/ap`: AP replication behavior
- `internal/cluster/cp`: Raft-backed CP behavior
- `internal/transport/http`: console and HTTP APIs
- `internal/transport/rpc`: registry and cluster gRPC servers
- `internal/transport/quic`: QUIC listener
- `internal/auth`: login, users, API keys
- `internal/settings`: system/profile/log-level settings

## Transport defaults from repo docs/config

- HTTP default: `:8500`
- gRPC: `auto`
- QUIC: `off` in example config
- Raft: `auto` in example config, meaningful for cluster + cp

## When to read more

- Read `docs/architecture_zh.md` for protocol boundaries and AP/CP responsibilities.
- Read `docs/user_guide_zh.md` for operator-facing startup and auth expectations.

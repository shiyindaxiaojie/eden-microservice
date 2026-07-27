# Consul Integration Example

This example now uses the official [`github.com/hashicorp/consul/api`](https://pkg.go.dev/github.com/hashicorp/consul/api) client directly.

The point of the demo is:

- keep normal Consul client code
- only change `CONSUL_ADDR`
- switch from real Consul to the registry without changing business logic

## Layout

- `cmd/auth-center`
- `cmd/user-center`
- `cmd/order-center`
- `internal/consulapi`
- `start.bat`

`internal/consulapi` is only a tiny demo helper around the official Consul client so the three services can share the same register/discovery/heartbeat flow.

## Start

Default registry address:

```text
CONSUL_ADDR=127.0.0.1:8500
```

Run:

```bash
./examples/service-discovery/consul/start.bat
```

## Services

- `consul-auth-center`
- `consul-user-center`
- `consul-order-center`

Ports:

- `auth-center`: `22002`, `22012`, `22022`
- `user-center`: `22001`, `22011`
- `order-center`: `22003`, `22013`

## HTTP Endpoints

- `GET /api/auth/token?user_id=1`
- `GET /api/auth/permissions/{userId}`
- `GET /api/users`
- `GET /api/users/{userId}`
- `GET /api/users/{userId}/profile`
- `GET /api/orders/create?user_id=1`
- `GET /api/orders/demo?user_id=1`
- `GET /api/health`

Examples:

```bash
curl "http://127.0.0.1:22002/api/auth/token?user_id=1"
curl "http://127.0.0.1:22001/api/users/1/profile"
curl "http://127.0.0.1:22003/api/orders/create?user_id=1"
curl "http://127.0.0.1:22003/api/orders/demo?user_id=1"
```


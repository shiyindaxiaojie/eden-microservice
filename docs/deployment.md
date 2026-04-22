# Focalors Deployment Guide

## Runtime Targets

Focalors supports three operating shapes:

- `standalone + ap` for local development and isolated validation
- `cluster + ap` for highly available deployments that prioritize availability
- `cluster + cp` for clusters that require stronger metadata consistency

## Start Commands

Use the default configuration:

```bash
go run ./cmd/server/main.go
```

Specify a configuration file explicitly:

```bash
go run ./cmd/server/main.go -config config/config.yaml
```

Common flags:

- `-config`
- `-data-dir`
- `-node-id`
- `-http-addr`
- `-mode`
- `-consistency`

## Configuration Baseline

Relevant files:

- [`config/config.yaml`](../config/config.yaml)
- [`config/config.yaml.example`](../config/config.yaml.example)

Key sections:

- node identity: `node_id`, `datacenter`, `data_dir`
- runtime mode: `mode`, `consistency`, `bootstrap`
- network listeners: `server.http`, `server.grpc`, `server.quic`, `server.raft`
- security and logging: `auth.jwt`, `log`

## Mode-Specific Notes

### Local Development

Recommended baseline:

```yaml
mode: "standalone"
consistency: "ap"
server:
  http: ":8500"
  grpc: "auto"
  quic: "off"
  raft: "off"
```

### AP Cluster

Recommended baseline:

```yaml
mode: "cluster"
consistency: "ap"
server:
  http: ":8500"
  grpc: "auto"
  quic: "off"
  raft: "off"
```

### CP Cluster

Bootstrap node:

```yaml
mode: "cluster"
consistency: "cp"
bootstrap: true
server:
  http: ":8500"
  grpc: ":9000"
  raft: "127.0.0.1:7000"
```

Follower node:

```yaml
mode: "cluster"
consistency: "cp"
bootstrap: false
server:
  http: ":8501"
  grpc: ":9001"
  raft: "127.0.0.1:7001"
```

Operational differences:

- `standalone + ap` has the lowest setup cost and no node coordination.
- `cluster + ap` focuses on replication and availability.
- `cluster + cp` requires an explicit bootstrap node and valid Raft connectivity.

## Console

Frontend development commands:

```bash
cd web
npm install
npm run dev
```

Default addresses:

- console: `http://127.0.0.1:2019`
- backend: `http://127.0.0.1:8500`

Default credentials:

- username: `admin`
- password: `admin`

## Validation Checks

```bash
curl http://127.0.0.1:8500/v1/cluster/members
curl http://127.0.0.1:8500/v1/catalog/services
curl "http://127.0.0.1:8500/v1/catalog/service/order-center?passing=true"
```

## Related Reading

- [Integration](./integration.md)
- [Architecture](./architecture.md)
- [Simplified Chinese deployment](./deployment_zh-CN.md)

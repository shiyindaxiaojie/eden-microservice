---
name: registry-core
description: Core specifications for Eden Go Registry (AP/CP modes, configs, health checks).
---

# Eden Go Registry Core Specifications

This skill documents the specific design decisions and architecture for the Registry module.

## 1. Consistency Modes

The registry supports two consistency modes, switchable via config (`mode`):

- **AP Mode (Default)**:
  - Decentralized (peer-to-peer), no Leader bottleneck.
  - Achieves eventual consistency via asynchronous HTTP broadcasting to a list of seed nodes.
  - Recommended for massive service discovery where high availability and partition tolerance are preferred.
- **CP Mode**:
  - Strongly consistent, uses HashiCorp Raft protocol for consensus.
  - Requires a static cluster setup or bootstrapping. All write requests must be processed by the Leader.
  - Recommended when absolute data correctness is more important than write availability during partitions.

## 2. Configuration (`configs/`)

- Configuration is managed via `spf13/viper`.
- Standard config file is `configs/config.yaml`.
- Environment variables override YAML files (prefix `REGISTRY_`, e.g., `REGISTRY_HTTP_ADDR`).
- Key properties:
  - `node_id`: Unique identifier for the node.
  - `mode`: `"ap"` or `"cp"`.
  - `http_addr`: Address for HTTP API (default `:8500`).
  - `raft_addr`: Address for Raft internal communication (default `127.0.0.1:7000`, CP mode).
  - `data_dir`: Directory for storing local disk data (e.g. boltdb, snapshots).
  - `seeds`: Array of seed nodes for AP mode broadcasting.

## 3. Data Flow & Registry Store

- The core `store.Registry` is an in-memory, thread-safe (sync.RWMutex) map.
- Write Path (CP): `HTTP Handler -> Raft -> FSM -> store.Registry`
- Write Path (AP): `HTTP Handler -> store.Registry` + `Async Broadcast to Seeds`
- Health Checks: Managed independently on each node via `health.Checker` checking TTLs (Time-To-Live). Instances not sending heartbeats within TTL are marked critical, and subsequently removed.

## 4. Frontend Integration

- The web dashboard (Vue 3 / Vite) polls `/v1/cluster/stats` and `/v1/events` to refresh UI.
- Make sure to add `Access-Control-Allow-Origin: *` to HTTP API responses during development.

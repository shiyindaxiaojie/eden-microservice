# Eden 通信协议架构

Eden 注册中心使用 4 种通信协议，服务于不同场景。本文档梳理各协议的用途、通信拓扑和关键代码路径。

## 1. 协议总览

| 协议 | 端口范围 | 用途 | 参与方 |
|------|---------|------|--------|
| **HTTP** | 8500-8599 | REST API（控制台 + SDK）+ 节点间内部同步 | 客户端 ↔ 注册中心，注册中心 ↔ 注册中心 |
| **gRPC** | 9000-9999 | 高性能服务注册/发现 + 节点间数据复制 | 客户端 ↔ 注册中心，注册中心 ↔ 注册中心 |
| **QUIC** | 10000-10999 | gRPC 的可选传输层（UDP），弱网优化 | 客户端 ↔ 注册中心 |
| **Raft** | 7000-7999 | CP 模式下的共识协议（Leader 选举 + 日志复制） | 注册中心 ↔ 注册中心（仅 CP 模式） |

## 2. 协议分层架构

```mermaid
graph TB
    subgraph "客户端 SDK (pkg/eden/client.go)"
        SDK[Eden Client]
    end

    subgraph "传输层"
        HTTP_T["HTTP/1.1 (TCP)"]
        GRPC_T["gRPC (TCP)"]
        QUIC_T["gRPC over QUIC (UDP)"]
    end

    subgraph "注册中心 Server"
        subgraph "API 层"
            HTTP_H["HTTP Handler<br/>(internal/handler/)"]
            GRPC_RS["gRPC RegistryServer<br/>(internal/grpc/server.go)"]
            GRPC_CS["gRPC ClusterServer<br/>(internal/grpc/cluster_server.go)"]
        end

        subgraph "业务层 (internal/service/)"
            CAT["CatalogService"]
            SET["SettingsService"]
        end

        subgraph "集群层 (internal/cluster/)"
            AP["AP Node<br/>广播复制"]
            CP["CP Node<br/>Raft 共识"]
        end

        subgraph "存储层 (internal/store/)"
            REG["Registry<br/>(ServiceStore + AuthStore + ConfigStore)"]
        end
    end

    SDK --> HTTP_T --> HTTP_H
    SDK --> GRPC_T --> GRPC_RS
    SDK --> QUIC_T --> GRPC_RS

    HTTP_H --> CAT
    HTTP_H --> SET
    GRPC_RS --> CAT

    CAT --> AP
    CAT --> CP
    AP --> REG
    CP --> REG

    AP -- "gRPC ReplicateLog" --> GRPC_CS
    GRPC_CS --> AP
```

## 3. 注册中心之间的通信

```mermaid
sequenceDiagram
    participant N1 as Node-1 (8500)
    participant N2 as Node-2 (8501)
    participant N3 as Node-3 (8502)

    Note over N1,N3: === AP 模式：异步广播 ===

    rect rgb(200, 230, 255)
        N1->>N1: Register(inst) 本地写入
        par gRPC 广播 (ReplicateLog)
            N1-->>N2: gRPC ReplicateLog(register, inst)
            N1-->>N3: gRPC ReplicateLog(register, inst)
        end
        Note right of N2: isReplicate=true<br/>不再转发
    end

    Note over N1,N3: === AP 模式：Anti-Entropy 全量同步（每30s）===

    rect rgb(230, 255, 230)
        N2->>N1: gRPC SyncDiscovery()
        N1-->>N2: 返回全量 services JSON
        N2->>N2: Merge(remote) 合并数据
    end

    Note over N1,N3: === CP 模式：Raft 共识 ===

    rect rgb(255, 230, 230)
        N1->>N1: Raft.Apply(cmd) Leader 提交日志
        N1-->>N2: Raft Log Replication (TCP :7001)
        N1-->>N3: Raft Log Replication (TCP :7002)
        N2->>N2: FSM.Apply(cmd) 状态机执行
        N3->>N3: FSM.Apply(cmd) 状态机执行
    end
```

### 关键代码路径

**AP 广播路径**：
```
catalogService.Register()                   # internal/service/catalog.go:31
  → apNode.Apply("register", inst, false)   # internal/cluster/ap/node.go:103
    → Registry.Register(inst)               # 本地写入
    → go broadcast("register", data)        # 异步广播
      → PeerManager.Broadcast()             # internal/cluster/peer_manager.go:172
        → peer.GetClient()                  # 获取 gRPC 连接
        → client.ReplicateLog()             # 发送到对端
```

**CP Raft 路径**：
```
catalogService.Register()                   # internal/service/catalog.go:31
  → cpNode.Apply(cmd, 5s)                   # internal/cluster/cp/node.go:101
    → raft.Apply(data, timeout)             # 提交到 Raft 集群
    → FSM.Apply(log)                        # internal/cluster/cp/fsm.go:58
      → registry.Register(inst)            # 所有节点的 FSM 都执行
```

## 4. 客户端与注册中心的通信

```mermaid
sequenceDiagram
    participant C as Client SDK
    participant R1 as Registry-1 (8500)
    participant R2 as Registry-2 (8501)

    Note over C,R2: === 初始化：节点发现 ===
    C->>R1: HTTP GET /v1/cluster/members
    R1-->>C: [{id, http_addr, grpc_addr, quic_addr, status}]

    Note over C,R2: === 服务注册（gRPC / QUIC）===
    C->>R1: gRPC Register(instance)
    R1-->>C: RegisterResponse{success: true}

    Note over C,R2: === 心跳保活（每10s）===
    loop 每隔 10s
        C->>R1: gRPC Heartbeat(service, id)
        R1-->>C: HeartbeatResponse{success: true}
    end

    Note over C,R2: === 节点故障转移 ===
    C-xR1: gRPC Heartbeat ❌ 连接失败
    Note right of C: 标记 R1 不健康<br/>触发 syncNodes()
    C->>R2: gRPC Heartbeat(service, id)
    R2-->>C: HeartbeatResponse{success: true}

    Note over C,R2: === 服务发现 ===
    C->>R2: gRPC Discover(serviceName)
    R2-->>C: [instance1, instance2, ...]

    Note over C,R2: === 服务订阅（Watch 流）===
    C->>R2: gRPC Watch(serviceName)  [Server Streaming]
    R2-->>C: WatchResponse{instances: [...]}
    Note right of R2: 服务变更时推送
    R2-->>C: WatchResponse{instances: [...updated...]}
```

## 5. 协议 → 文件映射

### HTTP 协议

| 文件 | 职责 |
|------|------|
| `internal/handler/handler.go` | 路由注册，CORS |
| `internal/handler/catalog_handler.go` | `/v1/catalog/*` 服务注册/注销/心跳/发现 |
| `internal/handler/cluster_handler.go` | `/v1/cluster/*` 集群成员管理，`/v1/node/info` 节点信息 |
| `internal/handler/settings_handler.go` | `/v1/settings/*` 配置管理 |
| `internal/handler/auth_handler.go` | `/v1/auth/*` 登录，JWT/RBAC/APIKey 中间件 |

> **特殊端点**：`/internal/sync/*` 路径用于节点间的 HTTP 内部同步（seeds、users、apikeys、settings），无鉴权。

### gRPC 协议

| 文件 | Proto 定义 | 职责 |
|------|-----------|------|
| `internal/grpc/server.go` | `api/proto/registry/v1/` | **RegistryService** — 客户端注册/注销/心跳/发现/Watch |
| `internal/grpc/cluster_server.go` | `api/proto/cluster/v1/` | **ClusterService** — 节点间数据复制(ReplicateLog)、全量同步(SyncDiscovery)、配置同步 |
| `internal/cluster/peer_manager.go` | — | gRPC 连接池管理，Broadcast 广播 |

### QUIC 协议

| 文件 | 职责 |
|------|------|
| `internal/grpc/quic_listener.go` | QUIC → net.Listener 适配器，供 gRPC Server 使用 |
| `pkg/eden/client.go` (dialQUIC) | SDK 侧 QUIC 拨号器，gRPC over QUIC 客户端 |
| `cmd/server/main.go` (L254-274) | 启动 QUIC gRPC Server，自签名 TLS 证书 |

> QUIC 运行在 UDP 上，复用同一个 gRPC Server（与 TCP gRPC 共享 RegistryService + ClusterService）。

### Raft 协议

| 文件 | 职责 |
|------|------|
| `internal/cluster/cp/node.go` | Raft 节点管理（创建、Join、Apply、Members） |
| `internal/cluster/cp/fsm.go` | Raft FSM 状态机（Apply 命令到 Registry） |
| `cmd/server/main.go` (L167-204) | Raft 初始化，Bootstrap，Join 到现有集群 |

> Raft 使用 `hashicorp/raft` 库，传输层为 TCP，日志存储使用 BoltDB。

## 6. 端口与服务对应

以 3 节点集群为例：

| 节点 | HTTP | gRPC | QUIC | Raft |
|------|------|------|------|------|
| node-1 | :8500 | :9000 | :10000 | :7000 |
| node-2 | :8501 | :9001 | :10001 | :7001 |
| node-3 | :8502 | :9002 | :10002 | :7002 |

## 7. AP vs CP 模式对比

| 维度 | AP 模式 | CP 模式 |
|------|---------|---------|
| **一致性** | 最终一致（秒级延迟） | 强一致（多数派确认） |
| **写入路径** | 本地写入 → 异步广播 | Leader Apply → Raft 复制 |
| **可用性** | 任意节点可写 | 仅 Leader 可写 |
| **节点间协议** | gRPC (ReplicateLog + SyncDiscovery) | Raft TCP (日志复制) |
| **故障影响** | 短暂数据不一致 | 少数派节点无法写入 |
| **适用场景** | 高可用优先 | 数据一致性优先 |

# Eden 用户手册

本文档用于说明 Eden 的启动方式、基础配置、认证约定、常用调用以及客户端集成方法。

## 1. 快速启动

### 1.1 启动后端

```bash
go run ./cmd/server/main.go -config configs/config.yaml
```

当前服务端支持的命令行参数如下：

- `-config`
- `-data-dir`
- `-node-id`
- `-http-addr`

### 1.2 启动前端控制台

```bash
cd web
npm install
npm run dev
```

默认开发地址如下：

- 控制台：`http://127.0.0.1:2019`
- 后端 API：`http://127.0.0.1:8500`

前端会将 `/v1/*` 代理到后端 HTTP API。

## 2. 配置说明

### 2.1 AP 集群节点示例

```yaml
node_id: "node-1"
mode: "cluster"
consistency: "ap"
http_addr: ":8500"
grpc_addr: ":0"
quic_addr: ""
data_dir: "./data"
transport:
  grpc: "auto"
  quic: "off"
  raft: "off"
```

说明：

- 集群成员统一在控制台的节点管理中维护，不再通过 `seeds` 写在常规配置里。
- `grpc_addr` 与 `quic_addr` 可以留空或设为 `:0`，服务端会自动分配端口。

### 2.2 CP 首节点示例

```yaml
node_id: "node-1"
mode: "cluster"
consistency: "cp"
http_addr: ":8500"
grpc_addr: ":9000"
raft_addr: "127.0.0.1:7000"
bootstrap: true
data_dir: "./data"
```

### 2.3 CP 普通节点示例

```yaml
node_id: "node-2"
mode: "cluster"
consistency: "cp"
http_addr: ":8501"
grpc_addr: ":9001"
raft_addr: "127.0.0.1:7001"
data_dir: "./data/node-2"
```

说明：

- 其他 CP 节点启动后，通过 Leader 控制台的节点管理添加进集群。
- `join` 不再作为常规配置项对外暴露。

## 3. 认证约定

### 3.1 控制台登录

登录接口为：

- `POST /v1/auth/login`

当前前端提交的不是明文密码，而是原始密码的 `SHA-256` 值。若直接调用该接口，应遵循同样约定。

默认情况下，若未覆盖 `auth.users`：

- 用户名：`admin`
- 原始密码：`admin`

对应的 `SHA-256(admin)` 为：

- `8c6976e5b5410415bde908bd4dee15dfb167a9c873fc4bb8a81f6f2ab448a918`

### 3.2 HTTP API Key

当 `auth.api_key.enabled=true` 时，以下 HTTP 接口要求 API Key：

- `POST /v1/catalog/register`
- `POST /v1/catalog/heartbeat`
- `POST /v1/catalog/topology/report`

传递方式如下：

- Header：`X-API-Key: <your-key>`
- Query：`?api_key=<your-key>`

## 4. 官方 SDK 集成

### 4.1 纯 gRPC 模式

纯 gRPC 模式要求客户端直接持有注册中心节点的 `grpc_addr` 列表。若需要与原有 SDK 相同的动态节点发现能力，应使用 `DiscoveryMode=auto`。

```go
client, err := eden.NewWithConfig(&eden.Config{
    Addresses:     []string{"127.0.0.1:9000", "127.0.0.1:9001"},
    Namespace:     "default",
    Datacenter:    "dc1",
    Transport:     "grpc",
    DiscoveryMode: "auto",
})
if err != nil {
    panic(err)
}
defer client.Close()
```

该模式下：

- 注册、心跳、发现、订阅、节点发现、拓扑上报全部走 `RegistryService`
- 节点发现走 `GetMembers`
- 拓扑上报走 `ReportTopology`
- 不需要调用 `GET /v1/cluster/members`
- 不需要调用 `POST /v1/catalog/topology/report`
- 若将 `DiscoveryMode` 改为 `static`，则关闭动态成员发现，仅在传入地址集合内做故障转移

### 4.2 纯 HTTP 模式

纯 HTTP 模式要求客户端只使用注册中心的 `http_addr`。

```go
client, err := eden.NewWithConfig(&eden.Config{
    Addresses:  []string{"http://127.0.0.1:8500", "http://127.0.0.1:8501"},
    Namespace:  "default",
    Datacenter: "dc1",
    Transport:  "http",
})
if err != nil {
    panic(err)
}
defer client.Close()
```

该模式下：

- 注册走 `POST /v1/catalog/register`
- 心跳走 `POST /v1/catalog/heartbeat`
- 发现走 `GET /v1/catalog/service/{name}`
- 实例下线走 `POST /v1/catalog/instance/status`
- SDK 订阅语义通过轮询实现
- 如启用动态成员同步，才会额外访问 `/v1/cluster/members`

### 4.3 QUIC 说明

`QUIC` 不是独立业务协议，而是 gRPC 的可选传输层。若需要在弱网场景下保留 gRPC 语义，可将 `Transport` 设置为 `quic`，并使用节点的 `quic_addr` 作为直连地址。

```go
client, err := eden.NewWithConfig(&eden.Config{
    Addresses:     []string{"127.0.0.1:10000"},
    Namespace:     "default",
    Datacenter:    "dc1",
    Transport:     "quic",
    DiscoveryMode: "static",
})
```

### 4.4 配置项说明

| 配置项 | 是否必填 | 说明 |
|---|---|---|
| `Addresses` | 是 | 入口地址列表；纯 gRPC/QUIC 模式填直接传输地址，HTTP 模式填 `http://` 地址 |
| `Transport` | 否 | `grpc`、`quic`、`http`；默认值为 `grpc` |
| `DiscoveryMode` | 否 | `auto` 或 `static`；`auto` 会按当前传输协议执行动态成员发现，`static` 会关闭成员同步 |
| `Namespace` | 否 | 命名空间 |
| `Datacenter` | 否 | 发现请求的数据中心过滤条件 |
| `APIKey` | 否 | 仅 HTTP 路径会自动写入 `X-API-Key` 请求头 |
| `CacheDir` | 否 | 本地缓存目录 |

## 5. 自定义客户端集成

### 5.1 自定义 HTTP 客户端

接口列表如下：

| 用途 | 方法 | 路径 |
|---|---|---|
| 注册实例 | `POST` | `/v1/catalog/register` |
| 心跳续约 | `POST` | `/v1/catalog/heartbeat` |
| 服务发现 | `GET` | `/v1/catalog/service/{name}` |
| 实例上下线 | `POST` | `/v1/catalog/instance/status` |
| 依赖拓扑上报 | `POST` | `/v1/catalog/topology/report` |
| 可选成员同步 | `GET` | `/v1/cluster/members` |

注册实例：

```bash
curl -X POST http://127.0.0.1:8500/v1/catalog/register \
  -H "Content-Type: application/json" \
  -d '{
    "id": "order-srv-1",
    "service_name": "order-service",
    "namespace": "default",
    "host": "127.0.0.1",
    "port": 8080,
    "weight": 100,
    "dc": "dc1",
    "metadata": {
      "version": "1.0.0"
    }
  }'
```

发送心跳：

```bash
curl -X POST http://127.0.0.1:8500/v1/catalog/heartbeat \
  -H "Content-Type: application/json" \
  -d '{
    "namespace": "default",
    "service_name": "order-service",
    "instance_id": "order-srv-1"
  }'
```

服务发现：

```bash
curl "http://127.0.0.1:8500/v1/catalog/service/user-center?passing=true&namespace=default&dc=dc1&consumer_service=order-center"
```

实例下线：

```bash
curl -X POST http://127.0.0.1:8500/v1/catalog/instance/status \
  -H "Content-Type: application/json" \
  -d '{
    "namespace": "default",
    "service_name": "order-service",
    "instance_id": "order-srv-1",
    "status": "offline"
  }'
```

说明：

- HTTP 路径支持 API Key。
- HTTP 路径当前没有与 gRPC `Watch` 对等的服务端推流接口；如需订阅语义，应由客户端自行轮询 `GET /v1/catalog/service/{name}`。
- `consumer_service` 查询参数可用于在发现链路中记录消费者与提供者关系。
- `/v1/cluster/members` 是可选接口，只在客户端需要动态维护节点列表时使用。

### 5.2 自定义 gRPC 客户端

协议入口如下：

- Proto 文件：`api/proto/registry/v1/registry.proto`
- 服务名：`eden.registry.v1.RegistryService`

RPC 列表如下：

| RPC | 类型 | 用途 |
|---|---|---|
| `Register` | Unary | 注册实例 |
| `Heartbeat` | Unary | 心跳续约 |
| `Discover` | Unary | 查询实例列表 |
| `Watch` | Server Streaming | 订阅实例变化 |
| `GetMembers` | Unary | 查询集群成员与节点地址 |
| `ReportTopology` | Unary | 上报消费者与提供者关系 |
| `SetInstanceStatus` | Unary | 设置实例为 `online` 或 `offline` |
| `Deregister` | Unary | 兼容接口，当前语义等价于下线实例 |

注册实例：

```bash
grpcurl -plaintext \
  -import-path ./api/proto \
  -proto registry/v1/registry.proto \
  -d '{
    "instance": {
      "id": "order-srv-1",
      "service_name": "order-service",
      "namespace": "default",
      "host": "127.0.0.1",
      "port": 8080,
      "weight": 100,
      "datacenter": "dc1",
      "metadata": {
        "version": "1.0.0"
      }
    }
  }' \
  127.0.0.1:9000 \
  eden.registry.v1.RegistryService/Register
```

服务发现：

```bash
grpcurl -plaintext \
  -H "x-consumer-service: order-center" \
  -import-path ./api/proto \
  -proto registry/v1/registry.proto \
  -d '{
    "namespace": "default",
    "service_name": "user-center",
    "healthy_only": true,
    "datacenter": "dc1"
  }' \
  127.0.0.1:9000 \
  eden.registry.v1.RegistryService/Discover
```

查询节点列表：

```bash
grpcurl -plaintext \
  -import-path ./api/proto \
  -proto registry/v1/registry.proto \
  -d '{}' \
  127.0.0.1:9000 \
  eden.registry.v1.RegistryService/GetMembers
```

订阅服务变化：

```bash
grpcurl -plaintext \
  -H "x-consumer-service: order-center" \
  -import-path ./api/proto \
  -proto registry/v1/registry.proto \
  -d '{
    "namespace": "default",
    "service_name": "user-center"
  }' \
  127.0.0.1:9000 \
  eden.registry.v1.RegistryService/Watch
```

上报依赖拓扑：

```bash
grpcurl -plaintext \
  -import-path ./api/proto \
  -proto registry/v1/registry.proto \
  -d '{
    "namespace": "default",
    "consumer_service": "order-center",
    "providers": ["user-center", "auth-center"],
    "checksum": "manual-demo"
  }' \
  127.0.0.1:9000 \
  eden.registry.v1.RegistryService/ReportTopology
```

说明：

- 自定义 gRPC 客户端可以通过 `GetMembers` 实现纯 gRPC 节点发现。
- 若希望记录消费者拓扑，可在 `Discover` 或 `Watch` 的 metadata 中附带 `x-consumer-service`。
- 如需主动、确定性地上报依赖关系，可直接调用 `ReportTopology`。
- 当前 gRPC 入口未实现与 HTTP 对齐的 API Key / JWT 拦截器；若生产环境需要统一鉴权，应在服务端补充 gRPC interceptor。

## 6. 常用运维调用

查看集群成员：

```bash
curl http://127.0.0.1:8500/v1/cluster/members
```

调整日志级别：

```bash
curl -X POST http://127.0.0.1:8500/v1/settings/storage \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "log_level": "DEBUG"
  }'
```

## 7. 选型建议

| 场景 | 推荐方案 |
|---|---|
| 只允许 gRPC 通信 | SDK 纯 gRPC 模式，或自定义 gRPC 客户端 |
| 只允许 HTTP 通信 | SDK 纯 HTTP 模式，或自定义 HTTP 客户端 |
| 需要实时订阅服务变化 | 优先使用 gRPC `Watch` |
| 需要与现有 REST 网关统一接入 | 优先使用 HTTP |
| 非 Go 语言接入 | 按语言生态选择自定义 HTTP 或自定义 gRPC 客户端 |

## 8. 相关文档

- [架构文档](./architecture_zh.md)

# Eden 用户使用说明

本文档旨在为用户提供清晰、准确的 Eden 分布式注册中心使用指南，包含快速启动、核心配置、认证授权方式以及客户端接入说明。

## 1. 快速启动

### 1.1 启动服务端进程

使用默认配置或指定配置文件启动注册中心：

```bash
go run ./cmd/server/main.go -config configs/config.yaml
```

**支持的命令行参数：**
- `-config`: 指定配置文件路径 (如 `configs/config.yaml`)。
- `-data-dir`: 指定数据持久化目录，其优先级高于配置文件。
- `-node-id`: 指定当前节点的唯一标识。
- `-http-addr`: 覆盖配置中的 HTTP 监听地址。

### 1.2 启动 Web 控制台

如果你需要使用可视化管理后台，可启动自带的前端控制台：

```bash
cd web
npm install
npm run dev
```

**默认地址：**
- 控制台访问地址：`http://127.0.0.1:2019`
- 服务端后台 API：`http://127.0.0.1:8500`

> 提示：开发模式下，前端默认会将 `/v1/*` 路径的请求代理至后端 HTTP API。


## 2. 核心配置说明

Eden 支持 `AP` (高可用) 和 `CP` (强一致) 两种集群模式，可通过修改 YAML 配置文件进行切换。

### 2.1 集群节点 (AP 模式)

在 AP 模式下，Eden 优先保证服务数据的高吞吐与可用性：

```yaml
node_id: "node-1"
mode: "cluster"
consistency: "ap"
http_addr: ":8500"
grpc_addr: ":0"       # 留空或设为 :0，系统将自动分配空闲端口
quic_addr: ""
data_dir: "./data"
transport:
  grpc: "auto"
  quic: "off"
  raft: "off"
```
**注意**：集群其他成员的信息统一在控制台进行维护和动态分发，无需在配置中使用 `seeds` 声明。

### 2.2 Leader 节点 (CP 模式)

在 CP 模式下，系统使用 Raft 协议保证强一致性。首个节点配置需开启 `bootstrap`：

```yaml
node_id: "node-1"
mode: "cluster"
consistency: "cp"
http_addr: ":8500"
grpc_addr: ":9000"
raft_addr: "127.0.0.1:7000"
bootstrap: true
data_dir: "./data/node-1"
```

### 2.3 Follower 节点 (CP 模式)

加入 CP 集群的普通节点配置示例：

```yaml
node_id: "node-2"
mode: "cluster"
consistency: "cp"
http_addr: ":8501"
grpc_addr: ":9001"
raft_addr: "127.0.0.1:7001"
data_dir: "./data/node-2"
```
**注意**：普通（Follower）节点启动后，需要在 Leader 节点的 Web 控制台中进行“添加节点 (Join)”操作。系统已弃用静态配置项中的 `join` 参数。


## 3. 认证授权机制

### 3.1 控制台登录

登录接口 `POST /v1/auth/login` 为了保证传输安全，不接受明文密码，需传递其 **SHA-256 哈希值**。

默认初始账户：
- 账户：`admin`
- 密码：`admin`
- 接口应提交的 SHA-256 密文：`8c6976e5b5410415bde908bd4dee15dfb167a9c873fc4bb8a81f6f2ab448a918`

### 3.2 HTTP API Key 鉴权

若服务端开启了 `auth.api_key.enabled=true` 拦截验证，以下重点接口调用时必须提供 API Key：
- `POST /v1/catalog/register`
- `POST /v1/catalog/heartbeat`
- `POST /v1/catalog/topology/report`

**参数传递规范：**
- **Header 方式 (推荐)**：追加 HTTP Header `X-API-Key: <your-key>`
- **Query 方式**：追加 URL 参数 `?api_key=<your-key>`


## 4. 官方 SDK 集成指南

官方 Go SDK 可以同时兼容极高效率的 gRPC 模式，以及备用的 HTTP 和 QUIC 协议。

### 4.1 纯 gRPC 接入 (推荐)

此模式要求直连注册中心暴露的 `grpc_addr` 端口阵列。建议启用 `DiscoveryMode="auto"` 从而开启服务注册中心集群动态成员发现能力。

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

### 4.2 纯 HTTP 接入

当应用环境受限，仅可使用基础的 HTTP/RESTful 通信时：

```go
client, err := eden.NewWithConfig(&eden.Config{
    Addresses:  []string{"http://127.0.0.1:8500", "http://127.0.0.1:8501"},
    Namespace:  "default",
    Datacenter: "dc1",
    Transport:  "http",
})
```
> 配置此模式后，SDK 缺少了底层的流控能力，会自动降级为长轮询刷新状态。

### 4.3 QUIC 接入 (弱网适用)

QUIC 是 gRPC 底层的替代传输层方案，针对复杂弱网或高丢包环境推荐使用 QUIC。在初始化时，确保直接指向节点的 `quic_addr`：

```go
client, err := eden.NewWithConfig(&eden.Config{
    Addresses:     []string{"127.0.0.1:10000"},
    Namespace:     "default",
    Datacenter:    "dc1",
    Transport:     "quic",
    DiscoveryMode: "static", // 依据业务情况灵活选择 auto/static
})
```


## 5. 多语言通用接入规范

当业务系统使用了未经官方 SDK 支持的框架或语言开发时，可自主对接本原生 API 标准。

### 5.1 原生 HTTP API 接入

- **注册实例**：`POST /v1/catalog/register`
- **心跳续约**：`POST /v1/catalog/heartbeat`
- **服务发现**：`GET /v1/catalog/service/{name}`
- **修改实例状态**：`POST /v1/catalog/instance/status`

**示例：以 Curl 注册微服务实例**
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
    "metadata": {"version": "1.0.0"}
  }'
```

### 5.2 原生 gRPC API 接入

基于底层协议 IDL 能够自由生成各类开发语言的通信桩代码。
- **Proto 契约文件**：`api/proto/registry/v1/registry.proto`
- **服务桩定义**：`eden.registry.v1.RegistryService`

核心方法：
- `Register` / `Heartbeat` / `Discover` (Unary 常规调用)
- `Watch` (Server Streaming 流式调用)
- `GetMembers` (用于自适应控制获取对等节点网络列表)

**示例：以 grpcurl 验证发起服务发现调用**
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

## 6. 常见运维命令

**查询当前集群成员拓扑**：
```bash
curl http://127.0.0.1:8500/v1/cluster/members
```

**动态调整系统存储层日志级别**：
```bash
curl -X POST http://127.0.0.1:8500/v1/settings/storage \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"log_level": "DEBUG"}'
```

## 7. 接入选型建议

| 业务场景 | 建议使用的接入方案 |
|---|---|
| 微服务内部通信，追求极低延迟和高性能 | 强烈建议：**官方 SDK + 纯 gRPC 模式**，使用 `Watch` 实时跟随系统状态。 |
| IoT设备或网络状况频繁断点重连、较高丢包 | 建议：**官方 SDK + QUIC 模式**。 |
| 未提供官方 SDK 的语言（如 Java/Python/Node等） | 依据开发语言引入 proto 自行生成 **定制 gRPC 客户端** 或退化采用 **RESTful HTTP 端点** 进行通信对接。 |


> 更多底层的通信交互逻辑、状态机实现、共识细节说明，请参阅同目录下的白皮书：[Eden 架构设计与技术规范](./architecture_zh.md)

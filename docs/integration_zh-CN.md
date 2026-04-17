# 芙卡洛斯接入与集成指南

## 1. 接入策略

对 Go 项目，统一推荐：

- 使用 [`pkg/sdk`](../pkg/sdk) 接入

其他接入路径是补充方案：

- 原生 HTTP API
- 原生 gRPC API
- Consul 兼容接入
- Nacos 兼容接入

如果是新项目，不建议先从兼容层开始；优先使用 `pkg/sdk`。

## 2. Go SDK 接入

### 2.1 推荐模式：gRPC

```go
client, err := sdk.NewWithConfig(&sdk.Config{
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

适用场景：

- Go 服务直接接入
- 希望使用订阅和自动成员发现
- 希望以统一 SDK 模型屏蔽协议细节

### 2.2 HTTP 模式

```go
client, err := sdk.NewWithConfig(&sdk.Config{
    Addresses:  []string{"http://127.0.0.1:8500"},
    Namespace:  "default",
    Datacenter: "dc1",
    Transport:  "http",
})
```

适用场景：

- 网络环境无法使用 gRPC
- 只需要基础注册、发现、心跳能力

约束：

- 订阅能力会退化为轮询

### 2.3 QUIC 模式

```go
client, err := sdk.NewWithConfig(&sdk.Config{
    Addresses:     []string{"127.0.0.1:10000"},
    Namespace:     "default",
    Datacenter:    "dc1",
    Transport:     "quic",
    DiscoveryMode: "static",
})
```

适用场景：

- 弱网环境
- 希望沿用 gRPC 语义但替换底层传输

## 3. SDK 使用模型

SDK 暴露的核心对象只有三类：

- `sdk.Config`
- `sdk.ServiceInstance`
- `sdk.Registry` / `sdk.Client`

典型调用流程：

1. 初始化客户端
2. 注册当前实例
3. 定时发送心跳
4. 发现依赖服务
5. 订阅依赖变更
6. 进程退出时注销实例

## 4. 原生 HTTP API

常用端点：

- `POST /v1/catalog/register`
- `POST /v1/catalog/heartbeat`
- `GET /v1/catalog/service/{name}`
- `POST /v1/catalog/instance/status`
- `POST /v1/catalog/topology/report`

示例：

```bash
curl -X POST http://127.0.0.1:8500/v1/catalog/register \
  -H "Content-Type: application/json" \
  -d '{
    "id": "order-srv-1",
    "service_name": "order-center",
    "namespace": "default",
    "host": "127.0.0.1",
    "port": 8080,
    "weight": 100,
    "datacenter": "dc1"
  }'
```

## 5. 原生 gRPC API

协议定义：

- Proto：[`api/proto/registry/v1/registry.proto`](../api/proto/registry/v1/registry.proto)
- Service：`eden.registry.v1.RegistryService`

核心方法：

- `Register`
- `Heartbeat`
- `Discover`
- `Watch`
- `GetMembers`
- `ReportTopology`

示例：

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

## 6. 兼容接入

### 6.1 Consul 兼容

适用对象：

- 已经使用 `github.com/hashicorp/consul/api` 的 Go 服务
- 现网依赖 Consul HTTP 接口，但短期不准备改业务侧注册逻辑

最小改动：

- 调整注册中心地址
- 保持现有 Consul 客户端调用模型不变

需要注意：

- 这里解决的是注册与发现迁移，不是完整 Consul 产品能力迁移。
- 如果现网使用了 Consul 的 KV、Service Mesh、ACL 体系等更宽能力，需要单独评估替代方案。

建议先看这些示例：

- [Consul 兼容示例说明](../examples/service-discovery/consul/README.md)
- [Consul 示例启动脚本](../examples/service-discovery/consul/start.bat)
- [服务发现示例总览](../examples/service-discovery/README.md)

### 6.2 Nacos 兼容

适用对象：

- 已经使用 `github.com/nacos-group/nacos-sdk-go/v2` Naming Client 的服务
- 业务侧希望保留现有 Naming Client 接入方式，只替换注册中心后端

最小改动：

- 调整服务端连接地址
- 保持注册、发现、订阅调用方式不变

需要注意：

- 这里针对的是 Naming 能力迁移，不包含 Nacos 配置中心能力。
- 如果现网同时依赖 Nacos Config，需要把注册发现迁移与配置迁移拆开处理。

建议先看这些示例：

- [Nacos 兼容示例说明](../examples/service-discovery/nacos/README.md)
- [Nacos 示例启动脚本](../examples/service-discovery/nacos/start.bat)
- [服务发现示例总览](../examples/service-discovery/README.md)

### 6.3 自定义协议接入

适用对象：

- 不使用仓库内 SDK
- 需要由其他语言或已有基础设施直接对接公开协议

可选路径：

- 直接调用公开 HTTP API
- 直接调用公开 gRPC API

建议按以下顺序阅读：

1. [服务发现示例总览](../examples/service-discovery/README.md)
2. [自定义协议示例说明](../examples/service-discovery/custom/README.md)
3. [`api/proto/registry/v1/registry.proto`](../api/proto/registry/v1/registry.proto)

## 7. 接入选型建议

| 场景 | 建议 |
| --- | --- |
| 新 Go 服务 | `pkg/sdk + grpc` |
| Go 服务但受限于网络环境 | `pkg/sdk + http` 或 `pkg/sdk + quic` |
| 非 Go 服务 | 优先 gRPC，其次 HTTP |
| 旧系统迁移 | Consul / Nacos 兼容接入 |

下一步阅读：

- [部署与运行](./deployment_zh-CN.md)
- [系统架构](./architecture_zh-CN.md)

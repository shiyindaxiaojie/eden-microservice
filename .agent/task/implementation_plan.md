# Eden Go Registry - 完整架构方案

## 一、节点同步 Bug 修复

### 根因（3个问题叠加）

1. **AP 广播缺认证**：`broadcast()` 裸 HTTP POST → 被 auth 中间件 401 拒绝
2. **Seeds 视角不对**：node1 存 `[8501, 8502]`，原样推给 node2，node2 peers 包含自身
3. **广播时机错**：standalone 下只本地存储，不广播

### 修复方案

新增 `/internal/sync/*` 端点（无认证），专用于节点间同步：

| 端点 | 用途 |
|------|------|
| `POST /internal/sync/seeds` | 同步节点列表 |
| `POST /internal/sync/users` | 同步用户 |
| `POST /internal/sync/apikeys` | 同步 API Key |
| `POST /internal/sync/settings` | 同步 mode/env |

修改 `SetSeedsWithReplication`：广播前**按目标节点视角转换** seeds（去掉目标自身地址，加上发起者地址）。

#### 涉及文件
- [MODIFY] `internal/handler/handler.go` — 注册内部路由
- [MODIFY] `internal/handler/cluster_handler.go` — 重写 `SetSeedsWithReplication` + 新增 `handleInternalSync*`
- [MODIFY] `internal/handler/settings_handler.go` — 用户/APIKey 同步走 `/internal/sync/*`
- [MODIFY] `internal/cluster/ap/node.go` — 广播 URL 改为内部端点

---

## 二、配置同步策略

**推荐：分层策略**

| 配置类型 | AP 模式 | CP 模式 |
|---------|---------|---------|
| 节点列表 | 内部 HTTP API | 内部 HTTP API（引导层在 Raft 之下） |
| 用户/APIKey | 内部 HTTP 广播 | Raft 日志复制（已有） |
| Mode/Env | 内部 HTTP 广播 | Raft 日志复制 |
| 服务注册数据 | 内部 HTTP 广播 | Raft 日志复制 |

> 节点列表不走 Raft：Raft 需要先知道成员才能启动（鸡生蛋问题），Seeds 是引导层。

---

## 三、Go SDK 设计

### 3.1 架构：接口抽象 + 适配器模式

```
pkg/registry/
├── registry.go          # 核心接口定义
├── eden/                # Eden 原生实现
│   └── client.go
├── consul/              # Consul API 适配器
│   └── adapter.go
├── nacos/               # Nacos API 适配器
│   └── adapter.go
└── factory.go           # 工厂方法，基于配置创建
```

### 3.2 核心接口

```go
package registry

type ServiceInstance struct {
    ID, ServiceName, Host string
    Port, Weight          int
    Metadata              map[string]string
    Healthy               bool
}

type Registry interface {
    Register(instance *ServiceInstance) error
    Deregister(instance *ServiceInstance) error
    Discovery(serviceName string) ([]*ServiceInstance, error)
    Subscribe(serviceName string, callback func([]*ServiceInstance)) error
    Heartbeat(instance *ServiceInstance) error
    Close() error
}

type Config struct {
    Type       string   // "eden", "consul", "nacos"
    Addresses  []string
    APIKey     string
    Namespace  string
    Datacenter string
}
```

### 3.3 适配层映射

| SDK 接口 | Eden 实现 | Consul 适配 | Nacos 适配 |
|---------|----------|------------|------------|
| `Register` | `POST /v1/catalog/register` | `Agent().ServiceRegister()` | `RegisterInstance()` |
| `Deregister` | `POST /v1/catalog/deregister` | `Agent().ServiceDeregister()` | `DeregisterInstance()` |
| `Discovery` | `GET /v1/catalog/service/{name}` | `Catalog().Service()` | `SelectInstances()` |
| `Heartbeat` | `POST /v1/catalog/heartbeat` | TTL `UpdateTTL()` | 内置心跳 |
| `Subscribe` | 轮询 / WebSocket | `Watch Plan` | `Subscribe()` |

### 3.4 示例应用

```
examples/service-discovery/
├── cmd/
│   ├── user-center/main.go      # 用户中心
│   ├── auth-center/main.go      # 认证中心
│   └── order-center/main.go     # 订单中心（跨服务调用）
├── config.yaml                   # type 一键切换
└── start.bat
```

---

## 四、gRPC 通信设计

### 4.1 Protobuf

```
api/proto/
├── registry/v1/registry.proto    # 客户端 ↔ 注册中心
└── cluster/v1/cluster.proto      # 节点 ↔ 节点
```

- 客户端：标准 gRPC (HTTP/2)，可选 QUIC
- 节点间：gRPC (Protobuf) 替换 `/internal/sync/*`
- CP Raft transport 保持原生 hashicorp/raft

### 4.2 渐进式实施

| 阶段 | 内容 |
|------|------|
| Phase 1 | 修复节点同步（内部 HTTP） |
| Phase 2 | SDK + gRPC 服务端 |
| Phase 3 | 节点间通信升级 gRPC |
| Phase 4 | QUIC 传输层 |

---

## 五、控制台驱动配置

启动只需：`node_id`, `http_addr`, `data_dir`，其余在控制台设置。
确保每种配置修改通过同步机制传播到所有节点。

# Protocol Architecture Documentation & SDK Enhancement

## Background

Eden 使用了 4 种通信协议（HTTP, gRPC, QUIC, Raft），职责分散在多个文件中，容易混淆。同时，SDK 存在多个关键问题导致主节点宕机后客户端无法自动转移：

1. **每次请求都创建新 gRPC 连接**（无连接池）
2. **节点发现间隔 2 分钟**（太慢，无法快速感知宕机）
3. **`Subscribe` 永不生效**（`grpcClient` 字段从未赋值）
4. **无服务 IP 缓存失效机制**（Pod 重启换 IP 后消费者仍用旧地址）
5. **失败后无即时标记 + 重试下一节点**

---

## Proposed Changes

### Part 1: Protocol Architecture Documentation

#### [NEW] [architecture_zh.md](file:///d:/Workspaces/Git/eden-go-registry/docs/architecture_zh.md)

创建完整的协议架构文档，包含：

1. **4 个协议的分层关系** — 哪个协议用于什么场景
2. **注册中心之间的通信** Mermaid 图（AP 广播、CP Raft、Anti-Entropy 全量同步）
3. **注册中心与客户端的通信** Mermaid 图（Register/Heartbeat/Discover/Watch）
4. **关键代码路径说明** — 从 SDK 到 Server 到 Cluster Node 的完整调用链
5. **协议-文件映射表** — 每个协议对应哪些文件、哪些端口

---

### Part 2: SDK Enhancement

#### [MODIFY] [client.go](file:///d:/Workspaces/Git/eden-go-registry/pkg/eden/client.go)

**改动 1: gRPC 连接池**

当前问题：每次 `Register`/`Heartbeat`/`Discovery` 都 `grpc.Dial()` + `defer conn.Close()`，大量连接创建开销。

修改方案：
- 在 `RegistryNode` 上维护一个复用的 `*grpc.ClientConn` 和 `pb.RegistryServiceClient`
- 连接失败时自动关闭旧连接、标记节点不健康、尝试重连
- 所有 gRPC 操作通过 `getGRPCClient(node)` 获取复用连接

```go
type RegistryNode struct {
    // ...existing fields...
    conn   *grpc.ClientConn          // Pooled gRPC connection
    client pb.RegistryServiceClient  // Cached client
    mu     sync.Mutex
    healthy bool                     // Health tracking
    lastFailure time.Time
}
```

**改动 2: 快速节点发现 + 健康感知**

当前问题：`startNodeDiscovery` 每 2 分钟同步一次，节点宕机后最长 2 分钟才能感知。

修改方案：
- 正常间隔从 2 分钟降低到 **30 秒**
- 失败后立即触发 `syncNodes()`（在 `executeWithFailover` 失败时）
- `syncNodes` 标记节点健康状态
- 优先使用健康节点，不健康的节点排到末尾

**改动 3: 改进 `executeWithFailover`**

当前问题：纯顺序遍历所有节点，失败后无标记。

修改方案：
- 失败时标记该节点为 `healthy=false`
- 健康节点排前面，不健康节点排后面
- 失败后异步触发一次 `syncNodes()` 刷新节点列表
- 主节点宕机时自动使用下一个健康节点

**改动 4: 修复 Subscribe + gRPC Watch 自动重连**

当前问题：`Subscribe` 检查 `c.grpcClient != nil`，但这个字段在 `NewWithConfig` 中从未被赋值，导致 Watch 模式永远不会启用。

修改方案：
- 使用连接池中的节点创建 Watch 流
- Watch 断开后自动重连到其他健康节点
- Fallback 到轮询模式（已有实现，保持兼容）

**改动 5: 服务缓存失效 — 解决 Pod 重启换 IP 问题**

场景：客户端 A 调用服务 B，B 在 Pod 中重启换了 IP。A 从注册中心拉取了旧 IP 缓存。

在注册中心层面，这已经通过 **TTL + Heartbeat** 机制解决：
- B 重启后用新 IP 重新注册
- 旧 IP 的实例因无心跳被 MarkCritical → 移除
- A 通过 `Subscribe`/`Discovery` 获取最新列表

但 SDK 侧需要确保：
- `Discovery` 结果不做本地永久缓存（每次都查询注册中心）
- `Subscribe` 能正确推送变更（上面已修复）
- 在 SDK 文档中建议用户使用 `Subscribe` 而非周期轮询

#### [MODIFY] [registry.go](file:///d:/Workspaces/Git/eden-go-registry/pkg/registry/registry.go)

无改动，接口保持不变。

---

### Part 3: Update Examples

#### [MODIFY] [main.go](file:///d:/Workspaces/Git/eden-go-registry/examples/service-discovery/cmd/user-center/main.go)

- 初始化时传入多个注册中心地址（`8500,8501,8502`）
- 演示 `Subscribe` 用法
- auth-center 和 order-center 同理

---

## User Review Required

> [!IMPORTANT]
> **SDK 连接管理策略**：计划将 gRPC 连接生命周期绑定到 `RegistryNode`（一个节点一个长连接）而非每次调用都创建新连接。这会改变连接管理模型。
>
> **向下兼容**：所有改动保持 `registry.Registry` 接口不变，只改 `eden.Client` 内部实现。

---

## Verification Plan

### Build Verification
```bash
cd d:\Workspaces\Git\eden-go-registry
go build ./...
```

### Manual Verification
由于没有现有测试用例，需要手动验证：

1. **启动 3 节点集群**：运行 `examples\cluster\start.bat`
2. **启动 service-discovery example**：运行 `examples\service-discovery\start.bat`
3. **验证注册成功**：打开 `http://127.0.0.1:8500` 控制台查看服务列表
4. **停掉主节点 8500**：关闭第一个节点的窗口
5. **验证 failover**：观察 service-discovery 的日志，确认心跳自动切换到 8501 或 8502
6. **验证 IP 变更感知**：重启一个 service （如 user-center），确认新 IP 被注册中心更新

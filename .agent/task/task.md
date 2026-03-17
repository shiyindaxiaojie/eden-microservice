# Eden Go Registry - 完整任务清单

## Phase 1：节点同步 Bug 修复
- [ ] 新增 `/internal/sync/*` 端点（无认证中间件）
  - [ ] `POST /internal/sync/seeds` — 接收并保存 seeds
  - [ ] `POST /internal/sync/users` — 接收并保存用户
  - [ ] `POST /internal/sync/apikeys` — 接收并保存 API Key
  - [ ] `POST /internal/sync/settings` — 接收并保存 mode/env
- [ ] 重写 `SetSeedsWithReplication`：按目标节点视角转换 seeds
- [ ] 修改 `ap/node.go` broadcast：广播 URL 改为 `/internal/sync/*`
- [ ] 测试：3 节点集群 seeds/users 同步

## Phase 2：Go SDK 设计与实现
- [ ] `pkg/registry/registry.go` — 核心接口 `Registry`
- [ ] `pkg/registry/factory.go` — 工厂方法 `NewRegistry(cfg)`
- [ ] `pkg/registry/eden/client.go` — Eden HTTP 实现
- [ ] `pkg/registry/consul/adapter.go` — Consul SDK 适配
- [ ] `pkg/registry/nacos/adapter.go` — Nacos SDK 适配
- [ ] 重构 `examples/service-discovery/`
  - [ ] `cmd/user-center/main.go` — 用户中心
  - [ ] `cmd/auth-center/main.go` — 认证中心
  - [ ] `cmd/order-center/main.go` — 订单中心（跨服务调用）
  - [ ] `config.yaml` — type 一键切换
  - [ ] `start.bat` / `start.sh`

## Phase 3：gRPC 服务端
- [ ] `api/proto/registry/v1/registry.proto` — 客户端通信协议
- [ ] `api/proto/cluster/v1/cluster.proto` — 节点间通信协议
- [ ] 生成 Go 代码（protoc）
- [ ] `internal/grpc/server.go` — gRPC 注册/发现/心跳/Watch
- [ ] `cmd/server/main.go` — 启动 gRPC listener
- [ ] 更新 SDK：`eden/client.go` 支持 gRPC 连接

## Phase 4：节点间 gRPC 通信
- [ ] `internal/grpc/cluster_server.go` — 节点间同步服务
- [ ] 替换 `/internal/sync/*` HTTP 为 gRPC 调用
- [ ] AP 广播改为 gRPC streaming
- [ ] CP Raft transport 保持 hashicorp/raft 原生

## Phase 5：QUIC 传输层
- [ ] 集成 `quic-go` 库
- [ ] gRPC over QUIC transport 实现
- [ ] SDK 配置项：`transport: grpc | quic`
- [ ] 弱网测试

## Phase 6：控制台驱动配置优化 [x]
- [x] 启动参数最小化（node_id, http_addr, data_dir）
- [x] 确保所有设置变更通过同步机制传播
  - [x] 更新 Proto 定义（添加 log_level）
  - [x] 实现 gRPC SyncSettings 逻辑
  - [x] 实现 AP 模式下的同步广播
- [x] 日志级别热更新
  - [x] 扩展 Registry Store 支持 log_level 持久化
  - [x] 实现 Logger 级别动态调整
  - [x] 更新 Settings Handler 支持 log_level 修改

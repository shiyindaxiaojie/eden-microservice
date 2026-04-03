# Eden

Eden 是一个基于 Go 实现的轻量级服务注册与发现中心，支持 AP 与 CP 两种一致性模式，并同时提供 HTTP、gRPC、QUIC 与控制台管理能力。

## 特性

- **容错与一致性**：AP 模式基于 gRPC 异步复制与反熵同步，强调高可用与分区容忍；CP 模式基于 Raft 共识，强调元数据强一致。
- **多协议支持**：官方 SDK 默认以 gRPC 作为业务协议，并将其完全切换为 HTTP 或实验性的 QUIC。客户端既可使用官方 `pkg/eden` SDK，也支持标准的 HTTP 或 gRPC 接口自行接入。
- **交互式 Web 控制台**：内置支持中英文双语 (i18n) 的现代化管理界面，提供服务注册节点管理、依赖拓扑可视化以及高级配置功能。
- **动态告警与事件系统**：具备基于滑动时间窗口的事件评估器，支持设置事件触发阈值并动态匹配告警规则。
- **多渠道系统通知**：支持跨平台告警投递，囊括飞书、钉钉、企业微信等常见 Webhook，并兼容 SMTP 邮件推送。
- **认证及安全控制**：内置基于 JWT 的无状态登录体系与基于 API Key 的网关签权。

## 快速开始

启动服务端：

```bash
go run ./cmd/server/main.go -config configs/config.yaml
```

启动前端控制台：

```bash
cd web
npm install
npm run dev
```

默认开发地址：

- 后端 API：`http://127.0.0.1:8500`
- 前端控制台：`http://127.0.0.1:2019`

## 目录结构

- `api/proto`：gRPC 协议定义
- `cmd/server`：服务端入口
- `internal/alert`：告警规则管理与事件评估中心
- `internal/catalog`：注册应用状态机与服务元信息管理
- `internal/cluster`：集群核心实现（AP / CP）
- `internal/notify`：系统通知渠道与发送引擎
- `internal/service`：业务服务层
- `internal/settings`：系统全局配置中心
- `internal/store`：存储与持久化实现
- `internal/transport`：网络传输层 (HTTP / gRPC)
- `pkg`：客户端 SDK 
- `web`：前端现代交互式控制台

## 文档

`docs` 目录仅保留两份主文档：

- [架构文档](./docs/architecture_zh.md)
- [用户手册](./docs/user_guide_zh.md)

## License

MIT

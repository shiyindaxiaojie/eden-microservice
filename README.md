# Eden

Eden 是一个基于 Go 实现的轻量级服务注册与发现中心，支持 AP 与 CP 两种一致性模式，并同时提供 HTTP、gRPC、QUIC 与控制台管理能力。

## 特性

- AP 模式基于 gRPC 异步复制与反熵同步，强调高可用与分区容忍。
- CP 模式基于 Raft 共识，强调元数据强一致。
- 官方 SDK 默认以 gRPC 作为业务协议，也可完全切换为 HTTP。
- 客户端既可以使用官方 `pkg/eden` SDK，也可以基于 HTTP 或 gRPC 协议自行接入。
- 提供 Web 控制台、JWT 登录、RBAC、API Key、命名空间与依赖拓扑能力。

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
- `internal/cluster`：集群核心实现（AP / CP）
- `internal/service`：业务服务层
- `internal/store`：存储与持久化实现
- `pkg`：客户端 SDK 与第三方注册中心适配层
- `web`：前端控制台

## 文档

`docs` 目录仅保留两份主文档：

- [架构文档](./docs/architecture_zh.md)
- [用户手册](./docs/user_guide_zh.md)

## License

MIT

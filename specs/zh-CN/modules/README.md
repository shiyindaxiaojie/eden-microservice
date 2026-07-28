# Monorepo 模块边界规范

## 1. 工作区

根目录 `go.work` 组合六个后端 module 和一个共享 module：

| 模块 | Go module | 责任 |
| --- | --- | --- |
| 注册中心 | `eden-microservice/apps/registry` | 注册、发现、健康、拓扑、Naming/Consul 兼容、Go SDK |
| 配置中心 | `eden-microservice/apps/config` | 配置资源、历史、监听、Nacos Config 兼容 |
| 网关路由 | `eden-microservice/apps/gateway` | 路由模型、发布策略、匹配、上游选择与代理运行时 |
| 权限控制 | `eden-microservice/apps/auth` | 登录、用户、角色与 API Key |
| 集群管理 | `eden-microservice/apps/cluster` | AP 复制、CP Raft、成员关系与运行设置 |
| 聚合服务 | `eden-microservice/apps/server` | 组合领域模块，装配统一 HTTP、gRPC、QUIC 和网关监听器 |
| 共享包 | `eden-microservice/packages` | 进程配置、加密、指标、复制协议和通用传输基础设施 |

控制台位于 `apps/ui`，不单独加入 Go workspace。

## 2. 目录约定

每个领域 module 使用以下结构：

```text
apps/<domain>
├─ go.mod
├─ api                 # 允许其他 module import 的稳定类型和协议
├─ module              # 容器、构造函数和组合入口
├─ internal            # 仅本 module 可见的领域实现
└─ cmd/<domain>        # 独立进程入口
```

聚合入口固定为 `apps/server/cmd/server`，运行时装配属于 `apps/server/module`。物理目录不重复
领域名或产品名前缀。聚合进程的 YAML 配置由 `apps/server/config` 持有；配置中心领域代码仍归
`apps/config`，共享的进程配置解析代码仍归 `packages/config`。

## 3. 依赖方向

- 任何 module 都不得 import 另一个 module 的 `internal`。
- 跨模块只依赖能力拥有方的 `api` 或 `module` 公共面；聚合 server 可以依赖全部公共面。
- 网关通过注册中心 `api/catalog` 的服务发现契约选择健康实例，不直接修改注册状态。
- 集群复制通过注册中心、权限控制的公共类型和共享复制协议同步聚合状态。
- `packages` 不依赖 `apps`；业务策略、HTTP 路由和持久化资源模型留在所属领域。
- 控制台和外部 API 路径保持现有契约，目录拆分不得改变兼容协议的响应形状。

## 4. 构建与验证

根目录先执行 `go work sync`。每个 Go module 必须能在自身目录运行 `go test ./...`；共享包变更
需要验证全部业务 module。结构契约测试必须确认 module 清单完整，并拒绝跨 module 的
`internal` import。

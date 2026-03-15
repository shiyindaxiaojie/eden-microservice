---
name: backend-common
description: Core Go backend development guidelines, project structure, patterns, and coding standards for eden-go-registry.
---

# Backend Common Skill

This skill covers the foundational rules for backend development in `eden-go-registry`.

## 1. Project Structure

- `cmd/`: Entry points (main.go).
- `configs/`: Configuration files (config.yaml, config.yaml.example).
- `internal/`: Private code.
  - `cluster/ap/`: AP mode (eventual consistency, peer-to-peer broadcast).
  - `cluster/cp/`: CP mode (strong consistency, Raft consensus).
  - `config/`: Configuration loading (Viper).
  - `handler/`: HTTP handlers, split by domain (see Handler 规范).
  - `health/`: Health checker (TTL-based).
  - `model/`: Data models.
  - `pkg/crypto/`: Internal crypto utilities (SHA256).
  - `store/`: In-memory registry store.
- `pkg/`: Public libraries.
- `web/`: Frontend (Vue 3).

## 2. Design Pattern Requirements

> **Every一个改动都必须遵循以下设计原则，无一例外。**

### 2.1 Single Responsibility Principle (SRP)

- **一个文件只做一类事**。Handler 文件按业务领域拆分，禁止在单个文件中混合多个领域的处理逻辑。
- 文件行数超过 200 行时，必须审视是否可以继续拆分。
- 工具函数（如 HTTP 响应）必须独立成文件（如 `response.go`）。

### 2.2 Handler 文件拆分规范

Handler 层必须按 **业务领域** 拆分为独立文件：

| 文件 | 职责 |
|------|------|
| `handler.go` | Handler 结构体、构造函数、路由注册、CORS |
| `response.go` | 统一 HTTP 响应工具（httpError, jsonOK） |
| `auth_handler.go` | JWT/RBAC/APIKey 中间件 + 登录 + Token 生成 |
| `catalog_handler.go` | 服务注册、注销、心跳、发现 |
| `cluster_handler.go` | 集群加入、成员列表、统计、事件流 |
| `settings_handler.go` | 用户管理、API Key 管理、一致性模式切换 |

**新增业务领域时**，必须新建独立的 `xxx_handler.go` 文件，**禁止往现有文件追加不相关的 handler**。

### 2.3 Interface-Based Design

- 核心组件（Store、Cluster Node）应定义接口以支持可测试性。
- 依赖注入通过构造函数参数传入，禁止使用全局变量。

### 2.4 Package Naming

- 使用单数形式：`config`（非 `configs`）、`model`（非 `models`）。
- 按功能模块命名：`crypto`（非 `utils`）。
- 禁止使用 `common`、`util`、`helper` 等泛泛的包名。

## 3. Configuration Rules

- 配置文件位于 `configs/config.yaml`。
- 示例配置位于 `configs/config.yaml.example`，敏感信息使用 `<placeholder>` 占位。
- 配置加载使用 Viper，支持 YAML + 环境变量（前缀 `REGISTRY_`）。
- 新增配置字段时，必须同步更新：
  1. `internal/config/config.go` 结构体
  2. `configs/config.yaml` 默认值
  3. `configs/config.yaml.example` 占位符

## 4. Error Handling & Response

- 使用统一的 `httpError(w, code, msg)` 返回错误。
- 使用统一的 `jsonOK(w, data)` 返回成功。
- 所有 Handler 方法的第一步是校验 HTTP Method。
- 错误信息面向开发者，不应泄露敏感内部信息。

## 5. Consistency Mode

- **AP Mode**: 最终一致，通过 HTTP 广播到 seed 节点。
- **CP Mode**: 强一致，通过 Raft 共识复制日志。
- 两种模式的 Node 在启动时均初始化，支持在线切换。
- Handler 中通过 `registry.GetMode()` 判断当前模式，分别调用 `apNode.Apply` 或 `cpNode.Apply`。

## 6. Code Style

- 使用 `go fmt` 格式化代码。
- 使用 `go vet` 做静态分析。
- 注释使用英文，简洁明了。
- 导出函数必须有 godoc 注释。

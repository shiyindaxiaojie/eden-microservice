# Eden Go Registry — 四大功能增强实现计划

本计划涵盖：命名空间隔离、服务列表 UI 优化、服务依赖关系图、客户端本地缓存。

## User Review Required

> [!IMPORTANT]
> **命名空间对 SDK 的兼容性设计**：命名空间作为隐藏可选项，`pkg/eden/client.go` 的现有 API 签名不变。新增 `Config.Namespace` 字段（默认空字符串 = "default" 命名空间）。未设置命名空间的客户端自动归属 "default" 命名空间，确保替换 Consul 代码不受影响。

> [!WARNING]
> **上线/下线操作 vs 注销**：原来的"注销"(Deregister) 是从注册中心完全移除实例。新的"下线"(Offline) 是将实例标记为不可用但仍保留在注册中心，并通知客户端停止心跳。"上线"(Online) 是恢复实例并通知客户端拉取最新数据继续心跳。这两个操作语义不同，需要确认是否保留原有的 Deregister 功能。

---

## Proposed Changes

### Feature 1: Namespace Isolation (命名空间隔离)

#### 后端改动

##### [NEW] [namespace_store.go](file:///d:/Workspaces/Git/eden-go-registry/internal/store/namespace_store.go)
- 新建 `NamespaceStore`，管理命名空间的 CRUD
- 数据持久化到 `data/namespace.json`
- 结构体：`Namespace { ID, Name, Description, CreatedAt, UpdatedAt }`
- 默认初始化包含 "default" 命名空间

##### [MODIFY] [service.go](file:///d:/Workspaces/Git/eden-go-registry/internal/model/service.go)
- `Instance` 结构体新增 `Namespace string` 字段（`json:"namespace,omitempty"`），默认空字符串表示 "default"

##### [MODIFY] [service_store.go](file:///d:/Workspaces/Git/eden-go-registry/internal/store/service_store.go)
- 内部 map 键增加命名空间维度：`services` 改为 `map[string]map[string]map[string]*Instance` (`namespace -> serviceName -> instanceID -> Instance`)
- 为保持向后兼容，空 namespace 自动映射到 "default"
- `ListServices`、`GetService` 等方法增加可选 namespace 参数

##### [MODIFY] [registry_store.go](file:///d:/Workspaces/Git/eden-go-registry/internal/store/registry_store.go)
- `Registry` 增加 `Namespaces *NamespaceStore` 字段
- `NewRegistry` 中初始化 `NamespaceStore`
- 新增便捷方法：`ListNamespaces`、`CreateNamespace`、`DeleteNamespace`

##### [NEW] [namespace_handler.go](file:///d:/Workspaces/Git/eden-go-registry/internal/handler/namespace_handler.go)
- REST API 路由：
  - `GET /v1/namespaces` — 列出所有命名空间
  - `POST /v1/namespace` — 创建命名空间
  - `PUT /v1/namespace` — 更新命名空间
  - `DELETE /v1/namespace?name=xxx` — 删除命名空间

##### [MODIFY] [handler.go](file:///d:/Workspaces/Git/eden-go-registry/internal/handler/handler.go)
- 注册命名空间相关路由

##### [MODIFY] [registry.proto](file:///d:/Workspaces/Git/eden-go-registry/api/proto/registry/v1/registry.proto)
- `ServiceInstance` message 新增 `string namespace = 9;`
- `DiscoverRequest` 新增 `string namespace = 4;`
- `WatchRequest` 新增 `string namespace = 2;`

#### 前端改动

##### [NEW] [namespace.vue](file:///d:/Workspaces/Git/eden-go-registry/web/src/views/namespace.vue)
- 命名空间管理页面，含列表展示、新增、编辑、删除功能
- 遵循 frosted-glass 主题和 `frontend-common` SKILL 布局规范

##### [MODIFY] [index.ts (router)](file:///d:/Workspaces/Git/eden-go-registry/web/src/router/index.ts)
- 新增 `/namespaces` 路由

##### [MODIFY] [App.vue](file:///d:/Workspaces/Git/eden-go-registry/web/src/App.vue)
- 侧边栏增加"命名空间"菜单项

##### [NEW] [namespace/index.ts (API)](file:///d:/Workspaces/Git/eden-go-registry/web/src/api/namespace/index.ts)
- Namespace CRUD API 调用封装

#### SDK 改动（最小化）

##### [MODIFY] [client.go](file:///d:/Workspaces/Git/eden-go-registry/pkg/eden/client.go)
- `Config` 结构体新增 `Namespace string`（已有？检查 `registry.Config` 已有 `Namespace` 字段，仅需透传）
- Register/Discovery/Watch 请求中透传 namespace（仅在 namespace 非空时）
- **不改变现有 API 签名**，完全向后兼容

---

### Feature 2: Service List UI Optimization (服务列表优化)

#### 后端改动

##### [MODIFY] [service_store.go](file:///d:/Workspaces/Git/eden-go-registry/internal/store/service_store.go)
- `ListServices` 返回增加 `SubscriberCount` 字段
- 新增 `GetSubscribers(serviceName) []SubscriberInfo` 方法

##### [MODIFY] [catalog.go](file:///d:/Workspaces/Git/eden-go-registry/internal/service/catalog.go)
- `ListServices` 返回 `ServiceSummary`（含 subscriber count）
- 新增 `SetInstanceStatus(serviceName, instanceID, status)` 用于上线/下线
- 新增 `GetSubscribers(serviceName)` 方法
- 上线/下线时通过 `NotifyFunc` 通知订阅者

##### [MODIFY] [interfaces.go](file:///d:/Workspaces/Git/eden-go-registry/internal/service/interfaces.go)
- `CatalogService` 接口增加 `SetInstanceStatus`、`GetSubscribers` 方法

##### [MODIFY] [catalog_handler.go](file:///d:/Workspaces/Git/eden-go-registry/internal/handler/catalog_handler.go)
- 新增 `handleSetInstanceStatus` — `POST /v1/catalog/instance/status`
  - body: `{ service_name, instance_id, status: "online"|"offline" }`
- 新增 `handleGetSubscribers` — `GET /v1/catalog/service/{name}/subscribers`

##### [MODIFY] [handler.go](file:///d:/Workspaces/Git/eden-go-registry/internal/handler/handler.go)
- 注册新路由

##### [MODIFY] [registry.proto](file:///d:/Workspaces/Git/eden-go-registry/api/proto/registry/v1/registry.proto)
- 新增 `rpc SetInstanceStatus` — 用于上线/下线通知 gRPC 客户端
- `WatchResponse` 增加 `string action` 字段（"update" | "offline" | "online"）

#### 前端改动

##### [MODIFY] [services.vue](file:///d:/Workspaces/Git/eden-go-registry/web/src/views/services.vue)
- 重新设计搜索栏：支持服务名称、IP、健康状态筛选
- 增加视图切换按钮（网格视图 / 列表视图）
- 网格视图：保留现有卡片样式，优化展示
- 列表视图：表格形式，含服务名、实例数、健康数、订阅者数
- 优化交互设计，增加统计面板

##### [MODIFY] [service-detail.vue](file:///d:/Workspaces/Git/eden-go-registry/web/src/views/service-detail.vue)
- "注销"按钮改为"下线"/"上线"按钮（根据实例当前状态显示）
- 增加订阅者数量列，点击弹出订阅者列表对话框
- 增加搜索过滤（IP、健康状态）

##### [MODIFY] [index.ts (API)](file:///d:/Workspaces/Git/eden-go-registry/web/src/api/registry/index.ts)
- 新增 `setInstanceStatus` API 调用
- 新增 `getSubscribers` API 调用
- `ServiceSummary` 增加 `subscriber_count` 字段

---

### Feature 3: Service Dependency Graph (服务依赖关系图)

#### 后端改动

##### [MODIFY] [catalog.go](file:///d:/Workspaces/Git/eden-go-registry/internal/service/catalog.go)
- 维护一个依赖关系 map: `dependencies map[consumerService][]providerService`
- Subscribe 操作记录依赖关系（消费方 → 提供方）
- 新增 `GetDependencyGraph(namespace)` 返回 `{ nodes: [], edges: [] }`

##### [MODIFY] [interfaces.go](file:///d:/Workspaces/Git/eden-go-registry/internal/service/interfaces.go)
- `CatalogService` 接口增加 `GetDependencyGraph(namespace string)` 方法

##### [MODIFY] [catalog_handler.go](file:///d:/Workspaces/Git/eden-go-registry/internal/handler/catalog_handler.go)
- 新增 `handleDependencyGraph` — `GET /v1/catalog/dependency-graph?namespace=xxx`

##### [MODIFY] [handler.go](file:///d:/Workspaces/Git/eden-go-registry/internal/handler/handler.go)
- 注册依赖图路由

#### 前端改动

##### [NEW] [dependency-graph.vue](file:///d:/Workspaces/Git/eden-go-registry/web/src/views/dependency-graph.vue)
- 使用 D3.js（或 vis-network）渲染服务依赖关系图
- 支持按命名空间切换
- 节点显示服务名称 + 实例数
- 边显示 provider → consumer 关系
- 支持拖拽、缩放

##### [MODIFY] [index.ts (router)](file:///d:/Workspaces/Git/eden-go-registry/web/src/router/index.ts)
- 新增 `/dependency-graph` 路由

##### [MODIFY] [App.vue](file:///d:/Workspaces/Git/eden-go-registry/web/src/App.vue)
- 侧边栏增加"依赖关系"菜单项

##### [MODIFY] [index.ts (API)](file:///d:/Workspaces/Git/eden-go-registry/web/src/api/registry/index.ts)
- 新增 `getDependencyGraph` API 调用

---

### Feature 4: Local Cache Mechanism (本地缓存机制)

#### SDK 改动

##### [NEW] [cache.go](file:///d:/Workspaces/Git/eden-go-registry/pkg/eden/cache.go)
- 本地文件缓存管理器 `LocalCache`
- 缓存路径：`~/.eden/cache/services.json`（可配置）
- 方法：
  - `Load() map[string][]*ServiceInstance` — 从本地文件加载缓存
  - `Save(map[string][]*ServiceInstance)` — 保存到本地文件
  - `Get(serviceName) []*ServiceInstance` — 获取某服务的缓存实例
  - `Update(serviceName, instances)` — 更新某服务的缓存
- 缓存文件带 TTL 校验，过期缓存有 warning 日志但仍可用

##### [MODIFY] [client.go](file:///d:/Workspaces/Git/eden-go-registry/pkg/eden/client.go)
- `Config` 新增 `CacheDir string`（可选，空表示禁用本地缓存）
- `Client` 新增 `cache *LocalCache` 字段
- **启动流程改为**：
  1. 先从本地缓存加载服务列表
  2. 尝试连接注册中心（同步 syncNodes）
  3. 如果连接失败，日志 warn 并继续使用缓存数据
- **Discovery 改为**：
  1. 正常走注册中心查询
  2. 成功后自动更新本地缓存
  3. 所有节点查询失败时 fallback 到本地缓存
- **Subscribe 改为**：
  - 收到推送时自动更新本地缓存
- **背景线程**：
  - 定期（如每 60 秒）全量更新本地缓存
- **所有变更对不启用缓存的用户透明** — `CacheDir` 为空时跳过所有缓存逻辑

---

## Verification Plan

### Automated Tests

由于项目目前无单元测试，以下为新增验证方案：

1. **Backend 编译验证**
   ```bash
   cd d:\Workspaces\Git\eden-go-registry
   go build ./...
   ```

2. **Proto 编译验证**（如有 protoc 环境）
   ```bash
   protoc --go_out=. --go-grpc_out=. api/proto/registry/v1/registry.proto
   ```

3. **前端编译验证**
   ```bash
   cd d:\Workspaces\Git\eden-go-registry\web
   npm run build
   ```

### Manual Verification

1. **命名空间功能**：
   - 启动服务端，打开前端页面
   - 进入"命名空间"菜单页面
   - 创建新命名空间
   - 确认 `data/namespace.json` 文件已生成
   - 删除命名空间后确认文件更新

2. **服务列表优化**：
   - 注册多个服务实例
   - 在前端验证：按名称搜索、按 IP 搜索、按健康状态筛选
   - 切换网格和列表视图
   - 对某实例执行"下线"，验证客户端停止心跳
   - 对下线实例执行"上线"，验证客户端恢复心跳
   - 点击订阅者数量查看订阅者列表

3. **依赖关系图**：
   - 启动多个示例服务（order-center、user-center）
   - order-center Subscribe user-center
   - 打开依赖关系图页面，验证显示正确的 provider/consumer 关系

4. **本地缓存**：
   - 配置 `CacheDir` 启动客户端
   - 注册并发现服务，验证缓存文件生成
   - 停止注册中心
   - 重启客户端，验证从缓存加载成功
   - 注册中心恢复后验证缓存自动更新

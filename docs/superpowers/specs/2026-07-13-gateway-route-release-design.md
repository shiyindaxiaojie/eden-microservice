# 网关路由与流量发布设计

## 目标

将现有仅使用本地 mock 的“路由管理”改为真实的“网关路由”模块，提供可持久化的 HTTP 反向代理，并让一条路由能绑定多个注册服务或静态上游。管理员可通过权重、金丝雀、BETA 名单和蓝绿切换控制流量。

## 设计决定

采用“路由内多发布目标 + 统一流量策略”，而非用多条相似路由和优先级拼接灰度。路由继续负责 host/path/method/header 匹配；目标负责服务发现或静态地址；流量策略只负责在目标间分配请求。这样路由匹配、发布选择、实例负载均衡是三个独立层级。

每个服务目标完整保存 `namespace`、`group` 与 `service_name`，不会把 catalog 的 `group@@service` 兼容键泄漏到控制台或 API。服务从 `/v1/catalog/services` 选择；静态目标保存经验证的 HTTP(S) endpoint。

## 领域边界

`internal/gateway` 拥有路由模型、持久化、验证、历史、快照、匹配、目标选择、代理和运行时统计。它通过小型服务发现接口读取 `catalog.Registry`，但不修改注册中心实例状态。`internal/transport/http` 只负责管理 API、鉴权和 JSON 编解码；`cmd/server` 装配独立数据面监听器。控制台仅调用原生 API，不保留 localStorage mock 作为路由数据源。

## 数据与发布模型

路由的稳定身份是 `namespace + id`。资源包含标准元数据、匹配条件、过滤器、超时、目标数组和流量策略。目标 ID 在路由内唯一。发布权重位于目标层之上：例如 `stable=95, canary=5`；服务实例权重只在一个服务目标内部选择实例。可信身份或 `X-Request-ID` 可获得稳定哈希分流；没有这类键的请求使用路由内计数器，防止共享 ingress 地址把权重压成单一目标。

BETA 使用可信反向代理提供的用户/租户 Header。用户匹配优先于租户匹配，同一身份不能同时指向多个目标。蓝绿模式保存唯一活动目标；BETA 可在切换前固定导流至非活动颜色。所有策略变更是带 revision 的原子路由更新，且运行时通过 `atomic.Value` 交换排序后的不可变快照。

## 数据面

数据面默认独立监听 `gateway.http`，仅当 `gateway.enabled=true` 时启动。请求执行：匹配路由、选择发布目标、解析健康实例或静态 endpoint、运行请求过滤器、反向代理并写入运行时指标和脱敏访问日志。控制端口与数据面端口共用时不在本期支持；启动会拒绝 wildcard/具体地址重叠的监听配置，这样可以保证 `/v1/*`、`/internal/*` 和兼容路径不会被通配路由吞掉。

返回码固定为：未匹配 `404`、没有可用服务实例 `503`、上游超时 `504`、其他代理失败 `502`。选中的发布目标不可用时不隐式回落到 stable，避免发布问题被隐藏。

## 安全与可观测性

BETA Header 仅在远端地址属于 `trusted_proxy_cidrs` 时受信；默认仅回环地址，生产环境需要明确配置。非可信来源携带的 BETA Header 会在代理至上游前剥离。URL 禁止 userinfo，日志与指标不记录 query、Authorization、Cookie、API Key 或完整身份 Header。运行时状态独立于控制面保存状态，并提供请求数、错误数、最近状态、目标健康摘要和数据面监听是否启用。

## API 与控制台

管理 API 位于 `/v1/gateway/*`。读取允许 admin/developer/viewer；写入仅 admin。路由写入采用 POST 创建、PUT 更新（含 expected_revision）、DELETE 删除、POST 启停动作。控制台的“网关路由”页提供服务列表选择器、静态 endpoint 编辑、精确/前缀路径选择、服务端分页、目标权重表、BETA 用户/租户名单与蓝绿活动目标切换。

## 验证策略

测试首先覆盖模型验证、持久化与 revision 冲突、BETA 优先级、稳定权重分配、蓝绿切换、服务发现和静态反向代理错误码。HTTP handler 测试覆盖授权后的 CRUD 形状，前端使用 TypeScript 类型检查和生产构建验证。

# HTTP API 规范

## 1. 路径分类

| 分类 | 路径 | 用途 |
| --- | --- | --- |
| Console/native | `/v1/*` | 控制台和自定义客户端 |
| Internal | `/internal/*` | 节点间同步 |
| Health | `/health` | 存活检查 |
| Nacos compatibility | `/nacos/v1/*`、`/v1/ns/*` | Nacos 客户端兼容 |
| Consul compatibility | `/v1/agent/*`、`/v1/health/*` | Consul 客户端兼容 |

## 2. 方法语义

| 方法 | 用途 | 幂等 |
| --- | --- | --- |
| `GET` | 查询 | 是 |
| `POST` | 创建、发布、动作 | 通常否 |
| `PUT` | 更新 | 是 |
| `DELETE` | 删除 | 是 |

## 3. 响应规则

Native API 使用 JSON 响应，错误使用当前 HTTP 层的错误格式。Compatibility API 必须优先匹配对应生态客户端的返回格式。

Naming 原生注册、发现、心跳和实例状态请求使用独立的 `group` 字段。调用方不提供 `group` 时，服务端在进入存储前将其归一化为 `default`；Nacos Naming 适配层将 `groupName` 映射到该字段，并继续按 Nacos 约定使用 `DEFAULT_GROUP` 作为默认值。服务名本身不得向原生 API 或控制台暴露 `group@@serviceName` 形式。

拓扑依赖来自运行时发现、订阅或 `/v1/catalog/topology/report` 主动上报，单独注册服务不会推断依赖。兼容客户端上报未携带 group 的服务名时，服务端先按 `default` 分组解析；该分组不存在且名称在命名空间内只对应一个分组时，再解析为唯一的分组服务身份。存在多个无法消歧的同名分组时，调用方必须提供带分组的服务身份。

Config 兼容示例：

| 场景 | 返回 |
| --- | --- |
| 查询成功 | 直接返回配置 content |
| 发布成功 | `true` |
| 删除成功 | `true` |
| 长轮询无变化 | 空响应或兼容客户端可识别的空变化列表 |

## 4. 鉴权

管理 API 默认需要登录态。客户端运行 API 可以使用 API Key、兼容协议认证或后续扩展认证。内部同步 API 不对公网暴露，部署时应限制访问来源。

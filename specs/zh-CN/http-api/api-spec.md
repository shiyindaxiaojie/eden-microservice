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

Config 兼容示例：

| 场景 | 返回 |
| --- | --- |
| 查询成功 | 直接返回配置 content |
| 发布成功 | `true` |
| 删除成功 | `true` |
| 长轮询无变化 | 空响应或兼容客户端可识别的空变化列表 |

## 4. 鉴权

管理 API 默认需要登录态。客户端运行 API 可以使用 API Key、兼容协议认证或后续扩展认证。内部同步 API 不对公网暴露，部署时应限制访问来源。


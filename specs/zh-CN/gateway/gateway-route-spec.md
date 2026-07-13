# Gateway 路由规范

## 1. RouteResource

路由资源字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `namespace` | string | 命名空间，默认 `default` |
| `id` | string | 路由 ID |
| `name` | string | 展示名 |
| `enabled` | bool | 是否启用 |
| `priority` | int | 优先级，数值越小越先匹配 |
| `match` | object | 匹配条件 |
| `targets` | []object | 发布目标，至少一个 |
| `traffic` | object | 目标间的发布策略 |
| `filters` | []object | 过滤器列表 |
| `timeout_ms` | int | 转发超时 |
| `revision` | uint64 | 版本 |
| `created_at` | time | 创建时间 |
| `updated_at` | time | 更新时间 |
| `created_by` | string | 创建人或系统来源 |
| `updated_by` | string | 最近修改人 |

## 2. 匹配条件

一期匹配字段：

| 字段 | 说明 |
| --- | --- |
| `hosts` | 可选，精确 host 或通配 host |
| `path_prefix` | 必填或与 `path` 二选一 |
| `path` | 精确路径 |
| `methods` | 可选，HTTP 方法列表 |
| `headers` | 可选，请求头精确匹配 |

同一路由内多个条件为 AND。不同路由按 `priority`、更长 path、创建时间稳定排序。

## 3. 发布目标

一条路由可以绑定多个发布目标，目标支持两种类型。流量策略和 BETA/金丝雀/蓝绿语义见 [`gateway-traffic-release-spec.md`](gateway-traffic-release-spec.md)。

```json
{
  "id": "order-v1",
  "type": "service",
  "service": {
    "service_name": "order-center",
    "group": "default",
    "namespace": "default"
  },
  "load_balance": "round_robin",
  "healthy_only": true
}
```

```json
{
  "id": "legacy",
  "type": "static",
  "static": {
    "endpoints": [{"url": "http://127.0.0.1:9001", "weight": 1}]
  },
  "load_balance": "round_robin"
}
```

服务目标使用 `internal/catalog` 查询健康实例；一期会将 `healthy_only` 归一化为 `true`，控制台不提供不健康实例的转发开关。静态目标由路由自身管理，不写入注册中心。

## 4. 过滤器

一期过滤器：

| 类型 | 参数 |
| --- | --- |
| `strip_prefix` | `parts` |
| `add_request_header` | `name`、`value` |
| `set_response_header` | `name`、`value` |

过滤器按数组顺序执行。请求头过滤器不得覆盖内部认证、追踪和转发安全头，除非后续安全规范明确允许。

# 资源模型规范

## 1. 通用资源层次

Eden 控制面资源使用命名空间隔离。不同领域可以在命名空间下定义自己的身份。

```text
namespace -> domain -> resource identity
```

| 领域 | 身份 |
| --- | --- |
| Naming | `namespace -> serviceName -> instanceId` |
| Config | `namespace -> group -> dataId` |
| Gateway Route | `namespace -> routeId` |

`namespace` 默认为 `default`。兼容 Nacos Config 时，请求参数 `tenant` 映射为 `namespace`。

## 2. 资源元数据

所有持久控制面资源应至少记录：

| 字段 | 含义 |
| --- | --- |
| `namespace` | 命名空间 |
| `created_at` | 创建时间 |
| `updated_at` | 最近更新时间 |
| `created_by` | 创建人或系统来源 |
| `updated_by` | 最近修改人 |
| `revision` | 单调递增版本号 |

配置资源还应记录 `md5`，网关路由还应记录 `enabled` 和 `priority`。

## 3. 命名规则

- `namespace` 不为空，默认 `default`。
- `group` 不为空，默认 `DEFAULT_GROUP`。
- `dataId` 不为空，允许点号和短横线，不能包含路径穿越语义。
- `routeId` 不为空，只允许字母、数字、点号、短横线和下划线。
- 所有用户输入在进入存储前必须 trim；兼容 API 中允许的历史字段应在适配层归一化。

## 4. 资源事件

配置发布、配置删除、路由创建、路由更新、路由删除和路由启停都应产生审计事件。事件不得记录完整配置内容，也不得记录敏感请求头、认证 token 或上游密钥。


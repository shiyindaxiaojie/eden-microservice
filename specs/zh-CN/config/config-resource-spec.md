# Config 资源规范

## 1. ConfigResource

配置资源字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `namespace` | string | 命名空间，默认 `default` |
| `group` | string | 配置分组，默认 `DEFAULT_GROUP` |
| `data_id` | string | 配置 ID |
| `content` | string | 完整配置内容 |
| `type` | string | 展示类型，如 `text`、`yaml`、`json`、`properties` |
| `md5` | string | content 的 md5 |
| `revision` | uint64 | 单调递增版本 |
| `description` | string | 说明 |
| `tags` | []string | 控制台筛选标签 |
| `created_at` | time | 创建时间 |
| `updated_at` | time | 更新时间 |
| `created_by` | string | 创建人 |
| `updated_by` | string | 修改人 |
| `deleted` | bool | 是否逻辑删除 |

## 2. 校验规则

- `data_id` 必填，长度建议不超过 255。
- `group` 为空时使用 `DEFAULT_GROUP`。
- `namespace` 为空时使用 `default`。
- `content` 可以为空字符串；空内容仍然是有效配置。
- `type` 只影响展示，不影响核心存储语义。
- `tags` 不参与资源身份。

## 3. 存储建议

一期推荐使用 bbolt 或现有文件持久化模式实现本地持久化。存储层至少需要 buckets：

| bucket | 内容 |
| --- | --- |
| `configs` | 当前有效配置 |
| `config_history` | 历史版本 |
| `config_index` | namespace/group/dataId 和搜索索引 |

CP 模式下 Config 写入应作为 Raft command 进入状态机。AP 模式下写入本地后向 peers 同步，并以 revision/md5 合并最终状态。


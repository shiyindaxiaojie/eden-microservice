# Config 发布与查询规范

## 1. 原生 HTTP API

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/v1/config` | 查询单个配置 |
| `GET` | `/v1/configs` | 列表和搜索 |
| `POST` | `/v1/config` | 创建配置 |
| `PUT` | `/v1/config` | 更新配置 |
| `DELETE` | `/v1/config` | 删除配置 |
| `GET` | `/v1/config/history` | 查询配置历史 |

单配置身份通过 query 或 JSON body 传递 `namespace`、`group`、`data_id`。

## 2. Nacos Config 兼容 API

一期必须支持常用 Nacos HTTP 路径：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/nacos/v1/cs/configs` | 查询配置内容 |
| `POST` | `/nacos/v1/cs/configs` | 发布配置 |
| `DELETE` | `/nacos/v1/cs/configs` | 删除配置 |
| `POST` | `/nacos/v1/cs/configs/listener` | 长轮询监听 |

兼容接口返回值应匹配 Nacos 客户端预期：查询成功直接返回 content；发布和删除成功返回 `true`；未找到时按 Nacos 客户端可识别的状态处理。

## 3. 发布行为

发布请求按身份定位资源：

1. 归一化 namespace、group、dataId。
2. 计算新 content 的 md5。
3. 如果 content 未变化，只更新必要元数据，不产生重复历史。
4. 如果 content 变化，保存当前版本到历史，再写入新版本。
5. 发布 ConfigChange 事件，唤醒精确监听者。

## 4. 查询行为

查询必须优先返回当前有效版本。已删除配置对普通查询不可见，但历史接口可查询删除前版本和删除事件。

## 5. CAS 行为

原生 API 可以支持 `expected_md5`。当请求携带 `expected_md5` 且与当前 md5 不一致时，写入失败并返回冲突错误。Nacos 兼容接口按上游客户端常用语义处理。


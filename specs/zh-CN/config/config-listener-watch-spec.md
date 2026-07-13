# Config 监听与 Watch 规范

## 1. 监听目标

一期只支持精确配置监听：

```text
namespace + group + dataId
```

模糊订阅、tag 订阅和灰度规则监听不进入一期。

## 2. 原生监听

原生 HTTP 可以提供：

```text
POST /v1/config/listener
```

请求包含客户端持有的 `namespace`、`group`、`data_id`、`md5` 和可选超时时间。服务端在以下情况返回：

- 当前 md5 与请求 md5 不一致，立即返回变化资源。
- 超时时间内资源发生变化，返回变化资源。
- 超时后仍无变化，返回空变化列表。

## 3. Nacos 兼容长轮询

`POST /nacos/v1/cs/configs/listener` 必须兼容 Nacos Config 客户端常见格式，读取 `Listening-Configs` 或表单中的监听内容，并返回发生变化的配置 key 列表。

`Listening-Configs` 中单项使用 Nacos 控制字符编码：字段顺序为
`dataId\x02group\x02md5[\x02tenant]`，多项使用 `\x01` 分隔。响应只包含
`dataId\x02group\x02tenant\x01` 形式的变化 key。等待时间读取
`Long-Pulling-Timeout` 请求头并受服务端最大等待时间限制。

服务端不得在长轮询响应中返回完整配置内容。客户端应在收到变化 key 后再次调用 `/nacos/v1/cs/configs` 查询内容。

## 4. 集群行为

配置变更在本节点提交后应唤醒本节点监听者。集群中其他节点收到同步或 Raft 提交后也应唤醒本地监听者。AP 模式允许通知延迟；CP 模式只有已提交版本可见。

## 5. 资源消耗边界

每个监听请求必须有最大等待时间。服务端应限制单请求监听 key 数量、单用户并发长轮询数量和全局监听数量，超出后返回可诊断错误。

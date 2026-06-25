# Gateway 运行时规范

## 1. 请求流程

```text
HTTP request
  -> route match
  -> upstream resolve
  -> load balance
  -> request filters
  -> reverse proxy
  -> response filters
  -> metrics and access log
```

## 2. 路由匹配

网关必须使用内存快照进行匹配。路由变更提交后生成新快照，后续请求使用新快照；正在执行的请求继续使用进入请求时看到的快照。

## 3. 上游选择

服务上游默认只选择健康实例。没有健康实例时返回 `503`，并记录路由 ID、服务名和命名空间。静态上游不可达时按反向代理错误处理。

## 4. 超时与错误

- 默认转发超时建议 30 秒。
- 路由未匹配返回 `404`。
- 上游无实例返回 `503`。
- 上游超时返回 `504`。
- 代理内部错误返回 `502`。

错误响应应避免泄露内部堆栈、完整上游地址中的凭证、请求认证信息。

## 5. 可观测性

每次请求至少记录 routeId、method、path、status、duration、upstream、error。访问日志不得记录完整 Authorization、Cookie 或 API Key。


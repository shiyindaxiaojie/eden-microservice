# Gateway 流量发布规范

## 1. 目标

本规范在基础路由和服务发现能力之上定义多目标流量发布。它支持：

- 路由绑定多个注册中心服务或静态上游；
- 目标间权重分流；
- 金丝雀发布；
- 按指定用户或租户稳定命中的 BETA 测试；
- 蓝绿发布和原子切换。

目标间的发布权重与服务实例的 `weight` 是两个不同层级。前者决定一个请求先进入哪个发布目标；后者只决定已经选定注册服务后，命中哪个服务实例。

## 2. 发布目标

`RouteResource.targets` 至少包含一个目标。目标 ID 在一条路由内唯一，只允许字母、数字、点号、短横线和下划线。

```json
{
  "id": "order-v2",
  "name": "订单服务 V2",
  "labels": {"version": "v2", "color": "green"},
  "type": "service",
  "service": {
    "namespace": "prod",
    "group": "default",
    "service_name": "order-service"
  },
  "load_balance": "weighted",
  "healthy_only": true
}
```

静态目标使用 `static.endpoints`；每个 endpoint 的 URL 必须是无认证信息、无 query 和 fragment 的 `http` 或 `https` URL。endpoint 的 `weight` 默认值为 `1`。

```json
{
  "id": "legacy-payment",
  "type": "static",
  "static": {
    "endpoints": [
      {"url": "https://10.0.8.21:8080", "weight": 3},
      {"url": "https://10.0.8.22:8080", "weight": 1}
    ]
  },
  "load_balance": "weighted"
}
```

支持的目标内负载均衡算法为 `round_robin`、`random` 和 `weighted`。注册服务一期只选择健康实例；静态地址在一期不执行主动健康检查，连接或响应失败按代理错误处理。

## 3. 发布策略

每条路由有一个 `traffic` 对象。`mode` 取值为 `weighted`、`canary` 或 `blue_green`；所有模式都可附加 BETA 规则。

```json
{
  "mode": "canary",
  "default_target_id": "order-v1",
  "weighted_targets": [
    {"target_id": "order-v1", "weight": 95},
    {"target_id": "order-v2", "weight": 5}
  ],
  "beta_targets": [
    {
      "target_id": "order-v2",
      "users": ["u-10086"],
      "tenants": ["acme"]
    }
  ]
}
```

### 3.1 权重与金丝雀

`weighted` 和 `canary` 都使用 `weighted_targets`，各项权重必须为正整数且总和恰好为 `100`。`canary` 是面向控制台和审计语义的发布模式：默认目标是稳定版本，其余目标是灰度版本；运行时选择规则与权重模式一致。

网关优先以可信用户 ID、可信租户 ID、`X-Request-ID` 的顺序构造稳定分流键，并对路由身份和该键作确定性哈希。因此，同一身份在权重不变时会稳定进入同一目标。没有这些键的请求使用路由内计数器轮转百分比桶，以保持整体权重比例；此类请求不承诺跨请求或重启的粘滞性，也不会把共享 ingress 的远端地址误当成用户身份。

### 3.2 BETA 测试

`beta_targets` 在百分比分流和蓝绿活动目标之前执行。用户精确匹配优先于租户匹配；同一个用户或租户不得分配给两个不同 BETA 目标，避免出现顺序依赖的结果。命中的用户/租户 100% 进入指定目标，直到规则被删除或改写。

一期仅信任由 `gateway.trusted_proxy_cidrs` 中的反向代理发送的 `X-Eden-User-ID` 与 `X-Eden-Tenant-ID`。来自非可信来源的同名 Header 绝不参与 BETA 选择，并会在转发到上游前剥离，避免客户端伪造身份影响服务端逻辑。默认只信任回环地址，生产部署必须显式配置其身份代理网段。

### 3.3 蓝绿发布

`blue_green` 模式要求 `active_target_id` 指向一个存在的目标。普通请求 100% 进入活动目标；`weighted_targets` 在此模式下不得出现。切换 `active_target_id` 是一次路由更新，提交后由运行时快照原子生效。BETA 规则可将指定用户或租户固定导向未活动颜色，以便切换前预览。

## 4. 选择与失败语义

单个请求的顺序如下：

```text
路由匹配
  -> BETA 用户/租户规则
  -> 权重或金丝雀目标（weighted/canary）
  -> 活动蓝绿目标（blue_green）
  -> 目标内实例/静态 endpoint 负载均衡
  -> 反向代理
```

目标选定后不隐式回退到其他发布目标。若一个服务目标没有合格实例，返回 `503`；上游超时返回 `504`；连接、TLS 或协议错误返回 `502`。显式回退策略属于后续能力，不能用隐式回退掩盖金丝雀或 BETA 目标不可用的问题。

## 5. 控制面与审计

路由创建、更新、删除和启停写入持久化历史记录，包含 action、operator、revision、摘要和时间，但不记录上游 URL 中的凭证（验证阶段禁止凭证）或请求身份 Header。更新使用 `expected_revision` 乐观并发控制；版本不匹配返回 `409`。

控制面保存成功后必须同步重建数据面内存快照。已经开始的请求继续使用进入时取得的快照，之后的请求使用新快照。

## 6. 非目标

本期不实现主动静态上游健康检查、自动权重推进、指标驱动自动回滚、JWT Claim 解析、WebSocket/gRPC 透传、熔断、限流和插件市场。集群内路由资源复制沿用后续统一控制面资源同步能力；本期路由存储与现有配置中心一样为节点本地持久化，控制台会明确展示数据面节点的本地运行状态。

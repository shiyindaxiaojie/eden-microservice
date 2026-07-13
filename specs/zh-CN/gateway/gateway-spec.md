# Gateway 规范

## 1. 定位

Gateway 是 HTTP API 网关领域。它负责根据路由规则匹配请求，选择上游实例，执行基础过滤器，并将请求反向代理到目标服务。

Gateway 不拥有服务注册生命周期。上游服务实例来自 Naming 领域，静态上游也只作为路由目标的一种来源。

## 2. 一期目标

一期提供 Spring Cloud Gateway 常用替代能力的最小闭环：

| 能力 | 一期要求 |
| --- | --- |
| 网关路由 | 创建、更新、删除、启停、排序 |
| 匹配 | host、精确 path 或 path prefix、method、header |
| 上游 | registry service 或 static URL |
| 负载均衡 | round_robin、random、weighted |
| 健康感知 | 一期只选择健康实例 |
| 过滤器 | strip_prefix、add_request_header、set_response_header |
| 转发 | HTTP reverse proxy，超时可配置 |
| 控制台 | 新增 `网关路由` 菜单 |

限流、熔断、JWT 鉴权、WebSocket、gRPC 透传、插件化过滤器可以后续扩展。

## 3. 控制面与数据面

控制面 API 使用现有管理端口 `/v1/gateway/*`。数据面推荐使用独立监听端口，例如：

```yaml
gateway:
  enabled: true
  http: ":8080"
```

一期要求网关数据面与控制面使用不重叠的监听地址；启动时会拒绝相同端口或 wildcard/具体地址重叠的配置，避免通配路由吞掉 `/v1/*`、`/internal/*`、`/nacos/*` 等控制和兼容路径。

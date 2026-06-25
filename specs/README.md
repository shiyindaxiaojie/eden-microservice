# Eden Microservice Specs

This directory contains design-level specifications for `eden-microservice`.
Specs are the source for implementation, tests, console behavior, and user
documentation alignment.

本目录包含 `eden-microservice` 的设计级规范。实现、测试、控制台行为和面向用户的文档都应以这些规范为准。

Spec hierarchy:

```text
Eden microservice control plane
  -> Resource model
  -> Foundation capabilities
     -> registry / config / gateway / console / auth / cluster
  -> Domain capabilities
     -> Naming / Config Center / API Gateway
  -> Interface specs
     -> Native HTTP / compatibility HTTP / gRPC / SDK
  -> Integration and adapter model
  -> Security and RBAC model
```

规范层次：

```text
Eden 微服务控制面
  -> 资源模型
  -> 基础能力
     -> 注册 / 配置 / 网关 / 控制台 / 鉴权 / 集群
  -> 领域能力
     -> 注册中心 / 配置中心 / API 网关
  -> 接口规范
     -> 原生 HTTP / 兼容 HTTP / gRPC / SDK
  -> 集成与适配器模型
  -> 安全与 RBAC 模型
```

Available language:

- [简体中文](zh-CN/README.md)


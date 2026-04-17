# 服务发现示例

`examples/service-discovery` 目录包含 4 套端到端示例，它们都围绕同一组微服务拓扑展开：

- `auth-center`
- `user-center`
- `order-center`

每套示例都会启动 2 到 3 个服务实例，并暴露 HTTP 接口，方便直接验证注册、发现、订阅和服务间调用链路。

## 示例分类

- `native`
  使用原生 `pkg/sdk` 客户端，演示直接接入注册中心的方式。
- `consul`
  使用官方 `github.com/hashicorp/consul/api` 客户端，对接注册中心提供的 Consul 兼容 HTTP 接口。目标效果是只改注册中心地址，业务代码不动。
- `nacos`
  使用官方 `github.com/nacos-group/nacos-sdk-go/v2` 命名客户端，对接注册中心提供的 Nacos 兼容 HTTP / gRPC 接口。目标效果同样是只改注册中心地址，业务代码不动。
- `custom`
  把示例视为外部项目，只通过公开的 HTTP / gRPC 协议接入，不依赖仓库内的 `pkg/registry` 或根目录 proto 包。

## 如何选择

- 想看原生 `pkg/sdk` 接入，读 `native`。
- 想看官方 Consul SDK 无缝切换，读 `consul`。
- 想看官方 Nacos SDK 无缝切换，读 `nacos`。
- 想看外部项目按协议自行集成，读 `custom`。

## 示例入口

- [native](./native/README.md)
- [consul](./consul/README.md)
- [nacos](./nacos/README.md)
- [custom](./custom/README.md)
- [English](./README.md)


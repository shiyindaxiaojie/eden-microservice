# 服务发现示例

`examples/service-discovery` 目录包含 4 套端到端示例，它们都围绕同一组微服务拓扑展开：

- `auth-center`
- `user-center`
- `order-center`

每套示例都会为每个服务启动 2 到 3 个实例，并暴露 HTTP 接口，方便直接验证注册、发现、订阅以及服务间调用链路。

## 示例分类

- `eden`
  使用原生 Eden 客户端，演示直接接入 Eden 服务发现。
- `consul`
  使用 Consul 兼容适配层，演示通过统一注册中心接口进行集成。
- `nacos`
  使用 Nacos 兼容适配层，演示通过统一注册中心接口进行集成。
- `custom`
  把示例视为“外部项目”。它不依赖仓库里的 `pkg/registry` 和根目录下的 proto 包，而是只按公开的 HTTP / gRPC 协议集成，并在 `custom/internal` 下保留自己的本地协议资产。

## 目录约定

- 每套示例都是自包含的。
- 每个服务都把集成主流程放在各自的 `main.go` 中。
- 启动脚本位于各自示例目录内。
- `examples/service-discovery/internal` 下没有共享示例辅助代码。

## 如何选择

- 如果想看原生 Eden 客户端接入，优先读 `eden`。
- 如果想看适配器式接入，读 `consul` 或 `nacos`。
- 如果想看“外部项目只按协议集成”的写法，读 `custom`。

## 示例入口

- [eden](./eden/README.md)
- [consul](./consul/README.md)
- [nacos](./nacos/README.md)
- [custom](./custom/README.md)

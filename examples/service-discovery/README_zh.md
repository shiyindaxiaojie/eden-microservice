# 服务发现示例

`examples/service-discovery` 目录现在包含 4 套彼此独立的集成示例：

- `eden`
- `consul`
- `nacos`
- `custom`

每个目录都自带自己的服务代码、启动脚本和说明文档，不再依赖 `examples/service-discovery/internal` 这种共享辅助目录。

每套示例都会启动同样的 3 个服务：

- `auth-center`
- `user-center`
- `order-center`

并且每个服务都会启动 2 到 3 个实例进程，便于验证注册、发现、订阅和服务间调用是否正常。

## 示例入口

- [eden](D:/Workspaces/Git/eden-go-registry/examples/service-discovery/eden/README.md)
- [consul](D:/Workspaces/Git/eden-go-registry/examples/service-discovery/consul/README.md)
- [nacos](D:/Workspaces/Git/eden-go-registry/examples/service-discovery/nacos/README.md)
- [custom](D:/Workspaces/Git/eden-go-registry/examples/service-discovery/custom/README.md)

最终目标：
实现一个通用的注册中心，极致的性能 和 低内存占用，并能适配旧系统集成 consul、nacos 的 API
已知问题：
1、在 node1 添加了 node2、node3，没有同步到 node1 保存的配置到 node2、node3，你看看是不是这样改，添加节点时，输入对应节点的 api-key
需求：
1、增加 go sdk 集成，在 examples\service-discovery 重构下，提供标准集成实例，并模拟 用户中心、认证中心、订单中心等服务，实现简单的服务注册、调用流程，需要考虑的是，除了这个标准 sdk，你还要提供适配 consul 集成、nacos 集成的 sdk，可以实现不修改代码，无缝切换
2、客户端 sdk 集成，和注册中心通信使用长连接 gRPC (基于 HTTP/2，默认），弱网环境可以选择 QUIC 协议
3、注册中心节点之间的通信，使用 gRPC (Protobuf) + 自定义 Raft/Gossip 逻辑
4、你看看我的代码逻辑，我希望所有配置尽量是在控制台界面调整，减少对 configs\config.yaml 的配置，后面启动多个节点，基本是用变量来代替 configs\config.yaml，其他的都是在控制台界面修改。目前有个问题，我在设置添加节点，用户什么的，我希望其他节点也能自动保存相关配置。这里应该使用哪个方案，是基于 HTTP API，还是基于 Raft/Gossip 逻辑？

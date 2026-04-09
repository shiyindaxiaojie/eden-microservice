# Eden 架构设计与技术规范

本文档旨在全面阐述 Eden 注册中心当前核心实现中的系统分层、通信协议矩阵、多节点一致性同步机制以及控制台交互模型。本文档所描述的架构与设计细节均以当前代码库的核心实现为准，不涉及尚未合并至主干路径的推演性或扩展性设计。

## 1. 架构目标

Eden 的核心目标是构建一款轻量级、高可用且支持在 `AP`（高可用性与分区容错性）与 `CP`（强一致性与分区容错性）模型之间平滑切换的分布式注册中心服务。当前系统全面覆盖并提供以下三大维度的核心能力：

- **基础注册与发现机制**：面向业务微服务，提供高并发的服务注册、服务发现、实例健康状态维护（心跳机制）、事件驱动的实时订阅，以及实例级别的优雅上下线调度。
- **系统治理与运维管控**：面向系统运维与管理员，提供基于前端的控制台（Console）、多语言国际化支持（i18n）、基于角色的访问控制（RBAC）、API Key 鉴权体系以及细粒度的动态运行配置管理。
- **分布式集群架构**：面向集群级别管理，内置反熵同步与无状态对等复制（AP 模式），并通过 Raft 共识算法保障强一致性的主从复制与状态机提交（CP 模式）。
- **可观测性与事件通知**：面向系统健康度评估，构建基于滑动窗口算法的自定义事件响应体系，并融合 Webhook 与 SMTP 多渠道的闭环系统告警链路。

## 2. 协议矩阵总览

| 通信协议 | 默认监听地址与端口 | 核心应用场景与边界 | 通信参与主体 |
|---|---|---|---|
| HTTP / RESTful | `http_addr` (默认 `:8500`) | 控制台管理 API、基于 HTTP 的客户端接入、SDK 组件可选同步机制、集群节点元数据发现及内部配置同步。 | 控制台 ↔ 注册中心<br>客户端 ↔ 注册中心<br>节点 ↔ 节点 |
| gRPC / HTTP/2 | `grpc_addr` (默认 `9000-9999`) | 官方 SDK 的默认业务数据通信、多语言 gRPC 客户端接入、分布式集群节点对等发现、微服务依赖拓扑上报，以及 AP 共识模式下的状态复制。 | 客户端 ↔ 注册中心<br>节点 ↔ 节点 |
| QUIC | `quic_addr` (默认 `10000-10999`) | 客户端弱网场景通信：作为 gRPC 的一种可选底层多路复用传输协议，旨在优化复杂弱网环境下的客户端链路质量。 | 客户端 ↔ 注册中心 |
| Raft (TCP) | `raft_addr` (建议 `127.0.0.1:7000+`) | 专注于 CP 模型下的 Leader 节点竞选流转、日志增量复制及有限状态机的最终确认与状态提交。 | 注册中心 ↔ 注册中心 |

**技术约束与规范：**
- `QUIC` 并非独立的业务应用层协议，而是 `gRPC` 通信的底层可选传输实现。
- Eden 官方 SDK 默认以 `gRPC` 协议执行业务通信。是否产生额外的 HTTP 管理或控制请求，取决于传入地址的规范及 `DiscoveryMode` 配置。
- 在 `grpc/quic` 与 `static` 参数共存的工作负载下，系统可实现客户端与注册中心之间彻底剥离 HTTP 会话（零 HTTP 通信隔离）。
- 若显式配置 `Transport=http`，客户端整体业务链路将降级为通过 HTTP/RESTful 完成。

## 3. 核心系统分层模型

```mermaid
graph TB
    subgraph "外部调用域"
        Client["客户端集群 (官方 SDK / 其他客户端)"]
        Console["Web 管理控制台"]
        Peer["其他物理节点 / 对等中心"]
    end

    subgraph "Eden 注册中心系统调度核心"
        HTTP["HTTP API 路由调度器 (Handler)"]
        GRPC_REG["gRPC 注册代理服务 (RegistryService)"]
        GRPC_CLUSTER["gRPC 集群状态同步服务 (ClusterService)"]
        CAT["服务生命周期内核 (CatalogService)"]
        ALERT["流式告警评估引擎 (Alert Evaluator)"]
        NOTIFY["异步事件派发组件 (Notification Engine)"]
        SET["配置与元数据代理层 (SettingsService)"]
        AUTH["统一安全与鉴权中心 (AuthService)"]
        AP["AP 无状态同步控制器 (AP Node & PeerManager)"]
        CP["CP 强一致状态控制器 (CP Node & FSM)"]
        STORE["持久化与内存级索引存储器 (Registry Storage)"]
    end

    Client -->|gRPC RPC: Register / Heartbeat / Discover / Watch / GetMembers / ReportTopology| GRPC_REG
    Client -->|HTTP RESTful: /v1/catalog/*| HTTP
    Client -->|HTTP 同步探测: /v1/cluster/members (可选)| HTTP
    Client -->|HTTP 拓扑探针: /v1/catalog/topology/report (可选)| HTTP

    Console -->|HTTP 运维与调度链路: /v1/auth/* /v1/settings/* /v1/cluster/* /v1/alert/* /v1/notify/*| HTTP

    Peer -->|gRPC 传输管道: ReplicateLog / SyncDiscovery| GRPC_CLUSTER
    Peer -->|HTTP 状态查询: /v1/node/info| HTTP
    Peer -->|HTTP 内部同步流: /internal/sync/*| HTTP
    Peer -->|TCP 二进制传输: Raft 心跳与选主流| CP

    HTTP --> CAT
    HTTP --> SET
    HTTP --> AUTH
    HTTP --> ALERT
    HTTP --> NOTIFY
    GRPC_REG --> CAT
    GRPC_CLUSTER --> AP

    CAT --> AP
    CAT --> CP
    CAT -->|状态机联动钩子 (OnEvent Hook)| ALERT
    ALERT -->|命中防御或阈值告警| NOTIFY
    SET --> AP
    SET --> CP
    AP --> STORE
    CP --> STORE
```

系统主入口的引导流程封装于 `cmd/server/main.go`。系统引导期间，将统一执行以下领域组件资源的初始化：
- 分布式 `Registry` 存储代理引擎
- 高性能 `AP Node` 系统实例
- 高可靠共识 `CP Node` 系统实例
- 聚合领域服务群集：`CatalogService`、`AuthService`、`SettingsService` 与 `ClusterService`
- 并行绑定多路复用网络监听层：HTTP Server、gRPC Server 与 QUIC Listener

由此表明，Eden 在现有系统内核设计中，通过单一进程封装实现 AP 与 CP 共识基座的双引擎协同运行，并依托当前环境配置进行上游业务路由写入流的动态派发，有效规避了异源构建及二进制部署文件的分裂冗余。

## 4. 客户端通信机制与交互范式

### 4.1 官方 SDK 核心 gRPC 通信范式

若 `pkg/eden` 配置选用 `grpc_addr` 组合且设定 `Transport=grpc` 属性，应用端连接与远端服务中心之间的所有流量数据通道均受 gRPC 连接托管控制：

- 若配置策略启用 `DiscoveryMode=auto`，实例的注册、周期健康性探活、发现协议、集群组成员自检与网络拓扑上传上报等控制动作，彻底归属及交接至 `eden.registry.v1.RegistryService` 服务全生命周期收口管理：
  - 探查集群物理结构与对等节调用 `GetMembers` RPC 端点。
  - 应用网络结构上传调用 `ReportTopology` RPC 端点。
  - 免于执行旧链路的 `GET /v1/cluster/members` 端点访问。
  - 免于执行传统表单态的 `POST /v1/catalog/topology/report` 端点提交。
- 若退化挂载静态模式（即 `DiscoveryMode=static`），则将屏蔽运行时成员扩缩容发现扫描程序，限定范围执行注册中心寻址调度和端侧无感主备故障规避。

### 4.2 官方 SDK 原生 HTTP 后备机制

当业务或底层指定策略 `pkg/eden` SDK 配置将 `Transport=http` 激活时，底层传输机制退为标准 HTTP/RESTful API 驱动：

- 实例资源建立访问：`POST /v1/catalog/register`。
- 健康度指标探测请求访问：`POST /v1/catalog/heartbeat`。
- 集群实例视图分发访问：`GET /v1/catalog/service/{name}`。
- 控制生命周期上架退还访问：`POST /v1/catalog/instance/status`。
- 系统异步订阅特征行为被自动代理降维成客户端轮询探查发现端点的持续调度动作。
- 需要额外触发集群同步扫描任务或者图拓扑收集记录的任务将直接引入 `/v1/cluster/members` 和 `/v1/catalog/topology/report` 。

### 4.3 无绑定协议栈跨平台 / 异构技术栈接入规范

目前面向第三方开发标准对等组件接入体系并完全支持的服务协议范畴囊括以下项核心规范：

- 原生 gRPC 通信组装接入：使用生成桩代码无缝绑定针对 `eden.registry.v1.RegistryService` 进行函数级寻址并请求调用。
- HTTP 通用端点报文整合模式：基于 Web 标准发起并发送资源载荷的交互方法访问映射路径集 `/v1/catalog/*`。

**接口行为调用约定映射：**

| 领域驱动业务域与交互动作 | gRPC 代理 RPC 定义 | HTTP/RESTful 端点资源路径约束 |
|---|---|---|
| 初始化实例声明接入注册池 | `Register` | `POST /v1/catalog/register` |
| 面向服务端心跳延续长生命周期租约 | `Heartbeat` | `POST /v1/catalog/heartbeat` |
| 通过标识获取实例路由的元查询机制 | `Discover` | `GET /v1/catalog/service/{name}` |
| 管控特定物理隔离节点的运转上线行为 | `SetInstanceStatus` / `Deregister` | `POST /v1/catalog/instance/status` |
| 实时推送配置变化等持续监听通道结构 | `Watch` | **缺乏支持（协议中不存在持久推送实现机制）** |
| 取回集群基础网络通讯列表矩阵图 | `GetMembers` | `GET /v1/cluster/members` |
| 第三类观察图构建汇报关联拓扑连接信息 | `ReportTopology` | `POST /v1/catalog/topology/report` |

**边界条件扩展说明与补充规范指引：**
- 现阶段开放了赋予自定义或底层定制的流操作 gRPC 的实现能力；均被推荐优先借调接入 `GetMembers` 探知实际节点群网。
- 非侵入及非弹性的固化封闭系统建议直接绑定死循环轮换静态表。
- gRPC 下属的关联网络与依赖感知支持依赖关系通过内置调用流头参数附加（Metadata 头指定 `x-consumer-service` 以在服务端追踪日志链路上游身份追踪统计）。

## 5. 高可用注册中心内节点同步机制

### 5.1 AP (高可用容错) 驱动网络逻辑

在关注集群总效或极端可用系统环境配置下优先 AP 平面的主同步信令和通道如下：

| 数据状态交互目的标的 | 选用底层流控或交互形式协议标准 | RPC 方法接口或 HTTP 请求地址路径映射点 |
|---|---|---|
| 面向事件驱动日志级主复制推送动作 | gRPC (底层应用流) | `ClusterService.ReplicateLog` |
| 服务目录周期全量核实或反熵重计算对齐 | gRPC (底层应用流) | `ClusterService.SyncDiscovery` |
| 对等物理控制环境探针请求调用 | HTTP / JSON 承载格式 | `GET /v1/node/info` |
| 集成种子信息流快速填充新加实例状态 | HTTP / JSON 承载格式 | `POST /internal/sync/seeds` |
| 治理面运行时内部全局变更配置覆盖发布 | HTTP / JSON 承载格式 | `POST /internal/sync/settings` |

**实现属性与边界：**
- 数据平面业务强关联高速公路信令机制严格依序绑定于高并发和性能流式的 gRPC 上。
- 基础控制通讯交互控制元数据仍然高度依赖更普遍开放适配良好的 HTTP 信令集规范作为保障屏障系统运转底座机制实现层。
- 组件 `PeerManager` 负责周期核验证与握手并提取保存基于网络请求建立维护好基础关联列表和入口记录；并在双向通道连通对等之前进行安全的前置检查，调用 `GET /v1/node/info` 读取节点信息。

### 5.2 CP (强一致协议) 共识数据节点同步机制

依靠于保证了严谨数据的可查可用分布式算法理论组建与实施网络。关键系统路径和控制机制见表约束要求：

| 主驱动协同同步业务流程指令层机制集 | 具体系统环境物理连接实现类型规范层级 | 管理接口实现层通讯入口 / Endpoints |
|---|---|---|
| 时钟和状态机的选举心跳共识执行流程核 | Raft 内核 TCP 数据链路底层框架流 | `raft_addr` 所指向通信端 |
| 准入资格注册引入加入集群内主导列表 | HTTP | `POST /v1/cluster/join` |
| 共识管理模块状态和群身份修改摘除流程 | HTTP | `/v1/cluster/member` |
| 单个服务实例信息自报系统查询核验接口 | HTTP | `GET /v1/node/info` |

需重申强调：在此模型设计中实现落地阶段机制并非单纯脱离 HTTP 单打独缠的依托算法实现管理任务操作（即使选用强共识环境模式，集群操作及安全审查系统调度机制层面，对 HTTP 信赖亦无可替代）。

### 5.3 数据交互实现现状及能力保留边界限定

在参考和阅读工程约束配置集内源设计底稿的模块化 RPC 定义清单 `api/proto/cluster/v1/cluster.proto` 涵盖存在以下声明方法组但由于工程边界条件或规划优先顺序当前执行态不全数启动，目前尚未作为工程稳定主路径合并运行代码组。预设定义内容如下：
- `SyncUser` / `DeleteUser`
- `SyncAPIKey` / `DeleteAPIKey`
- `SyncSettings`
- `ForwardToLeader` (代理请求向当前主节点实现写入层数据透明转向和跨转请求）

根据系统的现运行逻辑链路验证结论：
- 面向 AP 高负载和集群快速状态跟进的主力吞吐仍然严格挂载并指向给 `ReplicateLog`和`SyncDiscovery` 等重核心机制执行动作。
- 在当前交付环境内尚未接入完成 `ForwardToLeader` 。

综合并客观评估，所留档定义好的接口或服务描述在部分条件未成熟情况或未完全达到交付设计标准要求下，未必完全集成介入。

## 6. 面向系统运维控制面板层

考虑到工程安全及环境实施的可选管理和拓展操作隔离的建设需求；将应用端平台交互体系划分双模块，分为独立前置界面支持及业务处理的响应中心接口平台执行层分离技术构筑方案运行落地。控制台管控模式配置说明及路由指派规划规则参数为：
- Web 前端开发者环境初始映射端口：`http://127.0.0.1:2019`
- 接口级代理底层管控调度请求默认路由接反向访问的地址策略为 `http://127.0.0.1:8500`

内部通信模式均严格经由标准的管控层面 HTTP RPC / REST 通信进行管控集成：

| 领域模块规划组件任务与治理执行场景应用类别 | 后侧通信对应暴露资源接口请求结构 / 链路约定 |
|---|---|
| 应用访问安全性登陆核实与 JWT 会话校验核签验证 | `/v1/auth/login` |
| 系统服务及分布式结构数据下全集群监控资源图表状态取样与发现 | `/v1/catalog/services` 、 `/v1/catalog/service/{name}` |
| 跨机房物理环境基础设施组件拓扑节点操作群控和自愈组网列表信息获取 | `/v1/cluster/members` 、 `/v1/cluster/member` |
| 服务底座集群基础全局级别配置文件规则动态更新调整加载刷新系统 | `/v1/settings/system` 、 `/v1/settings/storage` |
| 安全角色认证分配、多组织或空间权限规则限定模型 (RBAC) 数据组处理域 | `/v1/rbac/*` |
| 安全通信 API Key 高级动态发行审计签名授权体系维护流控链路与验证 | `/v1/settings/apikey*` |

技术实质层面考量：前置 Web 发行管理终端环境本质只负责并负责管理数据表向操作控制界面转换；系统对外的扩展不包含针对交互业务提供和封装非 HTTP / 不标准协议类型私有第三独立特殊实现框架能力。

## 7. 组件层级代码组织归因及导航指向规划

| 系统子结构特征实现映射域 | 主核查参考代码底层落地组织目录源文件节点位置 |
|---|---|
| gRPC 应用终端服务侧接收调度及会话通讯代理机制接口层处理 | `api/proto/registry/v1/registry.proto`、`internal/grpc/server.go` |
| 原生 HTTP API 的基础路由转换处理中心及相关操作序列管理封装组件 | `internal/handler/catalog_handler.go` |
| 高阶故障转移能力自动探测路由官方统一 SDK | `pkg/eden/client.go` |
| AP (可用容错型) 去中心主备节点之间集群的数据最终同步传输核心业务组装 | `api/proto/cluster/v1/cluster.proto`、`internal/cluster/ap/node.go`、`internal/grpc/cluster_server.go` |
| 轻量网络 HTTP 端跨区元信节点自探查信息状态的对列更新任务管道控制器 | `internal/handler/cluster_handler.go`、`internal/service/settings.go` |
| 遵循特定模型及环境机制 CP 方式实施落地运行选举流及相关核心协议控制器 | `internal/cluster/cp/node.go`、`internal/cluster/cp/fsm.go` |
| 基础资源组件内置流系统健康告警事件核触发拦截并组装推送渠道逻辑封装代码片段源 | `internal/alert/evaluator.go`、`internal/notify/engine.go` |
| 支持前置应用国际化视图部署支持相关动态渲染界面组件支持层 | `web/src/utils/i18n.ts`、`web/src/views/settings.vue` |
| 全局统一指令入口配置读取和主程序驱动初始化阶段应用启动及装载中心源根进程引导点 | `cmd/server/main.go` |

## 8. 技术能力实施强制约束列表与规范边界说明文档边界说明书

- 明确系统在物理内网服务与通讯链路等互通交互状态的实现环节中暂未实施 QUIC 的协议环境构建及封装；该协议当前仅向位于极端网终边界外部访问链接通道中的非控制信令和具有弱连接断网问题客户端设备群环境予以专项补充和应对建设实施方案的定向增强型通讯兼容考虑策略。
- 目前位于数据应用及信息上传统计算的内部业务的 gRPC 连接协议管道当前缺乏系统预配 API 级别请求或应用基于 JWT Token 高规格链路授权访问保护等相关环境和安全支持实现配置，建议并首选采取依托物理防火墙或其他服务组进行强管控防线设定方案予以补充。
- 考虑到对于基础层系统的关键安全性防御要求，包含核心注册网络列表状态变更管理以及全局性数据层组件相关的一切设置修改均已通过统一约束被安全收敛在了 HTTP 的保护接口网内以实现防越权。


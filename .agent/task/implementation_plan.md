# 基于 Go 开发轻量级服务注册中心设计方案

针对目前使用的 FSU 资源占用高、Consul 功能过于冗余、Nacos/Zookeeper 依赖 Java 环境启动慢的问题，本项目（Eden Go Registry）的目标是打造一款纯 Go 实现的轻量级微服务注册中心。主要功能围绕**服务注册、服务发现与健康检查**，并借鉴 Consul 的核心思想以达到同样的一致性标准。

## User Review Required

> [!IMPORTANT]
> 此为初期的架构设计方案，请重点审阅：
> 1. 技术选型（包括采用 `hashicorp/raft` 作为一致性协议，内嵌 `boltdb` 作为持久化存储）。
> 2. 计划中是否包含所有必需的 API 和功能模块。
>确认无误后，我们将进入核心存储编码与协议层实现阶段。

## 需求分析：一个注册中心需要的基本功能与一致性保证

一个合格的服务注册中心必须具备以下核心功能：
1. **服务注册与注销**：服务提供者启动时上报自身网络地址及元数据（IP、Port、Weight等），停止时主动注销。
2. **服务发现与查询**：消费者通过接口查询可用服务列表，支持数据拉取及过滤功能。
3. **健康检查 (Health Check)**：维持注册表中实例的健康正确性。轻量化方案通常采用基于客户端的 **TTL 心跳续约**。
4. **状态变更通知 (Watch)**：当某个服务的节点增减时，能以较低的延迟通知客户端（如 HTTP Long Polling 或 gRPC 流式返回）。
5. **高可用与一致性**：集群部署能力，防止单点故障并保障各节点数据的统一。

**如何满足一致性（借鉴 Consul）**：
为了保证数据一致性，注册中心将采用 **CP**（强一致性）模型架构设计：
- **引入 Raft 协议**：集群中各节点通过 Raft 协议实现选主与日志同步。所有的**修改操作**（如注册、注销、健康心跳导致的状态变更）必须转交给 Leader 处理，并在大多数节点持久化并同步后才能算做提交（Commit）。
- **读写一致性**：写请求严格强一致性；对于读请求（服务发现），为了性能，可以允许 Follower 的**最终一致性（Stale Read）**，同时保留强制向 Leader 查询强一致数据的选项。

## Proposed Changes

根据上述分析，计划在项目中构建以下层级和模块：

### 核心数据/存储层 (Registry Store)
- 设计并发安全的内存注册表（维护 Service 和 Instance 数据）。
- 提供针对 HTTP/gRPC API 层面的核心存取封装。
- 引入 `go.etcd.io/bbolt` 进行核心配置和 Raft 快照等持久化落地。

### 分布式共识层 (Raft Consensus)
- 引入第三方库 `github.com/hashicorp/raft` 与 `hashicorp/raft-boltdb`。
- 实现 `FSM (Finite State Machine)`：Raft 应用日志时，同步修改内存的 Registry Store 及持久化存储记录。
- 实现 Peer Join HTTP API，便于同集群内其他节点加入为 Follower 或 Voter。

### 接口通信层 (Transport API)
借鉴 Consul API 设计，提供两种通信方式：
- **HTTP REST API**：
  - `/v1/catalog/register`：实例注册
  - `/v1/catalog/deregister`：实例摘除
  - `/v1/catalog/service/<service>`：服务发现及列表查询
  - `/v1/catalog/heartbeat`：TTL 更新与续命
- **gRPC API**：
  - 定义并编译 protobuf（如 `registry.proto`）。
  - 实现双向的高效注册与发现接口，并探讨后续实现 gRPC Server Stream Watch 通知的可行性。

### 后台健康维护与清理任务
- 启动后台协程，定时遍历内存的 Instance 列表；依据最后一次续约时间剔除或标记不健康的节点，并通过内部提案交由 Raft 提交下线变更。

---

### 前端管理控制台 (Web Dashboard)

技术栈遵循项目约定：**Vue 3 + TypeScript + Vite + Element Plus + Pinia + SCSS**，使用 **Frosted Glass 暗色主题**。

#### 页面规划

| 页面 | 路由 | 功能说明 |
|------|------|----------|
| **仪表盘 (Dashboard)** | `/` | 集群概览：节点数/Leader、服务总数、实例总数及健康率，近期事件流 |
| **服务列表** | `/services` | 展示所有已注册服务名称、实例数、健康实例数，支持搜索与筛选 |
| **服务详情 / 实例列表** | `/services/:name` | 该服务下所有实例（IP:Port、状态、权重、元数据、最后心跳时间），支持手动注销 |
| **集群节点** | `/cluster` | Raft 节点列表（地址、角色 Leader/Follower/Candidate、状态、CommitIndex） |

#### 项目结构 (`web/src/`)

```
web/src/
├── api/
│   └── registry/
│       ├── service.ts          # 服务 CRUD + 发现 API
│       ├── instance.ts         # 实例注册/注销/心跳 API
│       └── cluster.ts          # 集群节点状态 API
├── views/
│   └── registry/
│       ├── dashboard.vue       # 仪表盘
│       ├── services.vue        # 服务列表
│       ├── service-detail.vue  # 实例列表
│       └── cluster.vue         # 集群节点
├── store/
│   └── registry.ts             # Pinia Store（服务/集群数据缓存 + 轮询）
├── components/
│   └── registry/
│       ├── ServiceCard.vue     # 服务统计卡片
│       ├── InstanceTable.vue   # 实例表格组件
│       ├── ClusterStatus.vue   # 集群状态徽章
│       └── EventTimeline.vue   # 事件时间线
├── router/
│   └── registry.ts             # 路由定义
└── styles/
    └── registry.scss           # 页面专属样式（复用 main.scss 全局样式）
```

#### 关键交互设计

1. **仪表盘**
   - 顶部 4 张统计卡片（节点数、服务数、实例数、健康率），使用动画渐入效果。
   - 底部「近期事件」时间线，展示注册/注销/健康状态变更事件，通过 **轮询（10s）** 获取最新数据。

2. **服务列表**
   - 卡片式布局展示各服务，点击卡片跳转到该服务的实例列表。
   - 每张卡片展示：服务名（高亮）、实例总数、健康比例进度条。
   - 顶部搜索框支持按名称实时过滤。

3. **服务详情 / 实例列表**
   - 使用 `el-table` 展示实例列表，列：地址 (IP:Port)、状态（健康/不健康标签）、权重、元数据（JSON 展开）、最后心跳时间。
   - 操作列：手动注销按钮（调用 `/v1/catalog/deregister` API，需二次确认弹窗）。
   - 自动刷新开关（可配合 Pinia Store 的定时轮询）。

4. **集群节点**
   - 使用 `el-table` 展示节点信息。
   - Leader 节点使用醒目的 `el-tag type="success"` 高亮，Follower 为 `info`，Candidate 为 `warning`。
   - 展示各节点的 Raft 日志指标（CommitIndex、AppliedIndex）。

#### 后端 API 适配

前端需要的额外后端 HTTP 接口（补充到后端 Transport 层）：

| 接口 | 方法 | 说明 |
|------|------|------|
| `/v1/catalog/services` | GET | 返回所有服务名列表及各服务的实例计数 |
| `/v1/catalog/service/:name` | GET | 返回指定服务的实例详情列表 |
| `/v1/cluster/members` | GET | 返回 Raft 集群节点列表及角色信息 |
| `/v1/cluster/stats` | GET | 返回集群和注册表聚合统计（节点数、服务数、实例&健康率） |
| `/v1/events` | GET | 返回近期注册/注销/状态变更事件（分页） |

## Verification Plan

### 1. 业务逻辑单元测试 (Automated Tests)
- `go test` 验证内存及 BoltDB Registry Store 的写入、删除及查询行为。
- `go test` 验证基于 TTL 后台定期清理非活跃实例的核心逻辑。

### 2. 本地集群与容灾测试 (Manual Verification)
基于编译后的二进制文件在本地进行集群模拟和测试：
1. **集群搭建**：本地分别在不同端口（如 8000, 8001, 8002）启动三个节点，并互相 Join 以建立 Raft Leader 和 Followers。
2. **数据同步一致性验证**：向节点 1（Leader）发起 `/v1/catalog/register`，随后立即使用 `/v1/catalog/service` 从节点 2 或 3 查询该服务，验证数据实时同步成功。
3. **高可用性故障转移（Failover）**：手动的 `kill` 掉当前的 Leader 进程，观察集群其余两位成员在极短时间内发起选举并推举出新 Leader；随之继续向新 Leader 写入服务注册数据，确认集群读写功能不受影响且已存服务不会丢失。

### 3. 前端验证
- `npm run build` 确认 TypeScript 编译零错误。
- 启动 `npm run dev` + 后端，验证仪表盘统计数据、服务列表卡片渲染、实例表格与手动注销、集群节点角色展示均正常。
- 浏览器测试（手动）：注册一个新服务后，刷新服务列表页确认出现；在实例详情页点击注销后确认实例移除。

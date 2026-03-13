# Eden 注册中心设计文档

## 1. 架构目标
Eden 的设计目标是创建一个轻量、易用且具备强一致性/高可用性可切换能力的注册中心，能够作为 Consul 或 Nacos 的轻量替代方案。

## 2. 核心架构

### 2.1 存储模型 (Store)
- **内存存储**: 所有服务实例信息存储在内存中以保证毫秒级查询。
- **快照机制**: (计划中) CP 模式下支持将状态机快照持久化到磁盘。

### 2.2 一致性模型

#### AP 模式 (Availability & Partition Tolerance)
- **复制机制**: 采用广播形式。当一个节点收到注册请求，它会将该请求同步给所有配置的 Seeds 节点。
- **优势**: 容错性高，任意节点宕机不影响注册功能。

#### CP 模式 (Consistency & Partition Tolerance)
- **协议**: 基于 Hashicorp Raft 实现。
- **角色**: 划分为 Leader, Follower, Candidate。
- **写操作**: 必须由 Leader 接收并达成共识（Quorum）后生效。

### 2.3 健康检查机制
- **TTL 心跳**: 客户端必须在指定的 TTL（默认 30s）内发送心跳。
- **状态流转**: 
  - `passing` (健康): 正常服务。
  - `critical` (警告): 超过 TTL 未收到心跳。
  - `removed`: 长期不健康后自动剔除。

## 3. 接口规范
统一采用 RESTful 风格：
- `POST /v1/catalog/register`: 实例注册
- `POST /v1/catalog/heartbeat`: 心跳续约
- `GET /v1/catalog/service/:name`: 服务发现
- `GET /v1/cluster/stats`: 集群状态监控

## 4. 技术栈
- **Backend**: Go 1.21+, Gin/Standard Mux, Hashicorp Raft
- **Frontend**: Vue 3, Vite, Element Plus, Pinia

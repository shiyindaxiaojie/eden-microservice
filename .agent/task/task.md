# Eden Go Registry 项目任务表

## 1. 需求分析与设计 (当前阶段)
- [x] 分析微服务注册中心的核心需求
- [x] 确定技术选型 (Go, Raft, HTTP/gRPC)
- [x] 输出系统架构设计与实现计划 (Implementation Plan)

## 2. 核心数据结构与配置管理
- [x] 定义服务注册模型 (Service, Instance)
- [x] 实现内存中的服务注册表 (Registry) 及并发安全控制
- [x] 集成 `boltdb` 进行数据的本地持久化存储
- [x] 引入 `viper` 添加 YAML 配置支持 (configs 目录，端口与模式配置)

## 3. 分布式一致性与高可用 (AP/CP 模式)
- [x] (CP) 集成 `hashicorp/raft` 实现 FSM 和选主
- [x] (CP) 实现节点的加入 (Join) 与集群选主网络通信
- [x] (AP) 实现基于 Gossip 或异步 RPC 的对等数据广播与同步
- [x] 增加 AP / CP 工作模式切换逻辑 (默认 AP)

## 4. API 接口实现
- [x] 实现 HTTP API (Register, Deregister, Discovery, HealthCheck)
- [ ] 定义 gRPC Protobuf 并实现 gRPC API
- [ ] 实现服务状态的 Watch/Long Polling 机制

## 5. 健康检查
- [x] 实现基于 TTL 的客户端心跳机制
- [x] 定时清理下线或失联的实例

## 6. 前端管理控制台 (Web Dashboard)
- [x] 初始化 Vite + Vue 3 + TypeScript 项目 (`web/`)
- [x] 全局样式与 Frosted Glass 暗色主题
- [x] 仪表盘页面 (Dashboard)：统计卡片 + 事件时间线
- [x] 服务列表页面：卡片式展示 + 搜索过滤
- [x] 服务详情 / 实例列表页面：表格 + 手动注销
- [x] 集群节点页面：节点角色 + Raft 指标
- [x] API 层 (`api/registry/`) 与 Pinia Store

## 7. 测试与验证
- [ ] 单元测试 (Registry Store, FSM)
- [ ] 集群测试 (单节点功能，多节点数据同步及恢复)
- [ ] 前端构建与页面功能验证
- [ ] 性能压测与优化

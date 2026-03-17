# Eden (伊甸园) - 轻量级服务注册与发现中心

Eden 是一个基于 Go 语言实现的轻量级、高性能服务注册中心，旨在解决微服务架构中服务发现的痛点。它支持 **AP (可用性)** 和 **CP (一致性)** 两种模式，可根据业务场景灵活切换。

## ✨ 特性

- **多模式支持**: 
  - **AP 模式**: 基于 gRPC/QUIC 的异步同步，具备极高的可用性和分区容忍度。
  - **CP 模式**: 基于 Raft 共识算法的强一致性，适用于对元数据一致性要求极高的场景。
- **动态演进**: 支持在运行时动态调整集群模式及配置。
- **传输层优化**: 实验性支持 **QUIC** 协议，提升弱网环境下的集群同步稳定性。
- **高性能存储**: 纯内存存储 + Raft 日志/快照持久化，读写吞吐极高。
- **多适配器**: 提供 Consul 和 Nacos 适配器，支持零代码侵入迁移。
- **现代化 UI**: 响应式仪表盘，支持实时监控、成员管理及配置热更新。

## 🚀 快速开始

### 安装

确保已安装 Go 1.21+ 环境。

```bash
git clone https://github.com/shiyindaxiaojie/eden-go-registry.git
cd eden-go-registry
go mod tidy
```

### 启动服务端

```bash
# 使用默认配置启动
go run ./cmd/server/main.go -config configs/config.yaml
```

### 启动前端管理界面

```bash
cd web
npm install
npm run dev
```

访问 `http://localhost:5173` 即可进入管理后台。

## 目录结构

- `api/proto`: gRPC 协议定义
- `cmd/server`: 服务端入口
- `internal/cluster`: 集群核心实现 (AP/CP)
- `internal/manager`: 业务逻辑协调层
- `internal/store`: 内存存储与持久化引擎
- `pkg`: 客户端 SDK 与第三方注册中心适配器 (Consul/Nacos)
- `web`: 基于 Vue 3 + Element Plus 的前端管理后台

## 📚 文档

详细设计与使用说明请参考 [docs](./docs) 目录：
- [设计文档](./docs/design_zh.md)
- [使用手册](./docs/user_guide_zh.md)

## 📄 开源协议
MIT License

# Eden (伊甸园) - 轻量级服务注册与发现中心

Eden 是一个基于 Go 语言实现的轻量级、高性能服务注册中心，旨在解决微服务架构中服务发现的痛点。它支持 **AP (可用性)** 和 **CP (一致性)** 两种模式，可根据业务场景灵活切换。

## ✨ 特性

- **多模式支持**: 
  - **AP 模式**: 基于 Gossip 协议的最终一致性，具备极高的可用性，类似于 Consul/Eureka。
  - **CP 模式**: 基于 Raft 算法的强一致性，适用于对元数据一致性要求极高的场景。
- **健康检查**: 内置心跳机制，自动剔除故障实例。
- **现代化 UI**: 响应式仪表盘，支持暗黑/亮色模式切换，中英文双语。
- **轻量化**: 无外部数据库依赖，启动极快，内存占用极低。
- **RESTful API**: 标准 HTTP 接口，易于集成。

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
go run ./cmd/server/main.go
```

### 启动前端管理界面

```bash
cd web
yarn install
yarn dev
```

访问 `http://localhost:5173` 即可进入管理后台。

## 目录结构

- `cmd/server`: 服务端入口
- `internal/ap`: AP 模式逻辑实现
- `internal/raft`: CP 模式 (Raft) 逻辑实现
- `internal/store`: 注册中心核心存储引擎
- `internal/handler`: HTTP API 处理器
- `web`: 基于 Vue 3 + Element Plus 的前端管理后台
- `examples`: 集群测试和服务发现示例代码

## 📚 文档

详细设计与使用说明请参考 [docs](./docs) 目录：
- [设计文档](./docs/design_zh.md)
- [使用手册](./docs/user_guide_zh.md)

## 📄 开源协议
MIT License

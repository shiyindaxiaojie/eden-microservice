# Eden 注册中心使用手册

## 1. 部署说明

### 1.1 配置文件说明 (`configs/config.yaml`)

Eden 支持单机、AP 集群和 CP 集群三种部署模式。

#### AP 模式配置 (高可用)
```yaml
node_id: "node-1"
http_addr: ":8500"
data_dir: "./data"
# AP 模式下的集群种子地址（gRPC 端口，如有隔离请确保通畅）
seeds:
  - "127.0.0.1:8501"
  - "127.0.0.1:8502"
```

#### CP 模式配置 (强一致性)
```yaml
node_id: "node-1"
http_addr: ":8500"
raft_addr: "127.0.0.1:7000" # Raft 内部通信地址
data_dir: "./data"
```

### 1.2 启动服务
```bash
# 使用指定配置文件启动
go run ./cmd/server/main.go -config configs/config.yaml
```

## 2. 安全与认证

### 2.1 控制台登录 (JWT)
控制台访问需通过 `/v1/auth/login` 获取 JWT Token。默认用户名/密码：`admin` / `admin-password` (可在配置文件中修改)。

### 2.2 服务注册鉴权 (API Key)
服务提供者在调用管理类接口时必须提供 API Key。
- **Header 方式**: `X-API-Key: eden-default-app-key-123`
- **Query 方式**: `?api_key=eden-default-app-key-123`

## 3. 核心 API 调用示例

### 3.1 登录获取 Token
```bash
curl -X POST http://localhost:8500/v1/auth/login \
-H "Content-Type: application/json" \
-d '{"username":"admin","password":"admin-password"}'
```

### 3.2 注册服务实例
```bash
curl -X POST http://localhost:8500/v1/catalog/register?api_key=eden-default-app-key-123 \
-H "Content-Type: application/json" \
-d '{
  "id": "order-srv-1",
  "name": "order-service",
  "address": "192.168.1.10",
  "port": 8080,
  "check": {
    "ttl": "30s"
  }
}'
```

### 3.3 服务发现 (查询健康实例)
```bash
# 获取 order-service 的健康实例，且属于 dc1 数据中心
curl "http://localhost:8500/v1/catalog/service/order-service?passing=true&dc=dc1"
```

### 3.4 心跳续约
```bash
curl -X POST http://localhost:8500/v1/catalog/heartbeat?api_key=eden-default-app-key-123 \
-H "Content-Type: application/json" \
-d '{"service_name":"order-service", "instance_id":"order-srv-1"}'
```

## 4. 进阶高级配置

### 4.1 传输层优化 (QUIC)
在 `config.yaml` 中可以启用 QUIC 实验性支持，提升跨公网或弱网环境下的集群同步效率。
```yaml
# 启动参数支持
# go run ./cmd/server/main.go -use-quic=true
```

### 4.2 日志级别动态调整
无需重启服务，通过 API 即可热更新日志级别：
```bash
# 修改日志级别为 DEBUG
curl -X POST http://localhost:8500/v1/cluster/settings \
-H "Authorization: Bearer <TOKEN>" \
-d '{"log_level": "DEBUG"}'
```

## 5. 管理后台
前端后台默认启动在 `localhost:5173`。
- **服务治理**: 可视化查看所有注册的服务、实例详情及健康状态。
- **集群状态**: 监控每个节点的 CPU、内存使用情况及 Raft 角色（Leader/Follower）。
- **配置中心**: 在线修改集群运行模式（AP/CP 切换）及种子节点列表。


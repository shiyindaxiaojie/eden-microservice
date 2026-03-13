# Eden 注册中心使用手册

## 1. 部署说明

### 1.1 配置文件说明 (`registry.yaml`)
```yaml
mode: "ap"          # 模式：ap 或 cp
node_id: "node-1"   # 节点唯一标示
http_addr: ":8500"  # API 端口
data_dir: "./data"  # 数据存放目录
seeds:              # AP 模式下的集群种子地址
  - "http://127.0.0.1:8501"
```

### 1.2 启动单机模式
```bash
go run ./cmd/server/main.go -config configs/registry.yaml
```

## 2. 安全与认证

Eden 采用了双重认证机制，分别保护控制台和 API 调用。

### 2.1 控制台登录 (JWT)
控制台访问需通过 `/v1/auth/login` 获取 JWT Token。
- **RBAC 角色**: 
  - `admin`: 拥有完全访问权限。
  - `viewer`: 仅能查看，无法执行管理操作（如集群加入、手动注销等）。

### 2.2 服务注册鉴权 (API Key)
服务提供者在调用 `register`, `heartbeat`, `deregister` 接口时，必须提供 API Key。
- **Header 方式**: `X-API-Key: your-key-here`
- **Query 方式**: `?api_key=your-key-here`

## 3. API 调用示例 (cURL)

### 3.1 登录获取 Token (控制台使用)
```bash
curl -X POST http://localhost:8500/v1/auth/login \
-H "Content-Type: application/json" \
-d '{"username":"admin","password":"admin-password"}'
```

### 3.2 注册服务实例 (带 API Key)
```bash
curl -X POST http://localhost:8500/v1/catalog/register?api_key=eden-default-app-key-123 \
-H "Content-Type: application/json" \
-d '{
  "id": "order-srv-1",
  ...
}'
```

## 3. 集群测试 (Examples)
项目中提供了 `examples` 文件夹，包含自动化测试脚本：

- **Windows**: 进入 `examples/cluster` 运行 `start-windows.bat`。
- **Linux**: 进入 `examples/cluster` 运行 `./start-linux.sh`。

## 4. 管理后台
前端后台默认启动在 `localhost:5173`。
- **仪表盘**: 实时查看集群健康率和节点信息。
- **切换主题**: 右上角开关。
- **语言切换**: 支持中英文切换，默认英文。

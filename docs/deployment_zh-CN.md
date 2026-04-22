# 芙卡洛斯部署与运行指南

## 1. 运行目标

芙卡洛斯支持三种常见运行方式：

- `standalone + ap`
  单机运行，适合本地开发和功能验证。
- `cluster + ap`
  多节点高可用部署，强调可用性和复制效率。
- `cluster + cp`
  多节点强一致部署，强调元数据一致性。

如果没有明确要求，优先使用 `standalone + ap` 作为默认开发模式。

## 2. 启动命令

使用默认配置启动：

```bash
go run ./cmd/server/main.go
```

显式指定配置文件：

```bash
go run ./cmd/server/main.go -config config/config.yaml
```

常用命令行参数：

- `-config`
  指定配置文件路径。
- `-data-dir`
  覆盖数据目录。
- `-node-id`
  覆盖节点 ID。
- `-http-addr`
  覆盖 HTTP 监听地址。
- `-mode`
  覆盖运行模式：`standalone` 或 `cluster`。
- `-consistency`
  覆盖一致性模式：`ap` 或 `cp`。

## 3. 配置基线

配置文件见：

- [`config/config.yaml`](../config/config.yaml)
- [`config/config.yaml.example`](../config/config.yaml.example)

关键配置项分为四类：

### 3.1 节点身份

- `node_id`
- `datacenter`
- `data_dir`

### 3.2 运行模式

- `mode`
- `consistency`
- `bootstrap`

### 3.3 网络端口

- `server.http`
- `server.grpc`
- `server.quic`
- `server.raft`

### 3.4 安全与日志

- `auth.jwt`
- `log`

## 4. 模式建议

### 4.1 本地开发

建议配置：

```yaml
mode: "standalone"
consistency: "ap"
server:
  http: ":8500"
  grpc: "auto"
  quic: "off"
  raft: "off"
```

说明：

- HTTP 固定端口便于控制台代理和命令行调试。
- gRPC 自动端口适合本地快速验证。
- QUIC 和 Raft 默认关闭，避免增加复杂度。

### 4.2 AP 集群

建议配置：

```yaml
mode: "cluster"
consistency: "ap"
server:
  http: ":8500"
  grpc: "auto"
  quic: "off"
  raft: "off"
```

说明：

- AP 模式优先保证系统可用性和节点复制效率。
- 节点间成员信息和运行时设置通过控制面进行维护和分发。

### 4.3 CP 集群

Leader 节点示例：

```yaml
mode: "cluster"
consistency: "cp"
bootstrap: true
server:
  http: ":8500"
  grpc: ":9000"
  raft: "127.0.0.1:7000"
```

Follower 节点示例：

```yaml
mode: "cluster"
consistency: "cp"
bootstrap: false
server:
  http: ":8501"
  grpc: ":9001"
  raft: "127.0.0.1:7001"
```

说明：

- CP 模式下 Raft 地址必须明确可用。
- 首个节点需要 `bootstrap: true`。
- 后续节点通过控制台或集群管理接口加入。

## 5. 控制台

启动前端开发环境：

```bash
cd web
npm install
npm run dev
```

默认地址：

- 控制台：`http://127.0.0.1:2019`
- 后端：`http://127.0.0.1:8500`

默认账户：

- 用户名：`admin`
- 密码：`admin`

## 6. 认证

### 6.1 控制台登录

登录接口为 `POST /v1/auth/login`。

前端提交的是密码的 SHA-256 结果，而不是明文密码。

默认管理员账户：

- 用户名：`admin`
- 密码：`admin`

### 6.2 API Key

如果开启 API Key 校验，以下接口需要携带 `X-API-Key`：

- `POST /v1/catalog/register`
- `POST /v1/catalog/heartbeat`
- `POST /v1/catalog/topology/report`

## 7. 运行验证

常用检查命令：

查询集群成员：

```bash
curl http://127.0.0.1:8500/v1/cluster/members
```

查询服务列表：

```bash
curl http://127.0.0.1:8500/v1/catalog/services
```

查询单个服务实例：

```bash
curl "http://127.0.0.1:8500/v1/catalog/service/order-center?passing=true"
```

## 8. 何时选择哪种模式

| 场景 | 建议 |
| --- | --- |
| 本地联调 | `standalone + ap` |
| 小规模生产环境 | `cluster + ap` |
| 对注册元数据一致性要求高 | `cluster + cp` |

下一步阅读：

- [接入与集成](./integration_zh-CN.md)
- [系统架构](./architecture_zh-CN.md)

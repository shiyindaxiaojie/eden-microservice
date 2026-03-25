# pkg/consul 集成示例

本目录是一套独立的 `pkg/consul` 集成示例。这里的 `pkg/consul` 是仓库内置的兼容层，底层走 Eden 的 HTTP 接口。

## 目录结构

- `cmd/auth-center`
- `cmd/user-center`
- `cmd/order-center`
- `start.bat`

## 启动方式

默认注册中心地址：

```text
CONSUL_ADDR=127.0.0.1:8500
```

启动：

```bash
./examples/service-discovery/consul/start.bat
```

服务名：

- `consul-auth-center`
- `consul-user-center`
- `consul-order-center`

实例端口：

- `auth-center`：`22002`、`22012`、`22022`
- `user-center`：`22001`、`22011`
- `order-center`：`22003`、`22013`

## 服务依赖关系

- `auth-center` 依赖 `user-center`
- `user-center` 依赖 `auth-center`
- `order-center` 同时依赖 `auth-center` 和 `user-center`

## HTTP 测试接口

- `GET /api/auth/token?user_id=1`
- `GET /api/auth/permissions/{userId}`
- `GET /api/users`
- `GET /api/users/{userId}`
- `GET /api/users/{userId}/profile`
- `GET /api/orders/create?user_id=1`
- `GET /api/orders/demo?user_id=1`
- `GET /api/health`

测试示例：

```bash
curl "http://127.0.0.1:22002/api/auth/token?user_id=1"
curl "http://127.0.0.1:22001/api/users/1/profile"
curl "http://127.0.0.1:22003/api/orders/create?user_id=1"
curl "http://127.0.0.1:22003/api/orders/demo?user_id=1"
```

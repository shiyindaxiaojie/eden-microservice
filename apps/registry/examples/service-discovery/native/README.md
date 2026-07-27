# pkg/sdk 集成示例

本目录是一套独立的 `pkg/sdk` 服务发现示例，包含两种完全分开的接入方式：

- `gRPC`：位于 `cmd/grpc/*`
- `HTTP`：位于 `cmd/http/*`

两种方式不会在同一个入口文件里混用地址或协议：

- gRPC 示例只使用 `EDEN_GRPC_ADDRS`
- HTTP 示例只使用 `EDEN_HTTP_ADDRS`

## 目录结构

- `cmd/grpc/auth-center`
- `cmd/grpc/user-center`
- `cmd/grpc/order-center`
- `cmd/http/auth-center`
- `cmd/http/user-center`
- `cmd/http/order-center`
- `start-grpc.bat`
- `start-http.bat`

## 启动方式

### 1. gRPC 集成

默认注册中心直连地址：

```text
EDEN_GRPC_ADDRS=127.0.0.1:9000
```

启动：

```bash
./examples/service-discovery/native/start-grpc.bat
```

服务名：

- `native-grpc-auth-center`
- `native-grpc-user-center`
- `native-grpc-order-center`

实例端口：

- `auth-center`：`21002`、`21012`、`21022`
- `user-center`：`21001`、`21011`
- `order-center`：`21003`、`21013`

### 2. HTTP 集成

默认注册中心地址：

```text
EDEN_HTTP_ADDRS=http://127.0.0.1:8500
```

启动：

```bash
./examples/service-discovery/native/start-http.bat
```

服务名：

- `native-http-auth-center`
- `native-http-user-center`
- `native-http-order-center`

实例端口：

- `auth-center`：`21102`、`21112`、`21122`
- `user-center`：`21101`、`21111`
- `order-center`：`21103`、`21113`

## 服务依赖关系

- `auth-center` 依赖 `user-center`
- `user-center` 依赖 `auth-center`
- `order-center` 同时依赖 `auth-center` 和 `user-center`

## HTTP 测试接口

### auth-center

- `GET /api/auth/token?user_id=1`
- `GET /api/auth/permissions/{userId}`
- `GET /api/health`

示例：

```bash
curl "http://127.0.0.1:21002/api/auth/token?user_id=1"
curl "http://127.0.0.1:21102/api/auth/token?user_id=1"
```

### user-center

- `GET /api/users`
- `GET /api/users/{userId}`
- `GET /api/users/{userId}/profile`
- `GET /api/health`

示例：

```bash
curl "http://127.0.0.1:21001/api/users"
curl "http://127.0.0.1:21001/api/users/1/profile"
curl "http://127.0.0.1:21101/api/users/1/profile"
```

### order-center

- `GET /api/orders/create?user_id=1`
- `GET /api/orders/demo?user_id=1`
- `GET /api/health`

示例：

```bash
curl "http://127.0.0.1:21003/api/orders/create?user_id=1"
curl "http://127.0.0.1:21003/api/orders/demo?user_id=1"
curl "http://127.0.0.1:21103/api/orders/create?user_id=1"
```

接口响应里会返回被调用的上游实例地址，便于验证服务发现和服务间调用是否正确。


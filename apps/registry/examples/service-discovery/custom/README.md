# 自定义协议集成示例

本目录是一套独立的“自定义接入”示例，不使用 `pkg/sdk` SDK，而是直接按协议调用注册中心。

目录内分成两套完全隔离的实现：

- `gRPC`：位于 `cmd/grpc/*`，底层直接调用 `RegistryService`
- `HTTP`：位于 `cmd/http/*`，底层直接调用 `/v1/catalog/*`

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

### 1. 纯 gRPC 自定义接入

默认注册中心直连地址：

```text
CUSTOM_GRPC_ADDRS=127.0.0.1:9000
```

启动：

```bash
./examples/service-discovery/custom/start-grpc.bat
```

服务名：

- `custom-grpc-auth-center`
- `custom-grpc-user-center`
- `custom-grpc-order-center`

实例端口：

- `auth-center`：`24002`、`24012`、`24022`
- `user-center`：`24001`、`24011`
- `order-center`：`24003`、`24013`

### 2. 纯 HTTP 自定义接入

默认注册中心地址：

```text
CUSTOM_HTTP_ADDRS=http://127.0.0.1:8500
```

启动：

```bash
./examples/service-discovery/custom/start-http.bat
```

服务名：

- `custom-http-auth-center`
- `custom-http-user-center`
- `custom-http-order-center`

实例端口：

- `auth-center`：`24102`、`24112`、`24122`
- `user-center`：`24101`、`24111`
- `order-center`：`24103`、`24113`

## 服务依赖关系

- `auth-center` 依赖 `user-center`
- `user-center` 依赖 `auth-center`
- `order-center` 同时依赖 `auth-center` 和 `user-center`

## HTTP 测试接口

### auth-center

- `GET /api/auth/token?user_id=1`
- `GET /api/auth/permissions/{userId}`
- `GET /api/health`

### user-center

- `GET /api/users`
- `GET /api/users/{userId}`
- `GET /api/users/{userId}/profile`
- `GET /api/health`

### order-center

- `GET /api/orders/create?user_id=1`
- `GET /api/orders/demo?user_id=1`
- `GET /api/health`

测试示例：

```bash
curl "http://127.0.0.1:24002/api/auth/token?user_id=1"
curl "http://127.0.0.1:24001/api/users/1/profile"
curl "http://127.0.0.1:24003/api/orders/create?user_id=1"

curl "http://127.0.0.1:24102/api/auth/token?user_id=1"
curl "http://127.0.0.1:24101/api/users/1/profile"
curl "http://127.0.0.1:24103/api/orders/demo?user_id=1"
```


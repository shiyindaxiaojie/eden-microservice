<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from '../utils/i18n'

type FeatureCard = {
  title: string
  desc: string
  tags: string[]
}

type SourceCard = {
  title: string
  path: string
  desc: string
}

type QuickBlock = {
  title: string
  desc: string
  code?: string
  hint?: string
}

type IntegrationCard = {
  title: string
  badge: string
  desc: string
  code: string
  notes: string[]
}

type ApiRow = {
  endpoint: string
  method: string
  auth: string
  desc: string
}

type ApiGroup = {
  title: string
  note: string
  rows: ApiRow[]
}

const { locale, t } = useI18n()
const activeSection = ref('intro')

const sections = [
  { id: 'intro', title: 'intro' },
  { id: 'quickStart', title: 'quickStart' },
  { id: 'integration', title: 'integration' },
  { id: 'api', title: 'apiDesc' }
]

const isZh = computed(() => locale.value === 'zh')
const text = (zh: string, en: string) => (isZh.value ? zh : en)

const introLead = computed(() =>
  text(
    'Eden 是一个基于 Go 实现的服务注册与发现中心，当前代码同时提供 HTTP API、gRPC、gRPC over QUIC、AP/CP 双模式、命名空间、依赖拓扑、RBAC 与控制台能力。',
    'Eden is a Go-based service registry that currently ships with HTTP APIs, gRPC, gRPC over QUIC, AP/CP modes, namespaces, dependency topology, RBAC, and a management console.'
  )
)

const featureCards = computed<FeatureCard[]>(() => {
  if (isZh.value) {
    return [
      {
        title: '双模式注册中心',
        desc: '通过 `mode` 在 AP 与 CP 之间切换。AP 走异步复制，CP 走 Raft 强一致，启动入口统一在 `cmd/server/main.go`。',
        tags: ['AP', 'CP', 'Raft']
      },
      {
        title: '多协议接入',
        desc: 'HTTP 面向控制台与兼容集成，gRPC 是 Eden 原生接入主路径，QUIC 作为 gRPC 的可选传输层。',
        tags: ['HTTP', 'gRPC', 'QUIC']
      },
      {
        title: '客户端能力完整',
        desc: '`pkg/eden` 内置节点自动发现、故障转移、本地缓存、Watch 订阅、拓扑上报与心跳自恢复。',
        tags: ['Failover', 'Watch', 'Cache']
      },
      {
        title: '运维与治理',
        desc: '控制台与后端已支持命名空间、节点管理、API Key、JWT 登录、用户/RBAC、日志与事件配置。',
        tags: ['JWT', 'RBAC', 'Namespace']
      }
    ]
  }

  return [
    {
      title: 'Dual-mode Registry',
      desc: 'Switch between AP and CP with `mode`. AP uses asynchronous replication, while CP uses Raft with the same server entrypoint in `cmd/server/main.go`.',
      tags: ['AP', 'CP', 'Raft']
    },
    {
      title: 'Multiple Protocols',
      desc: 'HTTP is used for console and compatibility integrations, gRPC is the native Eden path, and QUIC is an optional transport for gRPC.',
      tags: ['HTTP', 'gRPC', 'QUIC']
    },
    {
      title: 'Rich Client Features',
      desc: '`pkg/eden` includes node discovery, failover, local cache, Watch subscriptions, topology reporting, and heartbeat self-recovery.',
      tags: ['Failover', 'Watch', 'Cache']
    },
    {
      title: 'Ops and Governance',
      desc: 'The console and backend already support namespaces, node management, API keys, JWT login, RBAC, logs, and event settings.',
      tags: ['JWT', 'RBAC', 'Namespace']
    }
  ]
})

const sourceCards = computed<SourceCard[]>(() => {
  if (isZh.value) {
    return [
      {
        title: '服务端启动入口',
        path: 'cmd/server/main.go',
        desc: '负责加载配置、初始化 AP/CP、启动 HTTP/gRPC/QUIC 服务以及健康检查。'
      },
      {
        title: 'HTTP 路由层',
        path: 'internal/handler/',
        desc: '定义 `/v1/catalog/*`、`/v1/auth/*`、`/v1/settings/*`、`/v1/cluster/*` 等接口。'
      },
      {
        title: '原生客户端',
        path: 'pkg/eden/client.go',
        desc: '封装注册、发现、订阅、心跳、节点同步、缓存与故障转移逻辑。'
      },
      {
        title: 'gRPC 协议定义',
        path: 'api/proto/registry/v1/registry.proto',
        desc: '定义 Register、SetInstanceStatus、Heartbeat、Discover、Watch 等原生接口。'
      }
    ]
  }

  return [
    {
      title: 'Server Entry',
      path: 'cmd/server/main.go',
      desc: 'Loads config, initializes AP/CP, and starts HTTP, gRPC, QUIC, plus health checking.'
    },
    {
      title: 'HTTP Route Layer',
      path: 'internal/handler/',
      desc: 'Defines `/v1/catalog/*`, `/v1/auth/*`, `/v1/settings/*`, `/v1/cluster/*`, and related endpoints.'
    },
    {
      title: 'Native Client',
      path: 'pkg/eden/client.go',
      desc: 'Implements registration, discovery, subscriptions, heartbeats, node sync, cache, and failover.'
    },
    {
      title: 'gRPC Contract',
      path: 'api/proto/registry/v1/registry.proto',
      desc: 'Defines Register, SetInstanceStatus, Heartbeat, Discover, Watch, and other native RPCs.'
    }
  ]
})

const quickBlocks = computed<QuickBlock[]>(() => {
  if (isZh.value) {
    return [
      {
        title: '1. 启动后端注册中心',
        desc: '当前服务端实际支持的命令行参数是 `-config`、`-data-dir`、`-node-id`、`-http-addr`。快速启动推荐直接使用配置文件。',
        code: `go run ./cmd/server/main.go -config configs/config.yaml`,
        hint: '默认配置下：HTTP API 监听 `:8500`，`mode=ap`，`data_dir=./data`，JWT 登录已开启。'
      },
      {
        title: '2. 如果要启用 CP 首节点，请改配置文件而不是加 CLI 参数',
        desc: '当前代码并没有 `-bootstrap` 和 `-mode=cp` 这两个启动参数，CP 首节点要通过配置项 `mode` 与 `bootstrap` 控制。',
        code: `node_id: "node-1"
mode: "cp"
http_addr: ":8500"
raft_addr: "127.0.0.1:7000"
bootstrap: true
data_dir: "./data"`,
        hint: '其他 CP 节点通过 `join: "http://127.0.0.1:8500"` 加入现有集群。'
      },
      {
        title: '3. 启动前端控制台',
        desc: '前端是独立的 Vite 应用，开发环境默认端口来自 `web/vite.config.ts`，当前为 `2019`，并反向代理 `/v1` 到 `http://127.0.0.1:8500`。',
        code: `cd web
npm install
npm run dev`,
        hint: '开发控制台默认地址为 `http://127.0.0.1:2019`，不是 `8500`。'
      },
      {
        title: '4. 冒烟测试：注册并发现一个实例',
        desc: '当前 `configs/config.yaml` 只开启了 JWT，没有开启 API Key，所以按默认配置可以直接调用注册接口；如果你开启了 `auth.api_key.enabled=true`，需要额外带上 `X-API-Key`。',
        code: `curl -X POST http://127.0.0.1:8500/v1/catalog/register \\
  -H "Content-Type: application/json" \\
  -d '{
    "id": "user-center-1",
    "service_name": "user-center",
    "namespace": "default",
    "host": "127.0.0.1",
    "port": 9001,
    "weight": 100,
    "dc": "dc1",
    "metadata": {"version": "1.0.0"}
  }'

curl "http://127.0.0.1:8500/v1/catalog/service/user-center?passing=true&namespace=default"`,
        hint: '仓库里也已经内置了完整示例，可参考 `examples/service-discovery/cmd/*`。'
      }
    ]
  }

  return [
    {
      title: '1. Start the Backend Registry',
      desc: 'The current server only supports `-config`, `-data-dir`, `-node-id`, and `-http-addr`. The recommended quick start is config-driven.',
      code: `go run ./cmd/server/main.go -config configs/config.yaml`,
      hint: 'With the default config: HTTP API listens on `:8500`, `mode=ap`, `data_dir=./data`, and JWT login is enabled.'
    },
    {
      title: '2. For a CP bootstrap node, change config instead of CLI flags',
      desc: 'The codebase does not expose `-bootstrap` or `-mode=cp` flags. CP bootstrap is controlled by `mode` and `bootstrap` in config.',
      code: `node_id: "node-1"
mode: "cp"
http_addr: ":8500"
raft_addr: "127.0.0.1:7000"
bootstrap: true
data_dir: "./data"`,
      hint: 'Additional CP nodes can join with `join: "http://127.0.0.1:8500"`.'
    },
    {
      title: '3. Start the Console',
      desc: 'The frontend is a standalone Vite app. In development, the port comes from `web/vite.config.ts`, currently `2019`, and `/v1` is proxied to `http://127.0.0.1:8500`.',
      code: `cd web
npm install
npm run dev`,
      hint: 'The development console lives at `http://127.0.0.1:2019`, not `8500`.'
    },
    {
      title: '4. Smoke Test: register and discover one instance',
      desc: 'The current `configs/config.yaml` enables JWT only, not API keys, so registration works without `X-API-Key` by default. If you enable API key auth, add the header.',
      code: `curl -X POST http://127.0.0.1:8500/v1/catalog/register \\
  -H "Content-Type: application/json" \\
  -d '{
    "id": "user-center-1",
    "service_name": "user-center",
    "namespace": "default",
    "host": "127.0.0.1",
    "port": 9001,
    "weight": 100,
    "dc": "dc1",
    "metadata": {"version": "1.0.0"}
  }'

curl "http://127.0.0.1:8500/v1/catalog/service/user-center?passing=true&namespace=default"`,
      hint: 'There is also a complete runnable demo under `examples/service-discovery/cmd/*`.'
    }
  ]
})

const accessItems = computed(() => {
  if (isZh.value) {
    return [
      { label: '后端 API', value: 'http://127.0.0.1:8500' },
      { label: '前端控制台', value: 'http://127.0.0.1:2019' },
      { label: '默认账号', value: 'admin / admin' },
      { label: '默认模式', value: 'ap' },
      { label: '认证状态', value: 'JWT 开启，API Key 默认未开启' }
    ]
  }

  return [
    { label: 'Backend API', value: 'http://127.0.0.1:8500' },
    { label: 'Console', value: 'http://127.0.0.1:2019' },
    { label: 'Default Login', value: 'admin / admin' },
    { label: 'Default Mode', value: 'ap' },
    { label: 'Auth State', value: 'JWT enabled, API key disabled by default' }
  ]
})

const integrationCards = computed<IntegrationCard[]>(() => {
  if (isZh.value) {
    return [
      {
        title: '原生 Eden SDK',
        badge: '推荐',
        desc: '适合直接接入 Eden 的 Go 服务。默认走 gRPC，可切到 QUIC 或 HTTP，并自动完成节点发现与故障转移。',
        code: `import (
  "fmt"
  "log"
  "time"

  "github.com/shiyindaxiaojie/eden-go-registry/pkg/eden"
  "github.com/shiyindaxiaojie/eden-go-registry/pkg/registry"
)

client, err := eden.NewWithConfig(&eden.Config{
  Addresses:  []string{"http://127.0.0.1:8500"},
  Datacenter: "dc1",
  Namespace:  "default",
  CacheDir:   "./.eden-cache",
  Transport:  "grpc", // grpc | quic | http
})
if err != nil {
  log.Fatal(err)
}
defer client.Close()

instance := &registry.ServiceInstance{
  ID:          "order-center-1",
  ServiceName: "order-center",
  Host:        "127.0.0.1",
  Port:        9003,
  Weight:      100,
  Metadata:    map[string]string{"version": "1.0.0"},
}

if err := client.Register(instance); err != nil {
  log.Fatal(err)
}

go func() {
  ticker := time.NewTicker(10 * time.Second)
  defer ticker.Stop()
  for range ticker.C {
    if err := client.Heartbeat(instance); err != nil {
      log.Printf("heartbeat failed: %v", err)
    }
  }
}()

client.Subscribe("user-center", func(instances []*registry.ServiceInstance) {
  log.Printf("user-center updated: %d instances", len(instances))
})

providers, err := client.Discovery("user-center")
if err != nil {
  log.Fatal(err)
}
fmt.Println(providers[0].Host, providers[0].Port)`,
        notes: [
          '`Addresses` 只需要提供一个可达入口，客户端会通过 `/v1/cluster/members` 自动同步其他节点。',
          '`Subscribe` 优先使用 gRPC Watch 流；如果不可用，会自动退化为轮询。',
          '`Heartbeat` 在实例丢失时会触发自动重新注册，适合做轻量自恢复。'
        ]
      },
      {
        title: '统一注册中心抽象',
        badge: '兼容层',
        desc: '如果业务代码希望在 Eden、Consul、Nacos 之间切换，可以使用 `pkg/registry` + `pkg/registry/factory`。',
        code: `import (
  "github.com/shiyindaxiaojie/eden-go-registry/pkg/registry"
  "github.com/shiyindaxiaojie/eden-go-registry/pkg/registry/factory"
)

reg, err := factory.NewRegistry(&registry.Config{
  Type:       "eden", // 也可以换成 consul / nacos
  Addresses:  []string{"http://127.0.0.1:8500"},
  Datacenter: "dc1",
})
if err != nil {
  panic(err)
}
defer reg.Close()

_ = reg.Register(&registry.ServiceInstance{
  ID:          "user-center-1",
  ServiceName: "user-center",
  Host:        "127.0.0.1",
  Port:        9001,
  Weight:      100,
})

instances, err := reg.Discovery("user-center")
if err != nil {
  panic(err)
}

_ = reg.Subscribe("user-center", func(items []*registry.ServiceInstance) {
  // react to updates
})`,
        notes: [
          '业务侧只依赖 `registry.Registry` 接口，减少切换注册中心的改动面。',
          '当前工厂在 `Type=eden` 时默认使用 gRPC 传输。',
          'Consul 与 Nacos 适配器也已经在 `pkg/consul`、`pkg/nacos` 中落地。'
        ]
      },
      {
        title: '直接走 HTTP 接口',
        badge: '最轻量',
        desc: '适合非 Go 服务，或只想做最小集成。下面的示例字段与后端 handler 当前实现完全对齐。',
        code: `payload := map[string]any{
  "id":           "user-center-1",
  "service_name": "user-center",
  "namespace":    "default",
  "host":         "127.0.0.1",
  "port":         9001,
  "weight":       100,
  "dc":           "dc1",
  "metadata":     map[string]string{"version": "1.0.0"},
}

body, _ := json.Marshal(payload)
req, _ := http.NewRequest(
  "POST",
  "http://127.0.0.1:8500/v1/catalog/register",
  bytes.NewReader(body),
)
req.Header.Set("Content-Type", "application/json")
// req.Header.Set("X-API-Key", "your-key") // 仅在 auth.api_key.enabled=true 时需要

resp, err := http.DefaultClient.Do(req)
if err != nil {
  panic(err)
}
defer resp.Body.Close()`,
        notes: [
          '注册与心跳接口的 API Key 校验只有在 `auth.api_key.enabled=true` 时才会生效。',
          '如果你需要记录调用拓扑，可在服务发现时传 `consumer_service`，或用 SDK 自动上报。',
          '控制台登录接口 `POST /v1/auth/login` 期望的是原始密码的 SHA-256 值，而不是明文密码。'
        ]
      }
    ]
  }

  return [
    {
      title: 'Native Eden SDK',
      badge: 'Recommended',
      desc: 'Best for Go services integrating directly with Eden. It uses gRPC by default, can switch to QUIC or HTTP, and handles node discovery plus failover automatically.',
      code: `import (
  "fmt"
  "log"
  "time"

  "github.com/shiyindaxiaojie/eden-go-registry/pkg/eden"
  "github.com/shiyindaxiaojie/eden-go-registry/pkg/registry"
)

client, err := eden.NewWithConfig(&eden.Config{
  Addresses:  []string{"http://127.0.0.1:8500"},
  Datacenter: "dc1",
  Namespace:  "default",
  CacheDir:   "./.eden-cache",
  Transport:  "grpc", // grpc | quic | http
})
if err != nil {
  log.Fatal(err)
}
defer client.Close()

instance := &registry.ServiceInstance{
  ID:          "order-center-1",
  ServiceName: "order-center",
  Host:        "127.0.0.1",
  Port:        9003,
  Weight:      100,
  Metadata:    map[string]string{"version": "1.0.0"},
}

if err := client.Register(instance); err != nil {
  log.Fatal(err)
}

go func() {
  ticker := time.NewTicker(10 * time.Second)
  defer ticker.Stop()
  for range ticker.C {
    if err := client.Heartbeat(instance); err != nil {
      log.Printf("heartbeat failed: %v", err)
    }
  }
}()

client.Subscribe("user-center", func(instances []*registry.ServiceInstance) {
  log.Printf("user-center updated: %d instances", len(instances))
})

providers, err := client.Discovery("user-center")
if err != nil {
  log.Fatal(err)
}
fmt.Println(providers[0].Host, providers[0].Port)`,
      notes: [
        '`Addresses` only needs one reachable entrypoint. The client syncs additional nodes through `/v1/cluster/members`.',
        '`Subscribe` prefers gRPC Watch and automatically falls back to polling when needed.',
        '`Heartbeat` can re-register an instance automatically when the registry loses it.'
      ]
    },
    {
      title: 'Unified Registry Abstraction',
      badge: 'Compatibility',
      desc: 'Use `pkg/registry` plus `pkg/registry/factory` when business code needs to switch between Eden, Consul, and Nacos.',
      code: `import (
  "github.com/shiyindaxiaojie/eden-go-registry/pkg/registry"
  "github.com/shiyindaxiaojie/eden-go-registry/pkg/registry/factory"
)

reg, err := factory.NewRegistry(&registry.Config{
  Type:       "eden", // can also be consul / nacos
  Addresses:  []string{"http://127.0.0.1:8500"},
  Datacenter: "dc1",
})
if err != nil {
  panic(err)
}
defer reg.Close()

_ = reg.Register(&registry.ServiceInstance{
  ID:          "user-center-1",
  ServiceName: "user-center",
  Host:        "127.0.0.1",
  Port:        9001,
  Weight:      100,
})

instances, err := reg.Discovery("user-center")
if err != nil {
  panic(err)
}

_ = reg.Subscribe("user-center", func(items []*registry.ServiceInstance) {
  // react to updates
})`,
      notes: [
        'Business code only depends on `registry.Registry`, which minimizes migration cost.',
        'The Eden factory path currently defaults to gRPC transport.',
        'Consul and Nacos adapters are already implemented under `pkg/consul` and `pkg/nacos`.'
      ]
    },
    {
      title: 'Direct HTTP Integration',
      badge: 'Minimal',
      desc: 'Useful for non-Go services or very small integrations. The payload below matches the current HTTP handlers exactly.',
      code: `payload := map[string]any{
  "id":           "user-center-1",
  "service_name": "user-center",
  "namespace":    "default",
  "host":         "127.0.0.1",
  "port":         9001,
  "weight":       100,
  "dc":           "dc1",
  "metadata":     map[string]string{"version": "1.0.0"},
}

body, _ := json.Marshal(payload)
req, _ := http.NewRequest(
  "POST",
  "http://127.0.0.1:8500/v1/catalog/register",
  bytes.NewReader(body),
)
req.Header.Set("Content-Type", "application/json")
// req.Header.Set("X-API-Key", "your-key") // only if auth.api_key.enabled=true

resp, err := http.DefaultClient.Do(req)
if err != nil {
  panic(err)
}
defer resp.Body.Close()`,
      notes: [
        'API key checks on register and heartbeat only apply when `auth.api_key.enabled=true`.',
        'To record consumer topology, pass `consumer_service` on discovery or use the SDK.',
        'The console login endpoint `POST /v1/auth/login` expects the SHA-256 value of the original password, not plaintext.'
      ]
    }
  ]
})

const loginExample = computed(() =>
  `curl -X POST http://127.0.0.1:8500/v1/auth/login \\
  -H "Content-Type: application/json" \\
  -d '{
    "username": "admin",
    "password": "8c6976e5b5410415bde908bd4dee15dfb167a9c873fc4bb8a81f6f2ab448a918"
  }'`
)

const apiGroups = computed<ApiGroup[]>(() => {
  if (isZh.value) {
    return [
      {
        title: '认证与集群元数据',
        note: '用于控制台登录、节点发现和基础信息查询。',
        rows: [
          {
            endpoint: '/v1/auth/login',
            method: 'POST',
            auth: '公开',
            desc: '控制台登录。`password` 需传原始密码的 SHA-256 值，前端登录页已自动处理。'
          },
          {
            endpoint: '/v1/auth/profile',
            method: 'GET / POST',
            auth: 'Bearer JWT',
            desc: '获取或更新当前用户昵称、手机号、邮箱等资料。'
          },
          {
            endpoint: '/v1/auth/password',
            method: 'POST',
            auth: 'Bearer JWT',
            desc: '修改当前用户密码，`old/new` 的处理方式与登录一致。'
          },
          {
            endpoint: '/v1/cluster/members',
            method: 'GET',
            auth: '公开',
            desc: '返回节点列表与 HTTP/gRPC/QUIC 地址，SDK 会用它做节点同步与故障转移。'
          },
          {
            endpoint: '/v1/node/info',
            method: 'GET',
            auth: '公开',
            desc: '返回本节点配置快照，节点管理页与集群加入逻辑会用到。'
          }
        ]
      },
      {
        title: '服务注册与发现',
        note: '最常用的一组接口，对应 `internal/handler/catalog_handler.go`。',
        rows: [
          {
            endpoint: '/v1/catalog/register',
            method: 'POST',
            auth: 'API Key（若已开启）',
            desc: '注册实例。常用字段：`id`、`service_name`、`namespace`、`host`、`port`、`weight`、`dc`、`metadata`。'
          },
          {
            endpoint: '/v1/catalog/heartbeat',
            method: 'POST',
            auth: 'API Key（若已开启）',
            desc: '实例续约。请求体包含 `namespace`、`service_name`、`instance_id`。'
          },
          {
            endpoint: '/v1/catalog/instance/status',
            method: 'POST',
            auth: 'API Key 或 Admin/Developer',
            desc: '将实例标记为 `online` 或 `offline`，用于手动上下线。'
          },
          {
            endpoint: '/v1/catalog/services',
            method: 'GET',
            auth: '公开',
            desc: '按命名空间列出所有服务。'
          },
          {
            endpoint: '/v1/catalog/service/{name}',
            method: 'GET',
            auth: '公开',
            desc: '服务发现接口。支持 `passing`、`namespace`、`dc`、`consumer_service` 查询参数。'
          },
          {
            endpoint: '/v1/catalog/service/{name}/subscribers',
            method: 'GET',
            auth: '公开',
            desc: '查询当前服务的订阅方列表。'
          },
          {
            endpoint: '/v1/catalog/dependency-graph',
            method: 'GET',
            auth: '公开',
            desc: '返回服务依赖拓扑图。'
          },
          {
            endpoint: '/v1/catalog/topology',
            method: 'GET',
            auth: '公开',
            desc: '返回按消费者聚合后的调用拓扑视图。'
          },
          {
            endpoint: '/v1/catalog/topology/report',
            method: 'POST',
            auth: 'API Key（若已开启）',
            desc: 'SDK 用于上报消费者与提供者关系，通常不需要手写调用。'
          }
        ]
      },
      {
        title: '命名空间、设置与节点管理',
        note: '这部分接口主要服务于控制台和集群管理能力。',
        rows: [
          {
            endpoint: '/v1/namespaces',
            method: 'GET',
            auth: 'Admin / Developer',
            desc: '列出所有命名空间。'
          },
          {
            endpoint: '/v1/namespace',
            method: 'POST / PUT / DELETE',
            auth: 'Admin / Developer',
            desc: '新增、修改、删除命名空间。删除时通过 `?name=` 指定目标。'
          },
          {
            endpoint: '/v1/settings/mode',
            method: 'GET / POST',
            auth: 'Admin / Developer',
            desc: '读取或修改运行模式、环境与日志级别。'
          },
          {
            endpoint: '/v1/settings/system',
            method: 'GET / POST',
            auth: 'Admin / Developer',
            desc: '读取或应用系统设置，如事件保留、实例移除延迟等。'
          },
          {
            endpoint: '/v1/settings/storage',
            method: 'GET / POST',
            auth: 'Admin / Developer',
            desc: '获取或更新日志保留、事件类型、心跳失败阈值等存储相关配置。'
          },
          {
            endpoint: '/v1/cluster/join',
            method: 'POST',
            auth: 'Admin',
            desc: 'CP 模式下将节点加入 Raft 集群。'
          },
          {
            endpoint: '/v1/cluster/member',
            method: 'POST / DELETE',
            auth: 'Admin',
            desc: '新增或移除节点，控制台“节点管理”页会调用它。'
          },
          {
            endpoint: '/v1/cluster/stats',
            method: 'GET',
            auth: 'Admin / Developer',
            desc: '返回节点数、角色、Leader、服务数、实例数、健康率与内存使用。'
          }
        ]
      },
      {
        title: 'RBAC 与 API Key 管理',
        note: '用于控制台后台治理，不是业务服务接入的必选项。',
        rows: [
          {
            endpoint: '/v1/rbac/users',
            method: 'GET',
            auth: 'Bearer JWT',
            desc: '获取用户列表。'
          },
          {
            endpoint: '/v1/rbac/user',
            method: 'POST',
            auth: 'Admin',
            desc: '新增或更新用户。'
          },
          {
            endpoint: '/v1/rbac/user/delete',
            method: 'DELETE',
            auth: 'Admin',
            desc: '删除非内置用户。'
          },
          {
            endpoint: '/v1/settings/apikeys',
            method: 'GET',
            auth: 'Admin',
            desc: '列出 API Key。'
          },
          {
            endpoint: '/v1/settings/apikey',
            method: 'POST',
            auth: 'Admin',
            desc: '新增 API Key。'
          },
          {
            endpoint: '/v1/settings/apikey/delete',
            method: 'DELETE',
            auth: 'Admin',
            desc: '删除 API Key。'
          }
        ]
      }
    ]
  }

  return [
    {
      title: 'Authentication and Cluster Metadata',
      note: 'Used by the console login flow, node discovery, and basic cluster lookups.',
      rows: [
        {
          endpoint: '/v1/auth/login',
          method: 'POST',
          auth: 'Public',
          desc: 'Console login. The `password` field must be the SHA-256 value of the original password. The frontend already does this.'
        },
        {
          endpoint: '/v1/auth/profile',
          method: 'GET / POST',
          auth: 'Bearer JWT',
          desc: 'Read or update the current user profile.'
        },
        {
          endpoint: '/v1/auth/password',
          method: 'POST',
          auth: 'Bearer JWT',
          desc: 'Change the current password. `old/new` follow the same hashed convention as login.'
        },
        {
          endpoint: '/v1/cluster/members',
          method: 'GET',
          auth: 'Public',
          desc: 'Returns node membership plus HTTP/gRPC/QUIC addresses. The SDK uses this for node sync and failover.'
        },
        {
          endpoint: '/v1/node/info',
          method: 'GET',
          auth: 'Public',
          desc: 'Returns the local node config snapshot.'
        }
      ]
    },
    {
      title: 'Service Registration and Discovery',
      note: 'These are the most commonly integrated endpoints and map to `internal/handler/catalog_handler.go`.',
      rows: [
        {
          endpoint: '/v1/catalog/register',
          method: 'POST',
          auth: 'API key (if enabled)',
          desc: 'Register an instance. Common fields: `id`, `service_name`, `namespace`, `host`, `port`, `weight`, `dc`, and `metadata`.'
        },
        {
          endpoint: '/v1/catalog/heartbeat',
          method: 'POST',
          auth: 'API key (if enabled)',
          desc: 'Renew an instance with `namespace`, `service_name`, and `instance_id`.'
        },
        {
          endpoint: '/v1/catalog/instance/status',
          method: 'POST',
          auth: 'API key or Admin/Developer',
          desc: 'Set an instance to `online` or `offline`.'
        },
        {
          endpoint: '/v1/catalog/services',
          method: 'GET',
          auth: 'Public',
          desc: 'List all services in a namespace.'
        },
        {
          endpoint: '/v1/catalog/service/{name}',
          method: 'GET',
          auth: 'Public',
          desc: 'Discover service instances. Supports `passing`, `namespace`, `dc`, and `consumer_service` query params.'
        },
        {
          endpoint: '/v1/catalog/service/{name}/subscribers',
          method: 'GET',
          auth: 'Public',
          desc: 'List subscribers of the given service.'
        },
        {
          endpoint: '/v1/catalog/dependency-graph',
          method: 'GET',
          auth: 'Public',
          desc: 'Return the dependency graph.'
        },
        {
          endpoint: '/v1/catalog/topology',
          method: 'GET',
          auth: 'Public',
          desc: 'Return the aggregated consumer-provider topology view.'
        },
        {
          endpoint: '/v1/catalog/topology/report',
          method: 'POST',
          auth: 'API key (if enabled)',
          desc: 'Used by the SDK to report consumer-provider topology automatically.'
        }
      ]
    },
    {
      title: 'Namespaces, Settings, and Node Management',
      note: 'These endpoints mainly serve the console and cluster operations.',
      rows: [
        {
          endpoint: '/v1/namespaces',
          method: 'GET',
          auth: 'Admin / Developer',
          desc: 'List namespaces.'
        },
        {
          endpoint: '/v1/namespace',
          method: 'POST / PUT / DELETE',
          auth: 'Admin / Developer',
          desc: 'Create, update, or delete a namespace. Deletion uses the `?name=` query parameter.'
        },
        {
          endpoint: '/v1/settings/mode',
          method: 'GET / POST',
          auth: 'Admin / Developer',
          desc: 'Read or update mode, environment, and log level.'
        },
        {
          endpoint: '/v1/settings/system',
          method: 'GET / POST',
          auth: 'Admin / Developer',
          desc: 'Read or apply system settings such as retention and removal delay.'
        },
        {
          endpoint: '/v1/settings/storage',
          method: 'GET / POST',
          auth: 'Admin / Developer',
          desc: 'Manage log retention, event types, and heartbeat failure thresholds.'
        },
        {
          endpoint: '/v1/cluster/join',
          method: 'POST',
          auth: 'Admin',
          desc: 'Join a node into the Raft cluster in CP mode.'
        },
        {
          endpoint: '/v1/cluster/member',
          method: 'POST / DELETE',
          auth: 'Admin',
          desc: 'Add or remove a node from the cluster.'
        },
        {
          endpoint: '/v1/cluster/stats',
          method: 'GET',
          auth: 'Admin / Developer',
          desc: 'Return node count, role, leader, service count, instance count, health rate, and memory usage.'
        }
      ]
    },
    {
      title: 'RBAC and API Key Management',
      note: 'Useful for console governance rather than service-side integration.',
      rows: [
        {
          endpoint: '/v1/rbac/users',
          method: 'GET',
          auth: 'Bearer JWT',
          desc: 'List users.'
        },
        {
          endpoint: '/v1/rbac/user',
          method: 'POST',
          auth: 'Admin',
          desc: 'Create or update a user.'
        },
        {
          endpoint: '/v1/rbac/user/delete',
          method: 'DELETE',
          auth: 'Admin',
          desc: 'Delete a non-built-in user.'
        },
        {
          endpoint: '/v1/settings/apikeys',
          method: 'GET',
          auth: 'Admin',
          desc: 'List API keys.'
        },
        {
          endpoint: '/v1/settings/apikey',
          method: 'POST',
          auth: 'Admin',
          desc: 'Create an API key.'
        },
        {
          endpoint: '/v1/settings/apikey/delete',
          method: 'DELETE',
          auth: 'Admin',
          desc: 'Delete an API key.'
        }
      ]
    }
  ]
})

const grpcRows = computed<ApiRow[]>(() => {
  if (isZh.value) {
    return [
      {
        endpoint: 'Register',
        method: 'Unary',
        auth: '原生客户端',
        desc: '注册实例，对应 `pkg/eden.Client.Register`。'
      },
      {
        endpoint: 'SetInstanceStatus',
        method: 'Unary',
        auth: '原生客户端',
        desc: '设置实例上下线状态，是当前替代 Deregister 的主路径。'
      },
      {
        endpoint: 'Heartbeat',
        method: 'Unary',
        auth: '原生客户端',
        desc: '续约实例，对应 `pkg/eden.Client.Heartbeat`。'
      },
      {
        endpoint: 'Discover',
        method: 'Unary',
        auth: '原生客户端',
        desc: '发现健康实例，支持命名空间与数据中心参数。'
      },
      {
        endpoint: 'Watch',
        method: 'Server Stream',
        auth: '原生客户端',
        desc: '服务变更订阅流；SDK 自动重连，并在失败时回退到轮询。'
      }
    ]
  }

  return [
    {
      endpoint: 'Register',
      method: 'Unary',
      auth: 'Native client',
      desc: 'Register an instance. Used by `pkg/eden.Client.Register`.'
    },
    {
      endpoint: 'SetInstanceStatus',
      method: 'Unary',
      auth: 'Native client',
      desc: 'Set an instance online or offline. This is the modern replacement for Deregister.'
    },
    {
      endpoint: 'Heartbeat',
      method: 'Unary',
      auth: 'Native client',
      desc: 'Renew an instance. Used by `pkg/eden.Client.Heartbeat`.'
    },
    {
      endpoint: 'Discover',
      method: 'Unary',
      auth: 'Native client',
      desc: 'Discover healthy instances with namespace and datacenter support.'
    },
    {
      endpoint: 'Watch',
      method: 'Server Stream',
      auth: 'Native client',
      desc: 'Streaming subscription for service changes. The SDK reconnects automatically and falls back to polling when needed.'
    }
  ]
})
</script>

<template>
  <div class="docs-container glass-card">
    <el-tabs v-model="activeSection" class="docs-tabs">
      <el-tab-pane
        v-for="s in sections"
        :key="s.id"
        :name="s.id"
        :label="(t.docs as any)[s.title]"
      >
        <div class="docs-pane-content">
          <div v-if="s.id === 'intro'">
            <div class="hero-block">
              <span class="hero-badge">{{ text('基于当前源码整理', 'Built from current source code') }}</span>
              <h2>{{ t.docs.intro }}</h2>
              <p class="lead">{{ introLead }}</p>
            </div>

            <div class="feature-grid">
              <div v-for="card in featureCards" :key="card.title" class="feature-item">
                <div class="feature-title">{{ card.title }}</div>
                <div class="feature-desc">{{ card.desc }}</div>
                <div class="tag-list">
                  <span v-for="tag in card.tags" :key="tag" class="tag-chip">{{ tag }}</span>
                </div>
              </div>
            </div>

            <div class="doc-section">
              <div class="section-title-row">
                <h3>{{ text('关键代码入口', 'Key Code Entry Points') }}</h3>
                <span class="section-note">{{ text('以下内容都已在当前仓库实现', 'All of these are already implemented in this repo') }}</span>
              </div>
              <div class="source-grid">
                <div v-for="item in sourceCards" :key="item.path" class="source-card">
                  <div class="source-title">{{ item.title }}</div>
                  <code class="inline-path">{{ item.path }}</code>
                  <p class="source-desc">{{ item.desc }}</p>
                </div>
              </div>
            </div>
          </div>

          <div v-else-if="s.id === 'quickStart'">
            <h2>{{ t.docs.quickStart }}</h2>

            <div v-for="block in quickBlocks" :key="block.title" class="doc-section">
              <div class="section-title-row">
                <h3>{{ block.title }}</h3>
              </div>
              <p class="section-desc">{{ block.desc }}</p>
              <pre v-if="block.code" class="code-block"><code>{{ block.code }}</code></pre>
              <p v-if="block.hint" class="code-hint">{{ block.hint }}</p>
            </div>

            <div class="doc-section">
              <div class="section-title-row">
                <h3>{{ text('默认访问信息', 'Default Access Information') }}</h3>
              </div>
              <div class="info-grid">
                <div v-for="item in accessItems" :key="item.label" class="info-card">
                  <div class="info-label">{{ item.label }}</div>
                  <div class="info-value"><code>{{ item.value }}</code></div>
                </div>
              </div>
            </div>
          </div>

          <div v-else-if="s.id === 'integration'">
            <h2>{{ t.docs.integration }}</h2>
            <p class="lead">
              {{ text('这一页只写当前仓库已经提供的真实接入方式，优先推荐 `pkg/eden`，其次是注册中心抽象层与直接 HTTP 集成。', 'This page only documents integration paths that already exist in the current repository: `pkg/eden` first, then the registry abstraction layer, and finally direct HTTP integration.') }}
            </p>

            <div class="integration-grid">
              <div v-for="item in integrationCards" :key="item.title" class="integration-card">
                <div class="card-header">
                  <div class="card-title">{{ item.title }}</div>
                  <span class="card-badge">{{ item.badge }}</span>
                </div>
                <p class="section-desc">{{ item.desc }}</p>
                <pre class="code-block"><code>{{ item.code }}</code></pre>
                <ul class="doc-list">
                  <li v-for="note in item.notes" :key="note">{{ note }}</li>
                </ul>
              </div>
            </div>
          </div>

          <div v-else-if="s.id === 'api'">
            <h2>{{ t.docs.apiDesc }}</h2>

            <div class="doc-section">
              <div class="section-title-row">
                <h3>{{ text('登录请求示例', 'Login Request Example') }}</h3>
                <span class="section-note">{{ text('默认密码 admin 的 SHA-256 已写入示例', 'The SHA-256 for the default password `admin` is already included') }}</span>
              </div>
              <pre class="code-block"><code>{{ loginExample }}</code></pre>
              <p class="code-hint">
                {{ text('如果你从控制台登录页面进入，前端已经自动做了 SHA-256 处理；只有自己手写调用 `/v1/auth/login` 时才需要关注这一点。', 'If you log in from the console UI, the frontend already applies SHA-256. You only need to care about this when calling `/v1/auth/login` manually.') }}
              </p>
            </div>

            <div v-for="group in apiGroups" :key="group.title" class="doc-section">
              <div class="section-title-row">
                <h3>{{ group.title }}</h3>
                <span class="section-note">{{ group.note }}</span>
              </div>
              <div class="table-wrap">
                <table class="docs-table">
                  <thead>
                    <tr>
                      <th>{{ text('路径', 'Endpoint') }}</th>
                      <th>{{ text('方法', 'Method') }}</th>
                      <th>{{ text('鉴权', 'Auth') }}</th>
                      <th>{{ text('说明', 'Description') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="row in group.rows" :key="`${group.title}-${row.endpoint}`">
                      <td><code>{{ row.endpoint }}</code></td>
                      <td>{{ row.method }}</td>
                      <td>{{ row.auth }}</td>
                      <td>{{ row.desc }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <div class="doc-section">
              <div class="section-title-row">
                <h3>{{ text('原生 gRPC 接口', 'Native gRPC APIs') }}</h3>
                <span class="section-note"><code>api/proto/registry/v1/registry.proto</code></span>
              </div>
              <div class="table-wrap">
                <table class="docs-table">
                  <thead>
                    <tr>
                      <th>{{ text('RPC', 'RPC') }}</th>
                      <th>{{ text('类型', 'Type') }}</th>
                      <th>{{ text('使用方', 'Caller') }}</th>
                      <th>{{ text('说明', 'Description') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="row in grpcRows" :key="row.endpoint">
                      <td><code>{{ row.endpoint }}</code></td>
                      <td>{{ row.method }}</td>
                      <td>{{ row.auth }}</td>
                      <td>{{ row.desc }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<style scoped>
.docs-container {
  height: 100%;
  padding: 24px;
  display: flex;
  flex-direction: column;
}

.docs-tabs {
  flex: 1;
  display: flex;
  flex-direction: column;
}

:deep(.el-tabs__content) {
  flex: 1;
  overflow-y: auto;
}

.docs-pane-content {
  padding: 20px 0 40px;
  max-width: 1120px;
}

.docs-pane-content h2 {
  margin: 0 0 16px;
  font-size: 32px;
  line-height: 1.2;
  color: var(--text-primary);
}

.hero-block {
  margin-bottom: 28px;
}

.hero-badge {
  display: inline-flex;
  align-items: center;
  padding: 6px 12px;
  margin-bottom: 14px;
  border-radius: 999px;
  background: rgba(59, 130, 246, 0.12);
  color: var(--accent-blue);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.04em;
}

.lead {
  margin: 0;
  font-size: 15px;
  line-height: 1.85;
  color: var(--text-secondary);
  max-width: 980px;
}

.feature-grid,
.source-grid,
.info-grid,
.integration-grid {
  display: grid;
  gap: 18px;
}

.feature-grid {
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  margin-bottom: 28px;
}

.source-grid,
.info-grid {
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
}

.integration-grid {
  grid-template-columns: 1fr;
}

.feature-item,
.source-card,
.info-card,
.integration-card {
  border: 1px solid var(--border-color);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.04);
  backdrop-filter: blur(8px);
}

.feature-item,
.source-card,
.integration-card {
  padding: 20px;
}

.info-card {
  padding: 18px;
}

.feature-title,
.source-title,
.card-title {
  font-size: 17px;
  font-weight: 700;
  color: var(--text-primary);
}

.feature-desc,
.source-desc,
.section-desc {
  margin: 10px 0 0;
  font-size: 14px;
  line-height: 1.8;
  color: var(--text-secondary);
}

.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 16px;
}

.tag-chip,
.card-badge {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 700;
}

.tag-chip {
  background: rgba(148, 163, 184, 0.12);
  color: var(--text-secondary);
}

.card-badge {
  background: rgba(59, 130, 246, 0.12);
  color: var(--accent-blue);
}

.inline-path,
.info-value code {
  display: inline-block;
  margin-top: 10px;
}

.doc-section {
  margin-top: 28px;
}

.section-title-row,
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.section-title-row h3 {
  margin: 0;
  font-size: 22px;
  color: var(--text-primary);
}

.section-note {
  font-size: 13px;
  color: var(--text-secondary);
}

.info-label {
  font-size: 13px;
  color: var(--text-secondary);
  margin-bottom: 8px;
}

.info-value {
  font-size: 15px;
  color: var(--text-primary);
}

.code-block {
  margin: 16px 0 0;
  padding: 18px 20px;
  border-radius: 16px;
  background: #0f172a;
  color: #e2e8f0;
  border: 1px solid rgba(255, 255, 255, 0.08);
  font-family: 'Fira Code', 'SFMono-Regular', Consolas, monospace;
  font-size: 13px;
  line-height: 1.75;
  white-space: pre-wrap;
  overflow-x: auto;
}

.code-hint {
  margin: 12px 0 0;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.75;
}

.doc-list {
  margin: 14px 0 0;
  padding-left: 18px;
  color: var(--text-secondary);
}

.doc-list li {
  margin-top: 8px;
  line-height: 1.75;
}

.table-wrap {
  margin-top: 16px;
  overflow-x: auto;
  border: 1px solid var(--border-color);
  border-radius: 16px;
}

.docs-table {
  width: 100%;
  border-collapse: collapse;
  background: rgba(255, 255, 255, 0.03);
}

.docs-table th,
.docs-table td {
  padding: 14px 16px;
  text-align: left;
  vertical-align: top;
  border-bottom: 1px solid var(--border-color);
  font-size: 14px;
  line-height: 1.7;
}

.docs-table th {
  font-weight: 700;
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.04);
}

.docs-table tr:last-child td {
  border-bottom: none;
}

code {
  color: var(--accent-blue);
  background: rgba(59, 130, 246, 0.08);
  padding: 2px 6px;
  border-radius: 6px;
  font-family: 'Fira Code', 'SFMono-Regular', Consolas, monospace;
}

@media (max-width: 768px) {
  .docs-container {
    padding: 18px;
  }

  .docs-pane-content h2 {
    font-size: 28px;
  }

  .section-title-row h3 {
    font-size: 20px;
  }

  .feature-grid,
  .source-grid,
  .info-grid {
    grid-template-columns: 1fr;
  }

  .code-block {
    padding: 16px;
    font-size: 12px;
  }
}
</style>

<script setup lang="ts">
import { ref } from 'vue'

type CodeLang = 'bash' | 'yaml' | 'go' | 'json' | 'proto'

type CodeSample = {
  lang: CodeLang
  code: string
}

const activeSection = ref('intro')

const serverStart: CodeSample = {
  lang: 'bash',
  code: `go run ./cmd/server/main.go -config configs/config.yaml`
}

const consoleStart: CodeSample = {
  lang: 'bash',
  code: `cd web
npm install
npm run dev`
}

const loginSample: CodeSample = {
  lang: 'bash',
  code: `curl -X POST http://127.0.0.1:8500/v1/auth/login \\
  -H "Content-Type: application/json" \\
  -d '{
    "username": "admin",
    "password": "8c6976e5b5410415bde908bd4dee15dfb167a9c873fc4bb8a81f6f2ab448a918"
  }'`
}

const smokeTest: CodeSample = {
  lang: 'bash',
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
    "metadata": {
      "version": "1.0.0"
    }
  }'

curl "http://127.0.0.1:8500/v1/catalog/service/user-center?passing=true&namespace=default&dc=dc1"`
}

const apConfig: CodeSample = {
  lang: 'yaml',
  code: `node_id: "node-1"
mode: "ap"
datacenter: "dc1"
data_dir: "./data"

http_addr: ":8500"
grpc_addr: ":0"
quic_addr: ""

seeds:
  - "http://127.0.0.1:8501"
  - "http://127.0.0.1:8502"

auth:
  jwt:
    enabled: true
    secret: "eden-jwt-console-secret-key"
  api_key:
    enabled: false
    keys: []`
}

const cpBootstrapConfig: CodeSample = {
  lang: 'yaml',
  code: `node_id: "node-1"
mode: "cp"
datacenter: "dc1"
data_dir: "./data"

http_addr: ":8500"
grpc_addr: ":9000"
quic_addr: ":10000"
raft_addr: "127.0.0.1:7000"

bootstrap: true
join: ""`
}

const cpJoinConfig: CodeSample = {
  lang: 'yaml',
  code: `node_id: "node-2"
mode: "cp"
datacenter: "dc1"
data_dir: "./data/node-2"

http_addr: ":8501"
grpc_addr: ":9001"
quic_addr: ":10001"
raft_addr: "127.0.0.1:7001"

bootstrap: false
join: "http://127.0.0.1:8500"`
}

const edenSdkSample: CodeSample = {
  lang: 'go',
  code: `package main

import (
  "context"
  "log"
  "os"
  "os/signal"
  "syscall"
  "time"

  "github.com/shiyindaxiaojie/eden-go-registry/pkg/eden"
  "github.com/shiyindaxiaojie/eden-go-registry/pkg/registry"
)

func main() {
  // Create the client from the registry entry configuration.
  client, err := eden.NewWithConfig(&eden.Config{
    Addresses:     []string{"http://127.0.0.1:8500"},
    Namespace:     "default",
    Datacenter:    "dc1",
    APIKey:        "replace-with-api-key",
    CacheDir:      "./.eden-cache",
    Transport:     "grpc",
    DiscoveryMode: "auto",
  })
  if err != nil {
    log.Fatal(err)
  }
  defer client.Close()

  // Describe the local instance that will be published.
  instance := &registry.ServiceInstance{
    ID:          "order-center-1",
    ServiceName: "order-center",
    Host:        "127.0.0.1",
    Port:        9003,
    Weight:      100,
    Datacenter:  "dc1",
    Metadata: map[string]string{
      "version": "1.0.0",
      "zone":    "cn-shanghai-a",
    },
  }

  // Register the instance before it starts serving traffic.
  if err := client.Register(instance); err != nil {
    log.Fatal(err)
  }

  heartbeatCtx, cancel := context.WithCancel(context.Background())
  defer cancel()

  // Keep the lease active with periodic heartbeats.
  go func() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    for {
      select {
      case <-heartbeatCtx.Done():
        return
      case <-ticker.C:
        if err := client.Heartbeat(instance); err != nil {
          log.Printf("heartbeat failed: %v", err)
        }
      }
    }
  }()

  // Subscribe to dependency changes for realtime updates.
  if err := client.Subscribe("user-center", func(items []*registry.ServiceInstance) {
    log.Printf("user-center changed: %d instances", len(items))
  }); err != nil {
    log.Printf("subscribe failed: %v", err)
  }

  // Discover providers before issuing business requests.
  providers, err := client.Discovery("user-center")
  if err != nil {
    log.Fatal(err)
  }
  log.Printf("ready to call user-center: %+v", providers)

  stop := make(chan os.Signal, 1)
  signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
  <-stop

  // Deregister the instance when the process exits.
  if err := client.Deregister(instance); err != nil {
    log.Printf("deregister failed: %v", err)
  }
}`
}

const nacosAdapterSample: CodeSample = {
  lang: 'go',
  code: `package main

import (
  "log"
  "time"

  nacosClients "github.com/shiyindaxiaojie/eden-go-registry/pkg/nacos/clients"
  nacosConst "github.com/shiyindaxiaojie/eden-go-registry/pkg/nacos/common/constant"
  nacosModel "github.com/shiyindaxiaojie/eden-go-registry/pkg/nacos/model"
  nacosVo "github.com/shiyindaxiaojie/eden-go-registry/pkg/nacos/vo"
)

func main() {
  // Replace the original Nacos SDK import path only.
  client, err := nacosClients.NewNamingClient(nacosVo.NacosClientParam{
    ClientConfig: &nacosConst.ClientConfig{
      NamespaceId: "public",
      TimeoutMs:   5000,
    },
    ServerConfigs: []nacosConst.ServerConfig{
      {
        IpAddr: "127.0.0.1",
        Port:   8500,
        Scheme: "http",
      },
    },
  })
  if err != nil {
    log.Fatal(err)
  }

  // Keep the original Nacos registration call style unchanged.
  ok, err := client.RegisterInstance(nacosVo.RegisterInstanceParam{
    Ip:          "127.0.0.1",
    Port:        9001,
    Weight:      100,
    Healthy:     true,
    Enable:      true,
    Ephemeral:   true,
    ServiceName: "user-center",
    Metadata: map[string]string{
      "version": "1.0.0",
    },
  })
  if err != nil || !ok {
    log.Fatal(err)
  }

  // Subscribe with the same callback signature used by the Nacos SDK.
  sub := &nacosVo.SubscribeParam{
    ServiceName: "auth-center",
    SubscribeCallback: func(items []nacosModel.Instance, err error) {
      if err != nil {
        log.Printf("subscribe failed: %v", err)
        return
      }
      log.Printf("auth-center changed: %d instances", len(items))
    },
  }
  if err := client.Subscribe(sub); err != nil {
    log.Fatal(err)
  }

  // Query providers through the original Nacos discovery method.
  providers, err := client.SelectInstances(nacosVo.SelectInstancesParam{
    ServiceName: "order-center",
    HealthyOnly: true,
  })
  if err != nil {
    log.Fatal(err)
  }
  log.Printf("order-center instances: %d", len(providers))

  time.Sleep(30 * time.Second)

  // Unsubscribe and deregister before shutdown.
  _ = client.Unsubscribe(sub)
  _, err = client.DeregisterInstance(nacosVo.DeregisterInstanceParam{
    Ip:          "127.0.0.1",
    Port:        9001,
    ServiceName: "user-center",
    Ephemeral:   true,
  })
  if err != nil {
    log.Fatal(err)
  }
}`
}

const consulAdapterSample: CodeSample = {
  lang: 'go',
  code: `package main

import (
  "log"

  consulapi "github.com/shiyindaxiaojie/eden-go-registry/pkg/consul/api"
)

func main() {
  // Replace the original Consul API import path only.
  cfg := consulapi.DefaultConfig()
  cfg.Address = "127.0.0.1:8500"
  cfg.Datacenter = "dc1"
  cfg.Token = "replace-with-api-key"

  client, err := consulapi.NewClient(cfg)
  if err != nil {
    log.Fatal(err)
  }

  // Register through the original Consul Agent API.
  if err := client.Agent().ServiceRegister(&consulapi.AgentServiceRegistration{
    ID:      "user-center-1",
    Name:    "user-center",
    Address: "127.0.0.1",
    Port:    9001,
    Meta: map[string]string{
      "version": "1.0.0",
    },
    Weights: &consulapi.AgentWeights{
      Passing: 100,
    },
  }); err != nil {
    log.Fatal(err)
  }

  // Keep the registration alive through the same TTL update method.
  if err := client.Agent().PassTTL("service:user-center-1", "heartbeat ok"); err != nil {
    log.Fatal(err)
  }

  // Query healthy providers through the original Health API surface.
  entries, _, err := client.Health().Service("auth-center", "", true, nil)
  if err != nil {
    log.Fatal(err)
  }
  log.Printf("auth-center instances: %d", len(entries))

  // Query catalog data through the original Catalog API surface.
  services, _, err := client.Catalog().Service("order-center", "", &consulapi.QueryOptions{
    Datacenter: "dc1",
  })
  if err != nil {
    log.Fatal(err)
  }
  log.Printf("order-center instances: %d", len(services))

  // Deregister before process exit.
  if err := client.Agent().ServiceDeregister("user-center-1"); err != nil {
    log.Fatal(err)
  }
}`
}

const httpClientSample: CodeSample = {
  lang: 'go',
  code: `package main

import (
  "bytes"
  "encoding/json"
  "fmt"
  "io"
  "log"
  "net/http"
  "time"
)

type serviceInstance struct {
  ID          string            \`json:"id,omitempty"\`
  ServiceName string            \`json:"service_name,omitempty"\`
  Namespace   string            \`json:"namespace,omitempty"\`
  Host        string            \`json:"host,omitempty"\`
  Port        int               \`json:"port,omitempty"\`
  Weight      int               \`json:"weight,omitempty"\`
  Datacenter  string            \`json:"dc,omitempty"\`
  Metadata    map[string]string \`json:"metadata,omitempty"\`
}

func main() {
  client := &http.Client{Timeout: 5 * time.Second}
  baseURL := "http://127.0.0.1:8500"
  apiKey := "replace-with-api-key"

  instance := serviceInstance{
    ID:          "inventory-center-1",
    ServiceName: "inventory-center",
    Namespace:   "default",
    Host:        "127.0.0.1",
    Port:        9100,
    Weight:      100,
    Datacenter:  "dc1",
    Metadata: map[string]string{
      "version": "1.0.0",
    },
  }

  // Register the local instance through the public HTTP API.
  if err := doJSON(client, "POST", baseURL+"/v1/catalog/register", apiKey, instance, nil); err != nil {
    log.Fatal(err)
  }

  // Send a heartbeat to renew the registration lease.
  if err := doJSON(client, "POST", baseURL+"/v1/catalog/heartbeat", apiKey, map[string]string{
    "namespace":    "default",
    "service_name": "inventory-center",
    "instance_id":  "inventory-center-1",
  }, nil); err != nil {
    log.Fatal(err)
  }

  // Discover healthy providers through the HTTP query interface.
  var providers []serviceInstance
  if err := doJSON(client, "GET", baseURL+"/v1/catalog/service/inventory-center?passing=true&namespace=default&dc=dc1", apiKey, nil, &providers); err != nil {
    log.Fatal(err)
  }
  log.Printf("inventory-center instances: %d", len(providers))

  time.Sleep(30 * time.Second)

  // Mark the instance offline before shutdown.
  if err := doJSON(client, "POST", baseURL+"/v1/catalog/instance/status", apiKey, map[string]string{
    "namespace":    "default",
    "service_name": "inventory-center",
    "instance_id":  "inventory-center-1",
    "status":       "offline",
  }, nil); err != nil {
    log.Fatal(err)
  }
}

func doJSON(client *http.Client, method, url, apiKey string, body any, out any) error {
  var reader io.Reader
  if body != nil {
    payload, err := json.Marshal(body)
    if err != nil {
      return err
    }
    reader = bytes.NewReader(payload)
  }

  req, err := http.NewRequest(method, url, reader)
  if err != nil {
    return err
  }
  req.Header.Set("Content-Type", "application/json")
  if apiKey != "" {
    req.Header.Set("X-API-Key", apiKey)
  }

  resp, err := client.Do(req)
  if err != nil {
    return err
  }
  defer resp.Body.Close()

  if resp.StatusCode < 200 || resp.StatusCode >= 300 {
    data, _ := io.ReadAll(resp.Body)
    return fmt.Errorf("status %d: %s", resp.StatusCode, string(data))
  }

  if out != nil {
    return json.NewDecoder(resp.Body).Decode(out)
  }
  return nil
}`
}

const grpcClientSample: CodeSample = {
  lang: 'go',
  code: `package main

import (
  "context"
  "io"
  "log"
  "time"

  registryv1 "github.com/shiyindaxiaojie/eden-go-registry/api/proto/registry/v1"
  "google.golang.org/grpc"
  "google.golang.org/grpc/credentials/insecure"
  "google.golang.org/grpc/metadata"
)

func main() {
  // Connect to the public gRPC endpoint.
  conn, err := grpc.Dial("127.0.0.1:9000", grpc.WithTransportCredentials(insecure.NewCredentials()))
  if err != nil {
    log.Fatal(err)
  }
  defer conn.Close()

  client := registryv1.NewRegistryServiceClient(conn)

  // Register a new service instance.
  if _, err := client.Register(context.Background(), &registryv1.RegisterRequest{
    Instance: &registryv1.ServiceInstance{
      Id:          "payment-center-1",
      ServiceName: "payment-center",
      Namespace:   "default",
      Host:        "127.0.0.1",
      Port:        9200,
      Weight:      100,
      Datacenter:  "dc1",
      Metadata: map[string]string{
        "version": "1.0.0",
      },
    },
  }); err != nil {
    log.Fatal(err)
  }

  // Send a heartbeat to keep the instance alive.
  if _, err := client.Heartbeat(context.Background(), &registryv1.HeartbeatRequest{
    Namespace:   "default",
    ServiceName: "payment-center",
    InstanceId:  "payment-center-1",
  }); err != nil {
    log.Fatal(err)
  }

  // Query available providers before routing traffic.
  discoverResp, err := client.Discover(context.Background(), &registryv1.DiscoverRequest{
    Namespace:   "default",
    ServiceName: "user-center",
    HealthyOnly: true,
    Datacenter:  "dc1",
  })
  if err != nil {
    log.Fatal(err)
  }
  log.Printf("user-center instances: %d", len(discoverResp.Instances))

  // Subscribe to provider changes through the Watch stream.
  watchCtx := metadata.AppendToOutgoingContext(context.Background(), "x-consumer-service", "payment-center")
  stream, err := client.Watch(watchCtx, &registryv1.WatchRequest{
    Namespace:   "default",
    ServiceName: "user-center",
  })
  if err != nil {
    log.Fatal(err)
  }

  go func() {
    for {
      resp, err := stream.Recv()
      if err == io.EOF {
        return
      }
      if err != nil {
        log.Printf("watch closed: %v", err)
        return
      }
      log.Printf("watch event: %s, instances=%d", resp.Action, len(resp.Instances))
    }
  }()

  time.Sleep(30 * time.Second)

  // Mark the instance offline before shutdown.
  if _, err := client.SetInstanceStatus(context.Background(), &registryv1.SetInstanceStatusRequest{
    Namespace:   "default",
    ServiceName: "payment-center",
    InstanceId:  "payment-center-1",
    Status:      "offline",
  }); err != nil {
    log.Fatal(err)
  }
}`
}

const apiKeyConfigSample: CodeSample = {
  lang: 'yaml',
  code: `auth:
  api_key:
    enabled: true
    keys:
      - "prod-service-key"
      - "test-service-key"`
}

const apiKeyRuntimeSample: CodeSample = {
  lang: 'bash',
  code: `curl -X POST http://127.0.0.1:8500/v1/settings/system \\
  -H "Authorization: Bearer <JWT_TOKEN>" \\
  -H "Content-Type: application/json" \\
  -d '{
    "api_key_auth_enabled": true
  }'`
}

function escapeHtml(value: string) {
  return value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;')
}

function placeholderToken(index: number) {
  const alphabet = 'abcdefghijklmnopqrstuvwxyz'
  let current = index
  let result = ''

  do {
    result = alphabet[current % alphabet.length] + result
    current = Math.floor(current / alphabet.length) - 1
  } while (current >= 0)

  return `@@TOKEN_${result}@@`
}

function highlightLine(line: string, lang: CodeLang) {
  let html = escapeHtml(line)
  const placeholders = new Map<string, string>()

  const stash = (value: string) => {
    const token = placeholderToken(placeholders.size)
    placeholders.set(token, value)
    return token
  }

  const wrap = (regex: RegExp, className: string) => {
    html = html.replace(regex, (match) => stash(`<span class="token ${className}">${match}</span>`))
  }

  if (lang === 'go') {
    wrap(/\/\/.*$/g, 'comment')
    wrap(/"(?:\\.|[^"])*"/g, 'string')
    wrap(/`[^`]*`/g, 'string')
    wrap(/\b(?:package|import|func|type|struct|interface|map|chan|var|const|return|defer|go|select|switch|case|default|if|else|for|range|nil|true|false)\b/g, 'keyword')
    wrap(/\b[A-Za-z_][A-Za-z0-9_]*(?=\()/g, 'func')
    wrap(/\b\d+(?:\.\d+)?\b/g, 'number')
  } else if (lang === 'bash') {
    wrap(/#.*$/g, 'comment')
    wrap(/"(?:\\.|[^"])*"/g, 'string')
    wrap(/'(?:\\.|[^'])*'/g, 'string')
    wrap(/\$[A-Za-z_][A-Za-z0-9_]*/g, 'variable')
    wrap(/--?[A-Za-z0-9-]+/g, 'flag')
    wrap(/\b(?:curl|cd|npm|go|grpcurl|export|set|echo)\b/g, 'builtin')
    wrap(/\b\d+(?:\.\d+)?\b/g, 'number')
  } else if (lang === 'yaml') {
    wrap(/#.*$/g, 'comment')
    wrap(/"(?:\\.|[^"])*"/g, 'string')
    wrap(/'(?:\\.|[^'])*'/g, 'string')
    wrap(/^[\s-]*[A-Za-z_][A-Za-z0-9_.-]*(?=\s*:)/g, 'property')
    wrap(/\b(?:true|false|null)\b/g, 'keyword')
    wrap(/\b\d+(?:\.\d+)?\b/g, 'number')
  } else if (lang === 'json') {
    wrap(/"(?:\\.|[^"])*"(?=\s*:)/g, 'property')
    wrap(/"(?:\\.|[^"])*"/g, 'string')
    wrap(/\b(?:true|false|null)\b/g, 'keyword')
    wrap(/\b\d+(?:\.\d+)?\b/g, 'number')
  } else if (lang === 'proto') {
    wrap(/\/\/.*$/g, 'comment')
    wrap(/"(?:\\.|[^"])*"/g, 'string')
    wrap(/\b(?:syntax|package|option|service|rpc|returns|message|repeated|stream|bool|string|int32|int64|map)\b/g, 'keyword')
    wrap(/\b[A-Za-z_][A-Za-z0-9_]*(?=\()/g, 'func')
    wrap(/\b\d+\b/g, 'number')
  }

  placeholders.forEach((value, token) => {
    html = html.replaceAll(token, value)
  })

  return html || '&nbsp;'
}

function renderCode(sample: CodeSample) {
  return sample.code
    .split('\n')
    .map((line, index) => `<div class="code-row"><span class="code-line-no">${index + 1}</span><span class="code-line-content">${highlightLine(line, sample.lang)}</span></div>`)
    .join('')
}
</script>

<template>
  <div class="docs-container glass-card">
    <el-tabs v-model="activeSection" class="docs-tabs">
      <el-tab-pane name="intro" label="系统介绍">
        <div class="docs-pane-content">
          <div class="surface-card">
            <p class="lead">
              Eden Registry 是面向微服务场景的服务注册与发现系统，提供服务实例注册、心跳续约、实例发现、变更订阅、命名空间隔离和访问鉴权能力。系统对外提供 HTTP API、gRPC 与 gRPC over QUIC 接入方式，可用于服务治理、环境隔离和应用侧统一接入。
            </p>
          </div>

          <div class="doc-section">
            <div class="section-title-row"><h3>核心功能</h3></div>
            <div class="feature-grid">
              <div class="surface-card">
                <div class="feature-title">服务注册与健康维护</div>
                <div class="feature-desc">统一维护服务实例、心跳续约、上下线状态、权重和元数据，形成稳定、可查询的服务目录。</div>
              </div>
              <div class="surface-card">
                <div class="feature-title">多协议发现与订阅</div>
                <div class="feature-desc">同时提供 HTTP、gRPC 和 gRPC over QUIC 接入能力，支持查询式发现与流式订阅。</div>
              </div>
              <div class="surface-card">
                <div class="feature-title">命名空间与关系视图</div>
                <div class="feature-desc">支持按命名空间隔离环境或租户，并对外提供订阅方、依赖图和拓扑视图等治理信息。</div>
              </div>
              <div class="surface-card">
                <div class="feature-title">控制台与访问控制</div>
                <div class="feature-desc">提供控制台登录、API Key、JWT 及命名空间管理能力，满足接入控制与日常运维要求。</div>
              </div>
            </div>
          </div>

          <div class="doc-section">
            <div class="section-title-row"><h3>与 Consul、Nacos 对比</h3></div>
            <div class="table-wrap">
              <table class="docs-table">
                <thead>
                  <tr>
                    <th>比较项</th>
                    <th>Eden Registry</th>
                    <th>Consul</th>
                    <th>Nacos</th>
                    <th>适用优势</th>
                  </tr>
                </thead>
                <tbody>
                  <tr>
                    <td>应用接入方式</td>
                    <td>提供 HTTP、gRPC 与 gRPC over QUIC 三类接入方式。</td>
                    <td>通常以 HTTP API、DNS 和生态 SDK 为主要入口。</td>
                    <td>通常以 OpenAPI 和官方 SDK 为主要入口。</td>
                    <td>在一个系统内同时覆盖轻量 REST 接入与强类型长连接接入，便于统一接入标准。</td>
                  </tr>
                  <tr>
                    <td>Go 服务接入闭环</td>
                    <td>提供从注册、续约到发现、订阅、下线的完整 Go 接入闭环。</td>
                    <td>通常围绕 HTTP API 或第三方 SDK 组织注册与发现链路。</td>
                    <td>通常围绕官方 SDK 与 OpenAPI 组合完成业务接入。</td>
                    <td>更适合希望统一客户端行为、减少重复封装和降低迁移成本的 Go 团队。</td>
                  </tr>
                  <tr>
                    <td>治理视角</td>
                    <td>聚焦服务、实例、命名空间、订阅方和依赖拓扑的一体化展示。</td>
                    <td>治理能力完整，但调用关系展示通常依赖额外生态组件。</td>
                    <td>控制台治理能力成熟，侧重命名与配置体系的统一管理。</td>
                    <td>对于仅关注注册发现和调用关系治理的团队，信息面更集中，阅读成本更低。</td>
                  </tr>
                  <tr>
                    <td>部署定位</td>
                    <td>聚焦服务注册发现、命名空间隔离和接入治理，强调轻量闭环。</td>
                    <td>覆盖服务发现、网络治理等更广泛的基础设施能力。</td>
                    <td>同时覆盖服务注册发现与配置管理场景。</td>
                    <td>在仅需服务注册发现能力的项目中，更容易形成清晰的职责边界。</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane name="quickStart" label="快速上手">
        <div class="docs-pane-content">
          <p class="lead">本节说明首次部署与验证流程，包括服务端启动、控制台启动、首次登录、注册与发现验证，以及常用配置项说明。</p>

          <div class="doc-section surface-card">
            <div class="section-title-row"><h3>1. 启动服务端</h3></div>
            <p class="section-desc">推荐优先通过配置文件启动。当前服务端支持的命令行覆盖参数为 <code>-config</code>、<code>-data-dir</code>、<code>-node-id</code> 和 <code>-http-addr</code>，其余能力建议通过配置文件维护。</p>
            <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">bash</span></div><div class="code-block" v-html="renderCode(serverStart)"></div></div>
            <ul class="doc-list">
              <li>默认 HTTP API 监听地址为 <code>:8500</code>，控制台开发代理默认转发到该地址。</li>
              <li>若需隔离数据目录或节点标识，可通过 <code>-data-dir</code> 与 <code>-node-id</code> 覆盖配置文件中的同名项。</li>
            </ul>
          </div>

          <div class="doc-section surface-card">
            <div class="section-title-row"><h3>2. 启动控制台</h3></div>
            <p class="section-desc">控制台是独立的 Vite 前端应用。开发模式下默认监听 <code>2019</code> 端口，并将 <code>/v1/*</code> 请求代理到后端 HTTP API。</p>
            <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">bash</span></div><div class="code-block" v-html="renderCode(consoleStart)"></div></div>
            <ul class="doc-list">
              <li>控制台默认访问地址为 <code>http://127.0.0.1:2019</code>。</li>
            </ul>
          </div>

          <div class="doc-section surface-card">
            <div class="section-title-row"><h3>3. 首次登录</h3></div>
            <p class="section-desc">默认管理员账号为 <code>admin / admin</code>。登录接口要求 <code>password</code> 传入原始密码的 SHA-256 值；若从控制台页面登录，浏览器会自动完成该处理。</p>
            <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">bash</span></div><div class="code-block" v-html="renderCode(loginSample)"></div></div>
          </div>

          <div class="doc-section surface-card">
            <div class="section-title-row"><h3>4. 注册与发现验证</h3></div>
            <p class="section-desc">以下示例通过 HTTP API 完成一个最小闭环：注册实例后立即发起发现查询，用于确认服务目录已经对外提供能力。</p>
            <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">bash</span></div><div class="code-block" v-html="renderCode(smokeTest)"></div></div>
            <ul class="doc-list">
              <li>若已启用 API Key 鉴权，请在注册请求中额外携带 <code>X-API-Key</code> 请求头。</li>
            </ul>
          </div>

          <div class="doc-section">
            <div class="section-title-row"><h3>默认访问信息</h3></div>
            <div class="info-grid">
              <div class="surface-card info-card"><div class="info-label">后端 HTTP API</div><div class="info-value"><code>http://127.0.0.1:8500</code></div></div>
              <div class="surface-card info-card"><div class="info-label">控制台地址</div><div class="info-value"><code>http://127.0.0.1:2019</code></div></div>
              <div class="surface-card info-card"><div class="info-label">管理员账号</div><div class="info-value"><code>admin / admin</code></div></div>
              <div class="surface-card info-card"><div class="info-label">默认命名空间</div><div class="info-value"><code>default</code></div></div>
              <div class="surface-card info-card"><div class="info-label">默认数据中心</div><div class="info-value"><code>dc1</code></div></div>
            </div>
          </div>

          <div class="doc-section">
            <div class="section-title-row"><h3>配置说明</h3></div>
            <div class="config-stack">
              <div class="surface-card config-card">
                <div class="section-title-row"><h3>单节点 / AP 模式示例</h3></div>
                <p class="section-desc">适合本地联调、测试环境或以可用性优先的轻量部署场景。</p>
                <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">yaml</span></div><div class="code-block" v-html="renderCode(apConfig)"></div></div>
                <div class="table-wrap compact">
                  <table class="docs-table">
                    <thead><tr><th>配置项</th><th>默认值</th><th>说明</th></tr></thead>
                    <tbody>
                      <tr><td><code>node_id</code></td><td>node-1</td><td>节点唯一标识，用于区分不同实例。</td></tr>
                      <tr><td><code>mode</code></td><td>ap</td><td>一致性模式，<code>ap</code> 表示优先可用。</td></tr>
                      <tr><td><code>datacenter</code></td><td>dc1</td><td>数据中心标签，用于服务发现时的机房维度过滤。</td></tr>
                      <tr><td><code>data_dir</code></td><td>./data</td><td>注册数据、鉴权数据和运行时配置的持久化目录。</td></tr>
                      <tr><td><code>http_addr</code></td><td>:8500</td><td>HTTP API 与控制台代理访问入口。</td></tr>
                      <tr><td><code>grpc_addr</code></td><td>:0</td><td>gRPC 监听地址；为空或 <code>:0</code> 时表示自动分配端口。</td></tr>
                      <tr><td><code>quic_addr</code></td><td>""</td><td>gRPC over QUIC 监听地址；为空时表示不显式指定。</td></tr>
                      <tr><td><code>seeds</code></td><td>[]</td><td>AP 模式下可选的种子节点列表，使用其他节点的 HTTP 地址。</td></tr>
                    </tbody>
                  </table>
                </div>
              </div>

              <div class="surface-card config-card">
                <div class="section-title-row"><h3>CP 首节点示例</h3></div>
                <p class="section-desc">适合对一致性要求更高的部署。首节点需要显式开启 <code>bootstrap</code>，并为 <code>raft_addr</code> 指定可被其他节点访问的明确 IP 地址。</p>
                <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">yaml</span></div><div class="code-block" v-html="renderCode(cpBootstrapConfig)"></div></div>
              </div>

              <div class="surface-card config-card">
                <div class="section-title-row"><h3>CP 加入节点示例</h3></div>
                <p class="section-desc">后续节点通过已存在节点的 HTTP 地址加入集群，避免业务接入方直接依赖底层通信地址。</p>
                <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">yaml</span></div><div class="code-block" v-html="renderCode(cpJoinConfig)"></div></div>
              </div>

              <div class="surface-card config-card">
                <div class="section-title-row"><h3>安全与运行时配置</h3></div>
                <div class="table-wrap compact">
                  <table class="docs-table">
                    <thead><tr><th>配置项</th><th>默认值</th><th>说明</th></tr></thead>
                    <tbody>
                      <tr><td><code>auth.jwt.enabled</code></td><td>true</td><td>是否开启控制台 JWT 登录鉴权。</td></tr>
                      <tr><td><code>auth.jwt.secret</code></td><td>eden-jwt-console-secret-key</td><td>JWT 签名密钥，生产环境应替换为自定义安全值。</td></tr>
                      <tr><td><code>auth.api_key.enabled</code></td><td>false</td><td>是否对 HTTP 注册、心跳和拓扑上报接口开启 API Key 校验。</td></tr>
                      <tr><td><code>auth.api_key.keys</code></td><td>[]</td><td>预置可用的 API Key 列表，也可在控制台中维护。</td></tr>
                      <tr><td><code>storage.event_retention_days</code></td><td>30</td><td>事件保留天数。</td></tr>
                      <tr><td><code>storage.log_retention_days</code></td><td>30</td><td>日志保留天数。</td></tr>
                      <tr><td><code>registry.heartbeat_max_failures</code></td><td>3</td><td>实例连续心跳失败的最大次数阈值。</td></tr>
                      <tr><td><code>registry.instance_removal_delay_seconds</code></td><td>600</td><td>实例被判定失效后的延迟移除时间，单位为秒。</td></tr>
                      <tr><td><code>log.level</code></td><td>INFO</td><td>服务端日志级别。</td></tr>
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          </div>

          <div class="doc-section surface-card">
            <div class="section-title-row"><h3>部署要求</h3></div>
            <ul class="doc-list">
              <li>若采用 CP 模式，<code>raft_addr</code> 应使用明确可达的 IP 地址，不建议配置为 <code>:端口</code> 或 <code>0.0.0.0:端口</code>。</li>
              <li><code>grpc_addr</code> 与 <code>quic_addr</code> 允许使用空值或 <code>:0</code> 进行自动端口分配；业务接入方应优先使用外部统一入口地址，而不是假设固定内部端口。</li>
            </ul>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane name="integration" label="客户端接入">
        <div class="docs-pane-content">
          <p class="lead">客户端接入提供 Eden SDK、Nacos 接入适配、Consul 接入适配、HTTP API 和 gRPC 五类方式。以下示例均采用 Go 代码，覆盖实例注册、心跳续约、服务发现、变更订阅和下线处理的完整流程。</p>

          <div class="anchor-nav">
            <a class="anchor-link" href="#integration-eden">Eden SDK</a>
            <a class="anchor-link" href="#integration-nacos">Nacos 接入适配</a>
            <a class="anchor-link" href="#integration-consul">Consul 接入适配</a>
            <a class="anchor-link" href="#integration-http">HTTP API</a>
            <a class="anchor-link" href="#integration-grpc">gRPC</a>
          </div>

          <div class="config-stack">
            <div id="integration-eden" class="surface-card integration-card">
              <div class="section-title-row"><div class="feature-title">Eden SDK 接入</div><span class="card-badge">推荐</span></div>
              <p class="section-desc">适用于直接接入 Eden Registry 的 Go 服务。该方式可在同一套业务代码中切换 <code>grpc</code>、<code>quic</code> 或 <code>http</code> 传输，并统一处理注册、续约、发现、订阅和下线流程。</p>
              <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">go</span></div><div class="code-block" v-html="renderCode(edenSdkSample)"></div></div>
              <ul class="doc-list">
                <li>当 <code>DiscoveryMode=auto</code> 时，<code>Addresses</code> 可写为统一 HTTP 入口地址，客户端会在内部补充可用节点信息。</li>
                <li>若希望直连传输端口，可将 <code>DiscoveryMode</code> 改为 <code>static</code>，并在 <code>grpc</code> 或 <code>quic</code> 模式下直接填写对应地址。</li>
              </ul>
            </div>

            <div id="integration-nacos" class="surface-card integration-card">
              <div class="section-title-row"><div class="feature-title">Nacos 接入适配</div><span class="card-badge">平滑迁移</span></div>
              <p class="section-desc">适用于已使用 Nacos SDK 的 Go 服务。迁移到 Eden Registry 时，可保留原有 <code>RegisterInstance</code>、<code>SelectInstances</code>、<code>Subscribe</code> 等调用方式，仅调整接入包路径和服务端连接配置。</p>
              <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">go</span></div><div class="code-block" v-html="renderCode(nacosAdapterSample)"></div></div>
              <ul class="doc-list">
                <li>适配层兼容 <code>github.com/nacos-group/nacos-sdk-go/v2</code> 的命名发现调用面，重点在于迁移时不改业务调用逻辑。</li>
                <li>建议优先替换接入包与地址配置，再按服务名验证注册、订阅和发现结果。</li>
              </ul>
            </div>

            <div id="integration-consul" class="surface-card integration-card">
              <div class="section-title-row"><div class="feature-title">Consul 接入适配</div><span class="card-badge">平滑迁移</span></div>
              <p class="section-desc">适用于已使用 Consul API 的 Go 服务。迁移到 Eden Registry 时，可保持原有 Agent、Catalog 和 Health 调用方式，仅调整 import 路径和服务端连接配置。</p>
              <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">go</span></div><div class="code-block" v-html="renderCode(consulAdapterSample)"></div></div>
              <ul class="doc-list">
                <li>适配层兼容 <code>github.com/hashicorp/consul/api</code> 的常用访问方式，适合将现有 Consul 接入迁移到 Eden Registry。</li>
                <li>若已启用 API Key，可通过 <code>cfg.Token</code> 统一下发，不需要在业务侧重复封装请求头。</li>
              </ul>
            </div>

            <div id="integration-http" class="surface-card integration-card">
              <div class="section-title-row"><div class="feature-title">HTTP API 接入</div><span class="card-badge">原生 HTTP</span></div>
              <p class="section-desc">适用于需要直接对接公开 HTTP API 并自行封装客户端的 Go 服务。该方式不依赖额外 SDK，可在业务侧统一实现注册、心跳、发现和下线流程。</p>
              <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">go</span></div><div class="code-block" v-html="renderCode(httpClientSample)"></div></div>
              <ul class="doc-list">
                <li>HTTP 注册请求中的数据中心字段名为 <code>dc</code>，不是 <code>datacenter</code>。</li>
                <li>实例状态切换接口可接受有效 API Key，也可接受管理员或开发者 JWT。</li>
              </ul>
            </div>

            <div id="integration-grpc" class="surface-card integration-card">
              <div class="section-title-row"><div class="feature-title">gRPC 接入</div><span class="card-badge">强类型</span></div>
              <p class="section-desc">适用于希望直接基于 proto 协议封装客户端、显式控制流式订阅或统一接入强类型 RPC 的 Go 服务。gRPC over QUIC 与 gRPC 共享相同的 RPC 语义。</p>
              <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">go</span></div><div class="code-block" v-html="renderCode(grpcClientSample)"></div></div>
              <ul class="doc-list">
                <li><code>Watch</code> 为服务端流式接口，适用于需要实时感知实例变化的调用方。</li>
                <li>若切换到 gRPC over QUIC，应保持相同的 proto 与 RPC 语义，仅将连接端点替换为 <code>quic_addr</code> 并使用兼容的 QUIC 拨号方式。</li>
              </ul>
            </div>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane name="api" label="API 接口">
        <div class="docs-pane-content">
          <p class="lead">系统对外提供 HTTP API、gRPC 与 gRPC over QUIC 三类协议。以下仅列出面向外部接入方的公开接口、鉴权方式与启用要求。</p>

          <div class="doc-section">
            <div class="section-title-row"><h3>协议说明</h3></div>
            <div class="table-wrap compact">
              <table class="docs-table">
                <thead><tr><th>协议</th><th>适用场景</th><th>说明</th></tr></thead>
                <tbody>
                  <tr><td><code>HTTP API</code></td><td>控制台、脚本、跨语言集成</td><td>采用 JSON over HTTP，适合轻量接入与管理类调用。</td></tr>
                  <tr><td><code>gRPC</code></td><td>Go 服务、强类型调用、流式订阅</td><td>提供请求响应与服务端流式订阅能力。</td></tr>
                  <tr><td><code>gRPC over QUIC</code></td><td>需要 QUIC 传输的 gRPC 场景</td><td>与 gRPC 共享相同的 proto 与 RPC 语义，仅传输层不同。</td></tr>
                </tbody>
              </table>
            </div>
          </div>

          <div class="doc-section">
            <div class="section-title-row"><h3>控制台登录示例</h3></div>
            <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">bash</span></div><div class="code-block" v-html="renderCode(loginSample)"></div></div>
          </div>

          <div class="doc-section">
            <div class="section-title-row"><h3>API Key 启用方式</h3></div>
            <div class="config-stack">
              <div class="surface-card config-card">
                <div class="feature-title">配置文件方式</div>
                <p class="section-desc">适用于首次部署或固定环境配置。</p>
                <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">yaml</span></div><div class="code-block" v-html="renderCode(apiKeyConfigSample)"></div></div>
              </div>
              <div class="surface-card config-card">
                <div class="feature-title">运行期方式</div>
                <p class="section-desc">适用于已登录控制台后的运维调整，需携带管理员或开发者 JWT。</p>
                <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">bash</span></div><div class="code-block" v-html="renderCode(apiKeyRuntimeSample)"></div></div>
              </div>
            </div>
          </div>

          <div class="doc-section">
            <div class="section-title-row"><h3>控制台认证接口</h3></div>
            <div class="table-wrap">
              <table class="docs-table">
                <thead><tr><th>方法</th><th>路径</th><th>鉴权</th><th>说明</th></tr></thead>
                <tbody>
                  <tr><td>POST</td><td><code>/v1/auth/login</code></td><td>公开接口</td><td>控制台登录接口。请求体字段为 <code>username</code> 和 <code>password</code>；其中 <code>password</code> 需传入原始密码的 SHA-256 值。</td></tr>
                </tbody>
              </table>
            </div>
          </div>

          <div class="doc-section">
            <div class="section-title-row"><h3>HTTP 服务注册与发现接口</h3></div>
            <div class="table-wrap">
              <table class="docs-table">
                <thead><tr><th>方法</th><th>路径</th><th>鉴权</th><th>说明</th></tr></thead>
                <tbody>
                  <tr><td>POST</td><td><code>/v1/catalog/register</code></td><td>API Key（开启后必填）</td><td>注册服务实例。常用字段包括 <code>id</code>、<code>service_name</code>、<code>namespace</code>、<code>host</code>、<code>port</code>、<code>weight</code>、<code>dc</code> 和 <code>metadata</code>。</td></tr>
                  <tr><td>POST</td><td><code>/v1/catalog/heartbeat</code></td><td>API Key（开启后必填）</td><td>续约服务实例。请求体字段为 <code>namespace</code>、<code>service_name</code> 和 <code>instance_id</code>。</td></tr>
                  <tr><td>POST</td><td><code>/v1/catalog/instance/status</code></td><td>API Key 或管理员 / 开发者 JWT</td><td>将实例状态切换为 <code>online</code> 或 <code>offline</code>，适用于优雅下线和人工恢复等场景。</td></tr>
                  <tr><td>GET</td><td><code>/v1/catalog/services</code></td><td>公开接口</td><td>按命名空间列出服务摘要信息。</td></tr>
                  <tr><td>GET</td><td><code>/v1/catalog/service/{name}</code></td><td>公开接口</td><td>查询服务实例列表，支持 <code>passing</code>、<code>namespace</code>、<code>dc</code> 和 <code>consumer_service</code> 查询参数。</td></tr>
                  <tr><td>GET</td><td><code>/v1/catalog/service/{name}/subscribers</code></td><td>公开接口</td><td>查询指定服务的订阅方列表。</td></tr>
                  <tr><td>GET</td><td><code>/v1/catalog/dependency-graph</code></td><td>公开接口</td><td>返回服务依赖关系图数据。</td></tr>
                  <tr><td>GET</td><td><code>/v1/catalog/topology</code></td><td>公开接口</td><td>返回服务调用拓扑视图。</td></tr>
                  <tr><td>POST</td><td><code>/v1/catalog/topology/report</code></td><td>API Key（开启后必填）</td><td>主动上报消费者与提供者关系，适用于 SDK 或自定义客户端维护调用拓扑。</td></tr>
                </tbody>
              </table>
            </div>
          </div>

          <div class="doc-section">
            <div class="section-title-row"><h3>命名空间管理接口</h3></div>
            <div class="table-wrap">
              <table class="docs-table">
                <thead><tr><th>方法</th><th>路径</th><th>鉴权</th><th>说明</th></tr></thead>
                <tbody>
                  <tr><td>GET</td><td><code>/v1/namespaces</code></td><td>管理员 / 开发者 JWT</td><td>列出全部命名空间。</td></tr>
                  <tr><td>POST</td><td><code>/v1/namespace</code></td><td>管理员 / 开发者 JWT</td><td>创建命名空间。</td></tr>
                  <tr><td>PUT</td><td><code>/v1/namespace</code></td><td>管理员 / 开发者 JWT</td><td>更新命名空间描述信息。</td></tr>
                  <tr><td>DELETE</td><td><code>/v1/namespace?name={name}</code></td><td>管理员 / 开发者 JWT</td><td>删除指定命名空间；默认命名空间不允许删除。</td></tr>
                </tbody>
              </table>
            </div>
          </div>

          <div class="doc-section">
            <div class="section-title-row"><h3>原生 gRPC RPC 列表</h3></div>
            <div class="table-wrap">
              <table class="docs-table">
                <thead><tr><th>RPC</th><th>类型</th><th>适用方</th><th>说明</th></tr></thead>
                <tbody>
                  <tr><td><code>Register</code></td><td>Unary</td><td>应用侧 gRPC 客户端</td><td>注册服务实例。</td></tr>
                  <tr><td><code>Heartbeat</code></td><td>Unary</td><td>应用侧 gRPC 客户端</td><td>续约服务实例。</td></tr>
                  <tr><td><code>Discover</code></td><td>Unary</td><td>应用侧 gRPC 客户端</td><td>查询健康实例列表。</td></tr>
                  <tr><td><code>Watch</code></td><td>Server Stream</td><td>应用侧 gRPC 客户端</td><td>订阅服务实例变更事件。</td></tr>
                  <tr><td><code>SetInstanceStatus</code></td><td>Unary</td><td>应用侧 gRPC 客户端</td><td>将实例状态设置为 <code>online</code> 或 <code>offline</code>。</td></tr>
                  <tr><td><code>ReportTopology</code></td><td>Unary</td><td>应用侧 gRPC 客户端</td><td>主动上报消费者与提供者关系。</td></tr>
                  <tr><td><code>Deregister</code></td><td>Unary</td><td>兼容保留接口</td><td>兼容保留的下线接口；新接入建议优先使用 <code>SetInstanceStatus</code>。</td></tr>
                </tbody>
              </table>
            </div>
          </div>

          <div class="doc-section surface-card">
            <div class="section-title-row"><h3>鉴权要求</h3></div>
            <ul class="doc-list">
              <li>HTTP 注册、心跳和拓扑上报接口在开启 API Key 后必须携带 <code>X-API-Key</code>。</li>
              <li>建议在配置文件中维护 <code>auth.api_key.enabled</code> 与 <code>auth.api_key.keys</code>，或通过 <code>/v1/settings/system</code> 在运行期启用。</li>
              <li>gRPC 与 gRPC over QUIC 共享同一套 RPC 协议定义，建议通过统一接入层或受控网络环境进行访问控制。</li>
            </ul>
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<style scoped>
.docs-container {
  --docs-bg: linear-gradient(180deg, rgba(255, 255, 255, 0.98) 0%, rgba(247, 250, 255, 0.98) 100%);
  --docs-card: rgba(255, 255, 255, 0.92);
  --docs-border: rgba(34, 62, 110, 0.1);
  --docs-shadow: 0 20px 50px rgba(30, 62, 120, 0.08);
  --docs-text: #1f2a44;
  --docs-muted: #58627a;
  --docs-accent: #3f7ce8;
  --docs-accent-soft: rgba(63, 124, 232, 0.12);
  --docs-success: #2f9b6a;
  --docs-warn: #c5751c;
  width: 100%;
  min-height: 100%;
  padding: 24px 28px 32px;
  background: var(--docs-bg);
  border: 1px solid var(--docs-border);
  border-radius: 24px;
  box-shadow: var(--docs-shadow);
}

.docs-tabs {
  width: 100%;
}

:deep(.el-tabs__header) {
  margin: 0 0 24px;
}

:deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: rgba(32, 54, 94, 0.08);
}

:deep(.el-tabs__item) {
  height: 48px;
  padding: 0 20px;
  color: #4a5672;
  font-size: 17px;
  font-weight: 600;
}

:deep(.el-tabs__item.is-active) {
  color: var(--docs-accent);
}

:deep(.el-tabs__active-bar) {
  height: 3px;
  border-radius: 999px;
  background: linear-gradient(90deg, #5d93f5 0%, #2f74ea 100%);
}

:deep(.el-tabs__content) {
  overflow: visible;
}

.docs-pane-content {
  display: flex;
  flex-direction: column;
  gap: 28px;
  color: var(--docs-text);
}

.docs-pane-content h2 {
  margin: 0;
  font-size: 42px;
  line-height: 1.15;
  letter-spacing: -0.02em;
  color: #17233c;
}

.lead {
  margin: 0;
  max-width: 1100px;
  color: var(--docs-muted);
  font-size: 17px;
  line-height: 1.9;
}

.hero-block {
  position: relative;
  padding: 32px 34px;
  border-radius: 24px;
  background:
    radial-gradient(circle at top right, rgba(103, 158, 255, 0.2), transparent 36%),
    linear-gradient(135deg, #fafdff 0%, #eef4ff 100%);
  border: 1px solid rgba(63, 124, 232, 0.14);
  overflow: hidden;
}

.hero-badge {
  display: inline-flex;
  align-items: center;
  padding: 7px 14px;
  border-radius: 999px;
  background: rgba(63, 124, 232, 0.1);
  color: var(--docs-accent);
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.hero-stat-grid,
.feature-grid,
.info-grid,
.protocol-grid,
.two-column-grid,
.integration-grid {
  display: grid;
  gap: 18px;
}

.hero-stat-grid {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.feature-grid,
.integration-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.info-grid,
.protocol-grid {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.two-column-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.config-stack {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.surface-card {
  padding: 24px;
  border-radius: 22px;
  background: var(--docs-card);
  border: 1px solid rgba(33, 57, 105, 0.08);
  box-shadow: 0 12px 32px rgba(38, 59, 105, 0.06);
}

.hero-stat-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 124px;
  justify-content: space-between;
}

.hero-stat-label {
  font-size: 13px;
  font-weight: 700;
  color: #61708d;
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.hero-stat-value {
  font-size: 16px;
  line-height: 1.8;
  color: var(--docs-text);
}

.feature-title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
  color: #182640;
}

.feature-desc,
.section-desc {
  margin: 0;
  color: var(--docs-muted);
  font-size: 15px;
  line-height: 1.85;
}

.section-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.section-title-row h3 {
  margin: 0;
  font-size: 28px;
  color: #182640;
}

.section-note {
  color: #69758f;
  font-size: 14px;
  line-height: 1.7;
}

.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 16px;
}

.anchor-nav {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  padding: 8px 0 4px;
}

.anchor-link {
  display: inline-flex;
  align-items: center;
  min-height: 36px;
  padding: 0 14px;
  border-radius: 999px;
  background: rgba(63, 124, 232, 0.08);
  color: #325fb4;
  font-size: 14px;
  font-weight: 700;
  text-decoration: none;
  transition: background 0.2s ease, color 0.2s ease, transform 0.2s ease;
}

.anchor-link:hover {
  background: rgba(63, 124, 232, 0.14);
  color: #244b95;
  transform: translateY(-1px);
}

.tag-chip,
.card-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 30px;
  padding: 0 12px;
  border-radius: 999px;
  font-size: 13px;
  font-weight: 700;
}

.tag-chip {
  background: rgba(63, 124, 232, 0.08);
  color: #446ebf;
}

.card-badge {
  background: rgba(47, 155, 106, 0.12);
  color: #247950;
  white-space: nowrap;
}

.doc-section {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.doc-section.surface-card {
  gap: 20px;
}

.integration-card {
  scroll-margin-top: 88px;
}

.info-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-height: 116px;
}

.info-label {
  color: #65728b;
  font-size: 14px;
  font-weight: 600;
}

.info-value {
  color: #16233e;
  font-size: 16px;
  line-height: 1.85;
  word-break: break-word;
}

.code-shell {
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.07);
  border-radius: 18px;
  background: #1e1e1e;
  box-shadow: 0 18px 36px rgba(15, 23, 42, 0.22);
}

.code-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
  background: linear-gradient(180deg, #252526 0%, #1f1f1f 100%);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.code-dots {
  display: inline-flex;
  gap: 8px;
}

.code-dots span {
  width: 11px;
  height: 11px;
  border-radius: 50%;
}

.code-dots span:nth-child(1) {
  background: #ff5f56;
}

.code-dots span:nth-child(2) {
  background: #ffbd2e;
}

.code-dots span:nth-child(3) {
  background: #27c93f;
}

.code-lang {
  color: #9da5b4;
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.12em;
}

.code-block {
  overflow-x: auto;
  padding: 18px 0;
  font-family: "Cascadia Code", "Fira Code", "JetBrains Mono", Consolas, monospace;
  font-size: 13px;
  line-height: 1.8;
}

:deep(.code-row) {
  display: grid;
  grid-template-columns: 52px minmax(0, 1fr);
  align-items: start;
  min-width: max-content;
  white-space: pre;
}

:deep(.code-line-no) {
  padding: 0 14px 0 16px;
  color: #6e7681;
  text-align: right;
  user-select: none;
}

:deep(.code-line-content) {
  display: block;
  padding-right: 24px;
  color: #d4d4d4;
}

:deep(.token.comment) {
  color: #6a9955;
}

:deep(.token.string) {
  color: #ce9178;
}

:deep(.token.keyword) {
  color: #569cd6;
}

:deep(.token.number) {
  color: #b5cea8;
}

:deep(.token.property) {
  color: #9cdcfe;
}

:deep(.token.func) {
  color: #dcdcaa;
}

:deep(.token.builtin) {
  color: #c586c0;
}

:deep(.token.flag) {
  color: #d7ba7d;
}

:deep(.token.variable) {
  color: #4fc1ff;
}

.doc-list {
  margin: 0;
  padding-left: 20px;
  color: var(--docs-muted);
  line-height: 1.85;
}

.doc-list li + li {
  margin-top: 10px;
}

.table-wrap {
  overflow-x: auto;
  border: 1px solid rgba(33, 57, 105, 0.08);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.96);
}

.table-wrap.compact .docs-table th,
.table-wrap.compact .docs-table td {
  padding-top: 14px;
  padding-bottom: 14px;
}

.docs-table {
  width: 100%;
  border-collapse: collapse;
  min-width: 820px;
}

.docs-table th,
.docs-table td {
  padding: 16px 18px;
  border-bottom: 1px solid rgba(33, 57, 105, 0.08);
  vertical-align: top;
  text-align: left;
  font-size: 14px;
  line-height: 1.8;
}

.docs-table th {
  background: rgba(63, 124, 232, 0.06);
  color: #30476e;
  font-weight: 700;
}

.docs-table td {
  color: #44516b;
}

.docs-table tbody tr:last-child td {
  border-bottom: none;
}

.note-panel {
  padding: 20px 22px;
  border-radius: 18px;
  border: 1px solid rgba(33, 57, 105, 0.08);
}

.note-panel.info {
  background: rgba(63, 124, 232, 0.07);
  border-color: rgba(63, 124, 232, 0.14);
}

.note-panel.warn {
  background: rgba(245, 163, 63, 0.09);
  border-color: rgba(197, 117, 28, 0.16);
}

.note-title {
  margin-bottom: 10px;
  font-size: 16px;
  font-weight: 700;
  color: #203250;
}

code {
  padding: 2px 6px;
  border-radius: 6px;
  background: rgba(63, 124, 232, 0.08);
  color: #2956a6;
  font-family: "Cascadia Code", "Fira Code", "JetBrains Mono", Consolas, monospace;
  font-size: 0.92em;
}

@media (max-width: 1024px) {
  .hero-stat-grid,
  .feature-grid,
  .integration-grid,
  .info-grid,
  .protocol-grid,
  .two-column-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .section-title-row {
    align-items: flex-start;
    flex-direction: column;
  }

  .docs-pane-content h2 {
    font-size: 36px;
  }
}

@media (max-width: 768px) {
  .docs-container {
    padding: 18px 16px 24px;
    border-radius: 18px;
  }

  :deep(.el-tabs__item) {
    padding: 0 12px;
    font-size: 15px;
  }

  .hero-block,
  .surface-card {
    padding: 20px;
  }

  .hero-stat-grid,
  .feature-grid,
  .integration-grid,
  .info-grid,
  .protocol-grid,
  .two-column-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .docs-pane-content {
    gap: 22px;
  }

  .docs-pane-content h2 {
    font-size: 30px;
  }

  .lead {
    font-size: 15px;
    line-height: 1.8;
  }

  .section-title-row h3 {
    font-size: 24px;
  }

  :deep(.code-row) {
    grid-template-columns: 44px minmax(0, 1fr);
  }

  :deep(.code-line-no) {
    padding-left: 10px;
    padding-right: 10px;
  }

  .docs-table {
    min-width: 680px;
  }
}
</style>

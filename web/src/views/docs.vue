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
mode: "cluster"
consistency: "ap"
datacenter: "dc1"
data_dir: "./data"

http_addr: ":8500"
grpc_addr: ":0"
quic_addr: ""

transport:
  grpc: "auto"
  quic: "off"
  raft: "off"
# 鑺傜偣鎴愬憳閫氳繃鎺у埗鍙版坊鍔?
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
mode: "cluster"
consistency: "cp"
datacenter: "dc1"
data_dir: "./data"

http_addr: ":8500"
grpc_addr: ":9000"
quic_addr: ":10000"
raft_addr: "127.0.0.1:7000"

transport:
  grpc: "auto"
  quic: "on"
  raft: "auto"

bootstrap: true
# 其余节点通过控制台加入`
}

const cpJoinConfig: CodeSample = {
  lang: 'yaml',
  code: `node_id: "node-2"
mode: "cluster"
consistency: "cp"
datacenter: "dc1"
data_dir: "./data/node-2"

http_addr: ":8501"
grpc_addr: ":9001"
quic_addr: ":10001"
raft_addr: "127.0.0.1:7001"

transport:
  grpc: "auto"
  quic: "on"
  raft: "auto"

bootstrap: false
# 启动后在 Leader 控制台中添加该节点`
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

  nacosClients "github.com/shiyindaxiaojie/eden-go-registry/internal/adapter/nacos/clients"
  nacosConst "github.com/shiyindaxiaojie/eden-go-registry/internal/adapter/nacos/common/constant"
  nacosModel "github.com/shiyindaxiaojie/eden-go-registry/internal/adapter/nacos/model"
  nacosVo "github.com/shiyindaxiaojie/eden-go-registry/internal/adapter/nacos/vo"
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

  consulapi "github.com/shiyindaxiaojie/eden-go-registry/internal/adapter/consul/api"
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
      <el-tab-pane name="intro" label="绯荤粺浠嬬粛">
        <div class="docs-pane-content">
          <div class="surface-card">
            <p class="lead">
              Eden Registry 鏄潰鍚戝井鏈嶅姟鍦烘櫙鐨勬湇鍔℃敞鍐屼笌鍙戠幇绯荤粺锛屾彁渚涙湇鍔″疄渚嬫敞鍐屻€佸績璺崇画绾︺€佸疄渚嬪彂鐜般€佸彉鏇磋闃呫€佸懡鍚嶇┖闂撮殧绂诲拰璁块棶閴存潈鑳藉姏銆傜郴缁熷澶栨彁渚?HTTP API銆乬RPC 涓?gRPC over QUIC 鎺ュ叆鏂瑰紡锛屽彲鐢ㄤ簬鏈嶅姟娌荤悊銆佺幆澧冮殧绂诲拰搴旂敤渚х粺涓€鎺ュ叆銆?
            </p>
          </div>

          <div class="doc-section">
            <div class="section-title-row"><h3>鏍稿績鍔熻兘</h3></div>
            <div class="feature-grid">
              <div class="surface-card">
                <div class="feature-title">鏈嶅姟娉ㄥ唽涓庡仴搴风淮鎶</div>
                <div class="feature-desc">缁熶竴缁存姢鏈嶅姟瀹炰緥銆佸績璺崇画绾︺€佷笂涓嬬嚎鐘舵€併€佹潈閲嶅拰鍏冩暟鎹紝褰㈡垚绋冲畾銆佸彲鏌ヨ鐨勬湇鍔＄洰褰曘€</div>
              </div>
              <div class="surface-card">
                <div class="feature-title">澶氬崗璁彂鐜颁笌璁㈤槄</div>
                <div class="feature-desc">鍚屾椂鎻愪緵 HTTP銆乬RPC 鍜?gRPC over QUIC 鎺ュ叆鑳藉姏锛屾敮鎸佹煡璇㈠紡鍙戠幇涓庢祦寮忚闃呫€</div>
              </div>
              <div class="surface-card">
                <div class="feature-title">鍛藉悕绌洪棿涓庡叧绯昏鍥</div>
                <div class="feature-desc">鏀寔鎸夊懡鍚嶇┖闂撮殧绂荤幆澧冩垨绉熸埛锛屽苟瀵瑰鎻愪緵璁㈤槄鏂广€佷緷璧栧浘鍜屾嫇鎵戣鍥剧瓑娌荤悊淇℃伅銆</div>
              </div>
              <div class="surface-card">
                <div class="feature-title">鎺у埗鍙颁笌璁块棶鎺у埗</div>
                <div class="feature-desc">鎻愪緵鎺у埗鍙扮櫥褰曘€丄PI Key銆丣WT 鍙婂懡鍚嶇┖闂寸鐞嗚兘鍔涳紝婊¤冻鎺ュ叆鎺у埗涓庢棩甯歌繍缁磋姹傘€</div>
              </div>
            </div>
          </div>

          <div class="doc-section">
            <div class="section-title-row"><h3>涓?Consul銆丯acos 瀵规瘮</h3></div>
            <div class="table-wrap">
              <table class="docs-table">
                <thead>
                  <tr>
                    <th>姣旇緝椤</th>
                    <th>Eden Registry</th>
                    <th>Consul</th>
                    <th>Nacos</th>
                    <th>閫傜敤浼樺娍</th>
                  </tr>
                </thead>
                <tbody>
                  <tr>
                    <td>搴旂敤鎺ュ叆鏂瑰紡</td>
                    <td>鎻愪緵 HTTP銆乬RPC 涓?gRPC over QUIC 涓夌被鎺ュ叆鏂瑰紡銆</td>
                    <td>閫氬父浠?HTTP API銆丏NS 鍜岀敓鎬?SDK 涓轰富瑕佸叆鍙ｃ€</td>
                    <td>閫氬父浠?OpenAPI 鍜屽畼鏂?SDK 涓轰富瑕佸叆鍙ｃ€</td>
                    <td>鍦ㄤ竴涓郴缁熷唴鍚屾椂瑕嗙洊杞婚噺 REST 鎺ュ叆涓庡己绫诲瀷闀胯繛鎺ユ帴鍏ワ紝渚夸簬缁熶竴鎺ュ叆鏍囧噯銆</td>
                  </tr>
                  <tr>
                    <td>Go 鏈嶅姟鎺ュ叆闂幆</td>
                    <td>鎻愪緵浠庢敞鍐屻€佺画绾﹀埌鍙戠幇銆佽闃呫€佷笅绾跨殑瀹屾暣 Go 鎺ュ叆闂幆銆</td>
                    <td>閫氬父鍥寸粫 HTTP API 鎴栫涓夋柟 SDK 缁勭粐娉ㄥ唽涓庡彂鐜伴摼璺€</td>
                    <td>閫氬父鍥寸粫瀹樻柟 SDK 涓?OpenAPI 缁勫悎瀹屾垚涓氬姟鎺ュ叆銆</td>
                    <td>鏇撮€傚悎甯屾湜缁熶竴瀹㈡埛绔涓恒€佸噺灏戦噸澶嶅皝瑁呭拰闄嶄綆杩佺Щ鎴愭湰鐨?Go 鍥㈤槦銆</td>
                  </tr>
                  <tr>
                    <td>娌荤悊瑙嗚</td>
                    <td>鑱氱劍鏈嶅姟銆佸疄渚嬨€佸懡鍚嶇┖闂淬€佽闃呮柟鍜屼緷璧栨嫇鎵戠殑涓€浣撳寲灞曠ず銆</td>
                    <td>娌荤悊鑳藉姏瀹屾暣锛屼絾璋冪敤鍏崇郴灞曠ず閫氬父渚濊禆棰濆鐢熸€佺粍浠躲€</td>
                    <td>鎺у埗鍙版不鐞嗚兘鍔涙垚鐔燂紝渚ч噸鍛藉悕涓庨厤缃綋绯荤殑缁熶竴绠＄悊銆</td>
                    <td>瀵逛簬浠呭叧娉ㄦ敞鍐屽彂鐜板拰璋冪敤鍏崇郴娌荤悊鐨勫洟闃燂紝淇℃伅闈㈡洿闆嗕腑锛岄槄璇绘垚鏈洿浣庛€</td>
                  </tr>
                  <tr>
                    <td>閮ㄧ讲瀹氫綅</td>
                    <td>鑱氱劍鏈嶅姟娉ㄥ唽鍙戠幇銆佸懡鍚嶇┖闂撮殧绂诲拰鎺ュ叆娌荤悊锛屽己璋冭交閲忛棴鐜€</td>
                    <td>瑕嗙洊鏈嶅姟鍙戠幇銆佺綉缁滄不鐞嗙瓑鏇村箍娉涚殑鍩虹璁炬柦鑳藉姏銆</td>
                    <td>鍚屾椂瑕嗙洊鏈嶅姟娉ㄥ唽鍙戠幇涓庨厤缃鐞嗗満鏅€</td>
                    <td>鍦ㄤ粎闇€鏈嶅姟娉ㄥ唽鍙戠幇鑳藉姏鐨勯」鐩腑锛屾洿瀹规槗褰㈡垚娓呮櫚鐨勮亴璐ｈ竟鐣屻€</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane name="quickStart" label="快速上手">
        <div class="docs-pane-content">
          <p class="lead">鏈妭璇存槑棣栨閮ㄧ讲涓庨獙璇佹祦绋嬶紝鍖呮嫭鏈嶅姟绔惎鍔ㄣ€佹帶鍒跺彴鍚姩銆侀娆＄櫥褰曘€佹敞鍐屼笌鍙戠幇楠岃瘉锛屼互鍙婂父鐢ㄩ厤缃」璇存槑銆</p>

          <div class="doc-section surface-card">
            <div class="section-title-row"><h3>1. 鍚姩鏈嶅姟绔</h3></div>
            <p class="section-desc">推荐优先通过配置文件启动。当前服务端支持的命令行覆盖参数包括 <code>-config</code>、<code>-data-dir</code>、<code>-node-id</code> 和 <code>-http-addr</code>，其余能力建议通过配置文件维护。</p>
            <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">bash</span></div><div class="code-block" v-html="renderCode(serverStart)"></div></div>
            <ul class="doc-list">
              <li>榛樿 HTTP API 鐩戝惉鍦板潃涓?<code>:8500</code>锛屾帶鍒跺彴寮€鍙戜唬鐞嗛粯璁よ浆鍙戝埌璇ュ湴鍧€銆</li>
              <li>鑻ラ渶闅旂鏁版嵁鐩綍鎴栬妭鐐规爣璇嗭紝鍙€氳繃 <code>-data-dir</code> 涓?<code>-node-id</code> 瑕嗙洊閰嶇疆鏂囦欢涓殑鍚屽悕椤广€</li>
            </ul>
          </div>

          <div class="doc-section surface-card">
            <div class="section-title-row"><h3>2. 鍚姩鎺у埗鍙</h3></div>
            <p class="section-desc">鎺у埗鍙版槸鐙珛鐨?Vite 鍓嶇搴旂敤銆傚紑鍙戞ā寮忎笅榛樿鐩戝惉 <code>2019</code> 绔彛锛屽苟灏?<code>/v1/*</code> 璇锋眰浠ｇ悊鍒板悗绔?HTTP API銆</p>
            <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">bash</span></div><div class="code-block" v-html="renderCode(consoleStart)"></div></div>
            <ul class="doc-list">
              <li>鎺у埗鍙伴粯璁よ闂湴鍧€涓?<code>http://127.0.0.1:2019</code>銆</li>
            </ul>
          </div>

          <div class="doc-section surface-card">
            <div class="section-title-row"><h3>3. 棣栨鐧诲綍</h3></div>
            <p class="section-desc">榛樿绠＄悊鍛樿处鍙蜂负 <code>admin / admin</code>銆傜櫥褰曟帴鍙ｈ姹?<code>password</code> 浼犲叆鍘熷瀵嗙爜鐨?SHA-256 鍊硷紱鑻ヤ粠鎺у埗鍙伴〉闈㈢櫥褰曪紝娴忚鍣ㄤ細鑷姩瀹屾垚璇ュ鐞嗐€</p>
            <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">bash</span></div><div class="code-block" v-html="renderCode(loginSample)"></div></div>
          </div>

          <div class="doc-section surface-card">
            <div class="section-title-row"><h3>4. 娉ㄥ唽涓庡彂鐜伴獙璇</h3></div>
            <p class="section-desc">浠ヤ笅绀轰緥閫氳繃 HTTP API 瀹屾垚涓€涓渶灏忛棴鐜細娉ㄥ唽瀹炰緥鍚庣珛鍗冲彂璧峰彂鐜版煡璇紝鐢ㄤ簬纭鏈嶅姟鐩綍宸茬粡瀵瑰鎻愪緵鑳藉姏銆</p>
            <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">bash</span></div><div class="code-block" v-html="renderCode(smokeTest)"></div></div>
            <ul class="doc-list">
              <li>鑻ュ凡鍚敤 API Key 閴存潈锛岃鍦ㄦ敞鍐岃姹備腑棰濆鎼哄甫 <code>X-API-Key</code> 璇锋眰澶淬€</li>
            </ul>
          </div>

          <div class="doc-section">
            <div class="section-title-row"><h3>榛樿璁块棶淇℃伅</h3></div>
            <div class="info-grid">
              <div class="surface-card info-card"><div class="info-label">鍚庣 HTTP API</div><div class="info-value"><code>http://127.0.0.1:8500</code></div></div>
              <div class="surface-card info-card"><div class="info-label">鎺у埗鍙板湴鍧€</div><div class="info-value"><code>http://127.0.0.1:2019</code></div></div>
              <div class="surface-card info-card"><div class="info-label">绠＄悊鍛樿处鍙</div><div class="info-value"><code>admin / admin</code></div></div>
              <div class="surface-card info-card"><div class="info-label">榛樿鍛藉悕绌洪棿</div><div class="info-value"><code>default</code></div></div>
              <div class="surface-card info-card"><div class="info-label">榛樿鏁版嵁涓績</div><div class="info-value"><code>dc1</code></div></div>
            </div>
          </div>

          <div class="doc-section">
            <div class="section-title-row"><h3>閰嶇疆璇存槑</h3></div>
            <div class="config-stack">
              <div class="surface-card config-card">
                <div class="section-title-row"><h3>鍗曡妭鐐?/ AP 妯″紡绀轰緥</h3></div>
                <p class="section-desc">閫傚悎鏈湴鑱旇皟銆佹祴璇曠幆澧冩垨浠ュ彲鐢ㄦ€т紭鍏堢殑杞婚噺閮ㄧ讲鍦烘櫙銆</p>
                <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">yaml</span></div><div class="code-block" v-html="renderCode(apConfig)"></div></div>
                <div class="table-wrap compact">
                  <table class="docs-table">
                    <thead><tr><th>閰嶇疆椤</th><th>榛樿鍊</th><th>璇存槑</th></tr></thead>
                    <tbody>
                      <tr><td><code>node_id</code></td><td>node-1</td><td>鑺傜偣鍞竴鏍囪瘑锛岀敤浜庡尯鍒嗕笉鍚屽疄渚嬨€</td></tr>
                      <tr><td><code>mode</code></td><td>ap</td><td>涓€鑷存€фā寮忥紝<code>ap</code> 琛ㄧず浼樺厛鍙敤銆</td></tr>
                      <tr><td><code>datacenter</code></td><td>dc1</td><td>鏁版嵁涓績鏍囩锛岀敤浜庢湇鍔″彂鐜版椂鐨勬満鎴跨淮搴﹁繃婊ゃ€</td></tr>
                      <tr><td><code>data_dir</code></td><td>./data</td><td>娉ㄥ唽鏁版嵁銆侀壌鏉冩暟鎹拰杩愯鏃堕厤缃殑鎸佷箙鍖栫洰褰曘€</td></tr>
                      <tr><td><code>http_addr</code></td><td>:8500</td><td>HTTP API 涓庢帶鍒跺彴浠ｇ悊璁块棶鍏ュ彛銆</td></tr>
                      <tr><td><code>grpc_addr</code></td><td>:0</td><td>gRPC 鐩戝惉鍦板潃锛涗负绌烘垨 <code>:0</code> 鏃惰〃绀鸿嚜鍔ㄥ垎閰嶇鍙ｃ€</td></tr>
                      <tr><td><code>quic_addr</code></td><td>""</td><td>gRPC over QUIC 鐩戝惉鍦板潃锛涗负绌烘椂琛ㄧず涓嶆樉寮忔寚瀹氥€</td></tr>
                      <tr><td><code>seeds</code></td><td>[]</td><td>AP 妯″紡涓嬪彲閫夌殑绉嶅瓙鑺傜偣鍒楄〃锛屼娇鐢ㄥ叾浠栬妭鐐圭殑 HTTP 鍦板潃銆</td></tr>
                    </tbody>
                  </table>
                </div>
              </div>

              <div class="surface-card config-card">
                <div class="section-title-row"><h3>CP 棣栬妭鐐圭ず渚</h3></div>
                <p class="section-desc">閫傚悎瀵逛竴鑷存€ц姹傛洿楂樼殑閮ㄧ讲銆傞鑺傜偣闇€瑕佹樉寮忓紑鍚?<code>bootstrap</code>锛屽苟涓?<code>raft_addr</code> 鎸囧畾鍙鍏朵粬鑺傜偣璁块棶鐨勬槑纭?IP 鍦板潃銆</p>
                <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">yaml</span></div><div class="code-block" v-html="renderCode(cpBootstrapConfig)"></div></div>
              </div>

              <div class="surface-card config-card">
                <div class="section-title-row"><h3>CP 鍔犲叆鑺傜偣绀轰緥</h3></div>
                <p class="section-desc">鍚庣画鑺傜偣閫氳繃宸插瓨鍦ㄨ妭鐐圭殑 HTTP 鍦板潃鍔犲叆闆嗙兢锛岄伩鍏嶄笟鍔℃帴鍏ユ柟鐩存帴渚濊禆搴曞眰閫氫俊鍦板潃銆</p>
                <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">yaml</span></div><div class="code-block" v-html="renderCode(cpJoinConfig)"></div></div>
              </div>

              <div class="surface-card config-card">
                <div class="section-title-row"><h3>瀹夊叏涓庤繍琛屾椂閰嶇疆</h3></div>
                <div class="table-wrap compact">
                  <table class="docs-table">
                    <thead><tr><th>閰嶇疆椤</th><th>榛樿鍊</th><th>璇存槑</th></tr></thead>
                    <tbody>
                      <tr><td><code>auth.jwt.enabled</code></td><td>true</td><td>鏄惁寮€鍚帶鍒跺彴 JWT 鐧诲綍閴存潈銆</td></tr>
                      <tr><td><code>auth.jwt.secret</code></td><td>eden-jwt-console-secret-key</td><td>JWT 绛惧悕瀵嗛挜锛岀敓浜х幆澧冨簲鏇挎崲涓鸿嚜瀹氫箟瀹夊叏鍊笺€</td></tr>
                      <tr><td><code>auth.api_key.enabled</code></td><td>false</td><td>鏄惁瀵?HTTP 娉ㄥ唽銆佸績璺冲拰鎷撴墤涓婃姤鎺ュ彛寮€鍚?API Key 鏍￠獙銆</td></tr>
                      <tr><td><code>auth.api_key.keys</code></td><td>[]</td><td>棰勭疆鍙敤鐨?API Key 鍒楄〃锛屼篃鍙湪鎺у埗鍙颁腑缁存姢銆</td></tr>
                      <tr><td><code>storage.event_retention_days</code></td><td>30</td><td>浜嬩欢淇濈暀澶╂暟銆</td></tr>
                      <tr><td><code>storage.log_retention_days</code></td><td>30</td><td>鏃ュ織淇濈暀澶╂暟銆</td></tr>
                      <tr><td><code>registry.heartbeat_max_failures</code></td><td>3</td><td>瀹炰緥杩炵画蹇冭烦澶辫触鐨勬渶澶ф鏁伴槇鍊笺€</td></tr>
                      <tr><td><code>registry.instance_removal_delay_seconds</code></td><td>600</td><td>瀹炰緥琚垽瀹氬け鏁堝悗鐨勫欢杩熺Щ闄ゆ椂闂达紝鍗曚綅涓虹銆</td></tr>
                      <tr><td><code>log.level</code></td><td>INFO</td><td>鏈嶅姟绔棩蹇楃骇鍒€</td></tr>
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          </div>

          <div class="doc-section surface-card">
            <div class="section-title-row"><h3>閮ㄧ讲瑕佹眰</h3></div>
            <ul class="doc-list">
              <li>若采用 CP 模式，<code>raft_addr</code> 应使用明确可达的 IP 地址，不建议配置为 <code>:端口</code> 或 <code>0.0.0.0:端口</code>。</li>
              <li><code>grpc_addr</code> 涓?<code>quic_addr</code> 鍏佽浣跨敤绌哄€兼垨 <code>:0</code> 杩涜鑷姩绔彛鍒嗛厤锛涗笟鍔℃帴鍏ユ柟搴斾紭鍏堜娇鐢ㄥ閮ㄧ粺涓€鍏ュ彛鍦板潃锛岃€屼笉鏄亣璁惧浐瀹氬唴閮ㄧ鍙ｃ€</li>
            </ul>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane name="integration" label="客户端接入">
        <div class="docs-pane-content">
          <p class="lead">瀹㈡埛绔帴鍏ユ彁渚?Eden SDK銆丯acos 鎺ュ叆閫傞厤銆丆onsul 鎺ュ叆閫傞厤銆丠TTP API 鍜?gRPC 浜旂被鏂瑰紡銆備互涓嬬ず渚嬪潎閲囩敤 Go 浠ｇ爜锛岃鐩栧疄渚嬫敞鍐屻€佸績璺崇画绾︺€佹湇鍔″彂鐜般€佸彉鏇磋闃呭拰涓嬬嚎澶勭悊鐨勫畬鏁存祦绋嬨€</p>

          <div class="anchor-nav">
            <a class="anchor-link" href="#integration-eden">Eden SDK</a>
            <a class="anchor-link" href="#integration-nacos">Nacos 鎺ュ叆閫傞厤</a>
            <a class="anchor-link" href="#integration-consul">Consul 鎺ュ叆閫傞厤</a>
            <a class="anchor-link" href="#integration-http">HTTP API</a>
            <a class="anchor-link" href="#integration-grpc">gRPC</a>
          </div>

          <div class="config-stack">
            <div id="integration-eden" class="surface-card integration-card">
              <div class="section-title-row"><div class="feature-title">Eden SDK 鎺ュ叆</div><span class="card-badge">鎺ㄨ崘</span></div>
              <p class="section-desc">适用于直接接入 Eden Registry 的 Go 服务。该方式可在同一套业务代码中切换 <code>grpc</code>、<code>quic</code> 或 <code>http</code> 传输，并统一处理注册、续约、发现、订阅和下线流程。</p>
              <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">go</span></div><div class="code-block" v-html="renderCode(edenSdkSample)"></div></div>
              <ul class="doc-list">
                <li>褰?<code>DiscoveryMode=auto</code> 鏃讹紝<code>Addresses</code> 鍙啓涓虹粺涓€ HTTP 鍏ュ彛鍦板潃锛屽鎴风浼氬湪鍐呴儴琛ュ厖鍙敤鑺傜偣淇℃伅銆</li>
                <li>鑻ュ笇鏈涚洿杩炰紶杈撶鍙ｏ紝鍙皢 <code>DiscoveryMode</code> 鏀逛负 <code>static</code>锛屽苟鍦?<code>grpc</code> 鎴?<code>quic</code> 妯″紡涓嬬洿鎺ュ～鍐欏搴斿湴鍧€銆</li>
              </ul>
            </div>

            <div id="integration-nacos" class="surface-card integration-card">
              <div class="section-title-row"><div class="feature-title">Nacos 鎺ュ叆閫傞厤</div><span class="card-badge">骞虫粦杩佺Щ</span></div>
              <p class="section-desc">适用于已经使用 Nacos SDK 的 Go 服务。迁移到 Eden Registry 时，可保留 <code>RegisterInstance</code>、<code>SelectInstances</code>、<code>Subscribe</code> 等调用方式，仅调整接入包路径和服务端连接配置。</p>
              <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">go</span></div><div class="code-block" v-html="renderCode(nacosAdapterSample)"></div></div>
              <ul class="doc-list">
                <li>閫傞厤灞傚吋瀹?<code>github.com/nacos-group/nacos-sdk-go/v2</code> 鐨勫懡鍚嶅彂鐜拌皟鐢ㄩ潰锛岄噸鐐瑰湪浜庤縼绉绘椂涓嶆敼涓氬姟璋冪敤閫昏緫銆</li>
                <li>寤鸿浼樺厛鏇挎崲鎺ュ叆鍖呬笌鍦板潃閰嶇疆锛屽啀鎸夋湇鍔″悕楠岃瘉娉ㄥ唽銆佽闃呭拰鍙戠幇缁撴灉銆</li>
              </ul>
            </div>

            <div id="integration-consul" class="surface-card integration-card">
              <div class="section-title-row"><div class="feature-title">Consul 鎺ュ叆閫傞厤</div><span class="card-badge">骞虫粦杩佺Щ</span></div>
              <p class="section-desc">閫傜敤浜庡凡浣跨敤 Consul API 鐨?Go 鏈嶅姟銆傝縼绉诲埌 Eden Registry 鏃讹紝鍙繚鎸佸師鏈?Agent銆丆atalog 鍜?Health 璋冪敤鏂瑰紡锛屼粎璋冩暣 import 璺緞鍜屾湇鍔＄杩炴帴閰嶇疆銆</p>
              <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">go</span></div><div class="code-block" v-html="renderCode(consulAdapterSample)"></div></div>
              <ul class="doc-list">
                <li>閫傞厤灞傚吋瀹?<code>github.com/hashicorp/consul/api</code> 鐨勫父鐢ㄨ闂柟寮忥紝閫傚悎灏嗙幇鏈?Consul 鎺ュ叆杩佺Щ鍒?Eden Registry銆</li>
                <li>鑻ュ凡鍚敤 API Key锛屽彲閫氳繃 <code>cfg.Token</code> 缁熶竴涓嬪彂锛屼笉闇€瑕佸湪涓氬姟渚ч噸澶嶅皝瑁呰姹傚ご銆</li>
              </ul>
            </div>

            <div id="integration-http" class="surface-card integration-card">
              <div class="section-title-row"><div class="feature-title">HTTP API 鎺ュ叆</div><span class="card-badge">鍘熺敓 HTTP</span></div>
              <p class="section-desc">閫傜敤浜庨渶瑕佺洿鎺ュ鎺ュ叕寮€ HTTP API 骞惰嚜琛屽皝瑁呭鎴风鐨?Go 鏈嶅姟銆傝鏂瑰紡涓嶄緷璧栭澶?SDK锛屽彲鍦ㄤ笟鍔′晶缁熶竴瀹炵幇娉ㄥ唽銆佸績璺炽€佸彂鐜板拰涓嬬嚎娴佺▼銆</p>
              <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">go</span></div><div class="code-block" v-html="renderCode(httpClientSample)"></div></div>
              <ul class="doc-list">
                <li>HTTP 娉ㄥ唽璇锋眰涓殑鏁版嵁涓績瀛楁鍚嶄负 <code>dc</code>锛屼笉鏄?<code>datacenter</code>銆</li>
                <li>瀹炰緥鐘舵€佸垏鎹㈡帴鍙ｅ彲鎺ュ彈鏈夋晥 API Key锛屼篃鍙帴鍙楃鐞嗗憳鎴栧紑鍙戣€?JWT銆</li>
              </ul>
            </div>

            <div id="integration-grpc" class="surface-card integration-card">
              <div class="section-title-row"><div class="feature-title">gRPC 鎺ュ叆</div><span class="card-badge">寮虹被鍨</span></div>
              <p class="section-desc">閫傜敤浜庡笇鏈涚洿鎺ュ熀浜?proto 鍗忚灏佽瀹㈡埛绔€佹樉寮忔帶鍒舵祦寮忚闃呮垨缁熶竴鎺ュ叆寮虹被鍨?RPC 鐨?Go 鏈嶅姟銆俫RPC over QUIC 涓?gRPC 鍏变韩鐩稿悓鐨?RPC 璇箟銆</p>
              <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">go</span></div><div class="code-block" v-html="renderCode(grpcClientSample)"></div></div>
              <ul class="doc-list">
                <li><code>Watch</code> 涓烘湇鍔＄娴佸紡鎺ュ彛锛岄€傜敤浜庨渶瑕佸疄鏃舵劅鐭ュ疄渚嬪彉鍖栫殑璋冪敤鏂广€</li>
                <li>鑻ュ垏鎹㈠埌 gRPC over QUIC锛屽簲淇濇寔鐩稿悓鐨?proto 涓?RPC 璇箟锛屼粎灏嗚繛鎺ョ鐐规浛鎹负 <code>quic_addr</code> 骞朵娇鐢ㄥ吋瀹圭殑 QUIC 鎷ㄥ彿鏂瑰紡銆</li>
              </ul>
            </div>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane name="api" label="API 鎺ュ彛">
        <div class="docs-pane-content">
          <p class="lead">绯荤粺瀵瑰鎻愪緵 HTTP API銆乬RPC 涓?gRPC over QUIC 涓夌被鍗忚銆備互涓嬩粎鍒楀嚭闈㈠悜澶栭儴鎺ュ叆鏂圭殑鍏紑鎺ュ彛銆侀壌鏉冩柟寮忎笌鍚敤瑕佹眰銆</p>

          <div class="doc-section">
            <div class="section-title-row"><h3>鍗忚璇存槑</h3></div>
            <div class="table-wrap compact">
              <table class="docs-table">
                <thead><tr><th>鍗忚</th><th>閫傜敤鍦烘櫙</th><th>璇存槑</th></tr></thead>
                <tbody>
                  <tr><td><code>HTTP API</code></td><td>鎺у埗鍙般€佽剼鏈€佽法璇█闆嗘垚</td><td>閲囩敤 JSON over HTTP锛岄€傚悎杞婚噺鎺ュ叆涓庣鐞嗙被璋冪敤銆</td></tr>
                  <tr><td><code>gRPC</code></td><td>Go 鏈嶅姟銆佸己绫诲瀷璋冪敤銆佹祦寮忚闃</td><td>鎻愪緵璇锋眰鍝嶅簲涓庢湇鍔＄娴佸紡璁㈤槄鑳藉姏銆</td></tr>
                  <tr><td><code>gRPC over QUIC</code></td><td>闇€瑕?QUIC 浼犺緭鐨?gRPC 鍦烘櫙</td><td>涓?gRPC 鍏变韩鐩稿悓鐨?proto 涓?RPC 璇箟锛屼粎浼犺緭灞備笉鍚屻€</td></tr>
                </tbody>
              </table>
            </div>
          </div>

          <div class="doc-section">
            <div class="section-title-row"><h3>鎺у埗鍙扮櫥褰曠ず渚</h3></div>
            <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">bash</span></div><div class="code-block" v-html="renderCode(loginSample)"></div></div>
          </div>

          <div class="doc-section">
            <div class="section-title-row"><h3>API Key 鍚敤鏂瑰紡</h3></div>
            <div class="config-stack">
              <div class="surface-card config-card">
                <div class="feature-title">閰嶇疆鏂囦欢鏂瑰紡</div>
                <p class="section-desc">閫傜敤浜庨娆￠儴缃叉垨鍥哄畾鐜閰嶇疆銆</p>
                <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">yaml</span></div><div class="code-block" v-html="renderCode(apiKeyConfigSample)"></div></div>
              </div>
              <div class="surface-card config-card">
                <div class="feature-title">杩愯鏈熸柟寮</div>
                <p class="section-desc">閫傜敤浜庡凡鐧诲綍鎺у埗鍙板悗鐨勮繍缁磋皟鏁达紝闇€鎼哄甫绠＄悊鍛樻垨寮€鍙戣€?JWT銆</p>
                <div class="code-shell"><div class="code-toolbar"><div class="code-dots"><span></span><span></span><span></span></div><span class="code-lang">bash</span></div><div class="code-block" v-html="renderCode(apiKeyRuntimeSample)"></div></div>
              </div>
            </div>
          </div>

          <div class="doc-section">
            <div class="section-title-row"><h3>鎺у埗鍙拌璇佹帴鍙</h3></div>
            <div class="table-wrap">
              <table class="docs-table">
                <thead><tr><th>鏂规硶</th><th>璺緞</th><th>閴存潈</th><th>璇存槑</th></tr></thead>
                <tbody>
                  <tr><td>POST</td><td><code>/v1/auth/login</code></td><td>鍏紑鎺ュ彛</td><td>鎺у埗鍙扮櫥褰曟帴鍙ｃ€傝姹備綋瀛楁涓?<code>username</code> 鍜?<code>password</code>锛涘叾涓?<code>password</code> 闇€浼犲叆鍘熷瀵嗙爜鐨?SHA-256 鍊笺€</td></tr>
                </tbody>
              </table>
            </div>
          </div>

          <div class="doc-section">
            <div class="section-title-row"><h3>HTTP 鏈嶅姟娉ㄥ唽涓庡彂鐜版帴鍙</h3></div>
            <div class="table-wrap">
              <table class="docs-table">
                <thead><tr><th>鏂规硶</th><th>璺緞</th><th>閴存潈</th><th>璇存槑</th></tr></thead>
                <tbody>
                  <tr><td>POST</td><td><code>/v1/catalog/register</code></td><td>API Key（开启后必填）</td><td>注册服务实例。常用字段包括 <code>id</code>、<code>service_name</code>、<code>namespace</code>、<code>host</code>、<code>port</code>、<code>weight</code>、<code>dc</code> 和 <code>metadata</code>。</td></tr>
                  <tr><td>POST</td><td><code>/v1/catalog/heartbeat</code></td><td>API Key（开启后必填）</td><td>续约服务实例。请求体字段包括 <code>namespace</code>、<code>service_name</code> 和 <code>instance_id</code>。</td></tr>
                  <tr><td>POST</td><td><code>/v1/catalog/instance/status</code></td><td>API Key 鎴栫鐞嗗憳 / 寮€鍙戣€?JWT</td><td>灏嗗疄渚嬬姸鎬佸垏鎹负 <code>online</code> 鎴?<code>offline</code>锛岄€傜敤浜庝紭闆呬笅绾垮拰浜哄伐鎭㈠绛夊満鏅€</td></tr>
                  <tr><td>GET</td><td><code>/v1/catalog/services</code></td><td>鍏紑鎺ュ彛</td><td>鎸夊懡鍚嶇┖闂村垪鍑烘湇鍔℃憳瑕佷俊鎭€</td></tr>
                  <tr><td>GET</td><td><code>/v1/catalog/service/{name}</code></td><td>公开接口</td><td>查询服务实例列表，支持 <code>passing</code>、<code>namespace</code>、<code>dc</code> 和 <code>consumer_service</code> 查询参数。</td></tr>
                  <tr><td>GET</td><td><code>/v1/catalog/service/{name}/subscribers</code></td><td>鍏紑鎺ュ彛</td><td>鏌ヨ鎸囧畾鏈嶅姟鐨勮闃呮柟鍒楄〃銆</td></tr>
                  <tr><td>GET</td><td><code>/v1/catalog/dependency-graph</code></td><td>鍏紑鎺ュ彛</td><td>杩斿洖鏈嶅姟渚濊禆鍏崇郴鍥炬暟鎹€</td></tr>
                  <tr><td>GET</td><td><code>/v1/catalog/topology</code></td><td>鍏紑鎺ュ彛</td><td>杩斿洖鏈嶅姟璋冪敤鎷撴墤瑙嗗浘銆</td></tr>
                  <tr><td>POST</td><td><code>/v1/catalog/topology/report</code></td><td>API Key锛堝紑鍚悗蹇呭～锛</td><td>涓诲姩涓婃姤娑堣垂鑰呬笌鎻愪緵鑰呭叧绯伙紝閫傜敤浜?SDK 鎴栬嚜瀹氫箟瀹㈡埛绔淮鎶よ皟鐢ㄦ嫇鎵戙€</td></tr>
                </tbody>
              </table>
            </div>
          </div>

          <div class="doc-section">
            <div class="section-title-row"><h3>鍛藉悕绌洪棿绠＄悊鎺ュ彛</h3></div>
            <div class="table-wrap">
              <table class="docs-table">
                <thead><tr><th>鏂规硶</th><th>璺緞</th><th>閴存潈</th><th>璇存槑</th></tr></thead>
                <tbody>
                  <tr><td>GET</td><td><code>/v1/namespaces</code></td><td>绠＄悊鍛?/ 寮€鍙戣€?JWT</td><td>鍒楀嚭鍏ㄩ儴鍛藉悕绌洪棿銆</td></tr>
                  <tr><td>POST</td><td><code>/v1/namespace</code></td><td>绠＄悊鍛?/ 寮€鍙戣€?JWT</td><td>鍒涘缓鍛藉悕绌洪棿銆</td></tr>
                  <tr><td>PUT</td><td><code>/v1/namespace</code></td><td>绠＄悊鍛?/ 寮€鍙戣€?JWT</td><td>鏇存柊鍛藉悕绌洪棿鎻忚堪淇℃伅銆</td></tr>
                  <tr><td>DELETE</td><td><code>/v1/namespace?name={name}</code></td><td>绠＄悊鍛?/ 寮€鍙戣€?JWT</td><td>鍒犻櫎鎸囧畾鍛藉悕绌洪棿锛涢粯璁ゅ懡鍚嶇┖闂翠笉鍏佽鍒犻櫎銆</td></tr>
                </tbody>
              </table>
            </div>
          </div>

          <div class="doc-section">
            <div class="section-title-row"><h3>鍘熺敓 gRPC RPC 鍒楄〃</h3></div>
            <div class="table-wrap">
              <table class="docs-table">
                <thead><tr><th>RPC</th><th>绫诲瀷</th><th>閫傜敤鏂</th><th>璇存槑</th></tr></thead>
                <tbody>
                  <tr><td><code>Register</code></td><td>Unary</td><td>搴旂敤渚?gRPC 瀹㈡埛绔</td><td>娉ㄥ唽鏈嶅姟瀹炰緥銆</td></tr>
                  <tr><td><code>Heartbeat</code></td><td>Unary</td><td>搴旂敤渚?gRPC 瀹㈡埛绔</td><td>缁害鏈嶅姟瀹炰緥銆</td></tr>
                  <tr><td><code>Discover</code></td><td>Unary</td><td>搴旂敤渚?gRPC 瀹㈡埛绔</td><td>鏌ヨ鍋ュ悍瀹炰緥鍒楄〃銆</td></tr>
                  <tr><td><code>Watch</code></td><td>Server Stream</td><td>搴旂敤渚?gRPC 瀹㈡埛绔</td><td>璁㈤槄鏈嶅姟瀹炰緥鍙樻洿浜嬩欢銆</td></tr>
                  <tr><td><code>SetInstanceStatus</code></td><td>Unary</td><td>搴旂敤渚?gRPC 瀹㈡埛绔</td><td>灏嗗疄渚嬬姸鎬佽缃负 <code>online</code> 鎴?<code>offline</code>銆</td></tr>
                  <tr><td><code>ReportTopology</code></td><td>Unary</td><td>搴旂敤渚?gRPC 瀹㈡埛绔</td><td>涓诲姩涓婃姤娑堣垂鑰呬笌鎻愪緵鑰呭叧绯汇€</td></tr>
                  <tr><td><code>Deregister</code></td><td>Unary</td><td>鍏煎淇濈暀鎺ュ彛</td><td>鍏煎淇濈暀鐨勪笅绾挎帴鍙ｏ紱鏂版帴鍏ュ缓璁紭鍏堜娇鐢?<code>SetInstanceStatus</code>銆</td></tr>
                </tbody>
              </table>
            </div>
          </div>

          <div class="doc-section surface-card">
            <div class="section-title-row"><h3>閴存潈瑕佹眰</h3></div>
            <ul class="doc-list">
              <li>HTTP 娉ㄥ唽銆佸績璺冲拰鎷撴墤涓婃姤鎺ュ彛鍦ㄥ紑鍚?API Key 鍚庡繀椤绘惡甯?<code>X-API-Key</code>銆</li>
              <li>寤鸿鍦ㄩ厤缃枃浠朵腑缁存姢 <code>auth.api_key.enabled</code> 涓?<code>auth.api_key.keys</code>锛屾垨閫氳繃 <code>/v1/settings/system</code> 鍦ㄨ繍琛屾湡鍚敤銆</li>
              <li>gRPC 涓?gRPC over QUIC 鍏变韩鍚屼竴濂?RPC 鍗忚瀹氫箟锛屽缓璁€氳繃缁熶竴鎺ュ叆灞傛垨鍙楁帶缃戠粶鐜杩涜璁块棶鎺у埗銆</li>
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



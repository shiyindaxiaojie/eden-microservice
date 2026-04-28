<script setup lang="ts">
import { ref } from 'vue'

type CodeSample = {
  lang: 'bash' | 'yaml' | 'go'
  code: string
}

type DocEntry = {
  title: string
  path: string
  audience: string
  summary: string
}

type Capability = {
  title: string
  summary: string
}

type IntegrationPath = {
  title: string
  badge: string
  summary: string
  bullets: string[]
}

const activeSection = ref('overview')

const docEntries: DocEntry[] = [
  {
    title: '部署与运行',
    path: 'docs/deployment_zh-CN.md',
    audience: '运维、平台工程师',
    summary: '说明启动方式、配置基线、运行模式和集群部署建议。',
  },
  {
    title: '接入与集成',
    path: 'docs/integration_zh-CN.md',
    audience: '业务开发者',
    summary: '说明 Go SDK、HTTP、gRPC 以及兼容接入的使用方式。',
  },
  {
    title: '系统架构',
    path: 'docs/architecture_zh-CN.md',
    audience: '架构师、核心开发者',
    summary: '说明系统定位、分层模型、协议分工和关键数据流。',
  },
]

const capabilities: Capability[] = [
  {
    title: '统一控制面',
    summary: '注册、发现、认证、设置、告警和通知在同一运行时中收口。',
  },
  {
    title: '双一致性模式',
    summary: '同一系统支持 standalone、cluster + ap、cluster + cp 三类运行形态。',
  },
  {
    title: '单一 Go SDK 出口',
    summary: '对外统一推荐 pkg/sdk，避免多套公共 API 长期并存。',
  },
  {
    title: '多协议与兼容接入',
    summary: '保留 HTTP、gRPC、QUIC 以及 Consul / Nacos 兼容路径。',
  },
]

const integrationPaths: IntegrationPath[] = [
  {
    title: 'Go SDK',
    badge: '推荐',
    summary: '新 Go 项目优先使用 pkg/sdk。统一处理注册、心跳、发现、订阅和下线。',
    bullets: [
      '同一套业务代码可切换 grpc、quic 或 http 传输。',
      'DiscoveryMode=auto 适合通过统一入口完成成员发现。',
      '只有在明确需要直连端口时再使用 static 模式。',
    ],
  },
  {
    title: 'HTTP API',
    badge: '通用',
    summary: '适合脚本、非 Go 服务或仅需要基础注册发现能力的场景。',
    bullets: [
      '注册、心跳、发现和实例状态控制都可通过公开 HTTP 端点完成。',
      '订阅能力会退化为轮询，不适合作为强实时主路径。',
    ],
  },
  {
    title: 'gRPC',
    badge: '强类型',
    summary: '适合多语言强类型客户端，或需要显式 Watch 流式订阅的场景。',
    bullets: [
      '协议定义位于 api/proto/registry/v1/registry.proto。',
      'Go SDK 默认优先选择 gRPC 作为主数据面。',
    ],
  },
  {
    title: 'Consul / Nacos 兼容',
    badge: '迁移',
    summary: '适合存量系统平滑迁移，目标是尽量少改业务侧调用模型。',
    bullets: [
      '兼容层是过渡路径，不是新项目的推荐起点。',
      '如果没有历史包袱，优先直接接入 pkg/sdk。',
    ],
  },
]

const serverStart: CodeSample = {
  lang: 'bash',
  code: `go run ./cmd/server/main.go

go run ./cmd/server/main.go -config config/config.yaml.example`,
}

const consoleStart: CodeSample = {
  lang: 'bash',
  code: `cd web
npm install
npm run dev`,
}

const apConfig: CodeSample = {
  lang: 'yaml',
  code: `node_id: "node-1"
mode: "cluster"
consistency: "ap"
datacenter: "dc1"
data_dir: "./data"

server:
  http: ":8500"
  # picks the first free port in 9000-9999, starting at :9000
  grpc: "auto"
  quic: "off"
  raft: "off"`,
}

const cpConfig: CodeSample = {
  lang: 'yaml',
  code: `node_id: "node-1"
mode: "cluster"
consistency: "cp"
bootstrap: true
datacenter: "dc1"
data_dir: "./data/node-1"

server:
  http: ":8500"
  grpc: ":9000"
  quic: "off"
  raft: "127.0.0.1:7000"`,
}

const sdkSample: CodeSample = {
  lang: 'go',
  code: `client, err := sdk.NewWithConfig(&sdk.Config{
    Addresses:     []string{"127.0.0.1:9000"},
    Namespace:     "default",
    Datacenter:    "dc1",
    Transport:     "grpc",
    DiscoveryMode: "auto",
})
if err != nil {
    panic(err)
}
defer client.Close()

instance := &sdk.ServiceInstance{
    ID:          "order-center-1",
    ServiceName: "order-center",
    Host:        "127.0.0.1",
    Port:        9003,
    Weight:      100,
}

if err := client.Register(instance); err != nil {
    panic(err)
}`,
}

const httpSample: CodeSample = {
  lang: 'bash',
  code: `curl -X POST http://127.0.0.1:8500/v1/catalog/register \
  -H "Content-Type: application/json" \
  -d '{
    "id": "user-center-1",
    "service_name": "user-center",
    "namespace": "default",
    "host": "127.0.0.1",
    "port": 9001,
    "weight": 100,
    "datacenter": "dc1"
  }'

curl "http://127.0.0.1:8500/v1/catalog/service/user-center?passing=true&namespace=default"`,
}

const grpcSample: CodeSample = {
  lang: 'bash',
  code: `grpcurl -plaintext \
  -H "x-consumer-service: order-center" \
  -import-path ./api/proto \
  -proto registry/v1/registry.proto \
  -d '{
    "namespace": "default",
    "service_name": "user-center",
    "healthy_only": true,
    "datacenter": "dc1"
  }' \
  127.0.0.1:9000 \
  eden.registry.v1.RegistryService/Discover`,
}
</script>

<template>
  <div class="docs-page">
    <div class="hero-card">
      <div class="hero-kicker">Documentation</div>
      <h1>Focalors</h1>
      <p class="hero-summary">
        芙卡洛斯是一个服务注册与发现控制平面。它统一提供注册、发现、订阅、拓扑、认证、控制台和
        AP / CP 两种一致性模式，并对 Go 应用统一推荐 <code>pkg/sdk</code> 作为接入入口。
      </p>
      <div class="hero-tags">
        <span>standalone</span>
        <span>cluster + ap</span>
        <span>cluster + cp</span>
        <span>pkg/sdk</span>
      </div>
    </div>

    <el-tabs v-model="activeSection" class="docs-tabs">
      <el-tab-pane name="overview" label="概览">
        <div class="pane-stack">
          <div class="section-card">
            <div class="section-title-row">
              <h2>阅读路径</h2>
            </div>
            <div class="doc-grid">
              <div v-for="entry in docEntries" :key="entry.path" class="doc-card">
                <div class="doc-card-head">
                  <h3>{{ entry.title }}</h3>
                  <span>{{ entry.audience }}</span>
                </div>
                <div class="doc-path">{{ entry.path }}</div>
                <p>{{ entry.summary }}</p>
              </div>
            </div>
          </div>

          <div class="section-card">
            <div class="section-title-row">
              <h2>系统能力</h2>
            </div>
            <div class="capability-grid">
              <div v-for="item in capabilities" :key="item.title" class="capability-card">
                <h3>{{ item.title }}</h3>
                <p>{{ item.summary }}</p>
              </div>
            </div>
          </div>

          <div class="section-card">
            <div class="section-title-row">
              <h2>默认建议</h2>
            </div>
            <ul class="doc-list">
              <li>本地开发优先使用 <code>standalone + ap</code>。</li>
              <li>新 Go 项目优先使用 <code>pkg/sdk + grpc</code>。</li>
              <li>兼容层适合迁移，不适合作为新项目主路径。</li>
              <li>如果没有明确一致性要求，优先选 AP 模式而不是直接进入 CP 模式。</li>
            </ul>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane name="deployment" label="部署与运行">
        <div class="pane-stack">
          <div class="section-card">
            <div class="section-title-row">
              <h2>启动命令</h2>
            </div>
            <div class="code-card">
              <div class="code-label">bash</div>
              <pre><code>{{ serverStart.code }}</code></pre>
            </div>
            <div class="code-card">
              <div class="code-label">bash</div>
              <pre><code>{{ consoleStart.code }}</code></pre>
            </div>
            <ul class="doc-list">
              <li>默认后端地址为 <code>http://127.0.0.1:8500</code>。</li>
              <li>默认前端开发地址为 <code>http://127.0.0.1:2019</code>。</li>
              <li>默认控制台账户为 <code>admin / admin</code>。</li>
            </ul>
          </div>

          <div class="section-grid two-col">
            <div class="section-card">
              <div class="section-title-row">
                <h2>AP 集群配置</h2>
              </div>
              <div class="code-card">
                <div class="code-label">yaml</div>
                <pre><code>{{ apConfig.code }}</code></pre>
              </div>
            </div>

            <div class="section-card">
              <div class="section-title-row">
                <h2>CP 集群配置</h2>
              </div>
              <div class="code-card">
                <div class="code-label">yaml</div>
                <pre><code>{{ cpConfig.code }}</code></pre>
              </div>
            </div>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane name="integration" label="接入与集成">
        <div class="pane-stack">
          <div class="section-card">
            <div class="section-title-row">
              <h2>接入策略</h2>
            </div>
            <p class="section-lead">
              对外只推荐一个 Go SDK 出口：<code>pkg/sdk</code>。HTTP、gRPC 和兼容层保留为补充路径。
            </p>
            <div class="integration-grid">
              <div v-for="path in integrationPaths" :key="path.title" class="integration-card">
                <div class="integration-head">
                  <h3>{{ path.title }}</h3>
                  <span>{{ path.badge }}</span>
                </div>
                <p>{{ path.summary }}</p>
                <ul class="doc-list compact">
                  <li v-for="bullet in path.bullets" :key="bullet">{{ bullet }}</li>
                </ul>
              </div>
            </div>
          </div>

          <div class="section-card">
            <div class="section-title-row">
              <h2>Go SDK 示例</h2>
            </div>
            <div class="code-card">
              <div class="code-label">go</div>
              <pre><code>{{ sdkSample.code }}</code></pre>
            </div>
          </div>

          <div class="section-grid two-col">
            <div class="section-card">
              <div class="section-title-row">
                <h2>HTTP API 示例</h2>
              </div>
              <div class="code-card">
                <div class="code-label">bash</div>
                <pre><code>{{ httpSample.code }}</code></pre>
              </div>
            </div>

            <div class="section-card">
              <div class="section-title-row">
                <h2>gRPC 示例</h2>
              </div>
              <div class="code-card">
                <div class="code-label">bash</div>
                <pre><code>{{ grpcSample.code }}</code></pre>
              </div>
            </div>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane name="architecture" label="架构">
        <div class="pane-stack">
          <div class="section-card">
            <div class="section-title-row">
              <h2>系统边界</h2>
            </div>
            <ul class="doc-list">
              <li>数据面负责注册、发现、心跳、订阅和拓扑上报。</li>
              <li>控制面负责认证、权限、设置、告警、通知和控制台接口。</li>
              <li>集群层负责 standalone、AP 复制和 CP 共识三类运行模式。</li>
            </ul>
          </div>

          <div class="section-card">
            <div class="section-title-row">
              <h2>协议分工</h2>
            </div>
            <div class="doc-grid protocol-grid">
              <div class="doc-card">
                <div class="doc-card-head">
                  <h3>HTTP</h3>
                  <span>管理面</span>
                </div>
                <p>用于控制台 API、通用接入和脚本化操作。</p>
              </div>
              <div class="doc-card">
                <div class="doc-card-head">
                  <h3>gRPC</h3>
                  <span>主数据面</span>
                </div>
                <p>用于 Go SDK 默认通信、多语言强类型接入和 Watch 流式订阅。</p>
              </div>
              <div class="doc-card">
                <div class="doc-card-head">
                  <h3>QUIC</h3>
                  <span>弱网补充</span>
                </div>
                <p>不是独立业务协议，而是 gRPC 的传输补充。</p>
              </div>
              <div class="doc-card">
                <div class="doc-card-head">
                  <h3>Raft</h3>
                  <span>CP 模式</span>
                </div>
                <p>只服务于 CP 模式下的共识、选主和日志复制。</p>
              </div>
            </div>
          </div>

          <div class="section-card">
            <div class="section-title-row">
              <h2>代码分区</h2>
            </div>
            <ul class="doc-list">
              <li><code>cmd/server</code>：进程入口与运行时装配。</li>
              <li><code>internal/catalog</code>：注册、发现、实例和拓扑核心领域。</li>
              <li><code>internal/cluster</code>：AP / CP 集群实现。</li>
              <li><code>internal/transport</code>：HTTP、gRPC、QUIC 出口。</li>
              <li><code>pkg/sdk</code>：对外唯一推荐的 Go SDK。</li>
            </ul>
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<style scoped>
.docs-page {
  display: grid;
  gap: 20px;
}

.hero-card,
.section-card,
.doc-card,
.capability-card,
.integration-card {
  border: 1px solid var(--el-border-color-light);
  border-radius: 20px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(247, 249, 252, 0.96));
  box-shadow: 0 16px 40px rgba(18, 38, 63, 0.08);
}

.hero-card {
  padding: 28px;
}

.hero-kicker {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: #4b6cb7;
  margin-bottom: 8px;
}

.hero-card h1 {
  margin: 0;
  font-size: 36px;
  line-height: 1.1;
  color: #172033;
}

.hero-summary {
  max-width: 880px;
  margin: 16px 0 0;
  font-size: 15px;
  line-height: 1.8;
  color: #43506a;
}

.hero-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 18px;
}

.hero-tags span,
.doc-path,
.integration-head span {
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  background: rgba(75, 108, 183, 0.12);
  color: #355091;
  font-size: 12px;
  font-weight: 700;
  padding: 6px 10px;
}

.docs-tabs :deep(.el-tabs__header) {
  margin-bottom: 16px;
}

.pane-stack,
.section-grid {
  display: grid;
  gap: 18px;
}

.two-col {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.section-card {
  padding: 22px;
}

.section-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.section-title-row h2 {
  margin: 0;
  font-size: 20px;
  color: #172033;
}

.section-lead {
  margin: 0;
  color: #49556f;
  line-height: 1.75;
}

.doc-grid,
.capability-grid,
.integration-grid {
  display: grid;
  gap: 16px;
}

.doc-grid,
.capability-grid {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.integration-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.doc-card,
.capability-card,
.integration-card {
  padding: 18px;
}

.doc-card-head,
.integration-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}

.doc-card h3,
.capability-card h3,
.integration-card h3 {
  margin: 0;
  font-size: 17px;
  color: #1e2a44;
}

.doc-card p,
.capability-card p,
.integration-card p {
  margin: 0;
  color: #51607d;
  line-height: 1.7;
}

.code-card {
  overflow: hidden;
  border-radius: 16px;
  background: #182033;
  margin-top: 12px;
}

.code-label {
  padding: 10px 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  color: #9fb7ff;
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.code-card pre {
  margin: 0;
  padding: 16px 18px;
  overflow-x: auto;
}

.code-card code {
  color: #edf3ff;
  font-size: 13px;
  line-height: 1.7;
  white-space: pre;
  font-family: 'Cascadia Code', 'SFMono-Regular', Consolas, monospace;
}

.doc-list {
  margin: 0;
  padding-left: 18px;
  color: #44516b;
  line-height: 1.8;
}

.doc-list.compact {
  margin-top: 10px;
}

.protocol-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

code {
  padding: 2px 6px;
  border-radius: 8px;
  background: rgba(75, 108, 183, 0.1);
  color: #2f4f92;
  font-family: 'Cascadia Code', 'SFMono-Regular', Consolas, monospace;
  font-size: 0.95em;
}

@media (max-width: 1024px) {
  .doc-grid,
  .capability-grid,
  .integration-grid,
  .two-col,
  .protocol-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .hero-card,
  .section-card,
  .doc-card,
  .capability-card,
  .integration-card {
    border-radius: 16px;
  }

  .hero-card {
    padding: 22px;
  }

  .hero-card h1 {
    font-size: 30px;
  }
}
</style>




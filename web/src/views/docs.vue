<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from '../utils/i18n'

const { t } = useI18n()
const activeSection = ref('intro')

const sections = [
  { id: 'intro', title: 'intro' },
  { id: 'quickStart', title: 'quickStart' },
  { id: 'integration', title: 'integration' },
  { id: 'api', title: 'apiDesc' }
]
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
            <h2>{{ t.docs.intro }}</h2>
            <p>Eden Registry 是一个轻量级、高性能的服务注册与发现中心。它支持 CP 和 AP 两种一致性模式，旨在提供极致的运行效率和最低的资源占用。</p>
            <div class="feature-grid">
              <div class="feature-item">
                <div class="feature-title">双引擎支持</div>
                <div class="feature-desc">内置 Raft 和 Gossip 协议，兼顾一致性与可用性。</div>
              </div>
              <div class="feature-item">
                <div class="feature-title">资源极致</div>
                <div class="feature-desc">纯 Go 编写，内存占用低于 100MB，启动秒级。</div>
              </div>
              <div class="feature-item">
                <div class="feature-title">安全加固</div>
                <div class="feature-desc">全量 API 支持 Key 认证，控制台 RBAC 权限精细化。</div>
              </div>
            </div>
          </div>

          <div v-else-if="s.id === 'quickStart'">
            <h2>{{ t.docs.quickStart }}</h2>
            <h3>快速启动服务端</h3>
            <div class="command-container">
              <pre class="code-block">./server -bootstrap -mode=cp</pre>
              <p class="code-hint">使用 -bootstrap 参数启动集群首个节点。</p>
            </div>
            
            <h3>默认访问信息</h3>
            <div class="info-card">
              <p><strong>控制台地址:</strong> <code>http://localhost:8500</code></p>
              <p><strong>初始账号:</strong> <code>admin</code> / <code>admin123</code></p>
            </div>
          </div>

          <div v-else-if="s.id === 'integration'">
            <h2>{{ t.docs.integration }}</h2>
            <h3>Go 服务注册示例</h3>
            <pre class="code-block">
func register() {
    inst := map[string]interface{}{
        "service_name": "user-service",
        "host": "127.0.0.1",
        "port": 8081,
    }
    body, _ := json.Marshal(inst)
    
    req, _ := http.NewRequest("POST", "http://localhost:8500/v1/catalog/register", bytes.NewReader(body))
    req.Header.Set("X-API-Key", "your_api_key")
    
    resp, _ := http.DefaultClient.Do(req)
    // ...
}</pre>
          </div>

          <div v-else-if="s.id === 'api'">
            <h2>{{ t.docs.apiDesc }}</h2>
            <table class="docs-table">
              <thead>
                <tr>
                  <th>Endpoint</th>
                  <th>Method</th>
                  <th>Description</th>
                </tr>
              </thead>
              <tbody>
                <tr><td><code>/v1/catalog/register</code></td><td>POST</td><td>服务实例注册</td></tr>
                <tr><td><code>/v1/catalog/deregister</code></td><td>POST</td><td>实例主动下线</td></tr>
                <tr><td><code>/v1/catalog/heartbeat</code></td><td>POST</td><td>健康心跳上报</td></tr>
                <tr><td><code>/v1/catalog/services</code></td><td>GET</td><td>服务发现接口</td></tr>
              </tbody>
            </table>
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
  padding: 20px 0;
  max-width: 900px;
}

.docs-pane-content h2 {
  margin-top: 0;
  font-size: 24px;
  color: var(--text-primary);
  margin-bottom: 24px;
}

.feature-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 20px;
  margin-top: 24px;
}

.feature-item {
  padding: 20px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid var(--border-color);
  border-radius: 12px;
}

.feature-title {
  font-weight: 600;
  color: var(--accent-blue);
  margin-bottom: 8px;
}

.feature-desc {
  font-size: 14px;
  color: var(--text-secondary);
  line-height: 1.6;
}

.code-block {
  background: #0f172a;
  padding: 20px;
  border-radius: 12px;
  font-family: 'Fira Code', monospace;
  font-size: 14px;
  color: #e2e8f0;
  line-height: 1.6;
  border: 1px solid rgba(255, 255, 255, 0.1);
  overflow-x: auto;
}

.info-card {
  background: var(--bg-glass);
  padding: 16px;
  border-radius: 8px;
  border-left: 4px solid var(--accent-blue);
}

.docs-table {
  width: 100%;
  border-collapse: collapse;
}

.docs-table th, .docs-table td {
  padding: 16px;
  text-align: left;
  border-bottom: 1px solid var(--border-color);
}

.docs-table th {
  color: var(--text-secondary);
  font-weight: 600;
  background: rgba(255, 255, 255, 0.02);
}

code {
  color: var(--accent-blue);
  background: rgba(59, 130, 246, 0.1);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: monospace;
}
</style>


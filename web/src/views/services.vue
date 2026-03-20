<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Grid, List, RefreshRight, Search } from '@element-plus/icons-vue'
import {
  getNamespaces,
  getServices,
  getSubscribers,
  getTopology,
  type Namespace,
  type ServiceSummary,
  type TopologyGraph,
  type TopologyNode,
} from '../api/registry'
import TopologyGraphCanvas from '../components/topology-graph.vue'
import { useI18n } from '../utils/i18n'

type PanelMode = 'catalog' | 'topology'
type CatalogMode = 'grid' | 'list'
type HealthFilter = '' | 'healthy' | 'degraded'

const { locale } = useI18n()
const route = useRoute()
const router = useRouter()

const isZh = computed(() => locale.value === 'zh')
const text = (zh: string, en: string) => (isZh.value ? zh : en)

const namespaces = ref<Namespace[]>([])
const currentNamespace = ref((route.query.namespace as string) || 'default')
const activePanel = ref<PanelMode>((route.query.view as PanelMode) || 'catalog')
const catalogMode = ref<CatalogMode>('list')
const healthFilter = ref<HealthFilter>('')
const search = ref('')
const loading = ref(false)
const currentPage = ref(1)
const pageSize = ref(12)

const services = ref<ServiceSummary[]>([])
const topology = ref<TopologyGraph>({ namespace: 'default', nodes: [], edges: [] })
const subscribersMap = ref<Record<string, string[]>>({})
const selectedTopologyNode = ref('')

let refreshTimer: ReturnType<typeof setInterval> | null = null

const topologyNodeMap = computed<Record<string, TopologyNode>>(() =>
  Object.fromEntries(topology.value.nodes.map((node) => [node.id, node])),
)

const topologyMeta = computed(() => {
  const upstream: Record<string, string[]> = {}
  const downstream: Record<string, string[]> = {}
  for (const edge of topology.value.edges) {
    if (!upstream[edge.source]) upstream[edge.source] = []
    if (!downstream[edge.target]) downstream[edge.target] = []
    upstream[edge.source]!.push(edge.target)
    downstream[edge.target]!.push(edge.source)
  }
  Object.values(upstream).forEach((items) => items.sort())
  Object.values(downstream).forEach((items) => items.sort())
  return { upstream, downstream }
})

const filteredServices = computed(() => {
  const query = search.value.trim().toLowerCase()
  return services.value.filter((service) => {
    const topologyNode = topologyNodeMap.value[service.name]
    const instances = topologyNode?.instances || []
    const matchesSearch =
      !query ||
      service.name.toLowerCase().includes(query) ||
      instances.some((instance) => `${instance.host}:${instance.port}`.toLowerCase().includes(query))
    const matchesHealth =
      !healthFilter.value ||
      (healthFilter.value === 'healthy'
        ? service.instance_count > 0 && service.healthy_count === service.instance_count
        : service.healthy_count < service.instance_count)
    return matchesSearch && matchesHealth
  })
})

const totalPages = computed(() => Math.max(1, Math.ceil(filteredServices.value.length / pageSize.value)))

const pagedServices = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredServices.value.slice(start, start + pageSize.value)
})

const selectedNodeDetail = computed<TopologyNode | null>(() => topologyNodeMap.value[selectedTopologyNode.value] || null)
const selectedUpstream = computed(() => topologyMeta.value.upstream[selectedTopologyNode.value] || [])
const selectedDownstream = computed(() => topologyMeta.value.downstream[selectedTopologyNode.value] || [])

function serviceHealthTone(service: ServiceSummary) {
  if (service.instance_count === 0) return '#94a3b8'
  if (service.healthy_count === service.instance_count) return '#16a34a'
  if (service.healthy_count > 0) return '#d97706'
  return '#dc2626'
}

function serviceHealthText(service: ServiceSummary) {
  if (service.instance_count === 0) return text('无实例', 'No instance')
  if (service.healthy_count === service.instance_count) return text('健康', 'Healthy')
  if (service.healthy_count > 0) return text('部分异常', 'Partial')
  return text('严重异常', 'Critical')
}

function previewSubscriberCount(serviceName: string) {
  return subscribersMap.value[serviceName]?.length || 0
}

function previewAddress(serviceName: string) {
  const instances = topologyNodeMap.value[serviceName]?.instances || []
  if (instances.length === 0) return '-'
  const [first] = instances
  if (!first) return '-'
  return `${first.host}:${first.port}`
}

function openService(serviceName: string) {
  router.push({
    path: `/services/${encodeURIComponent(serviceName)}`,
    query: { namespace: currentNamespace.value },
  })
}

function openTopology(serviceName: string) {
  selectedTopologyNode.value = serviceName
  activePanel.value = 'topology'
}

function selectTopologyNode(nodeId: string) {
  selectedTopologyNode.value = nodeId
}

async function fetchNamespaces() {
  const response = await getNamespaces()
  namespaces.value = response.data || []
  if (!namespaces.value.find((item) => item.name === currentNamespace.value) && namespaces.value.length > 0) {
    currentNamespace.value = namespaces.value[0]!.name
  }
}

async function fetchWorkspace() {
  loading.value = true
  try {
    const [servicesRes, topologyRes] = await Promise.all([
      getServices(currentNamespace.value),
      getTopology(currentNamespace.value),
    ])
    services.value = servicesRes.data || []
    topology.value = topologyRes.data || { namespace: currentNamespace.value, nodes: [], edges: [] }

    const subscribersEntries = await Promise.all(
      services.value.map(async (service) => {
        try {
          const response = await getSubscribers(service.name, currentNamespace.value)
          return [service.name, response.data || []] as const
        } catch {
          return [service.name, []] as const
        }
      }),
    )
    subscribersMap.value = Object.fromEntries(subscribersEntries)

    if (!selectedTopologyNode.value || !topologyNodeMap.value[selectedTopologyNode.value]) {
      selectedTopologyNode.value = topology.value.nodes[0]?.id || ''
    }
  } finally {
    loading.value = false
  }
}

function restartRefreshLoop() {
  if (refreshTimer) clearInterval(refreshTimer)
  refreshTimer = setInterval(() => {
    fetchWorkspace().catch((error) => console.error('refresh workspace failed', error))
  }, 5000)
}

watch([currentNamespace, activePanel], async () => {
  currentPage.value = 1
  await router.replace({
    query: {
      ...route.query,
      namespace: currentNamespace.value,
      view: activePanel.value,
    },
  })
  await fetchWorkspace()
})

watch([search, healthFilter], () => {
  currentPage.value = 1
})

watch(pageSize, () => {
  currentPage.value = 1
})

watch(filteredServices, () => {
  if (currentPage.value > totalPages.value) currentPage.value = totalPages.value
})

onMounted(async () => {
  await fetchNamespaces()
  await fetchWorkspace()
  restartRefreshLoop()
})

onBeforeUnmount(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<template>
  <div class="services-page-shell">
    <section class="toolbar-row">
      <div class="toolbar-left">
        <el-select v-model="currentNamespace" size="large" class="toolbar-namespace">
          <el-option
            v-for="namespace in namespaces"
            :key="namespace.name"
            :label="namespace.name"
            :value="namespace.name"
          />
        </el-select>

        <el-input
          v-model="search"
          size="large"
          clearable
          :prefix-icon="Search"
          :placeholder="text('搜索服务或实例 IP:Port', 'Search by service or instance IP:Port')"
          class="toolbar-search"
        />

        <el-select v-model="healthFilter" size="large" clearable class="toolbar-health" :placeholder="text('健康状态', 'Health')">
          <el-option :label="text('全部', 'All')" value="" />
          <el-option :label="text('健康', 'Healthy')" value="healthy" />
          <el-option :label="text('异常', 'Degraded')" value="degraded" />
        </el-select>
      </div>

      <div class="toolbar-right">
        <div class="view-switch">
          <button type="button" :class="{ active: activePanel === 'catalog' }" @click="activePanel = 'catalog'">
            {{ text('服务列表', 'Service List') }}
          </button>
          <button type="button" :class="{ active: activePanel === 'topology' }" @click="activePanel = 'topology'">
            {{ text('拓扑图', 'Topology') }}
          </button>
        </div>

        <div v-if="activePanel === 'catalog'" class="mode-switch">
          <button type="button" :class="{ active: catalogMode === 'list' }" :title="text('列表视图', 'List View')" @click="catalogMode = 'list'">
            <el-icon><List /></el-icon>
          </button>
          <button type="button" :class="{ active: catalogMode === 'grid' }" :title="text('卡片视图', 'Card View')" @click="catalogMode = 'grid'">
            <el-icon><Grid /></el-icon>
          </button>
        </div>

        <el-button type="primary" :icon="RefreshRight" class="refresh-button" @click="fetchWorkspace">
          {{ text('刷新', 'Refresh') }}
        </el-button>
      </div>
    </section>

    <section v-if="activePanel === 'catalog'" class="content-stage" v-loading="loading">
      <div v-if="filteredServices.length === 0 && !loading" class="empty-stage">
        <strong>{{ text('当前命名空间暂无服务', 'No services in current namespace') }}</strong>
        <p>{{ text('切换命名空间或等待实例注册后，这里会自动更新。', 'Switch namespace or wait for services to register.') }}</p>
      </div>

      <template v-else>
        <div class="content-scroll">
          <div v-if="catalogMode === 'grid'" class="card-grid">
            <article v-for="service in pagedServices" :key="service.name" class="service-card">
              <div class="card-head">
                <div class="service-title">
                  <strong @click="openService(service.name)">{{ service.name }}</strong>
                  <span>{{ currentNamespace }}</span>
                </div>
                <span class="status-dot" :style="{ backgroundColor: serviceHealthTone(service) }"></span>
              </div>

              <div class="service-address">{{ previewAddress(service.name) }}</div>

              <div class="card-meta">
                <div>
                  <span>{{ text('实例', 'Instances') }}</span>
                  <strong>{{ service.instance_count }}</strong>
                </div>
                <div>
                  <span>{{ text('健康', 'Healthy') }}</span>
                  <strong :style="{ color: serviceHealthTone(service) }">{{ service.healthy_count }}</strong>
                </div>
                <div>
                  <span>{{ text('订阅方', 'Subscribers') }}</span>
                  <strong>{{ previewSubscriberCount(service.name) }}</strong>
                </div>
              </div>

              <div class="card-actions">
                <span class="health-text" :style="{ color: serviceHealthTone(service) }">{{ serviceHealthText(service) }}</span>
                <div class="inline-actions">
                  <button type="button" @click="openTopology(service.name)">{{ text('拓扑', 'Topology') }}</button>
                  <button type="button" @click="openService(service.name)">{{ text('详情', 'Detail') }}</button>
                </div>
              </div>
            </article>
          </div>

          <div v-else class="table-wrap">
            <el-table :data="pagedServices" height="100%" style="width: 100%">
              <el-table-column :label="text('服务', 'Service')" min-width="260">
                <template #default="{ row }">
                  <div class="service-cell" @click="openService(row.name)">
                    <strong>{{ row.name }}</strong>
                    <span>{{ currentNamespace }}</span>
                  </div>
                </template>
              </el-table-column>

              <el-table-column :label="text('主实例地址', 'Primary Instance')" min-width="180">
                <template #default="{ row }">
                  <span class="table-address">{{ previewAddress(row.name) }}</span>
                </template>
              </el-table-column>

              <el-table-column prop="instance_count" :label="text('实例数', 'Instances')" width="90" />

              <el-table-column :label="text('健康数', 'Healthy')" width="90">
                <template #default="{ row }">
                  <span :style="{ color: serviceHealthTone(row), fontWeight: 700 }">{{ row.healthy_count }}</span>
                </template>
              </el-table-column>

              <el-table-column :label="text('订阅方', 'Subscribers')" width="100">
                <template #default="{ row }">{{ previewSubscriberCount(row.name) }}</template>
              </el-table-column>

              <el-table-column :label="text('状态', 'Status')" width="110">
                <template #default="{ row }">
                  <span :style="{ color: serviceHealthTone(row), fontWeight: 600 }">{{ serviceHealthText(row) }}</span>
                </template>
              </el-table-column>

              <el-table-column :label="text('操作', 'Actions')" width="150" fixed="right">
                <template #default="{ row }">
                  <div class="inline-actions compact">
                    <button type="button" @click="openTopology(row.name)">{{ text('拓扑', 'Topology') }}</button>
                    <button type="button" @click="openService(row.name)">{{ text('详情', 'Detail') }}</button>
                  </div>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </div>

        <footer class="stage-footer">
          <div class="footer-info">
            <span>{{ filteredServices.length }} {{ text('条记录', 'records') }}</span>
            <span>{{ text('第', 'Page ') }}{{ currentPage }} / {{ totalPages }}{{ isZh ? ' 页' : '' }}</span>
          </div>

          <el-pagination
            v-model:current-page="currentPage"
            v-model:page-size="pageSize"
            :page-sizes="[12, 20, 50]"
            :total="filteredServices.length"
            layout="total, sizes, prev, pager, next"
            background
          />
        </footer>
      </template>
    </section>

    <section v-else class="topology-stage" v-loading="loading">
      <div class="topology-canvas-shell">
        <TopologyGraphCanvas
          :graph="topology"
          :loading="loading"
          :selected-node="selectedTopologyNode"
          @select="selectTopologyNode"
        />
      </div>

      <aside class="topology-side">
        <div class="side-head">
          <span>{{ text('当前服务', 'Selected Service') }}</span>
          <strong>{{ selectedNodeDetail?.name || text('未选择', 'None') }}</strong>
        </div>

        <div v-if="selectedNodeDetail" class="side-scroll">
          <div class="side-summary">
            <div>
              <span>{{ text('命名空间', 'Namespace') }}</span>
              <strong>{{ selectedNodeDetail.namespace }}</strong>
            </div>
            <div>
              <span>{{ text('实例数', 'Instances') }}</span>
              <strong>{{ selectedNodeDetail.instance_count }}</strong>
            </div>
            <div>
              <span>{{ text('健康实例', 'Healthy') }}</span>
              <strong>{{ selectedNodeDetail.healthy_count }}</strong>
            </div>
          </div>

          <section class="side-section">
            <header>
              <strong>{{ text('实例', 'Instances') }}</strong>
              <span>{{ selectedNodeDetail.instances.length }}</span>
            </header>

            <div v-if="selectedNodeDetail.instances.length" class="instance-list">
              <article v-for="instance in selectedNodeDetail.instances" :key="instance.id" class="instance-item">
                <div class="instance-text">
                  <strong>{{ instance.host }}:{{ instance.port }}</strong>
                  <span>{{ instance.id }}</span>
                </div>
                <em :style="{ color: instance.status === 'passing' ? '#16a34a' : '#dc2626' }">
                  {{ instance.status === 'passing' ? text('健康', 'Healthy') : text('异常', 'Critical') }}
                </em>
              </article>
            </div>

            <p v-else class="empty-line">{{ text('当前没有实例', 'No live instances') }}</p>
          </section>

          <section class="side-section">
            <header>
              <strong>{{ text('上游依赖', 'Upstream') }}</strong>
              <span>{{ selectedUpstream.length }}</span>
            </header>
            <div v-if="selectedUpstream.length" class="tag-list">
              <span v-for="serviceName in selectedUpstream" :key="serviceName">{{ serviceName }}</span>
            </div>
            <p v-else class="empty-line">{{ text('暂无上游依赖', 'No upstream services') }}</p>
          </section>

          <section class="side-section">
            <header>
              <strong>{{ text('下游服务', 'Downstream') }}</strong>
              <span>{{ selectedDownstream.length }}</span>
            </header>
            <div v-if="selectedDownstream.length" class="tag-list">
              <span v-for="serviceName in selectedDownstream" :key="serviceName">{{ serviceName }}</span>
            </div>
            <p v-else class="empty-line">{{ text('暂无下游服务', 'No downstream services') }}</p>
          </section>

          <el-button class="detail-button" @click="openService(selectedNodeDetail.name)">
            {{ text('查看详情', 'Open detail') }}
          </el-button>
        </div>

        <div v-else class="empty-stage side-empty">
          <strong>{{ text('未选择服务', 'No service selected') }}</strong>
          <p>{{ text('点击拓扑图节点查看详情。', 'Select a node in topology graph.') }}</p>
        </div>
      </aside>
    </section>
  </div>
</template>

<style scoped>
.services-page-shell {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: calc(100vh - var(--header-height) - 48px);
  min-height: 0;
  overflow: hidden;
  background: #fff;
}

.toolbar-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 0 0;
  flex-shrink: 0;
}

.toolbar-left,
.toolbar-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.toolbar-left {
  flex: 1;
  min-width: 0;
}

.toolbar-right {
  flex-shrink: 0;
}

.toolbar-namespace {
  width: 180px;
}

.toolbar-search {
  flex: 1;
}

.toolbar-health {
  width: 160px;
}

:deep(.toolbar-namespace .el-select__wrapper),
:deep(.toolbar-health .el-select__wrapper),
:deep(.toolbar-search .el-input__wrapper) {
  min-height: 42px;
  border-radius: 0;
  background: #fff !important;
  border: 0 !important;
  box-shadow: inset 0 -1px 0 rgba(15, 23, 42, 0.12) !important;
}

.view-switch,
.mode-switch {
  display: inline-flex;
  align-items: center;
  gap: 0;
  background: #fff;
}

.view-switch button,
.mode-switch button,
.inline-actions button {
  border: 0;
  background: transparent;
  color: #64748b;
  cursor: pointer;
}

.view-switch button {
  min-width: 96px;
  height: 40px;
  padding: 0 14px;
  font-weight: 700;
}

.mode-switch button {
  width: 36px;
  height: 36px;
}

.view-switch button.active,
.mode-switch button.active {
  color: #2563eb;
}

.refresh-button {
  min-height: 40px;
  padding: 0 14px;
  border-radius: 0;
}

.content-stage {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  background: #fff;
}

.content-scroll {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.table-wrap,
.card-grid {
  height: 100%;
  min-height: 0;
}

.card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 12px;
  padding: 8px 0;
  overflow: auto;
}

.service-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  background: #fff;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.06);
}

.card-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.service-title {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.service-title strong {
  color: #0f172a;
  font-size: 16px;
  font-weight: 800;
  cursor: pointer;
}

.service-title span {
  color: #94a3b8;
  font-size: 12px;
}

.status-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.service-address,
.table-address {
  color: #2563eb;
  font-family: 'JetBrains Mono', 'Fira Code', 'Menlo', monospace;
  font-size: 12px;
}

.card-meta {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.card-meta div {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.card-meta span {
  color: #94a3b8;
  font-size: 12px;
}

.card-meta strong {
  color: #0f172a;
  font-size: 18px;
  font-weight: 700;
}

.card-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.health-text {
  font-weight: 700;
}

.inline-actions {
  display: inline-flex;
  align-items: center;
  gap: 12px;
}

.inline-actions.compact {
  gap: 10px;
}

.inline-actions button {
  padding: 0;
  color: #2563eb;
  font-size: 13px;
  font-weight: 600;
}

.table-wrap {
  overflow: hidden;
}

:deep(.table-wrap .el-table) {
  height: 100%;
  --el-table-bg-color: transparent;
  --el-table-tr-bg-color: transparent;
  --el-table-row-hover-bg-color: rgba(37, 99, 235, 0.04);
  --el-table-border-color: rgba(15, 23, 42, 0.08);
}

:deep(.table-wrap .el-table__inner-wrapper::before) {
  display: none;
}

:deep(.table-wrap .el-table th.el-table__cell) {
  background: #fff;
  color: #64748b;
  font-size: 12px;
  font-weight: 700;
  border-bottom: 1px solid rgba(15, 23, 42, 0.08);
}

:deep(.table-wrap .el-table td.el-table__cell) {
  padding-top: 10px;
  padding-bottom: 10px;
  border-bottom: 1px solid rgba(15, 23, 42, 0.06);
}

.service-cell {
  display: flex;
  align-items: baseline;
  gap: 8px;
  cursor: pointer;
}

.service-cell strong {
  color: #0f172a;
  font-size: 15px;
  font-weight: 700;
}

.service-cell span {
  color: #94a3b8;
  font-size: 12px;
}

.stage-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 0 0;
  flex-shrink: 0;
}

.footer-info {
  display: flex;
  align-items: center;
  gap: 12px;
  color: #64748b;
  font-size: 13px;
}

.empty-stage {
  display: flex;
  flex: 1;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 0;
  color: #64748b;
}

.empty-stage strong {
  color: #0f172a;
  font-size: 24px;
  font-weight: 800;
}

.topology-stage {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: 12px;
  flex: 1;
  min-height: 0;
}

.topology-canvas-shell,
.topology-side {
  background: #fff;
  min-height: 0;
  overflow: hidden;
}

.topology-canvas-shell :deep(.topology-board) {
  height: 100%;
  min-height: 0;
  background: #fff;
  border: 0;
  border-radius: 0;
}

.topology-canvas-shell :deep(.graph-canvas) {
  height: 100% !important;
  min-height: 0;
}

.topology-canvas-shell :deep(.board-empty) {
  height: 100% !important;
  min-height: 0;
}

.topology-canvas-shell :deep(.board-toolbar) {
  top: 12px;
  left: 12px;
  box-shadow: none;
}

.topology-canvas-shell :deep(.board-toolbar button) {
  background: #fff;
  border: 1px solid rgba(15, 23, 42, 0.08);
}

.topology-side {
  display: flex;
  flex-direction: column;
}

.side-head {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px 0 12px;
  border-bottom: 1px solid rgba(15, 23, 42, 0.08);
}

.side-head span {
  color: #94a3b8;
  font-size: 12px;
}

.side-head strong {
  color: #0f172a;
  font-size: 28px;
  font-weight: 800;
}

.side-scroll {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 0;
  overflow: auto;
  padding-top: 12px;
}

.side-summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.side-summary div {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.side-summary span,
.side-section header span,
.empty-line {
  color: #94a3b8;
  font-size: 12px;
}

.side-summary strong {
  color: #0f172a;
  font-size: 18px;
  font-weight: 700;
}

.side-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.side-section header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.side-section header strong {
  color: #0f172a;
  font-size: 14px;
}

.instance-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.instance-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.instance-text {
  min-width: 0;
}

.instance-text strong {
  display: block;
  color: #0f172a;
  font-size: 13px;
}

.instance-text span {
  display: block;
  margin-top: 4px;
  color: #94a3b8;
  font-size: 11px;
  word-break: break-all;
}

.instance-item em {
  font-style: normal;
  font-size: 12px;
  font-weight: 700;
}

.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.tag-list span {
  color: #64748b;
  font-size: 12px;
}

.detail-button {
  align-self: flex-start;
  min-height: 40px;
  border-radius: 0;
}

@media (max-width: 1180px) {
  .toolbar-row {
    flex-direction: column;
    align-items: stretch;
  }

  .toolbar-left,
  .toolbar-right {
    width: 100%;
  }

  .toolbar-right {
    justify-content: space-between;
  }

  .topology-stage {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 900px) {
  .toolbar-left,
  .stage-footer {
    flex-direction: column;
    align-items: stretch;
  }

  .toolbar-namespace,
  .toolbar-health,
  .toolbar-search,
  .view-switch,
  .mode-switch {
    width: 100%;
  }

  .view-switch button,
  .mode-switch button {
    flex: 1;
  }

  .card-grid,
  .side-summary {
    grid-template-columns: 1fr;
  }
}
</style>

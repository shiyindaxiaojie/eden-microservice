<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Bottom, Grid, List, RefreshLeft, Search, Share, Top } from '@element-plus/icons-vue'
import {
  getNamespaces,
  getServices,
  getSubscribers,
  getTopology,
  type Namespace,
  type ServiceSummary,
  type TopologyGraph,
  type TopologyInstance,
  type TopologyNode,
} from '../api/registry'
import TopologyGraphCanvas from '../components/topology-graph.vue'
import { useI18n } from '../utils/i18n'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import en from 'element-plus/es/locale/lang/en'

type PanelMode = 'catalog' | 'topology'
type CatalogMode = 'grid' | 'list'
type HealthFilter = '' | 'healthy' | 'degraded'
type ServiceHealthState = 'empty' | 'healthy' | 'partial' | 'critical'

interface ServiceEntry extends ServiceSummary {
  instances: TopologyInstance[]
  primaryAddress: string
  subscribers: number
  upstreamCount: number
  downstreamCount: number
  healthState: ServiceHealthState
  healthText: string
  healthTone: string
  healthSurface: string
  healthBorder: string
  healthRatio: number
  namespace: string
}

const autoRefreshSeconds = 15

const { locale } = useI18n()
const elLocale = computed(() => (locale.value === 'zh' ? zhCn : en))
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
const searchIP = ref('')
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

function serviceHealthState(service: Pick<ServiceSummary, 'instance_count' | 'healthy_count'>): ServiceHealthState {
  if (service.instance_count === 0) return 'empty'
  if (service.healthy_count === service.instance_count) return 'healthy'
  if (service.healthy_count > 0) return 'partial'
  return 'critical'
}

function serviceHealthTheme(state: ServiceHealthState) {
  switch (state) {
    case 'healthy':
      return {
        tone: '#10b981',
        surface: 'rgba(16, 185, 129, 0.08)',
        border: 'rgba(16, 185, 129, 0.18)',
      }
    case 'partial':
      return {
        tone: '#f59e0b',
        surface: 'rgba(245, 158, 11, 0.08)',
        border: 'rgba(245, 158, 11, 0.18)',
      }
    case 'critical':
      return {
        tone: '#ef4444',
        surface: 'rgba(239, 68, 68, 0.08)',
        border: 'rgba(239, 68, 68, 0.18)',
      }
    default:
      return {
        tone: '#64748b',
        surface: 'rgba(100, 116, 139, 0.08)',
        border: 'rgba(100, 116, 139, 0.18)',
      }
  }
}

function serviceHealthTextByState(state: ServiceHealthState) {
  switch (state) {
    case 'healthy':
      return text('健康', 'Healthy')
    case 'partial':
      return text('部分异常', 'Partial')
    case 'critical':
      return text('不可用', 'Critical')
    default:
      return text('暂无实例', 'No instance')
  }
}


function normalizeServicesPayload(payload: unknown, graph: TopologyGraph): ServiceSummary[] {
  const summaries = new Map<string, ServiceSummary>()

  for (const node of graph.nodes || []) {
    summaries.set(node.name, {
      name: node.name,
      instance_count: node.instance_count || 0,
      healthy_count: node.healthy_count || 0,
    })
  }

  if (Array.isArray(payload)) {
    for (const item of payload) {
      if (!item || typeof item !== 'object') continue

      const row = item as Partial<ServiceSummary>
      const name = typeof row.name === 'string' ? row.name : ''
      if (!name) continue

      const existing = summaries.get(name)
      summaries.set(name, {
        name,
        instance_count: typeof row.instance_count === 'number' ? row.instance_count : existing?.instance_count || 0,
        healthy_count: typeof row.healthy_count === 'number' ? row.healthy_count : existing?.healthy_count || 0,
        subscriber_count: typeof row.subscriber_count === 'number' ? row.subscriber_count : existing?.subscriber_count,
      })
    }
  } else if (payload && typeof payload === 'object') {
    for (const name of Object.keys(payload as Record<string, unknown>)) {
      if (!name) continue
      const existing = summaries.get(name)
      summaries.set(name, {
        name,
        instance_count: existing?.instance_count || 0,
        healthy_count: existing?.healthy_count || 0,
        subscriber_count: existing?.subscriber_count,
      })
    }
  }

  return [...summaries.values()].sort((left, right) => left.name.localeCompare(right.name))
}
const serviceEntries = computed<ServiceEntry[]>(() =>
  services.value.map((service) => {
    const topologyNode = topologyNodeMap.value[service.name]
    const instances = topologyNode?.instances || []
    const state = serviceHealthState(service)
    const theme = serviceHealthTheme(state)
    const primary = instances[0]

    return {
      ...service,
      instances,
      primaryAddress: primary ? `${primary.host}:${primary.port}${instances.length > 1 ? ` (+${instances.length - 1})` : ''}` : text('暂无实例', 'No instance'),
      subscribers: subscribersMap.value[service.name]?.length || 0,
      upstreamCount: topologyMeta.value.upstream[service.name]?.length || 0,
      downstreamCount: topologyMeta.value.downstream[service.name]?.length || 0,
      healthState: state,
      healthText: serviceHealthTextByState(state),
      healthTone: theme.tone,
      healthSurface: theme.surface,
      healthBorder: theme.border,
      healthRatio: service.instance_count ? Math.round((service.healthy_count / service.instance_count) * 100) : 0,
      namespace: topologyNode?.namespace || currentNamespace.value,
    }
  }),
)

const filteredServices = computed(() => {
  const query = search.value.trim().toLowerCase()
  const ipQuery = searchIP.value.trim().toLowerCase()

  return serviceEntries.value.filter((service) => {
    const matchesSearch = !query || service.name.toLowerCase().includes(query)
    const matchesIP = !ipQuery || service.instances.some((instance) => `${instance.host}:${instance.port}`.toLowerCase().includes(ipQuery))

    const matchesHealth =
      !healthFilter.value ||
      (healthFilter.value === 'healthy' ? service.healthState === 'healthy' : service.healthState !== 'healthy')

    return matchesSearch && matchesIP && matchesHealth
  })
})

const totalPages = computed(() => Math.max(1, Math.ceil(filteredServices.value.length / pageSize.value)))

const pagedServices = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredServices.value.slice(start, start + pageSize.value)
})

const totalServices = computed(() => serviceEntries.value.length)
// Unused total instances lines removed
const healthyServiceCount = computed(() => serviceEntries.value.filter((service) => service.healthState === 'healthy').length)
const degradedServiceCount = computed(() => serviceEntries.value.filter((service) => service.healthState !== 'healthy').length)

// overallHealthRate removed

const selectedNodeDetail = computed<TopologyNode | null>(() => topologyNodeMap.value[selectedTopologyNode.value] || null)
const serviceEntryMap = computed(() => {
  const map: Record<string, (typeof serviceEntries.value)[0]> = {}
  serviceEntries.value.forEach((s) => {
    map[s.name] = s
  })
  return map
})
const selectedServiceEntry = computed(() =>
  selectedNodeDetail.value ? serviceEntryMap.value[selectedNodeDetail.value.name] || null : null,
)
const selectedUpstream = computed(() => topologyMeta.value.upstream[selectedTopologyNode.value] || [])
const selectedDownstream = computed(() => topologyMeta.value.downstream[selectedTopologyNode.value] || [])

// selectedNodeHealth removed

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

function resetFilters() {
  currentNamespace.value = 'default'
  search.value = ''
  searchIP.value = ''
  healthFilter.value = ''
  fetchWorkspace(true)
}

async function fetchWorkspace(showLoading = true) {
  if (showLoading) {
    loading.value = true
  }

  try {
    const [servicesRes, topologyRes] = await Promise.all([
      getServices(currentNamespace.value),
      getTopology(currentNamespace.value),
    ])

    topology.value = topologyRes.data || { namespace: currentNamespace.value, nodes: [], edges: [] }
    services.value = normalizeServicesPayload(servicesRes.data, topology.value)

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
    if (showLoading) {
      loading.value = false
    }
  }
}

function restartRefreshLoop() {
  if (refreshTimer) clearInterval(refreshTimer)

  refreshTimer = setInterval(() => {
    fetchWorkspace(false).catch((error) => console.error('refresh workspace failed', error))
  }, autoRefreshSeconds * 1000)
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

watch([search, searchIP, healthFilter], () => {
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
  <div class="svc-shell">
    <div class="svc-main glass-card">
      <!-- ─── Toolbar ─── -->
      <div class="svc-toolbar" data-guide="services-toolbar">
        <!-- Row 1: Filters & Controls -->
        <div class="toolbar-row">
          <!-- Left: Namespace + Search -->
          <div class="toolbar-group">
            <div class="field-item">
              <span class="field-label">{{ text('命名空间', 'Namespace') }}</span>
              <el-select v-model="currentNamespace" size="default" class="ns-select">
                <el-option
                  v-for="namespace in namespaces"
                  :key="namespace.name"
                  :label="namespace.name"
                  :value="namespace.name"
                />
              </el-select>
            </div>
          </div>

          <div class="toolbar-group">
            <div class="field-item">
              <span class="field-label">{{ text('服务名称', 'Service Name') }}</span>
              <el-input
                v-model="search"
                size="default"
                clearable
                :prefix-icon="Search"
                :placeholder="text('输入服务名', 'Service')"
                class="search-input"
              />
            </div>
          </div>

          <div class="toolbar-group">
            <div class="field-item">
              <span class="field-label">{{ text('IP地址', 'IP Address') }}</span>
              <el-input
                v-model="searchIP"
                size="default"
                clearable
                :prefix-icon="Search"
                :placeholder="text('输入 IP', 'IP')"
                class="search-input"
              />
            </div>
          </div>

          <div class="toolbar-group">
            <button
              type="button"
              class="refresh-btn"
              :title="text('重置', 'Reset Filters')"
              @click="resetFilters()"
            >
              <el-icon><RefreshLeft /></el-icon>
            </button>
          </div>

          <!-- Right: Filters + View switch -->
          <div class="toolbar-group right-align">
            <div class="pill-group">
              <button
                type="button"
                :class="{ active: healthFilter === '' }"
                @click="healthFilter = ''"
              >
                {{ text('全部', 'All') }}
                <span class="pill-count">{{ totalServices }}</span>
              </button>
              <button
                type="button"
                :class="{ active: healthFilter === 'healthy' }"
                @click="healthFilter = 'healthy'"
              >
                {{ text('健康', 'OK') }}
                <span class="pill-count">{{ healthyServiceCount }}</span>
              </button>
              <button
                type="button"
                :class="{ active: healthFilter === 'degraded' }"
                @click="healthFilter = 'degraded'"
              >
                {{ text('异常', 'Err') }}
                <span class="pill-count" :class="{ 'err-count': degradedServiceCount > 0 }">{{ degradedServiceCount }}</span>
              </button>
            </div>

            <div class="toolbar-sep" />

            <div class="pill-group icon-pills">
              <button
                type="button"
                :class="{ active: activePanel === 'catalog' && catalogMode === 'list' }"
                :title="text('列表', 'List View')"
                @click="activePanel = 'catalog'; catalogMode = 'list'"
              >
                <el-icon><List /></el-icon>
              </button>

              <button
                type="button"
                :class="{ active: activePanel === 'catalog' && catalogMode === 'grid' }"
                :title="text('卡片', 'Card View')"
                @click="activePanel = 'catalog'; catalogMode = 'grid'"
              >
                <el-icon><Grid /></el-icon>
              </button>

              <button
                type="button"
                data-guide="services-topology-switch"
                :class="{ active: activePanel === 'topology' }"
                :title="text('拓扑', 'Topology View')"
                @click="activePanel = 'topology'"
              >
                <el-icon><Share /></el-icon>
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- ─── Catalog Panel ─── -->
      <section v-if="activePanel === 'catalog'" class="svc-content" v-loading="loading">
        <div v-if="filteredServices.length === 0 && !loading" class="empty-panel">
          <div class="empty-inner">
            <strong>{{ text('当前筛选下没有匹配的服务', 'No services matched') }}</strong>
            <p>{{ text('试试切换命名空间或清空搜索条件。', 'Try another namespace or clear filters.') }}</p>
          </div>
        </div>

        <template v-else>
          <!-- Card View -->
          <div v-if="catalogMode === 'grid'" class="card-grid">
            <article
              v-for="service in pagedServices"
              :key="service.name"
              class="svc-card"
              :style="{ '--svc-tone': service.healthTone, '--svc-surface': service.healthSurface, '--svc-border': service.healthBorder }"
              @click="openService(service.name)"
            >
              <div class="card-head">
                <strong class="card-name">{{ service.name }}</strong>
                <div class="status-wrap">
                  <span class="status-label" :style="{ color: service.healthTone }">{{ service.healthText }}</span>
                  <span class="health-dot" :style="{ background: service.healthTone, boxShadow: `0 0 6px ${service.healthTone}` }"></span>
                </div>
              </div>

              <div class="card-body">
                <div class="inst-matrix">
                  <el-tooltip
                    v-for="inst in service.instances"
                    :key="inst.id"
                    :content="`${inst.host}:${inst.port} (${inst.status === 'passing' ? (isZh ? '健康' : 'Healthy') : (isZh ? '异常' : 'Critical')})`"
                    placement="top"
                  >
                    <div
                      class="inst-dot"
                      :class="inst.status"
                    ></div>
                  </el-tooltip>
                  <div v-if="service.instances.length === 0" class="empty-text">
                    {{ text('暂无实例', 'No instances') }}
                  </div>
                </div>

                <div class="health-info">
                  <div class="health-bar-container">
                    <div class="health-bar-fill" :style="{ width: `${service.healthRatio}%`, background: service.healthTone }"></div>
                  </div>
                  <span class="health-val">{{ service.healthy_count }}/{{ service.instance_count }}</span>
                </div>
              </div>

              <div class="card-footer">
                <div class="metrics">
                  <el-tooltip :content="text('实例数量', 'Instances')">
                    <div class="metric-item">
                      <el-icon><Monitor /></el-icon>
                      <span>{{ service.instance_count }}</span>
                    </div>
                  </el-tooltip>
                  <div class="metric-sep"></div>
                  <el-tooltip :content="text('依赖服务', 'Dependencies')">
                    <div class="metric-item">
                      <el-icon><Top /></el-icon>
                      <span>{{ service.upstreamCount }}</span>
                    </div>
                  </el-tooltip>
                  <el-tooltip :content="text('被依赖服务', 'Dependents')">
                    <div class="metric-item">
                      <el-icon><Bottom /></el-icon>
                      <span>{{ service.downstreamCount }}</span>
                    </div>
                  </el-tooltip>
                </div>
                <button type="button" class="topo-btn" @click.stop="openTopology(service.name)">
                  <el-icon><Share /></el-icon>
                </button>
              </div>



            </article>
          </div>

          <!-- List View -->
          <div v-if="catalogMode === 'list'" class="table-wrap">
            <el-table :data="pagedServices" height="100%" style="width: 100%; font-size: 14px;">
              <el-table-column type="index" :label="text('序号', 'No.')" width="60" align="center" />
              <el-table-column :label="text('服务', 'Service')" min-width="150" prop="name" />
              <el-table-column :label="text('命名空间', 'Namespace')" min-width="80" prop="namespace" />

              <el-table-column :label="text('地址', 'Address')" min-width="110">
                <template #default="{ row }">
                  <code class="cell-addr" style="font-family: inherit; font-size: 14px;">{{ row.instances[0] ? `${row.instances[0].host}:${row.instances[0].port}` : '-' }}</code>
                  <span v-if="row.instances.length > 1" style="font-size: 12px; color: var(--text-muted); margin-left: 4px;">+{{ row.instances.length - 1 }}</span>
                </template>
              </el-table-column>

              <el-table-column :label="text('状态', 'Status')" min-width="80">
                <template #default="{ row }">
                  <div class="cell-health">
                    <span class="health-badge" :style="{ color: row.healthTone, background: row.healthSurface, borderColor: row.healthBorder, fontSize: '13px' }">
                      {{ row.healthText }}
                    </span>
                    <b style="font-size: 14px;">{{ row.healthy_count }}/{{ row.instance_count }}</b>
                  </div>
                </template>
              </el-table-column>

              <el-table-column :label="text('实例', 'Inst')" width="80" align="center">
                <template #default="{ row }">{{ row.instance_count }}</template>
              </el-table-column>

              <el-table-column :label="text('依赖服务', 'Dependencies')" width="140" align="center">
                <template #default="{ row }">
                  <el-popover placement="bottom" :width="200" trigger="hover" :disabled="!row.upstreamCount">
                    <template #reference>
                      <span :style="{ fontWeight: 600, cursor: row.upstreamCount ? 'pointer' : 'default', color: row.upstreamCount ? 'var(--accent-blue)' : 'var(--text-muted)', fontSize: '14px' }">
                        {{ row.upstreamCount }}
                      </span>
                    </template>
                    <div style="font-size: 13px;">
                      <div v-for="sn in topologyMeta?.upstream[row.name] || []" :key="sn" @click="openTopology(sn)" style="cursor: pointer; color: var(--accent-blue); padding: 4px 0; border-bottom: 1px dashed var(--border-color);">{{ sn }}</div>
                    </div>
                  </el-popover>
                </template>
              </el-table-column>

              <el-table-column :label="text('被依赖服务', 'Dependents')" width="130" align="center">
                <template #default="{ row }">
                  <el-popover placement="bottom" :width="200" trigger="hover" :disabled="!row.downstreamCount">
                    <template #reference>
                      <span :style="{ fontWeight: 600, cursor: row.downstreamCount ? 'pointer' : 'default', color: row.downstreamCount ? 'var(--accent-blue)' : 'var(--text-muted)', fontSize: '14px' }">
                        {{ row.downstreamCount }}
                      </span>
                    </template>
                    <div style="font-size: 13px;">
                      <div v-for="sn in topologyMeta?.downstream[row.name] || []" :key="sn" @click="openTopology(sn)" style="cursor: pointer; color: var(--accent-blue); padding: 4px 0; border-bottom: 1px dashed var(--border-color);">{{ sn }}</div>
                    </div>
                  </el-popover>
                </template>
              </el-table-column>

              <el-table-column :label="text('操作', 'Actions')" width="180" fixed="right">
                <template #default="{ row }">
                  <div class="cell-actions" style="display: flex; gap: 8px;">
                    <el-button link :icon="Share" @click="openTopology(row.name)">{{ text('拓扑', 'Topo') }}</el-button>
                    <el-button link :icon="Grid" @click="openService(row.name)">{{ text('详情', 'Detail') }}</el-button>
                  </div>
                </template>
              </el-table-column>
            </el-table>
          </div>

          <footer class="svc-footer">
            <span class="footer-info">{{ filteredServices.length }} {{ text('条', 'records') }}</span>
            <el-config-provider :locale="elLocale">
              <el-pagination
                v-model:current-page="currentPage"
                v-model:page-size="pageSize"
                :page-sizes="[12, 20, 50]"
                :total="filteredServices.length"
                layout="sizes, prev, pager, next"
                background
              />
            </el-config-provider>
          </footer>
        </template>
      </section>

      <!-- ─── Topology Panel ─── -->
      <section v-else class="topo-stage">
        <div class="topo-canvas" v-loading="loading" data-guide="services-topology-canvas">
          <TopologyGraphCanvas
            :graph="topology"
            :loading="loading"
            :selected-node="selectedTopologyNode"
            @select="selectTopologyNode"
          />
        </div>

        <aside class="topo-side">
          <div v-if="selectedNodeDetail" class="side-inner side-card">
            <div class="side-header">
              <div class="side-kicker-row">
                <span class="side-meta">{{ text('当前服务', 'Current Service') }}</span>
                <span
                  v-if="selectedServiceEntry"
                  class="side-health-badge"
                  :style="{ color: selectedServiceEntry.healthTone, background: selectedServiceEntry.healthSurface, borderColor: selectedServiceEntry.healthBorder }"
                >
                  {{ selectedServiceEntry.healthText }}
                </span>
              </div>
              <div class="side-header-row">
                <h3 class="side-title">
                  {{ selectedNodeDetail.name }}
                  <el-tag size="small" type="info" class="ns-tag">{{ selectedNodeDetail.namespace }}</el-tag>
                </h3>
                <a class="detail-link" @click="openService(selectedNodeDetail.name)">
                  <el-icon><Monitor /></el-icon> {{ text('查看详情', 'View detail') }}
                </a>
              </div>
              <p class="side-subtitle">
                {{ selectedServiceEntry?.primaryAddress || text('暂无实例', 'No instance') }}
              </p>
              <div class="side-summary-grid">
                <div class="summary-pill">
                  <span class="summary-label">{{ text('实例', 'Instances') }}</span>
                  <strong>{{ selectedNodeDetail.instances.length }}</strong>
                </div>
                <div class="summary-pill">
                  <span class="summary-label">{{ text('依赖服务', 'Dependencies') }}</span>
                  <strong>{{ selectedUpstream.length }}</strong>
                </div>
                <div class="summary-pill">
                  <span class="summary-label">{{ text('被依赖服务', 'Dependents') }}</span>
                  <strong>{{ selectedDownstream.length }}</strong>
                </div>
                <div class="summary-pill">
                  <span class="summary-label">{{ text('健康率', 'Health') }}</span>
                  <strong>{{ selectedServiceEntry ? `${selectedServiceEntry.healthy_count}/${selectedServiceEntry.instance_count}` : '--' }}</strong>
                </div>
              </div>
            </div>

            <div class="side-scroll side-body">
              <div class="side-group">
                <div class="group-head">
                  <div class="group-title">
                    {{ text('实例列表', 'Instances') }}
                    <span class="count">{{ selectedNodeDetail.instances.length }}</span>
                  </div>
                  <span class="group-hint">{{ text('实时状态', 'Live status') }}</span>
                </div>
                <div v-if="selectedNodeDetail.instances.length" class="inst-list">
                  <div v-for="instance in selectedNodeDetail.instances" :key="instance.id" class="simple-inst">
                    <div class="inst-copy">
                      <span class="inst-addr">{{ instance.host }}:{{ instance.port }}</span>
                      <span class="inst-id">{{ instance.id }}</span>
                    </div>
                    <span class="inst-state" :class="{ ok: instance.status === 'passing', err: instance.status !== 'passing' }">
                      {{ instance.status === 'passing' ? text('健康', 'OK') : text('异常', 'Err') }}
                    </span>
                  </div>
                </div>
                <div v-else class="empty-text">{{ text('无可用实例', 'No instances') }}</div>
              </div>

              <div class="side-group">
                <div class="group-head">
                  <div class="group-title">
                    {{ text('依赖服务', 'Dependencies') }}
                    <span class="count">{{ selectedUpstream.length }}</span>
                  </div>
                  <span class="group-hint">{{ text('点击切换', 'Click to inspect') }}</span>
                </div>
                <div v-if="selectedUpstream.length" class="dep-list">
                  <div
                    v-for="sn in selectedUpstream"
                    :key="sn"
                    @click="selectTopologyNode(sn)"
                    class="dep-item"
                  >
                    <div class="dep-copy">
                      <span class="dep-name">{{ sn }}</span>
                      <span class="dep-caption">{{ text('依赖服务', 'Dependency service') }}</span>
                    </div>
                    <div v-if="serviceEntryMap[sn]" class="dep-info">
                      <span
                        class="dep-status"
                        :style="{ color: serviceEntryMap[sn].healthTone, background: serviceEntryMap[sn].healthSurface }"
                      >
                        {{ serviceEntryMap[sn].healthy_count }}/{{ serviceEntryMap[sn].instance_count }}
                      </span>
                      <span class="dep-status-text" :style="{ color: serviceEntryMap[sn].healthTone }">{{ serviceEntryMap[sn].healthText }}</span>
                    </div>
                  </div>
                </div>
                <div v-else class="empty-text">{{ text('无', 'None') }}</div>
              </div>

              <div class="side-group">
                <div class="group-head">
                  <div class="group-title">
                    {{ text('被依赖服务', 'Dependents') }}
                    <span class="count">{{ selectedDownstream.length }}</span>
                  </div>
                  <span class="group-hint">{{ text('点击切换', 'Click to inspect') }}</span>
                </div>
                <div v-if="selectedDownstream.length" class="dep-list">
                  <div
                    v-for="sn in selectedDownstream"
                    :key="sn"
                    @click="selectTopologyNode(sn)"
                    class="dep-item"
                  >
                    <div class="dep-copy">
                      <span class="dep-name">{{ sn }}</span>
                      <span class="dep-caption">{{ text('依赖当前服务', 'Dependent service') }}</span>
                    </div>
                    <div v-if="serviceEntryMap[sn]" class="dep-info">
                      <span
                        class="dep-status"
                        :style="{ color: serviceEntryMap[sn].healthTone, background: serviceEntryMap[sn].healthSurface }"
                      >
                        {{ serviceEntryMap[sn].healthy_count }}/{{ serviceEntryMap[sn].instance_count }}
                      </span>
                      <span class="dep-status-text" :style="{ color: serviceEntryMap[sn].healthTone }">{{ serviceEntryMap[sn].healthText }}</span>
                    </div>
                  </div>
                </div>
                <div v-else class="empty-text">{{ text('无', 'None') }}</div>
              </div>
            </div>
          </div>

          <div v-else class="side-empty">
            {{ text('请在左侧拓扑图中选择一个节点查看详细信息。', 'Select a node from the topology graph.') }}
          </div>
        </aside>
      </section>
    </div>
  </div>
</template>

<style scoped>
/* ===== Shell ===== */
.svc-shell {
  display: flex;
  flex-direction: column;
  /* 100vh - header(64) - page-body top/bottom padding(24*2) */
  height: calc(100vh - var(--header-height) - 48px);
  min-height: 0;
  overflow: hidden;
  box-sizing: border-box;
}

.svc-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  background: var(--bg-card);
  backdrop-filter: blur(20px);
  border: 1px solid var(--border-color);
  border-radius: 0;
}

/* glass-card removed as a class and absorbed into .svc-main for clarity */

/* ===== Toolbar ===== */
.svc-toolbar {
  flex-shrink: 0;
  padding: 16px 24px;
  border-bottom: 1px solid var(--border-color);
}

.toolbar-row {
  display: flex;
  align-items: center;
  gap: 0;
}

.toolbar-group {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 16px;
}

.toolbar-group:first-child {
  padding-left: 0;
}

.toolbar-group:last-child {
  padding-right: 0;
}

.toolbar-group.right-align {
  margin-left: auto;
}

/* Vertical divider between groups */
.toolbar-divider {
  width: 1px;
  height: 28px;
  background: var(--border-color);
  flex-shrink: 0;
}

/* Smaller separator within a group */
.toolbar-sep {
  width: 1px;
  height: 20px;
  background: var(--border-color);
  flex-shrink: 0;
  margin: 0 2px;
}

/* -- Namespace & Search -- */
.ns-select {
  width: 140px;
  flex-shrink: 0;
}

.search-input {
  width: 160px;
  flex-shrink: 0;
}

:deep(.ns-select .el-select__wrapper),
:deep(.search-input .el-input__wrapper) {
  background: rgba(255, 255, 255, 0.04) !important;
  border: 1px solid var(--border-color) !important;
  box-shadow: none !important;
  border-radius: 6px;
  color: var(--text-primary);
  height: 32px;
}

:deep(.ns-select .el-select__wrapper:hover),
:deep(.search-input .el-input__wrapper:hover),
:deep(.ns-select .el-select__wrapper:focus-within),
:deep(.search-input .el-input__wrapper:focus-within) {
  border-color: rgba(59, 130, 246, 0.4) !important;
  background: rgba(255, 255, 255, 0.06) !important;
}

:deep(.ns-select .el-select__wrapper input),
:deep(.search-input .el-input__inner) {
  color: var(--text-primary) !important;
  font-size: 14px;
}

:deep(.search-input .el-input__prefix .el-icon) {
  color: var(--text-muted);
}

/* -- Stat chips -- */
.stats-group {
  gap: 12px;
}

.stat-chip {
  display: flex;
  align-items: baseline;
  gap: 4px;
  white-space: nowrap;
}

.stat-value {
  font-size: 14px;
  font-weight: 700;
  color: var(--text-primary);
  font-variant-numeric: tabular-nums;
}

.stat-label {
  font-size: 12px;
  color: var(--text-muted);
}

.stat-chip.warn .stat-value {
  color: var(--accent-orange);
}

/* -- Pill groups -- */
.pill-group {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 2px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.03);
}

.pill-group button {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--text-muted);
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;
  white-space: nowrap;
  font-family: inherit;
  font-size: 14px;
}

.pill-group button:hover {
  color: var(--text-secondary);
  background: rgba(255, 255, 255, 0.04);
}

.pill-count {
  font-weight: 600;
  opacity: 0.5;
  font-variant-numeric: tabular-nums;
  font-size: 14px;
}

.pill-group button.active {
  background: rgba(59, 130, 246, 0.12);
  color: var(--accent-blue);
  font-weight: 600;
}

.pill-group button.active .pill-count {
  opacity: 0.8;
}

.pill-sep {
  width: 1px;
  height: 14px;
  background: rgba(255, 255, 255, 0.08);
  margin: 0 4px;
  align-self: center;
}

.icon-pills button {
  padding: 4px 7px;
}

.refresh-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.15s ease;
  margin-left: -20px;
}

.refresh-btn:hover {
  color: var(--accent-blue);
  background: rgba(59, 130, 246, 0.08);
}

/* ===== Content area ===== */
.svc-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden; /* confined to table scroll */
  padding: 12px 24px 12px;
}

/* ===== Card grid ===== */
.card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 10px;
  flex: 1;
  min-height: 0;
  overflow: auto;
  align-content: start;
  padding: 2px;
}

.card-grid::-webkit-scrollbar { width: 4px; }
.card-grid::-webkit-scrollbar-track { background: transparent; }
.card-grid::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.08); border-radius: 2px; }

.svc-card {
  display: flex;
  flex-direction: column;
  padding: 16px;
  border-radius: 12px;
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  cursor: pointer;
  transition: all 0.2s ease-out;
  position: relative;
  overflow: hidden;
}

.svc-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  width: 4px;
  height: 100%;
  background: var(--svc-tone);
  opacity: 0.6;
}

.svc-card:hover {
  border-color: var(--svc-tone);
  background: var(--bg-card-hover, rgba(255, 255, 255, 0.05));
  box-shadow: inset 0 0 0 1px var(--svc-tone); /* Internal Highlight border */
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.card-name {
  font-size: 16px;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: -0.01em;
}

.status-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-label {
  font-size: 12px;
  font-weight: 600;
  opacity: 0.9;
}

.health-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.card-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 10px;
}

.inst-matrix {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  min-height: 32px;
  align-items: center;
}

.inst-dot {
  width: 14px;
  height: 14px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.1);
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  border: 1px solid rgba(255, 255, 255, 0.05);
}

.inst-dot:hover {
  transform: scale(1.25);
  z-index: 10;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
}

.inst-dot.passing {
  background: #10b981;
  box-shadow: 0 0 8px rgba(16, 185, 129, 0.2);
}

.inst-dot.critical {
  background: #ef4444;
  box-shadow: 0 0 8px rgba(239, 68, 68, 0.2);
}

.health-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.health-bar-container {
  flex: 1;
  height: 6px;
  background: var(--border-color);
  border-radius: 3px;
  overflow: hidden;
}

.health-bar-fill {
  height: 100%;
  border-radius: 3px;
  transition: width 0.6s ease;
}

.health-val {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}

.card-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: auto;
}

.metrics {
  display: flex;
  align-items: center;
  gap: 16px;
}

.metric-item {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--text-muted);
  transition: color 0.2s;
}

.metric-item .el-icon {
  font-size: 14px;
}

.metric-item span {
  font-size: 13px;
  font-weight: 500;
}

.metric-sep {
  width: 1px;
  height: 12px;
  background: var(--border-color);
}

.topo-btn {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 8px;
  background: var(--border-color);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s;
}

.topo-btn:hover {
  background: var(--accent-blue);
  color: #fff;
  transform: rotate(15deg);
}

/* ===== Table view ===== */
.table-wrap {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  background: transparent;
  display: flex;
  flex-direction: column;
}

:deep(.table-wrap .el-table) {
  height: 100%;
  --el-table-bg-color: transparent;
  --el-table-tr-bg-color: transparent;
  --el-table-row-hover-bg-color: rgba(255, 255, 255, 0.03);
  --el-table-border-color: rgba(255, 255, 255, 0.04);
  --el-table-header-bg-color: transparent;
  --el-table-header-text-color: var(--text-muted);
  --el-table-text-color: var(--text-primary);
}

:deep(.table-wrap .el-table__inner-wrapper::before) {
  display: none;
}

:deep(.table-wrap .el-table th.el-table__cell) {
  background: transparent;
  color: var(--text-muted);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

:deep(.table-wrap .el-table td.el-table__cell) {
  padding-top: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.03);
}

:deep(.table-wrap .el-table tr:hover > td.el-table__cell) {
  background: rgba(255, 255, 255, 0.02) !important;
}

:deep(.table-wrap .el-table__fixed-right) {
  box-shadow: none;
}

.cell-svc {
  display: flex;
  flex-direction: column;
  gap: 2px;
  cursor: pointer;
}

.cell-svc strong {
  color: var(--text-primary);
  font-weight: 600;
  transition: color 0.15s ease;
}

.cell-svc:hover strong {
  color: var(--accent-blue);
}

.cell-svc span {
  color: var(--text-muted);
  opacity: 0.5;
}

.cell-addr {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  color: var(--text-secondary);
  background: none;
  padding: 0;
  border: none;
}

.cell-health {
  display: flex;
  align-items: center;
  gap: 8px;
}

.health-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 7px;
  border-radius: 4px;
  border: none;
  font-weight: 600;
  font-style: normal;
  white-space: nowrap;
}

.health-bar-sm {
  flex: 1;
  height: 3px;
  min-width: 36px;
  border-radius: 3px;
  background: rgba(255, 255, 255, 0.06);
  overflow: hidden;
}

.health-bar-sm span {
  display: block;
  height: 100%;
  border-radius: 3px;
}

.cell-health b {
  color: var(--text-muted);
  white-space: nowrap;
  font-weight: 500;
}

.dep-cell {
  color: var(--text-muted);
}

.cell-actions {
  display: flex;
  gap: 4px;
}

.cell-actions button {
  padding: 4px 10px;
  border: 0;
  border-radius: 4px;
  background: rgba(59, 130, 246, 0.06);
  color: var(--accent-blue);
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;
  font-family: inherit;
}

.cell-actions button:hover {
  background: rgba(59, 130, 246, 0.14);
}

/* ===== Footer ===== */
.svc-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 0 0;
  flex-shrink: 0;
}

.footer-info {
  color: var(--text-muted);
  opacity: 0.8; /* slightly increased opacity for 14px readable contrast */
}

:deep(.svc-footer .el-pagination) {
  flex-wrap: wrap;
  justify-content: flex-end;
  --el-pagination-font-size: 14px;
}

:deep(.svc-footer .el-pagination *) {
  font-size: 14px !important;
}

/* ===== Empty ===== */
.empty-panel {
  flex: 1;
  display: grid;
  place-items: center;
  min-height: 200px;
}

.empty-inner {
  text-align: center;
}

.empty-inner strong {
  display: block;
  color: var(--text-secondary);
  font-weight: 600;
  margin-bottom: 6px;
}

.empty-inner p {
  color: var(--text-muted);
  opacity: 0.6;
}

/* ===== Topology Panel ===== */
.topo-stage {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 344px;
  gap: 16px;
  flex: 1;
  min-height: 0;
  padding: 16px 24px 16px;
}

.topo-canvas {
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

.topo-side {
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

.side-inner {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  gap: 0;
}

.side-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 0;
  padding-right: 4px;
}

.side-scroll::-webkit-scrollbar {
  width: 6px;
}

.side-scroll::-webkit-scrollbar-thumb {
  background: rgba(148, 163, 184, 0.28);
  border-radius: 999px;
}

.side-card {
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 14px;
  background: #ffffff;
  box-shadow: none;
}

.side-header {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 16px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.14);
}

.side-kicker-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.side-meta {
  color: var(--text-muted);
  font-size: 12px;
  letter-spacing: 0.02em;
}

.side-health-badge {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border: 1px solid;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 700;
  white-space: nowrap;
}

.side-header-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.side-title-wrap {
  min-width: 0;
}

.side-title {
  color: var(--text-primary);
  font-size: 16px;
  font-weight: 700;
  line-height: 1.2;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.ns-tag {
  font-weight: 500;
  font-size: 11px;
}

.side-subtitle {
  display: none;
}

.side-summary-grid {
  display: none;
}

.summary-pill {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px 14px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.04);
}

.summary-pill strong {
  color: var(--text-primary);
  font-size: 18px;
  line-height: 1.1;
  font-variant-numeric: tabular-nums;
}

.summary-label {
  color: var(--text-muted);
  font-size: 11px;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.svc-content {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding: 16px 24px 12px;
}

.card-grid {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
  padding: 2px 2px 20px;
}

/* Hide scrollbar for Chrome/Safari */
.card-grid::-webkit-scrollbar {
  width: 6px;
}
.card-grid::-webkit-scrollbar-thumb {
  background: rgba(255,255,255,0.05);
  border-radius: 10px;
}

.side-group {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 0;
  border-top: 1px solid rgba(148, 163, 184, 0.12);
}

.side-body {
  padding: 0 16px 16px;
}

.side-body .side-group:first-child {
  border-top: 0;
}

.side-body .side-group:last-child {
  padding-bottom: 0;
}

.group-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.group-title {
  color: var(--text-primary);
  font-size: 14px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 6px;
}

.group-hint {
  color: var(--text-muted);
  font-size: 11px;
  white-space: nowrap;
}

.count {
  background: rgba(59, 130, 246, 0.1);
  padding: 2px 6px;
  border-radius: 999px;
  font-size: 10px;
  color: var(--accent-blue);
}

.inst-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.simple-inst {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  background: #ffffff;
  border-radius: 12px;
  border: 1px solid rgba(148, 163, 184, 0.14);
}

.inst-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.inst-addr {
  font-family: inherit;
  font-size: 13px;
  color: var(--text-primary);
  font-weight: 600;
}

.inst-id {
  color: var(--text-muted);
  font-size: 11px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.inst-state {
  display: inline-flex;
  align-items: center;
  padding: 4px 9px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 700;
  white-space: nowrap;
  border: 1px solid transparent;
}

.inst-state.ok {
  color: #10b981;
  background: rgba(16, 185, 129, 0.12);
  border-color: rgba(16, 185, 129, 0.18);
}

.inst-state.err {
  color: #ef4444;
  background: rgba(239, 68, 68, 0.12);
  border-color: rgba(239, 68, 68, 0.18);
}

.clickable-dep {
  cursor: pointer;
  transition: opacity 0.2s;
}

.clickable-dep:hover {
  opacity: 0.7;
}

.dep-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.dep-item {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 13px;
  background: #ffffff;
  border-radius: 12px;
  cursor: pointer;
  transition: border-color 0.2s ease, background 0.2s ease;
  border: 1px solid rgba(148, 163, 184, 0.14);
}

.dep-item:hover {
  background: #f8fbff;
  border-color: rgba(59, 130, 246, 0.2);
}

.dep-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.dep-name {
  color: var(--text-primary);
  font-size: 13px;
  font-weight: 600;
}

.dep-caption {
  color: var(--text-muted);
  font-size: 11px;
}

.dep-info {
  display: flex;
  align-items: flex-end;
  flex-direction: column;
  gap: 8px;
  flex-shrink: 0;
}

.dep-status-text {
  font-size: 11px;
  font-weight: 500;
  opacity: 0.9;
}

.dep-status {
  font-size: 11px;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.88);
  font-variant-numeric: tabular-nums;
}

.empty-text {
  color: var(--text-muted);
  font-size: 12px;
  opacity: 0.85;
}

.side-group .empty-text,
.section-empty {
  padding: 16px 14px;
  border-radius: 12px;
  background: #ffffff;
  border: 1px dashed rgba(148, 163, 184, 0.18);
}

.detail-link {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--accent-blue);
  font-size: 12px;
  font-weight: 600;
  padding: 4px 0;
  border-radius: 0;
  background: transparent;
  border: 0;
  cursor: pointer;
  text-decoration: none;
  transition: opacity 0.2s;
}

.detail-link:hover {
  opacity: 0.9;
  text-decoration: none;
}

.side-empty {
  flex: 1;
  display: grid;
  place-items: center;
  padding: 28px 18px;
  text-align: center;
  color: var(--text-muted);
  font-size: 12px;
  border: 1px dashed rgba(148, 163, 184, 0.18);
  border-radius: 14px;
  background: #ffffff;
}

/* Field items for toolbar */
.field-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.field-label {
  font-size: 14px;
  color: var(--text-secondary);
  font-weight: 700;
  white-space: nowrap;
}

.err-count {
  color: #ef4444 !important;
  background: rgba(239, 68, 68, 0.1) !important;
}

/* ===== Responsive ===== */
@media (max-width: 1180px) {
  .toolbar-row {
    flex-wrap: wrap;
    gap: 8px;
  }

  .toolbar-divider {
    display: none;
  }

  .toolbar-group {
    padding: 0;
  }

  .toolbar-group:first-child {
    width: 100%;
  }

  .stats-group {
    width: auto;
  }

  .topo-stage {
    grid-template-columns: 1fr;
  }

  .side-summary-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .svc-toolbar {
    padding: 10px 16px;
  }

  .svc-content {
    padding: 12px 16px 8px;
  }

  .topo-stage {
    padding: 12px 16px;
  }

  .side-summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .toolbar-group {
    flex-wrap: wrap;
  }

  .ns-select,
  .search-input {
    width: 100% !important;
  }

  .pill-group {
    width: 100%;
  }

  .pill-group button {
    flex: 1;
    justify-content: center;
  }

  .card-grid {
    grid-template-columns: 1fr;
  }

  .svc-footer {
    flex-direction: column;
    align-items: stretch;
  }

  .side-stats {
    grid-template-columns: 1fr;
  }
}
</style>

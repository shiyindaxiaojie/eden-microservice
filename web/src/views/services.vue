<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  getDependencyGraph,
  getNamespaces,
  getServiceInstances,
  getServices,
  getSubscribers,
  type Instance,
  type Namespace,
  type ServiceSummary,
} from '../api/registry'
import { useI18n } from '../utils/i18n'
import { CircleCheck, Grid, List, Monitor, Search, Share, User } from '@element-plus/icons-vue'

interface DependencyGraphResponse {
  namespace: string
  nodes: Array<{ id: string; name: string }>
  edges: Array<{ source: string; target: string }>
}

interface ServiceDependencyMeta {
  providers: string[]
  consumers: string[]
}

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const services = ref<ServiceSummary[]>([])
const namespaces = ref<Namespace[]>([])
const currentNamespace = ref((route.query.namespace as string) || 'default')
const instancesByService = ref<Record<string, Instance[]>>({})
const dependencyGraph = ref<DependencyGraphResponse>({ namespace: 'default', nodes: [], edges: [] })
const dependencyMap = ref<Record<string, ServiceDependencyMeta>>({})
const search = ref('')
const healthFilter = ref<'healthy' | 'degraded' | ''>('')
const loading = ref(true)
const viewMode = ref<'grid' | 'list'>('list')

const subDialogVisible = ref(false)
const subLoading = ref(false)
const currentService = ref('')
const subscribersList = ref<string[]>([])

const dependencyDrawerVisible = ref(false)
const dependencyService = ref('')

const filtered = computed(() => {
  const query = search.value.trim().toLowerCase()
  return services.value.filter((service) => {
    const instances = instancesByService.value[service.name] || []
    const deps = dependencyMap.value[service.name] || { providers: [], consumers: [] }
    const matchesSearch =
      !query ||
      service.name.toLowerCase().includes(query) ||
      instances.some((instance) => `${instance.host}:${instance.port}`.toLowerCase().includes(query)) ||
      deps.providers.some((name) => name.toLowerCase().includes(query)) ||
      deps.consumers.some((name) => name.toLowerCase().includes(query))

    const matchesHealth =
      !healthFilter.value ||
      (healthFilter.value === 'healthy'
        ? service.healthy_count === service.instance_count
        : service.healthy_count < service.instance_count)

    return matchesSearch && matchesHealth
  })
})

const stats = computed(() => {
  const dependencyCount = dependencyGraph.value.edges.length
  const relatedCount = services.value.filter((service) => {
    const meta = dependencyMap.value[service.name]
    return meta && (meta.providers.length > 0 || meta.consumers.length > 0)
  }).length

  return {
    serviceCount: services.value.length,
    dependencyCount,
    relatedCount,
  }
})

const currentDependencyMeta = computed<ServiceDependencyMeta>(() => {
  return dependencyMap.value[dependencyService.value] || { providers: [], consumers: [] }
})

function buildDependencyMap(serviceNames: string[], graph: DependencyGraphResponse) {
  const serviceSet = new Set(serviceNames)
  for (const name of graph.nodes.map((node) => node.id)) {
    serviceSet.add(name)
  }

  const map: Record<string, ServiceDependencyMeta> = {}
  for (const name of serviceSet) {
    map[name] = { providers: [], consumers: [] }
  }

  for (const edge of graph.edges) {
    const consumer = edge.source
    const provider = edge.target

    if (!map[consumer]) map[consumer] = { providers: [], consumers: [] }
    if (!map[provider]) map[provider] = { providers: [], consumers: [] }

    if (!map[consumer]!.providers.includes(provider)) {
      map[consumer]!.providers.push(provider)
    }
    if (!map[provider]!.consumers.includes(consumer)) {
      map[provider]!.consumers.push(consumer)
    }
  }

  for (const name of Object.keys(map)) {
    map[name]!.providers.sort()
    map[name]!.consumers.sort()
  }

  dependencyMap.value = map
}

function previewNames(names: string[]) {
  if (names.length === 0) return t.value.common.none
  return names.slice(0, 2).join(' / ')
}

function dependencyTotal(name: string) {
  const meta = dependencyMap.value[name]
  if (!meta) return 0
  return meta.providers.length + meta.consumers.length
}

function healthRate(service: ServiceSummary) {
  if (service.instance_count === 0) return 0
  return (service.healthy_count / service.instance_count) * 100
}

function barColor(rate: number) {
  if (rate >= 80) return 'var(--accent-green)'
  if (rate >= 50) return 'var(--accent-orange)'
  return 'var(--accent-red)'
}

function dependencyBadgeType(count: number) {
  if (count === 0) return 'info'
  if (count < 3) return 'success'
  return 'warning'
}

function openService(name: string) {
  router.push({
    path: `/services/${encodeURIComponent(name)}`,
    query: { namespace: currentNamespace.value },
  })
}

function openDependencies(name: string) {
  dependencyService.value = name
  dependencyDrawerVisible.value = true
}

async function fetchNamespaces() {
  try {
    const response = await getNamespaces()
    namespaces.value = response.data || []
    if (!namespaces.value.find((namespace) => namespace.name === currentNamespace.value) && namespaces.value.length > 0) {
      currentNamespace.value = namespaces.value[0]!.name
    }
  } catch (error) {
    console.error('fetch namespaces failed', error)
  }
}

async function fetchServices() {
  loading.value = true
  try {
    const [servicesRes, graphRes] = await Promise.all([
      getServices(currentNamespace.value),
      getDependencyGraph(currentNamespace.value),
    ])

    services.value = servicesRes.data || []
    dependencyGraph.value = graphRes.data || { namespace: currentNamespace.value, nodes: [], edges: [] }
    buildDependencyMap(
      services.value.map((service) => service.name),
      dependencyGraph.value,
    )

    const entries = await Promise.all(
      services.value.map(async (service) => {
        try {
          const detailRes = await getServiceInstances(service.name, currentNamespace.value)
          return [service.name, detailRes.data || []] as const
        } catch {
          return [service.name, []] as const
        }
      }),
    )
    instancesByService.value = Object.fromEntries(entries)
  } catch (error) {
    console.error('fetch services failed', error)
  } finally {
    loading.value = false
  }
}

async function showSubscribers(name: string) {
  currentService.value = name
  subscribersList.value = []
  subDialogVisible.value = true
  subLoading.value = true
  try {
    const response = await getSubscribers(name, currentNamespace.value)
    subscribersList.value = response.data || []
  } catch (error) {
    console.error('failed to fetch subscribers', error)
  } finally {
    subLoading.value = false
  }
}

watch(currentNamespace, async (namespace) => {
  await router.replace({ query: { ...route.query, namespace } })
  await fetchServices()
})

onMounted(async () => {
  await fetchNamespaces()
  await fetchServices()
})
</script>

<template>
  <div class="services-page">
    <section class="services-summary-row">
      <div class="summary-chip summary-chip-namespace">
        <span>{{ t.services.namespace }}</span>
        <strong>{{ currentNamespace }}</strong>
      </div>
      <div class="summary-chip">
        <span>{{ t.nav.services }}</span>
        <strong>{{ stats.serviceCount }}</strong>
      </div>
      <div class="summary-chip">
        <span>{{ t.services.dependency }}</span>
        <strong>{{ stats.dependencyCount }}</strong>
      </div>
      <div class="summary-chip">
        <span>{{ t.services.viewDependencies }}</span>
        <strong>{{ stats.relatedCount }}</strong>
      </div>
    </section>

    <section class="services-controls-row">
      <el-select v-model="currentNamespace" size="large" class="toolbar-namespace">
        <el-option v-for="namespace in namespaces" :key="namespace.name" :label="namespace.name" :value="namespace.name" />
      </el-select>

      <el-input
        v-model="search"
        :placeholder="t.services.searchPlaceholder"
        size="large"
        clearable
        :prefix-icon="Search"
        class="toolbar-search"
      />

      <el-select
        v-model="healthFilter"
        size="large"
        clearable
        class="toolbar-health"
        :placeholder="t.services.healthFilter"
      >
        <el-option :label="t.services.healthyOption" value="healthy" />
        <el-option :label="t.services.degradedOption" value="degraded" />
      </el-select>

      <div class="view-switch-minimal">
        <button type="button" class="switch-minimal-btn" :class="{ active: viewMode === 'list' }" @click="viewMode = 'list'" :title="t.settings.listView">
          <el-icon><List /></el-icon>
        </button>
        <button type="button" class="switch-minimal-btn" :class="{ active: viewMode === 'grid' }" @click="viewMode = 'grid'" :title="t.settings.gridView">
          <el-icon><Grid /></el-icon>
        </button>
      </div>
    </section>

    <section v-loading="loading">
      <div v-if="!loading && filtered.length === 0" class="empty-panel services-empty-panel">
        <div class="empty-state">
          <el-icon><Grid /></el-icon>
          <p>{{ t.services.noServices }}</p>
        </div>
      </div>

      <div v-else-if="viewMode === 'list'" class="services-table-wrap">
        <el-table :data="filtered" style="width: 100%">
          <el-table-column prop="name" :label="t.services.service" min-width="200">
            <template #default="{ row }">
              <div class="service-name-cell" @click="openService(row.name)">
                <strong>{{ row.name }}</strong>
                <span>{{ currentNamespace }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="instance_count" :label="t.services.instances" min-width="88" />
          <el-table-column prop="healthy_count" :label="t.services.healthy" min-width="88">
            <template #default="{ row }">
              <span :style="{ color: barColor(healthRate(row)) }">{{ row.healthy_count }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="subscriber_count" :label="t.services.subscribers" min-width="90">
            <template #default="{ row }">
              <el-button link type="primary" @click="showSubscribers(row.name)">{{ row.subscriber_count || 0 }}</el-button>
            </template>
          </el-table-column>
          <el-table-column :label="t.services.upstream" min-width="180">
            <template #default="{ row }">
              <span class="dependency-inline-text">{{ previewNames(dependencyMap[row.name]?.providers || []) }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="t.services.downstream" min-width="180">
            <template #default="{ row }">
              <span class="dependency-inline-text">{{ previewNames(dependencyMap[row.name]?.consumers || []) }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="t.services.healthRate" min-width="140">
            <template #default="{ row }">
              <div class="health-inline">
                <span>{{ healthRate(row).toFixed(0) }}%</span>
                <div class="health-inline-bar">
                  <div class="fill" :style="{ width: `${healthRate(row)}%`, background: barColor(healthRate(row)) }"></div>
                </div>
              </div>
            </template>
          </el-table-column>
          <el-table-column :label="t.common.actions" width="120" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" @click="openDependencies(row.name)">{{ t.services.viewDependencies }}</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div v-else class="service-grid services-card-grid-dense">
        <article v-for="(service, index) in filtered" :key="service.name" class="service-card dense-card" :style="{ animationDelay: `${index * 0.03}s` }">
          <div class="dense-card-head">
            <div class="dense-title-block">
              <div class="svc-name" @click="openService(service.name)">{{ service.name }}</div>
              <span class="dense-namespace">{{ currentNamespace }}</span>
            </div>
            <el-tag size="small" effect="plain" :type="dependencyBadgeType(dependencyTotal(service.name))">
              {{ dependencyTotal(service.name) }}
            </el-tag>
          </div>

          <div class="dense-stats-row">
            <span><el-icon><Monitor /></el-icon>{{ service.instance_count }}</span>
            <span><el-icon><CircleCheck /></el-icon>{{ service.healthy_count }}</span>
            <button type="button" class="dense-link" @click="showSubscribers(service.name)"><el-icon><User /></el-icon>{{ service.subscriber_count || 0 }}</button>
          </div>

          <div class="dense-dependency-row">
            <div>
              <label>{{ t.services.upstream }}</label>
              <strong>{{ previewNames(dependencyMap[service.name]?.providers || []) }}</strong>
            </div>
            <div>
              <label>{{ t.services.downstream }}</label>
              <strong>{{ previewNames(dependencyMap[service.name]?.consumers || []) }}</strong>
            </div>
          </div>

          <div class="dense-card-foot">
            <span class="dense-health">{{ healthRate(service).toFixed(0) }}%</span>
            <div class="health-inline-bar dense-health-bar">
              <div class="fill" :style="{ width: `${healthRate(service)}%`, background: barColor(healthRate(service)) }"></div>
            </div>
            <el-button link type="primary" @click="openDependencies(service.name)">
              <el-icon><Share /></el-icon>
            </el-button>
          </div>
        </article>
      </div>
    </section>

    <el-dialog v-model="subDialogVisible" :title="t.services.subscribersTitle.replace('{service}', currentService)" width="520px" append-to-body>
      <div v-loading="subLoading" class="dialog-body">
        <div v-if="subscribersList.length === 0 && !subLoading" class="dialog-empty">{{ t.services.noSubscribers }}</div>
        <div v-else class="subscribers-list">
          <div v-for="subscriber in subscribersList" :key="subscriber" class="subscriber-item">
            <el-icon><User /></el-icon>
            <span>{{ subscriber }}</span>
          </div>
        </div>
      </div>
    </el-dialog>

    <el-drawer v-model="dependencyDrawerVisible" :title="`${dependencyService} · ${t.services.dependencyTitle}`" size="420px" append-to-body>
      <div class="dependency-drawer">
        <section class="drawer-section">
          <div class="drawer-section-head">
            <h4>{{ t.services.upstream }}</h4>
            <el-tag size="small" effect="plain">{{ currentDependencyMeta.providers.length }}</el-tag>
          </div>
          <div v-if="currentDependencyMeta.providers.length" class="drawer-tags">
            <el-tag v-for="provider in currentDependencyMeta.providers" :key="provider" effect="plain">{{ provider }}</el-tag>
          </div>
          <el-empty v-else :description="t.services.noDependencies" :image-size="70" />
        </section>

        <section class="drawer-section">
          <div class="drawer-section-head">
            <h4>{{ t.services.downstream }}</h4>
            <el-tag size="small" type="success" effect="plain">{{ currentDependencyMeta.consumers.length }}</el-tag>
          </div>
          <div v-if="currentDependencyMeta.consumers.length" class="drawer-tags">
            <el-tag v-for="consumer in currentDependencyMeta.consumers" :key="consumer" type="success" effect="plain">{{ consumer }}</el-tag>
          </div>
          <el-empty v-else :description="t.services.noDependencies" :image-size="70" />
        </section>
      </div>
    </el-drawer>
  </div>
</template>

<style scoped>
.services-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.services-summary-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.summary-chip {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  min-height: 34px;
  padding: 0 12px;
  border-left: 2px solid rgba(15, 23, 42, 0.12);
  color: var(--text-secondary);
  font-size: 12px;
}

.summary-chip strong {
  color: var(--text-primary);
  font-size: 18px;
  line-height: 1;
}

.summary-chip-namespace {
  padding-left: 0;
  border-left: 0;
}

.services-controls-row {
  display: flex;
  align-items: center;
  gap: 12px;
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
  border: 0 !important;
  border-radius: 6px !important;
  background: rgba(15, 23, 42, 0.04) !important;
  box-shadow: none !important;
}

.view-switch-minimal {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.switch-minimal-btn {
  width: 38px;
  height: 38px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
}

.switch-minimal-btn:hover {
  background: rgba(15, 23, 42, 0.05);
  color: var(--text-primary);
}

.switch-minimal-btn.active {
  background: var(--accent-blue);
  color: white;
}

.services-empty-panel {
  min-height: 180px;
}

.services-table-wrap {
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--bg-card);
  overflow: hidden;
}

.service-name-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
  cursor: pointer;
}

.service-name-cell strong,
.svc-name {
  font-size: 16px;
  color: var(--text-primary);
}

.service-name-cell span {
  color: var(--text-secondary);
  font-size: 12px;
}

.service-name-cell:hover strong,
.svc-name:hover,
.dense-link:hover {
  color: var(--accent-blue);
}

.dependency-inline-text {
  color: var(--text-secondary);
  font-size: 12px;
}

.health-inline {
  display: flex;
  align-items: center;
  gap: 10px;
}

.health-inline-bar {
  flex: 1;
  height: 4px;
  background: rgba(15, 23, 42, 0.08);
  overflow: hidden;
}

.health-inline-bar .fill {
  height: 100%;
}

.services-card-grid-dense {
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 12px;
}

.dense-card {
  min-height: 164px;
  padding: 12px;
  border-radius: 8px;
  cursor: default;
}

.dense-card-head,
.dense-card-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.dense-title-block {
  min-width: 0;
}

.svc-name {
  margin-bottom: 4px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  cursor: pointer;
}

.dense-namespace {
  color: var(--text-secondary);
  font-size: 12px;
}

.dense-stats-row {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 6px;
  margin-top: 10px;
}

.dense-stats-row span,
.dense-link {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 32px;
  padding: 0 8px;
  background: rgba(15, 23, 42, 0.04);
  color: var(--text-secondary);
  font-size: 12px;
}

.dense-link {
  border: 0;
  cursor: pointer;
}

.dense-dependency-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  margin-top: 10px;
}

.dense-dependency-row > div {
  min-width: 0;
  padding: 8px;
  background: rgba(15, 23, 42, 0.03);
}

.dense-dependency-row label {
  display: block;
  margin-bottom: 4px;
  color: var(--text-secondary);
  font-size: 11px;
}

.dense-dependency-row strong {
  display: block;
  font-size: 12px;
  font-weight: 500;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.dense-card-foot {
  margin-top: auto;
}

.dense-health {
  min-width: 44px;
  color: var(--accent-green);
  font-weight: 700;
}

.dense-health-bar {
  flex: 1;
}

.dialog-body {
  min-height: 120px;
}

.dialog-empty {
  margin-top: 42px;
  text-align: center;
  color: var(--text-muted);
}

.subscribers-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.subscriber-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  background: var(--bg-glass);
  border: 1px solid var(--border-color);
}

.dependency-drawer {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.drawer-section {
  padding: 18px;
  border: 1px solid var(--border-color);
  background: var(--bg-card);
}

.drawer-section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.drawer-section-head h4 {
  font-size: 16px;
}

.drawer-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

@media (max-width: 980px) {
  .services-controls-row {
    flex-wrap: wrap;
  }

  .toolbar-namespace,
  .toolbar-health,
  .toolbar-search {
    width: 100%;
  }

  .services-card-grid-dense,
  .dense-stats-row,
  .dense-dependency-row {
    grid-template-columns: 1fr;
  }
}
</style>

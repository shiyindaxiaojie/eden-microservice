<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Delete,
  EditPen,
  Plus,
  RefreshLeft,
  Search,
  Switch,
} from '@element-plus/icons-vue'
import {
  deleteMockRoute,
  listMockRoutes,
  saveMockRoute,
  toggleMockRoute,
  type GatewayFilter,
  type GatewayLoadBalance,
  type GatewayRoute,
  type GatewayUpstreamType,
} from '../api/mock/control-plane'
import { useI18n } from '../utils/i18n'

type RouteFilter = 'all' | 'enabled' | 'disabled' | 'degraded'

interface RouteForm {
  namespace: string
  id: string
  name: string
  enabled: boolean
  priority: number
  hostsText: string
  pathPrefix: string
  methods: string[]
  headersText: string
  upstreamType: GatewayUpstreamType
  serviceName: string
  upstreamNamespace: string
  urlsText: string
  loadBalance: GatewayLoadBalance
  healthyOnly: boolean
  timeoutMs: number
  filtersText: string
}

const { text, elementLocale } = useI18n()

const loading = ref(false)
const routes = ref<GatewayRoute[]>([])
const selectedRouteKey = ref('')
const query = ref('')
const namespaceFilter = ref('all')
const stateFilter = ref<RouteFilter>('all')
const drawerVisible = ref(false)
const editorMode = ref<'create' | 'edit'>('create')
const submitting = ref(false)
const currentPage = ref(1)
const pageSize = ref(12)

const methodOptions = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH'] as const
const lbOptions: GatewayLoadBalance[] = ['round_robin', 'random', 'weighted']

const form = ref<RouteForm>({
  namespace: 'default',
  id: '',
  name: '',
  enabled: true,
  priority: 50,
  hostsText: 'api.eden.local',
  pathPrefix: '/api/demo',
  methods: ['GET'],
  headersText: '',
  upstreamType: 'service',
  serviceName: 'demo-service',
  upstreamNamespace: 'default',
  urlsText: '',
  loadBalance: 'round_robin',
  healthyOnly: true,
  timeoutMs: 30000,
  filtersText: 'strip_prefix parts=2',
})

const routeKey = (route: Pick<GatewayRoute, 'namespace' | 'id'>) => `${route.namespace}@@${route.id}`

const namespaceOptions = computed(() => {
  const values = new Set(['default', 'prod', 'dev'])
  routes.value.forEach((route) => values.add(route.namespace))
  return [...values]
})

const filteredRoutes = computed(() => {
  const term = query.value.trim().toLowerCase()

  return routes.value.filter((route) => {
    const matchesNamespace = namespaceFilter.value === 'all' || route.namespace === namespaceFilter.value
    const matchesState =
      stateFilter.value === 'all' ||
      (stateFilter.value === 'enabled' && route.enabled) ||
      (stateFilter.value === 'disabled' && !route.enabled) ||
      (stateFilter.value === 'degraded' && route.last_status === 'degraded')
    const targetText = route.upstream.type === 'service'
      ? route.upstream.service_name || ''
      : (route.upstream.urls || []).join(',')
    const matchesTerm =
      !term ||
      route.id.toLowerCase().includes(term) ||
      route.name.toLowerCase().includes(term) ||
      route.match.path_prefix.toLowerCase().includes(term) ||
      targetText.toLowerCase().includes(term)

    return matchesNamespace && matchesState && matchesTerm
  })
})

const pagedRoutes = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredRoutes.value.slice(start, start + pageSize.value)
})

const totalPages = computed(() => Math.max(1, Math.ceil(filteredRoutes.value.length / pageSize.value)))

function formatDateTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value || '-'
  const pad = (input: number) => `${input}`.padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function statusText(route: GatewayRoute) {
  if (!route.enabled) return text('已停用', 'Disabled')
  if (route.last_status === 'degraded') return text('部分异常', 'Degraded')
  return text('运行中', 'Healthy')
}

function statusClass(route: GatewayRoute) {
  if (!route.enabled) return 'disabled'
  return route.last_status === 'degraded' ? 'degraded' : 'healthy'
}

function upstreamLabel(route: GatewayRoute) {
  if (route.upstream.type === 'service') {
    return `${route.upstream.namespace || route.namespace}/${route.upstream.service_name}`
  }
  return (route.upstream.urls || []).join(', ')
}

function parseHeaders(input: string) {
  const result: Record<string, string> = {}
  input
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .forEach((line) => {
      const [name, ...rest] = line.split('=')
      if (name && rest.length) {
        result[name.trim()] = rest.join('=').trim()
      }
    })
  return result
}

function stringifyHeaders(headers: Record<string, string>) {
  return Object.entries(headers)
    .map(([key, value]) => `${key}=${value}`)
    .join('\n')
}

function parseFilters(input: string): GatewayFilter[] {
  return input
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => {
      const [type, ...parts] = line.split(/\s+/)
      const args = Object.fromEntries(
        parts
          .map((part) => part.split('='))
          .filter(([key, value]) => key && value)
          .map(([key, value]) => [key, value]),
      )

      if (type === 'strip_prefix') {
        return { type, parts: Number(args.parts || 1) } as GatewayFilter
      }
      if (type === 'set_response_header') {
        return { type, name: args.name || 'X-Route', value: args.value || 'eden' } as GatewayFilter
      }
      return { type: 'add_request_header', name: args.name || 'X-Gateway', value: args.value || 'eden' }
    })
}

function stringifyFilters(filters: GatewayFilter[]) {
  return filters.map((filter) => {
    if (filter.type === 'strip_prefix') return `strip_prefix parts=${filter.parts || 1}`
    return `${filter.type} name=${filter.name || 'X-Gateway'} value=${filter.value || 'eden'}`
  }).join('\n')
}

async function fetchRoutes() {
  loading.value = true
  try {
    routes.value = (await listMockRoutes()).sort((left, right) => left.priority - right.priority || left.id.localeCompare(right.id))
    if (!selectedRouteKey.value || !routes.value.find((route) => routeKey(route) === selectedRouteKey.value)) {
      selectedRouteKey.value = routes.value[0] ? routeKey(routes.value[0]) : ''
    }
  } finally {
    loading.value = false
  }
}

function selectRoute(row: GatewayRoute) {
  selectedRouteKey.value = routeKey(row)
}

function routeRowClass({ row }: { row: GatewayRoute }) {
  return routeKey(row) === selectedRouteKey.value ? 'selected-row' : ''
}

function resetFilters() {
  query.value = ''
  namespaceFilter.value = 'all'
  stateFilter.value = 'all'
  currentPage.value = 1
}

function openCreate() {
  editorMode.value = 'create'
  form.value = {
    namespace: namespaceFilter.value === 'all' ? 'default' : namespaceFilter.value,
    id: '',
    name: '',
    enabled: true,
    priority: 50,
    hostsText: 'api.eden.local',
    pathPrefix: '/api/demo',
    methods: ['GET'],
    headersText: '',
    upstreamType: 'service',
    serviceName: 'demo-service',
    upstreamNamespace: 'default',
    urlsText: '',
    loadBalance: 'round_robin',
    healthyOnly: true,
    timeoutMs: 30000,
    filtersText: 'strip_prefix parts=2',
  }
  drawerVisible.value = true
}

function openEdit(route: GatewayRoute) {
  editorMode.value = 'edit'
  form.value = {
    namespace: route.namespace,
    id: route.id,
    name: route.name,
    enabled: route.enabled,
    priority: route.priority,
    hostsText: route.match.hosts.join('\n'),
    pathPrefix: route.match.path_prefix,
    methods: [...route.match.methods],
    headersText: stringifyHeaders(route.match.headers),
    upstreamType: route.upstream.type,
    serviceName: route.upstream.service_name || '',
    upstreamNamespace: route.upstream.namespace || route.namespace,
    urlsText: (route.upstream.urls || []).join('\n'),
    loadBalance: route.upstream.load_balance,
    healthyOnly: route.upstream.healthy_only,
    timeoutMs: route.timeout_ms,
    filtersText: stringifyFilters(route.filters),
  }
  drawerVisible.value = true
}

async function submitRoute() {
  const payload = form.value
  if (!payload.id.trim() || !payload.name.trim()) {
    ElMessage.warning(text('请填写路由 ID 和名称', 'Route ID and name are required'))
    return
  }
  if (!payload.pathPrefix.trim().startsWith('/')) {
    ElMessage.warning(text('路径前缀必须以 / 开头', 'Path prefix must start with /'))
    return
  }
  if (payload.upstreamType === 'service' && !payload.serviceName.trim()) {
    ElMessage.warning(text('请填写上游服务名', 'Upstream service is required'))
    return
  }
  if (payload.upstreamType === 'static' && !payload.urlsText.trim()) {
    ElMessage.warning(text('请填写静态上游 URL', 'Static upstream URL is required'))
    return
  }

  submitting.value = true
  try {
    const saved = await saveMockRoute({
      namespace: payload.namespace.trim() || 'default',
      id: payload.id.trim(),
      name: payload.name.trim(),
      enabled: payload.enabled,
      priority: Number(payload.priority) || 50,
      match: {
        hosts: payload.hostsText.split('\n').map((item) => item.trim()).filter(Boolean),
        path_prefix: payload.pathPrefix.trim(),
        methods: payload.methods,
        headers: parseHeaders(payload.headersText),
      },
      upstream: {
        type: payload.upstreamType,
        service_name: payload.upstreamType === 'service' ? payload.serviceName.trim() : undefined,
        namespace: payload.upstreamType === 'service' ? payload.upstreamNamespace.trim() || payload.namespace : undefined,
        urls: payload.upstreamType === 'static'
          ? payload.urlsText.split('\n').map((item) => item.trim()).filter(Boolean)
          : undefined,
        load_balance: payload.loadBalance,
        healthy_only: payload.healthyOnly,
      },
      filters: parseFilters(payload.filtersText),
      timeout_ms: Number(payload.timeoutMs) || 30000,
    })
    selectedRouteKey.value = routeKey(saved)
    drawerVisible.value = false
    ElMessage.success(text('路由已保存', 'Route saved'))
    await fetchRoutes()
  } finally {
    submitting.value = false
  }
}

async function handleToggle(route: GatewayRoute) {
  const nextEnabled = !route.enabled
  try {
    await ElMessageBox.confirm(
      nextEnabled
        ? text(`确定启用路由 "${route.name}" 吗？`, `Enable route "${route.name}"?`)
        : text(`确定停用路由 "${route.name}" 吗？正在经过该路由的请求不会被中断。`, `Disable route "${route.name}"? Requests already in flight will not be interrupted.`),
      nextEnabled ? text('启用路由', 'Enable route') : text('停用路由', 'Disable route'),
      {
        confirmButtonText: nextEnabled ? text('启用', 'Enable') : text('停用', 'Disable'),
        cancelButtonText: text('取消', 'Cancel'),
        type: 'warning',
      },
    )
    await toggleMockRoute(route.namespace, route.id, nextEnabled)
    ElMessage.success(nextEnabled ? text('路由已启用', 'Route enabled') : text('路由已停用', 'Route disabled'))
    await fetchRoutes()
  } catch (error) {
    if (error !== 'cancel') {
      console.error(error)
    }
  }
}

async function handleDelete(route: GatewayRoute) {
  try {
    await ElMessageBox.confirm(
      text(`确定删除路由 "${route.name}" 吗？`, `Delete route "${route.name}"?`),
      text('删除路由', 'Delete route'),
      {
        confirmButtonText: text('删除', 'Delete'),
        cancelButtonText: text('取消', 'Cancel'),
        type: 'warning',
      },
    )
    await deleteMockRoute(route.namespace, route.id)
    ElMessage.success(text('路由已删除', 'Route deleted'))
    await fetchRoutes()
  } catch (error) {
    if (error !== 'cancel') {
      console.error(error)
    }
  }
}

watch([query, namespaceFilter, stateFilter], () => {
  currentPage.value = 1
})

watch(pageSize, () => {
  currentPage.value = 1
})

watch(filteredRoutes, () => {
  if (currentPage.value > totalPages.value) currentPage.value = totalPages.value
})

onMounted(fetchRoutes)
</script>

<template>
  <div class="route-shell">
    <section class="route-panel">
      <div class="route-toolbar">
        <div class="field-item">
          <span class="field-label">{{ text('命名空间', 'Namespace') }}</span>
          <el-select v-model="namespaceFilter" class="small-select">
            <el-option :label="text('全部', 'All')" value="all" />
            <el-option v-for="namespace in namespaceOptions" :key="namespace" :label="namespace" :value="namespace" />
          </el-select>
        </div>

        <div class="field-item">
          <span class="field-label">{{ text('搜索', 'Search') }}</span>
          <el-input
            v-model="query"
            :prefix-icon="Search"
            clearable
            class="search-input"
            :placeholder="text('路由 / 路径 / 上游', 'Route / path / upstream')"
          />
        </div>

        <button class="icon-btn" type="button" :title="text('重置筛选', 'Reset filters')" @click="resetFilters">
          <el-icon><RefreshLeft /></el-icon>
        </button>

        <div class="toolbar-spacer"></div>

        <div class="pill-group">
          <button type="button" :class="{ active: stateFilter === 'all' }" @click="stateFilter = 'all'">
            {{ text('全部', 'All') }}
          </button>
          <button type="button" :class="{ active: stateFilter === 'enabled' }" @click="stateFilter = 'enabled'">
            {{ text('启用', 'Enabled') }}
          </button>
          <button type="button" :class="{ active: stateFilter === 'disabled' }" @click="stateFilter = 'disabled'">
            {{ text('停用', 'Disabled') }}
          </button>
          <button type="button" :class="{ active: stateFilter === 'degraded' }" @click="stateFilter = 'degraded'">
            {{ text('异常', 'Degraded') }}
          </button>
        </div>

        <el-button type="primary" :icon="Plus" @click="openCreate">
          {{ text('新建路由', 'New route') }}
        </el-button>
      </div>

      <div class="route-workbench" v-loading="loading">
        <div class="route-list">
          <el-table
            :data="pagedRoutes"
            height="100%"
            highlight-current-row
            :row-class-name="routeRowClass"
            @row-click="selectRoute"
          >
            <el-table-column :label="text('优先级', 'Priority')" width="90">
              <template #default="{ row }">
                <span class="priority-cell">{{ row.priority }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="text('路由', 'Route')" min-width="220">
              <template #default="{ row }">
                <div class="route-cell">
                  <strong>{{ row.name }}</strong>
                  <span>{{ row.namespace }} / {{ row.id }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column :label="text('匹配路径', 'Path')" min-width="190">
              <template #default="{ row }">
                <code class="path-chip">{{ row.match.path_prefix }}</code>
              </template>
            </el-table-column>
            <el-table-column :label="text('上游', 'Upstream')" min-width="210">
              <template #default="{ row }">
                <div class="upstream-cell">
                  <el-tag size="small" effect="plain" :type="row.upstream.type === 'service' ? 'success' : 'warning'">
                    {{ row.upstream.type }}
                  </el-tag>
                  <span>{{ upstreamLabel(row) }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column :label="text('状态', 'Status')" width="120">
              <template #default="{ row }">
                <span class="status-pill" :class="statusClass(row)">{{ statusText(row) }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="text('更新时间', 'Updated')" width="170">
              <template #default="{ row }">{{ formatDateTime(row.updated_at) }}</template>
            </el-table-column>
            <el-table-column :label="text('操作', 'Actions')" width="220" fixed="right">
              <template #default="{ row }">
                <div class="row-actions">
                  <el-button link :icon="Switch" @click.stop="handleToggle(row)">
                    {{ row.enabled ? text('停用', 'Disable') : text('启用', 'Enable') }}
                  </el-button>
                  <el-button link type="primary" :icon="EditPen" @click.stop="openEdit(row)">
                    {{ text('编辑', 'Edit') }}
                  </el-button>
                  <el-button link type="danger" :icon="Delete" @click.stop="handleDelete(row)">
                    {{ text('删除', 'Delete') }}
                  </el-button>
                </div>
              </template>
            </el-table-column>
          </el-table>

          <footer class="route-footer">
            <span>{{ filteredRoutes.length }} {{ text('条', 'records') }}</span>
            <el-config-provider :locale="elementLocale">
              <el-pagination
                v-model:current-page="currentPage"
                v-model:page-size="pageSize"
                :page-sizes="[12, 20, 50]"
                :total="filteredRoutes.length"
                layout="sizes, prev, pager, next"
                background
              />
            </el-config-provider>
          </footer>
        </div>

      </div>
    </section>

    <el-drawer
      v-model="drawerVisible"
      :title="editorMode === 'create' ? text('新建路由', 'New route') : text('编辑路由', 'Edit route')"
      direction="rtl"
      size="640px"
      class="form-drawer"
    >
      <div class="drawer-form">
        <el-form label-position="top">
          <div class="form-grid">
            <el-form-item :label="text('命名空间', 'Namespace')">
              <el-select v-model="form.namespace" filterable allow-create>
                <el-option v-for="namespace in namespaceOptions" :key="namespace" :label="namespace" :value="namespace" />
              </el-select>
            </el-form-item>
            <el-form-item :label="text('路由 ID', 'Route ID')">
              <el-input v-model="form.id" :disabled="editorMode === 'edit'" />
            </el-form-item>
          </div>
          <div class="form-grid">
            <el-form-item :label="text('名称', 'Name')">
              <el-input v-model="form.name" />
            </el-form-item>
            <el-form-item :label="text('优先级', 'Priority')">
              <el-input-number v-model="form.priority" :min="1" :max="999" controls-position="right" />
            </el-form-item>
          </div>
          <div class="form-grid">
            <el-form-item :label="text('启用', 'Enabled')">
              <el-switch v-model="form.enabled" />
            </el-form-item>
            <el-form-item :label="text('超时毫秒', 'Timeout ms')">
              <el-input-number v-model="form.timeoutMs" :min="1000" :step="1000" controls-position="right" />
            </el-form-item>
          </div>

          <el-divider content-position="left">{{ text('匹配条件', 'Predicates') }}</el-divider>
          <el-form-item label="Host">
            <el-input v-model="form.hostsText" type="textarea" :rows="2" :placeholder="text('每行一个 host，可为空', 'One host per line, optional')" />
          </el-form-item>
          <el-form-item :label="text('路径前缀', 'Path prefix')">
            <el-input v-model="form.pathPrefix" />
          </el-form-item>
          <el-form-item :label="text('方法', 'Methods')">
            <el-select v-model="form.methods" multiple collapse-tags collapse-tags-tooltip>
              <el-option v-for="method in methodOptions" :key="method" :label="method" :value="method" />
            </el-select>
          </el-form-item>
          <el-form-item :label="text('Header 匹配', 'Header matches')">
            <el-input v-model="form.headersText" type="textarea" :rows="2" placeholder="X-Env=demo" />
          </el-form-item>

          <el-divider content-position="left">{{ text('上游', 'Upstream') }}</el-divider>
          <div class="form-grid">
            <el-form-item :label="text('类型', 'Type')">
              <el-select v-model="form.upstreamType">
                <el-option :label="text('注册服务', 'Service')" value="service" />
                <el-option :label="text('静态地址', 'Static URLs')" value="static" />
              </el-select>
            </el-form-item>
            <el-form-item :label="text('负载均衡', 'Load balance')">
              <el-select v-model="form.loadBalance">
                <el-option v-for="lb in lbOptions" :key="lb" :label="lb" :value="lb" />
              </el-select>
            </el-form-item>
          </div>
          <div v-if="form.upstreamType === 'service'" class="form-grid">
            <el-form-item :label="text('上游命名空间', 'Upstream namespace')">
              <el-input v-model="form.upstreamNamespace" />
            </el-form-item>
            <el-form-item :label="text('服务名', 'Service name')">
              <el-input v-model="form.serviceName" />
            </el-form-item>
          </div>
          <el-form-item v-else :label="text('静态 URL', 'Static URLs')">
            <el-input v-model="form.urlsText" type="textarea" :rows="3" placeholder="http://127.0.0.1:8080" />
          </el-form-item>
          <el-form-item :label="text('健康实例优先', 'Healthy only')">
            <el-switch v-model="form.healthyOnly" />
          </el-form-item>

          <el-divider content-position="left">{{ text('过滤器', 'Filters') }}</el-divider>
          <el-form-item>
            <el-input
              v-model="form.filtersText"
              type="textarea"
              :rows="4"
              placeholder="strip_prefix parts=2&#10;add_request_header name=X-Gateway value=eden"
            />
          </el-form-item>
        </el-form>
      </div>
      <template #footer>
        <div class="drawer-footer">
          <el-button @click="drawerVisible = false">{{ text('取消', 'Cancel') }}</el-button>
          <el-button type="primary" :loading="submitting" @click="submitRoute">
            {{ text('保存路由', 'Save route') }}
          </el-button>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<style scoped lang="scss">
.route-shell {
  display: flex;
  flex-direction: column;
  gap: 0;
  height: 100%;
  min-height: 0;
}

.route-hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 18px;
  align-items: center;
  padding: 16px 18px;
  border: 1px solid var(--border-color);
  background: var(--bg-card);
  border-radius: 0;
}

.hero-kicker,
.detail-kicker {
  display: block;
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.hero-copy h2 {
  margin: 4px 0;
  color: var(--text-primary);
  font-size: 22px;
  line-height: 1.15;
}

.hero-copy p {
  margin: 0;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.55;
}

.route-flow {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 10px 12px;
  border: 1px solid rgba(59, 130, 246, 0.16);
  background: rgba(59, 130, 246, 0.06);
  border-radius: 8px;
}

.route-flow span {
  color: var(--accent-blue);
  font-size: 12px;
  font-weight: 800;
  white-space: nowrap;
}

.route-flow i {
  width: 22px;
  height: 1px;
  background: rgba(59, 130, 246, 0.42);
}

.route-metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.metric-card {
  padding: 13px 14px;
  border: 1px solid var(--border-color);
  background: var(--bg-card);
  border-radius: 0;
}

.metric-card span {
  display: block;
  color: var(--text-muted);
  font-size: 12px;
  margin-bottom: 6px;
}

.metric-card strong {
  color: var(--text-primary);
  font-size: 22px;
  font-variant-numeric: tabular-nums;
}

.route-panel {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  border: 0;
  background: transparent;
  border-radius: 0;
  overflow: hidden;
}

.route-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  padding: 0 0 18px;
  border-bottom: 1px solid var(--border-color);
}

.toolbar-spacer {
  flex: 1;
}

.field-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.field-label {
  color: var(--text-secondary);
  font-size: 13px;
  font-weight: 700;
  white-space: nowrap;
}

.small-select {
  width: 150px;
}

.search-input {
  width: 260px;
}

.icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  padding: 0;
  border: 0;
  border-radius: 6px;
  color: var(--text-muted);
  background: transparent;
  cursor: pointer;
}

.icon-btn:hover {
  color: var(--accent-blue);
  background: rgba(59, 130, 246, 0.08);
}

.pill-group {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 2px;
  border-radius: 6px;
  background: rgba(148, 163, 184, 0.1);
}

.pill-group button {
  display: inline-flex;
  align-items: center;
  height: 30px;
  padding: 0 10px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--text-muted);
  font-family: inherit;
  font-size: 13px;
  font-weight: 700;
  cursor: pointer;
}

.pill-group button.active {
  color: var(--accent-blue);
  background: rgba(59, 130, 246, 0.12);
}

.route-workbench {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.route-list {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding: 18px 0 0;
  overflow: hidden;
}

:deep(.el-table) {
  flex: 1;
  min-height: 0;
  height: auto !important;
  --el-table-bg-color: transparent;
  --el-table-tr-bg-color: transparent;
  --el-table-row-hover-bg-color: rgba(59, 130, 246, 0.04);
  --el-table-border-color: var(--border-color);
  --el-table-header-bg-color: transparent;
  --el-table-header-text-color: var(--text-muted);
  --el-table-text-color: var(--text-primary);
}

:deep(.el-table__inner-wrapper::before) {
  display: none;
}

:deep(.el-table__inner-wrapper) {
  min-height: 0;
}

:deep(.selected-row td.el-table__cell) {
  background: rgba(59, 130, 246, 0.06) !important;
}

:deep(.el-table__body-wrapper) {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.priority-cell {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 34px;
  height: 26px;
  padding: 0 8px;
  border-radius: 6px;
  color: var(--accent-blue);
  background: rgba(59, 130, 246, 0.08);
  font-weight: 800;
}

.route-cell {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.route-cell strong {
  color: var(--text-primary);
  font-weight: 800;
}

.route-cell span {
  color: var(--text-muted);
  font-size: 12px;
}

.path-chip {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  padding: 0 8px;
  border-radius: 6px;
  color: var(--accent-cyan);
  background: rgba(6, 182, 212, 0.08);
  font-family: 'JetBrains Mono', 'Fira Code', Menlo, Consolas, monospace;
  font-size: 12px;
  font-weight: 700;
}

.upstream-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.upstream-cell span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 26px;
  padding: 0 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 800;
  white-space: nowrap;
}

.status-pill.healthy {
  color: var(--accent-green);
  background: rgba(16, 185, 129, 0.12);
}

.status-pill.degraded {
  color: var(--accent-orange);
  background: rgba(245, 158, 11, 0.13);
}

.status-pill.disabled {
  color: var(--text-muted);
  background: rgba(148, 163, 184, 0.14);
}

.row-actions {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  white-space: nowrap;
}

.row-actions :deep(.el-button) {
  margin-left: 0;
}

.row-actions :deep(.el-button span) {
  white-space: nowrap;
}

.route-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  margin-top: auto;
  padding-top: 14px;
  color: var(--text-muted);
  font-size: 13px;
  flex-shrink: 0;
}

.route-footer > span {
  margin-right: auto;
}

.route-detail {
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px;
  border-left: 1px solid var(--border-color);
  background: rgba(255, 255, 255, 0.02);
  overflow-y: auto;
}

.detail-head {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.detail-icon {
  width: 42px;
  height: 42px;
  flex-shrink: 0;
  display: grid;
  place-items: center;
  border-radius: 8px;
  color: var(--accent-green);
  background: rgba(16, 185, 129, 0.12);
}

.detail-icon.degraded {
  color: var(--accent-orange);
  background: rgba(245, 158, 11, 0.12);
}

.detail-icon.disabled {
  color: var(--text-muted);
  background: rgba(148, 163, 184, 0.12);
}

.detail-head h3 {
  margin: 2px 0 0;
  color: var(--text-primary);
  font-size: 18px;
  line-height: 1.25;
}

.detail-status-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  color: var(--text-muted);
  font-size: 12px;
}

.flow-card {
  display: grid;
  gap: 8px;
  padding: 12px;
  border: 1px solid rgba(59, 130, 246, 0.14);
  border-radius: 8px;
  background: rgba(59, 130, 246, 0.05);
}

.flow-step {
  display: grid;
  grid-template-columns: 72px minmax(0, 1fr);
  gap: 8px;
  align-items: center;
}

.flow-step span,
.detail-section h4,
.traffic-card span,
.detail-grid span {
  color: var(--text-muted);
  font-size: 12px;
}

.flow-step strong {
  min-width: 0;
  color: var(--text-primary);
  font-size: 13px;
  overflow-wrap: anywhere;
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.detail-grid div,
.traffic-card {
  padding: 12px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--bg-secondary);
}

.detail-grid div {
  display: grid;
  gap: 5px;
}

.detail-grid .el-icon {
  color: var(--accent-blue);
}

.detail-grid strong,
.traffic-card strong {
  color: var(--text-primary);
  font-size: 14px;
}

.detail-section {
  display: grid;
  gap: 8px;
  padding: 12px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--bg-secondary);
}

.detail-section h4 {
  margin: 0;
  font-weight: 800;
}

.detail-section p {
  margin: 0;
  color: var(--text-primary);
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.hint {
  color: var(--text-muted);
  font-size: 12px;
}

.filter-stack {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.filter-chip {
  padding: 5px 8px;
  border-radius: 6px;
  color: var(--accent-blue);
  background: rgba(59, 130, 246, 0.08);
  font-size: 12px;
  font-weight: 700;
}

.traffic-card {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.traffic-card div {
  display: grid;
  gap: 5px;
}

.drawer-form {
  padding-right: 6px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.drawer-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

@media (max-width: 1200px) {
  .route-hero,
  .route-toolbar {
    grid-template-columns: 1fr;
    flex-wrap: wrap;
  }

  .route-flow,
  .toolbar-spacer {
    width: 100%;
  }

  .route-workbench {
    grid-template-columns: 1fr;
  }

  .route-detail {
    border-left: 0;
    border-top: 1px solid var(--border-color);
    max-height: 460px;
  }
}

@media (max-width: 760px) {
  .route-shell {
    height: 100%;
    min-height: 0;
  }

  .route-metrics,
  .detail-grid,
  .traffic-card,
  .form-grid {
    grid-template-columns: 1fr;
  }

  .field-item,
  .search-input,
  .small-select,
  .pill-group {
    width: 100%;
  }

  .route-toolbar,
  .route-flow,
  .route-footer {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>

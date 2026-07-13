<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, EditPen, Plus, RefreshLeft, Search, Switch } from '@element-plus/icons-vue'
import {
  createGatewayRoute,
  deleteGatewayRoute,
  listGatewayRoutes,
  listGatewayRuntime,
  setGatewayRouteEnabled,
  updateGatewayRoute,
  type GatewayBetaTarget,
  type GatewayFilter,
  type GatewayLoadBalance,
  type GatewayRoute,
  type GatewayRouteInput,
  type GatewayRuntimeStatus,
  type GatewayTarget,
  type GatewayTargetType,
  type GatewayTrafficMode,
} from '../api/gateway'
import { getServices } from '../api/registry'
import { useI18n } from '../utils/i18n'

type RouteFilter = 'all' | 'enabled' | 'disabled'
type PathMatchMode = 'prefix' | 'exact'

interface ServiceChoice {
  key: string
  namespace: string
  group: string
  name: string
}

interface EditableTarget {
  id: string
  name: string
  type: GatewayTargetType
  serviceNamespace: string
  serviceGroup: string
  serviceName: string
  staticEndpointsText: string
  loadBalance: GatewayLoadBalance
  weight: number
  betaUsersText: string
  betaTenantsText: string
}

interface RouteForm {
  namespace: string
  id: string
  name: string
  enabled: boolean
  priority: number
  hostsText: string
  pathMatchMode: PathMatchMode
  path: string
  methods: string[]
  headersText: string
  timeoutMs: number
  filtersText: string
  trafficMode: GatewayTrafficMode
  defaultTargetID: string
  activeTargetID: string
  targets: EditableTarget[]
}

const { text, elementLocale } = useI18n()

const methodOptions = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'HEAD', 'OPTIONS'] as const
const loadBalanceOptions: GatewayLoadBalance[] = ['round_robin', 'random', 'weighted']
const trafficModeOptions: GatewayTrafficMode[] = ['weighted', 'canary', 'blue_green']

const loading = ref(false)
const submitting = ref(false)
const routes = ref<GatewayRoute[]>([])
const routeTotal = ref(0)
const runtimeByRoute = ref<Record<string, GatewayRuntimeStatus>>({})
const serviceChoices = ref<ServiceChoice[]>([])
const query = ref('')
const namespaceFilter = ref('all')
const stateFilter = ref<RouteFilter>('all')
const currentPage = ref(1)
const pageSize = ref(12)
const drawerVisible = ref(false)
const editorMode = ref<'create' | 'edit'>('create')
const editingRevision = ref(0)
let routeFetchVersion = 0
let serviceFetchVersion = 0
const canManage = computed(() => {
  const role = localStorage.getItem('user_role')
  return !role || role === 'admin'
})

function newTarget(namespace: string, index = 1): EditableTarget {
  return {
    id: index === 1 ? 'stable' : `target-${index}`,
    name: index === 1 ? text('稳定版本', 'Stable') : '',
    type: 'service',
    serviceNamespace: namespace || 'default',
    serviceGroup: 'default',
    serviceName: '',
    staticEndpointsText: '',
    loadBalance: 'round_robin',
    weight: index === 1 ? 100 : 0,
    betaUsersText: '',
    betaTenantsText: '',
  }
}

function newForm(namespace = 'default'): RouteForm {
  return {
    namespace,
    id: '',
    name: '',
    enabled: true,
    priority: 50,
    hostsText: '',
    pathMatchMode: 'prefix',
    path: '/api/demo',
    methods: ['GET'],
    headersText: '',
    timeoutMs: 30_000,
    filtersText: 'strip_prefix parts=2',
    trafficMode: 'weighted',
    defaultTargetID: 'stable',
    activeTargetID: 'stable',
    targets: [newTarget(namespace)],
  }
}

const form = ref<RouteForm>(newForm())

const routeKey = (route: Pick<GatewayRoute, 'namespace' | 'id'>) => `${route.namespace}@@${route.id}`

const namespaceOptions = computed(() => {
  const values = new Set(['default'])
  routes.value.forEach((route) => values.add(route.namespace))
  if (form.value.namespace.trim()) values.add(form.value.namespace.trim())
  return [...values].sort()
})

const totalPages = computed(() => Math.max(1, Math.ceil(routeTotal.value / pageSize.value)))

function formatDateTime(value?: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (input: number) => `${input}`.padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function targetLabel(target: GatewayTarget) {
  if (target.type === 'service' && target.service) {
    return `${target.service.namespace}/${target.service.group}/${target.service.service_name}`
  }
  const count = target.static?.endpoints.length || 0
  return `${count} ${text('个静态地址', 'static endpoints')}`
}

function releaseLabel(mode: GatewayTrafficMode) {
  if (mode === 'canary') return text('金丝雀', 'Canary')
  if (mode === 'blue_green') return text('蓝绿', 'Blue-green')
  return text('权重', 'Weighted')
}

function statusText(route: GatewayRoute) {
  if (!route.enabled) return text('已停用', 'Disabled')
  const runtime = runtimeByRoute.value[routeKey(route)]
  if (!runtime?.data_plane_enabled) return text('数据面未启用', 'Data plane off')
  if ((runtime?.last_status || 0) >= 500) return text('异常', 'Degraded')
  return text('运行中', 'Active')
}

function statusClass(route: GatewayRoute) {
  if (!route.enabled) return 'disabled'
  const runtime = runtimeByRoute.value[routeKey(route)]
  if (!runtime?.data_plane_enabled) return 'disabled'
  return (runtime.last_status || 0) >= 500 ? 'degraded' : 'healthy'
}

function parseLines(value: string) {
  return value.split('\n').map((item) => item.trim()).filter(Boolean)
}

function parseCommaList(value: string) {
  return value.split(',').map((item) => item.trim()).filter(Boolean)
}

function parseHeaders(value: string) {
  const headers: Record<string, string> = {}
  parseLines(value).forEach((line) => {
    const [name, ...values] = line.split('=')
    if (name?.trim() && values.length) headers[name.trim()] = values.join('=').trim()
  })
  return headers
}

function stringifyHeaders(headers?: Record<string, string>) {
  return Object.entries(headers || {}).map(([name, value]) => `${name}=${value}`).join('\n')
}

function parseFilters(value: string): GatewayFilter[] {
  return parseLines(value).map((line) => {
    const [type, ...parts] = line.split(/\s+/)
    const args = Object.fromEntries(parts.map((item) => item.split('=')).filter(([key, item]) => key && item))
    if (type === 'strip_prefix') return { type, parts: Number(args.parts || 1) }
    if (type === 'set_response_header') return { type, name: args.name || '', value: args.value || '' }
    return { type: 'add_request_header', name: args.name || '', value: args.value || '' }
  })
}

function stringifyFilters(filters?: GatewayFilter[]) {
  return (filters || []).map((filter) => {
    if (filter.type === 'strip_prefix') return `strip_prefix parts=${filter.parts || 1}`
    return `${filter.type} name=${filter.name || ''} value=${filter.value || ''}`
  }).join('\n')
}

function parseStaticEndpoints(value: string) {
  return parseLines(value).map((line) => {
    const [url, ...argumentsList] = line.split(/\s+/)
    const weightArgument = argumentsList.find((argument) => argument.startsWith('weight='))
    return { url: url || '', weight: Math.max(1, Number(weightArgument?.slice('weight='.length) || 1)) }
  })
}

function stringifyStaticEndpoints(target: GatewayTarget) {
  return (target.static?.endpoints || []).map((endpoint) => `${endpoint.url} weight=${endpoint.weight || 1}`).join('\n')
}

function serviceChoicesFromPayload(payload: unknown, namespace: string) {
  const choices = new Map<string, ServiceChoice>()
  const add = (name: string, group = 'default') => {
    const normalizedName = name.trim()
    const normalizedGroup = group.trim() || 'default'
    if (!normalizedName) return
    const choice: ServiceChoice = {
      key: `${namespace}@@${normalizedGroup}@@${normalizedName}`,
      namespace,
      group: normalizedGroup,
      name: normalizedName,
    }
    choices.set(choice.key, choice)
  }
  const addQualified = (qualifiedName: string) => {
    const separator = qualifiedName.indexOf('@@')
    if (separator > 0) {
      add(qualifiedName.slice(separator + 2), qualifiedName.slice(0, separator))
      return
    }
    add(qualifiedName)
  }
  if (Array.isArray(payload)) {
    payload.forEach((item) => {
      if (typeof item === 'string') {
        addQualified(item)
        return
      }
      if (!item || typeof item !== 'object') return
      const row = item as { name?: unknown, group?: unknown, qualified_name?: unknown }
      if (typeof row.name === 'string') {
        add(row.name, typeof row.group === 'string' ? row.group : 'default')
      } else if (typeof row.qualified_name === 'string') {
        addQualified(row.qualified_name)
      }
    })
  } else if (payload && typeof payload === 'object') {
    Object.keys(payload as Record<string, unknown>).forEach(addQualified)
  }
  return [...choices.values()].sort((left, right) => left.key.localeCompare(right.key))
}

async function fetchServiceChoices() {
  const version = ++serviceFetchVersion
  try {
    const namespace = form.value.namespace.trim() || 'default'
    const response = await getServices(namespace)
    if (version !== serviceFetchVersion) return
    serviceChoices.value = serviceChoicesFromPayload(response.data, namespace)
  } catch {
    if (version === serviceFetchVersion) serviceChoices.value = []
  }
}

async function fetchRoutes() {
  const version = ++routeFetchVersion
  loading.value = true
  try {
    const namespace = namespaceFilter.value === 'all' ? undefined : namespaceFilter.value
    const enabled = stateFilter.value === 'enabled' ? true : stateFilter.value === 'disabled' ? false : undefined
    const [list, statuses] = await Promise.all([
      listGatewayRoutes({
        namespace,
        query: query.value.trim() || undefined,
        enabled,
        page: currentPage.value,
        page_size: pageSize.value,
      }),
      listGatewayRuntime(namespace).catch(() => []),
    ])
    if (version !== routeFetchVersion) return
    routes.value = list.data
    routeTotal.value = list.total
    runtimeByRoute.value = Object.fromEntries(statuses.map((status) => [routeKey(status), status]))
    if (currentPage.value > totalPages.value) currentPage.value = totalPages.value
  } catch (error) {
    if (version !== routeFetchVersion) return
    console.error(error)
    ElMessage.error(text('无法加载网关路由', 'Unable to load gateway routes'))
  } finally {
    if (version === routeFetchVersion) loading.value = false
  }
}

function resetFilters() {
  query.value = ''
  namespaceFilter.value = 'all'
  stateFilter.value = 'all'
  currentPage.value = 1
}

function addTarget() {
  let index = form.value.targets.length + 1
  while (form.value.targets.some((target) => target.id === `target-${index}`)) index += 1
  form.value.targets.push(newTarget(form.value.namespace, index))
}

function removeTarget(index: number) {
  if (form.value.targets.length === 1) {
    ElMessage.warning(text('路由至少需要一个发布目标', 'A route needs at least one release target'))
    return
  }
  form.value.targets.splice(index, 1)
  normalizeSelectedTargets()
}

function normalizeSelectedTargets() {
  const targetIDs = new Set(form.value.targets.map((target) => target.id.trim()).filter(Boolean))
  if (!targetIDs.has(form.value.defaultTargetID)) form.value.defaultTargetID = form.value.targets[0]?.id || ''
  if (!targetIDs.has(form.value.activeTargetID)) form.value.activeTargetID = form.value.targets[0]?.id || ''
}

function applyServiceChoice(target: EditableTarget, value: string) {
  const choice = serviceChoices.value.find((item) => item.key === value)
  if (!choice) return
  target.serviceNamespace = choice.namespace
  target.serviceGroup = choice.group
  target.serviceName = choice.name
}

function openCreate() {
  editorMode.value = 'create'
  editingRevision.value = 0
  form.value = newForm(namespaceFilter.value === 'all' ? 'default' : namespaceFilter.value)
  drawerVisible.value = true
  void fetchServiceChoices()
}

function openEdit(route: GatewayRoute) {
  editorMode.value = 'edit'
  editingRevision.value = route.revision
  const weights = new Map((route.traffic.weighted_targets || []).map((item) => [item.target_id, item.weight]))
  const betaByTarget = new Map((route.traffic.beta_targets || []).map((item) => [item.target_id, item]))
  form.value = {
    namespace: route.namespace,
    id: route.id,
    name: route.name,
    enabled: route.enabled,
    priority: route.priority,
    hostsText: (route.match.hosts || []).join('\n'),
    pathMatchMode: route.match.path ? 'exact' : 'prefix',
    path: route.match.path || route.match.path_prefix || '',
    methods: [...(route.match.methods || [])],
    headersText: stringifyHeaders(route.match.headers),
    timeoutMs: route.timeout_ms,
    filtersText: stringifyFilters(route.filters),
    trafficMode: route.traffic.mode,
    defaultTargetID: route.traffic.default_target_id || route.targets[0]?.id || '',
    activeTargetID: route.traffic.active_target_id || route.targets[0]?.id || '',
    targets: route.targets.map((target) => {
      const beta = betaByTarget.get(target.id)
      return {
        id: target.id,
        name: target.name || '',
        type: target.type,
        serviceNamespace: target.service?.namespace || route.namespace,
        serviceGroup: target.service?.group || 'default',
        serviceName: target.service?.service_name || '',
        staticEndpointsText: stringifyStaticEndpoints(target),
        loadBalance: target.load_balance,
        weight: weights.get(target.id) || 0,
        betaUsersText: (beta?.users || []).join(', '),
        betaTenantsText: (beta?.tenants || []).join(', '),
      }
    }),
  }
  drawerVisible.value = true
  void fetchServiceChoices()
}

function makeTargets(): GatewayTarget[] {
  return form.value.targets.map((target) => {
    const base = {
      id: target.id.trim(),
      name: target.name.trim() || undefined,
      type: target.type,
      load_balance: target.loadBalance,
      healthy_only: target.type === 'service',
    }
    if (target.type === 'service') {
      return {
        ...base,
        service: {
          namespace: target.serviceNamespace.trim() || form.value.namespace.trim() || 'default',
          group: target.serviceGroup.trim() || 'default',
          service_name: target.serviceName.trim(),
        },
      } as GatewayTarget
    }
    return {
      ...base,
      static: { endpoints: parseStaticEndpoints(target.staticEndpointsText) },
    } as GatewayTarget
  })
}

function makeBetaTargets(): GatewayBetaTarget[] {
  return form.value.targets
    .map((target) => ({
      target_id: target.id.trim(),
      users: parseCommaList(target.betaUsersText),
      tenants: parseCommaList(target.betaTenantsText),
    }))
    .filter((target) => target.users.length > 0 || target.tenants.length > 0)
}

function buildRoutePayload(): GatewayRouteInput {
  const routeID = form.value.id.trim()
  const namespace = form.value.namespace.trim() || 'default'
  const targets = makeTargets()
  if (!routeID || !/^[A-Za-z0-9._-]+$/.test(routeID)) throw new Error(text('路由 ID 只能包含字母、数字、点、短横线和下划线', 'Route ID contains unsupported characters'))
  if (!form.value.name.trim()) throw new Error(text('请填写路由名称', 'Route name is required'))
  if (!form.value.path.trim().startsWith('/')) throw new Error(text('匹配路径必须以 / 开头', 'Match path must start with /'))
  if (targets.length === 0 || targets.some((target) => !target.id)) throw new Error(text('每个发布目标都需要 ID', 'Every release target needs an ID'))
  if (new Set(targets.map((target) => target.id)).size !== targets.length) throw new Error(text('发布目标 ID 不能重复', 'Release target IDs must be unique'))
  if (targets.some((target) => target.type === 'service' && !target.service?.service_name)) throw new Error(text('请选择或填写注册服务', 'Select or enter a registry service'))
  if (targets.some((target) => target.type === 'static' && !(target.static?.endpoints.length))) throw new Error(text('静态目标至少需要一个 URL', 'A static target needs at least one URL'))

  const betaTargets = makeBetaTargets()
  const traffic = form.value.trafficMode === 'blue_green'
    ? {
        mode: form.value.trafficMode,
        active_target_id: form.value.activeTargetID,
        beta_targets: betaTargets,
      }
    : {
        mode: form.value.trafficMode,
        default_target_id: form.value.defaultTargetID,
        weighted_targets: form.value.targets.map((target) => ({ target_id: target.id.trim(), weight: Number(target.weight) || 0 })),
        beta_targets: betaTargets,
      }

  if (form.value.trafficMode !== 'blue_green') {
    const totalWeight = form.value.targets.reduce((sum, target) => sum + (Number(target.weight) || 0), 0)
    if (totalWeight !== 100 || form.value.targets.some((target) => (Number(target.weight) || 0) < 1)) {
      throw new Error(text('权重模式下所有发布目标的权重必须为正整数且总和为 100', 'Release weights must be positive integers totaling 100'))
    }
  }

  return {
    namespace,
    id: routeID,
    name: form.value.name.trim(),
    enabled: form.value.enabled,
    priority: Number(form.value.priority) || 0,
    match: {
      hosts: parseLines(form.value.hostsText),
      ...(form.value.pathMatchMode === 'exact'
        ? { path: form.value.path.trim() }
        : { path_prefix: form.value.path.trim() }),
      methods: form.value.methods,
      headers: parseHeaders(form.value.headersText),
    },
    targets,
    traffic,
    filters: parseFilters(form.value.filtersText),
    timeout_ms: Number(form.value.timeoutMs) || 30_000,
  }
}

async function submitRoute() {
  if (!canManage.value) return
  submitting.value = true
  try {
    const payload = buildRoutePayload()
    if (editorMode.value === 'create') {
      await createGatewayRoute(payload)
    } else {
      await updateGatewayRoute({ ...payload, revision: editingRevision.value })
    }
    drawerVisible.value = false
    ElMessage.success(text('网关路由已保存', 'Gateway route saved'))
    await fetchRoutes()
  } catch (error) {
    const message = error instanceof Error ? error.message : text('保存网关路由失败', 'Failed to save gateway route')
    ElMessage.error(message)
  } finally {
    submitting.value = false
  }
}

async function handleToggle(route: GatewayRoute) {
  const nextEnabled = !route.enabled
  try {
    await ElMessageBox.confirm(
      nextEnabled
        ? text(`确定启用网关路由 “${route.name}” 吗？`, `Enable gateway route “${route.name}”?`)
        : text(`确定停用网关路由 “${route.name}” 吗？正在转发的请求不会被中断。`, `Disable gateway route “${route.name}”? Requests already in flight will continue.`),
      nextEnabled ? text('启用网关路由', 'Enable gateway route') : text('停用网关路由', 'Disable gateway route'),
      { confirmButtonText: nextEnabled ? text('启用', 'Enable') : text('停用', 'Disable'), cancelButtonText: text('取消', 'Cancel'), type: 'warning' },
    )
    await setGatewayRouteEnabled(route, nextEnabled)
    await fetchRoutes()
  } catch (error) {
    if (error !== 'cancel') console.error(error)
  }
}

async function handleDelete(route: GatewayRoute) {
  try {
    await ElMessageBox.confirm(
      text(`确定删除网关路由 “${route.name}” 吗？`, `Delete gateway route “${route.name}”?`),
      text('删除网关路由', 'Delete gateway route'),
      { confirmButtonText: text('删除', 'Delete'), cancelButtonText: text('取消', 'Cancel'), type: 'warning' },
    )
    await deleteGatewayRoute(route)
    await fetchRoutes()
  } catch (error) {
    if (error !== 'cancel') console.error(error)
  }
}

watch([query, namespaceFilter, stateFilter, pageSize], () => {
  if (currentPage.value !== 1) {
    currentPage.value = 1
    return
  }
  void fetchRoutes()
})

watch(currentPage, () => { void fetchRoutes() })

watch(() => form.value.namespace, () => {
  if (drawerVisible.value) void fetchServiceChoices()
})

watch(() => form.value.targets.map((target) => target.id).join('|'), normalizeSelectedTargets)

onMounted(() => {
  void fetchRoutes()
})
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
          <el-input v-model="query" :prefix-icon="Search" clearable class="search-input" :placeholder="text('路由 / 服务 / 发布策略', 'Route / service / release')" />
        </div>
        <button class="icon-btn" type="button" :title="text('重置筛选', 'Reset filters')" @click="resetFilters">
          <el-icon><RefreshLeft /></el-icon>
        </button>
        <div class="toolbar-spacer" />
        <div class="pill-group">
          <button type="button" :class="{ active: stateFilter === 'all' }" @click="stateFilter = 'all'">{{ text('全部', 'All') }}</button>
          <button type="button" :class="{ active: stateFilter === 'enabled' }" @click="stateFilter = 'enabled'">{{ text('启用', 'Enabled') }}</button>
          <button type="button" :class="{ active: stateFilter === 'disabled' }" @click="stateFilter = 'disabled'">{{ text('停用', 'Disabled') }}</button>
        </div>
        <el-button v-if="canManage" type="primary" :icon="Plus" @click="openCreate">{{ text('新建网关路由', 'New gateway route') }}</el-button>
      </div>

      <div class="route-list" v-loading="loading">
        <el-table :data="routes" height="100%">
          <el-table-column :label="text('优先级', 'Priority')" width="90">
            <template #default="{ row }"><span class="priority-cell">{{ row.priority }}</span></template>
          </el-table-column>
          <el-table-column :label="text('网关路由', 'Gateway route')" min-width="190">
            <template #default="{ row }">
              <div class="route-cell"><strong>{{ row.name }}</strong><span>{{ row.id }}</span></div>
            </template>
          </el-table-column>
          <el-table-column :label="text('匹配路径', 'Path')" min-width="170">
            <template #default="{ row }"><code class="path-chip">{{ row.match.path_prefix || row.match.path }}</code></template>
          </el-table-column>
          <el-table-column :label="text('发布目标', 'Release targets')" min-width="260">
            <template #default="{ row }">
              <div class="target-summary">
                <span v-for="target in row.targets" :key="target.id"><el-tag size="small" :type="target.type === 'service' ? 'success' : 'warning'">{{ target.id }}</el-tag>{{ targetLabel(target) }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column :label="text('发布策略', 'Release')" width="120">
            <template #default="{ row }"><el-tag effect="plain">{{ releaseLabel(row.traffic.mode) }}</el-tag></template>
          </el-table-column>
          <el-table-column :label="text('运行状态', 'Runtime')" width="120">
            <template #default="{ row }"><span class="status-pill" :class="statusClass(row)">{{ statusText(row) }}</span></template>
          </el-table-column>
          <el-table-column :label="text('更新时间', 'Updated')" width="170">
            <template #default="{ row }">{{ formatDateTime(row.updated_at) }}</template>
          </el-table-column>
          <el-table-column v-if="canManage" :label="text('操作', 'Actions')" width="210" fixed="right">
            <template #default="{ row }">
              <div class="row-actions">
                <el-button link :icon="Switch" @click="handleToggle(row)">{{ row.enabled ? text('停用', 'Disable') : text('启用', 'Enable') }}</el-button>
                <el-button link type="primary" :icon="EditPen" @click="openEdit(row)">{{ text('编辑', 'Edit') }}</el-button>
                <el-button link type="danger" :icon="Delete" @click="handleDelete(row)">{{ text('删除', 'Delete') }}</el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>
        <footer class="route-footer">
          <span>{{ routeTotal }} {{ text('条', 'records') }}</span>
          <el-config-provider :locale="elementLocale">
            <el-pagination v-model:current-page="currentPage" v-model:page-size="pageSize" :page-sizes="[12, 20, 50]" :total="routeTotal" layout="sizes, prev, pager, next" background />
          </el-config-provider>
        </footer>
      </div>
    </section>

    <el-drawer v-model="drawerVisible" :title="editorMode === 'create' ? text('新建网关路由', 'New gateway route') : text('编辑网关路由', 'Edit gateway route')" direction="rtl" size="760px" class="form-drawer">
      <div class="drawer-form">
        <el-form label-position="top">
          <div class="form-grid">
            <el-form-item :label="text('命名空间', 'Namespace')"><el-select v-model="form.namespace" filterable allow-create><el-option v-for="namespace in namespaceOptions" :key="namespace" :label="namespace" :value="namespace" /></el-select></el-form-item>
            <el-form-item :label="text('路由 ID', 'Route ID')"><el-input v-model="form.id" :disabled="editorMode === 'edit'" /></el-form-item>
            <el-form-item :label="text('名称', 'Name')"><el-input v-model="form.name" /></el-form-item>
            <el-form-item :label="text('优先级', 'Priority')"><el-input-number v-model="form.priority" controls-position="right" /></el-form-item>
            <el-form-item :label="text('启用', 'Enabled')"><el-switch v-model="form.enabled" /></el-form-item>
            <el-form-item :label="text('超时毫秒', 'Timeout ms')"><el-input-number v-model="form.timeoutMs" :min="1" :max="300000" :step="1000" controls-position="right" /></el-form-item>
          </div>

          <el-divider content-position="left">{{ text('匹配条件', 'Match') }}</el-divider>
          <div class="form-grid">
            <el-form-item label="Host"><el-input v-model="form.hostsText" type="textarea" :rows="2" :placeholder="text('每行一个，可留空', 'One per line, optional')" /></el-form-item>
            <el-form-item :label="text('路径类型', 'Path type')"><el-select v-model="form.pathMatchMode"><el-option :label="text('前缀匹配', 'Prefix')" value="prefix" /><el-option :label="text('精确匹配', 'Exact')" value="exact" /></el-select></el-form-item>
            <el-form-item :label="form.pathMatchMode === 'exact' ? text('精确路径', 'Exact path') : text('路径前缀', 'Path prefix')"><el-input v-model="form.path" /></el-form-item>
            <el-form-item :label="text('方法', 'Methods')"><el-select v-model="form.methods" multiple collapse-tags><el-option v-for="method in methodOptions" :key="method" :label="method" :value="method" /></el-select></el-form-item>
            <el-form-item :label="text('Header 匹配', 'Header matches')"><el-input v-model="form.headersText" type="textarea" :rows="2" placeholder="X-Env=prod" /></el-form-item>
          </div>

          <div class="section-heading">
            <el-divider content-position="left">{{ text('发布目标：路由 → 服务 / 静态地址', 'Release targets: route → service / static') }}</el-divider>
            <el-button plain type="primary" size="small" :icon="Plus" @click="addTarget">{{ text('添加目标', 'Add target') }}</el-button>
          </div>
          <article v-for="(target, index) in form.targets" :key="`${index}-${target.id}`" class="target-card">
            <div class="target-card-heading">
              <strong>{{ text('目标', 'Target') }} {{ index + 1 }}</strong>
              <el-button link type="danger" :icon="Delete" @click="removeTarget(index)">{{ text('移除', 'Remove') }}</el-button>
            </div>
            <div class="form-grid">
              <el-form-item :label="text('目标 ID', 'Target ID')"><el-input v-model="target.id" /></el-form-item>
              <el-form-item :label="text('展示名', 'Display name')"><el-input v-model="target.name" /></el-form-item>
              <el-form-item :label="text('类型', 'Type')"><el-select v-model="target.type"><el-option :label="text('注册服务', 'Registry service')" value="service" /><el-option :label="text('静态地址', 'Static endpoint')" value="static" /></el-select></el-form-item>
              <el-form-item :label="text('目标内负载均衡', 'Target load balance')"><el-select v-model="target.loadBalance"><el-option v-for="option in loadBalanceOptions" :key="option" :label="option" :value="option" /></el-select></el-form-item>
              <el-form-item v-if="form.trafficMode !== 'blue_green'" :label="text('发布权重', 'Release weight')"><el-input-number v-model="target.weight" :min="0" :max="100" controls-position="right" /></el-form-item>
            </div>
            <template v-if="target.type === 'service'">
              <el-form-item :label="text('从服务列表选择', 'Choose from service list')">
                <el-select filterable clearable :model-value="`${target.serviceNamespace}@@${target.serviceGroup}@@${target.serviceName}`" @change="applyServiceChoice(target, String($event || ''))">
                  <el-option v-for="service in serviceChoices" :key="service.key" :label="`${service.namespace} / ${service.group} / ${service.name}`" :value="service.key" />
                </el-select>
              </el-form-item>
              <div class="form-grid">
                <el-form-item :label="text('服务命名空间', 'Service namespace')"><el-input v-model="target.serviceNamespace" /></el-form-item>
                <el-form-item :label="text('服务分组', 'Service group')"><el-input v-model="target.serviceGroup" /></el-form-item>
                <el-form-item :label="text('服务名', 'Service name')"><el-input v-model="target.serviceName" /></el-form-item>
              </div>
            </template>
            <el-form-item v-else :label="text('静态 Endpoint', 'Static endpoints')">
              <el-input v-model="target.staticEndpointsText" type="textarea" :rows="3" placeholder="https://10.0.8.21:8080 weight=3&#10;https://10.0.8.22:8080 weight=1" />
            </el-form-item>
            <div class="form-grid beta-grid">
              <el-form-item :label="text('BETA 指定用户', 'BETA users')"><el-input v-model="target.betaUsersText" :placeholder="text('逗号分隔，命中后稳定进入此目标', 'Comma-separated; always reaches this target')" /></el-form-item>
              <el-form-item :label="text('BETA 指定租户', 'BETA tenants')"><el-input v-model="target.betaTenantsText" :placeholder="text('逗号分隔，优先于百分比分流', 'Comma-separated; before percentage routing')" /></el-form-item>
            </div>
          </article>

          <el-divider content-position="left">{{ text('发布策略', 'Release policy') }}</el-divider>
          <div class="form-grid">
            <el-form-item :label="text('策略', 'Mode')"><el-select v-model="form.trafficMode"><el-option v-for="option in trafficModeOptions" :key="option" :label="releaseLabel(option)" :value="option" /></el-select></el-form-item>
            <el-form-item v-if="form.trafficMode !== 'blue_green'" :label="text('默认稳定目标', 'Default target')"><el-select v-model="form.defaultTargetID"><el-option v-for="target in form.targets" :key="target.id" :label="target.id || text('未命名目标', 'Unnamed target')" :value="target.id" /></el-select></el-form-item>
            <el-form-item v-else :label="text('当前活动颜色 / 目标', 'Active color / target')"><el-select v-model="form.activeTargetID"><el-option v-for="target in form.targets" :key="target.id" :label="target.id || text('未命名目标', 'Unnamed target')" :value="target.id" /></el-select></el-form-item>
          </div>
          <p class="policy-hint" v-if="form.trafficMode === 'blue_green'">{{ text('普通请求 100% 进入活动目标；BETA 用户/租户可以先预览另一颜色。', 'Normal traffic goes 100% to the active target; BETA users and tenants can preview another color.') }}</p>
          <p class="policy-hint" v-else>{{ text('权重总和必须为 100。BETA 用户/租户始终优先于权重和金丝雀选择。', 'Weights must total 100. BETA users and tenants always take precedence over weighted and canary selection.') }}</p>

          <el-divider content-position="left">{{ text('过滤器', 'Filters') }}</el-divider>
          <el-form-item><el-input v-model="form.filtersText" type="textarea" :rows="4" placeholder="strip_prefix parts=2&#10;add_request_header name=X-Gateway value=eden&#10;set_response_header name=X-Route value=orders" /></el-form-item>
        </el-form>
      </div>
      <template #footer>
        <div class="drawer-footer"><el-button @click="drawerVisible = false">{{ text('取消', 'Cancel') }}</el-button><el-button type="primary" :loading="submitting" @click="submitRoute">{{ text('保存网关路由', 'Save gateway route') }}</el-button></div>
      </template>
    </el-drawer>
  </div>
</template>

<style scoped lang="scss">
.route-shell, .route-panel, .route-list { min-height: 0; }
.route-shell { display: flex; height: 100%; }
.route-panel { display: flex; flex: 1; flex-direction: column; overflow: hidden; }
.route-toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; padding: 0 0 18px; border-bottom: 1px solid var(--border-color); }
.field-item { display: flex; align-items: center; gap: 8px; }
.field-label { color: var(--text-secondary); font-size: 13px; font-weight: 700; }
.small-select { width: 145px; }
.search-input { width: 260px; }
.toolbar-spacer { flex: 1; }
.icon-btn { display: grid; width: 32px; height: 32px; padding: 0; place-items: center; border: 0; border-radius: 6px; color: var(--text-muted); background: transparent; cursor: pointer; }
.icon-btn:hover { color: var(--accent-blue); background: rgba(59, 130, 246, .08); }
.pill-group { display: inline-flex; gap: 2px; padding: 2px; border-radius: 6px; background: rgba(148, 163, 184, .1); }
.pill-group button { height: 30px; padding: 0 9px; border: 0; border-radius: 4px; color: var(--text-muted); background: transparent; font: inherit; font-size: 13px; font-weight: 700; cursor: pointer; }
.pill-group button.active { color: var(--accent-blue); background: rgba(59, 130, 246, .12); }
.route-list { display: flex; flex: 1; flex-direction: column; padding-top: 18px; overflow: hidden; }
:deep(.el-table) { flex: 1; --el-table-bg-color: transparent; --el-table-tr-bg-color: transparent; --el-table-header-bg-color: transparent; --el-table-border-color: var(--border-color); --el-table-text-color: var(--text-primary); }
:deep(.el-table th.el-table__cell), :deep(.el-table td.el-table__cell), :deep(.el-table__fixed-right), :deep(.el-table__fixed-right-patch) { background: transparent !important; }
:deep(.el-table__inner-wrapper::before) { display: none; }
.priority-cell { display: inline-flex; min-width: 34px; height: 26px; align-items: center; justify-content: center; border-radius: 6px; color: var(--accent-blue); background: rgba(59, 130, 246, .08); font-weight: 800; }
.route-cell { display: grid; gap: 3px; }
.route-cell strong { color: var(--text-primary); }
.route-cell span, .policy-hint { color: var(--text-muted); font-size: 12px; }
.path-chip { padding: 4px 8px; border-radius: 6px; color: var(--accent-cyan); background: rgba(6, 182, 212, .08); font: 700 12px 'JetBrains Mono', monospace; }
.target-summary { display: grid; gap: 5px; }
.target-summary span { display: flex; align-items: center; gap: 6px; min-width: 0; overflow: hidden; color: var(--text-secondary); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.status-pill { display: inline-flex; min-height: 25px; align-items: center; justify-content: center; padding: 0 9px; border-radius: 999px; font-size: 12px; font-weight: 800; }
.status-pill.healthy { color: var(--accent-green); background: rgba(16, 185, 129, .12); }
.status-pill.degraded { color: var(--accent-orange); background: rgba(245, 158, 11, .13); }
.status-pill.disabled { color: var(--text-muted); background: rgba(148, 163, 184, .14); }
.row-actions { display: inline-flex; gap: 8px; }
.row-actions :deep(.el-button) { margin-left: 0; }
.route-footer { display: flex; align-items: center; gap: 12px; padding-top: 14px; color: var(--text-muted); font-size: 13px; }
.route-footer > span { margin-right: auto; }
.drawer-form { padding-right: 8px; }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 12px; }
.section-heading { display: flex; align-items: center; gap: 10px; }
.section-heading :deep(.el-divider) { flex: 1; }
.target-card { margin: 10px 0; padding: 14px; border: 1px solid var(--border-color); border-radius: 8px; background: var(--bg-secondary); }
.target-card-heading { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; color: var(--text-primary); }
.beta-grid { padding-top: 4px; border-top: 1px dashed var(--border-color); }
.policy-hint { margin: 0 0 12px; line-height: 1.5; }
.drawer-footer { display: flex; justify-content: flex-end; gap: 10px; }
@media (max-width: 760px) { .form-grid { grid-template-columns: 1fr; } .field-item, .small-select, .search-input, .pill-group { width: 100%; } .route-toolbar { align-items: stretch; } }
</style>

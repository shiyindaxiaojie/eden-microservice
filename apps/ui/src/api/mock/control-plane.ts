export type ConfigContentType = 'properties' | 'yaml' | 'json' | 'text'
export type ConfigHistoryAction = 'publish' | 'delete' | 'rollback'
export type GatewayLoadBalance = 'round_robin' | 'random' | 'weighted'
export type GatewayUpstreamType = 'service' | 'static'
export type GatewayFilterType = 'strip_prefix' | 'add_request_header' | 'set_response_header'

export interface ConfigResource {
  namespace: string
  group: string
  data_id: string
  content: string
  type: ConfigContentType
  md5: string
  revision: number
  description: string
  tags: string[]
  created_at: string
  updated_at: string
  created_by: string
  updated_by: string
}

export interface ConfigHistoryEntry {
  namespace: string
  group: string
  data_id: string
  content: string
  type: ConfigContentType
  md5: string
  revision: number
  action: ConfigHistoryAction
  operator: string
  summary: string
  created_at: string
}

export interface GatewayFilter {
  type: GatewayFilterType
  name?: string
  value?: string
  parts?: number
}

export interface GatewayRoute {
  namespace: string
  id: string
  name: string
  enabled: boolean
  priority: number
  match: {
    hosts: string[]
    path_prefix: string
    path?: string
    methods: string[]
    headers: Record<string, string>
  }
  upstream: {
    type: GatewayUpstreamType
    service_name?: string
    namespace?: string
    urls?: string[]
    load_balance: GatewayLoadBalance
    healthy_only: boolean
  }
  filters: GatewayFilter[]
  timeout_ms: number
  revision: number
  created_at: string
  updated_at: string
  last_status: 'healthy' | 'degraded' | 'disabled'
  request_count: number
  error_rate: number
}

const CONFIGS_KEY = 'eden-microservice.mock.configs'
const CONFIG_HISTORY_KEY = 'eden-microservice.mock.config-history'
const ROUTES_KEY = 'eden-microservice.mock.routes'

const delay = (ms = 120) => new Promise((resolve) => window.setTimeout(resolve, ms))
const copy = <T>(value: T): T => JSON.parse(JSON.stringify(value)) as T
const nowISO = () => new Date().toISOString()

function readStorage<T>(key: string, fallback: T): T {
  if (typeof localStorage === 'undefined') return copy(fallback)
  const raw = localStorage.getItem(key)
  if (!raw) return copy(fallback)

  try {
    return JSON.parse(raw) as T
  } catch {
    return copy(fallback)
  }
}

function writeStorage<T>(key: string, value: T) {
  if (typeof localStorage === 'undefined') return
  localStorage.setItem(key, JSON.stringify(value))
}

function mockMD5(content: string) {
  let hashA = 0x811c9dc5
  let hashB = 0x1000193

  for (let i = 0; i < content.length; i += 1) {
    hashA ^= content.charCodeAt(i)
    hashA = Math.imul(hashA, 0x01000193)
    hashB = Math.imul(hashB ^ content.charCodeAt(i), 0x85ebca6b)
  }

  const left = (hashA >>> 0).toString(16).padStart(8, '0')
  const right = (hashB >>> 0).toString(16).padStart(8, '0')
  return `${left}${right}${left}${right}`.slice(0, 32)
}

function identityOf(item: Pick<ConfigResource, 'namespace' | 'group' | 'data_id'>) {
  return `${item.namespace}@@${item.group}@@${item.data_id}`
}

const initialConfigs: ConfigResource[] = [
  {
    namespace: 'default',
    group: 'DEFAULT_GROUP',
    data_id: 'order-service.yaml',
    type: 'yaml' as ConfigContentType,
    content: 'server:\n  port: 8081\nfeature:\n  paymentAudit: true\n  couponEngine: v2\n',
    md5: '',
    revision: 3,
    description: '订单服务运行配置',
    tags: ['runtime', 'order'],
    created_at: '2026-06-18T09:10:00.000Z',
    updated_at: '2026-06-25T08:30:00.000Z',
    created_by: 'system',
    updated_by: 'admin',
  },
  {
    namespace: 'default',
    group: 'DEFAULT_GROUP',
    data_id: 'gateway.properties',
    type: 'properties' as ConfigContentType,
    content: 'gateway.timeout=30000\ngateway.trace=true\ngateway.lb=round_robin\n',
    md5: '',
    revision: 5,
    description: '网关默认策略',
    tags: ['gateway'],
    created_at: '2026-06-19T11:20:00.000Z',
    updated_at: '2026-06-25T07:45:00.000Z',
    created_by: 'admin',
    updated_by: 'developer',
  },
  {
    namespace: 'prod',
    group: 'PAYMENT',
    data_id: 'payment-client.json',
    type: 'json' as ConfigContentType,
    content: '{\n  "provider": "mock-pay",\n  "retry": 2,\n  "timeoutMs": 2500\n}\n',
    md5: '',
    revision: 2,
    description: '支付客户端配置',
    tags: ['payment', 'client'],
    created_at: '2026-06-20T02:12:00.000Z',
    updated_at: '2026-06-24T16:10:00.000Z',
    created_by: 'admin',
    updated_by: 'admin',
  },
].map((item) => ({ ...item, md5: mockMD5(item.content) }))

const initialHistory: ConfigHistoryEntry[] = initialConfigs.map((item) => ({
  namespace: item.namespace,
  group: item.group,
  data_id: item.data_id,
  content: item.content,
  type: item.type,
  md5: item.md5,
  revision: item.revision,
  action: 'publish',
  operator: item.updated_by,
  summary: '初始化演示版本',
  created_at: item.updated_at,
}))

const initialRoutes: GatewayRoute[] = [
  {
    namespace: 'default',
    id: 'order-api',
    name: '订单 API',
    enabled: true,
    priority: 10,
    match: {
      hosts: ['api.eden.local'],
      path_prefix: '/api/orders',
      methods: ['GET', 'POST', 'PUT'],
      headers: { 'X-Env': 'demo' },
    },
    upstream: {
      type: 'service',
      service_name: 'order-center',
      namespace: 'default',
      load_balance: 'round_robin',
      healthy_only: true,
    },
    filters: [
      { type: 'strip_prefix', parts: 2 },
      { type: 'add_request_header', name: 'X-Gateway', value: 'eden' },
    ],
    timeout_ms: 30000,
    revision: 4,
    created_at: '2026-06-18T03:40:00.000Z',
    updated_at: '2026-06-25T08:20:00.000Z',
    last_status: 'healthy',
    request_count: 128430,
    error_rate: 0.018,
  },
  {
    namespace: 'default',
    id: 'user-api',
    name: '用户 API',
    enabled: true,
    priority: 20,
    match: {
      hosts: ['api.eden.local'],
      path_prefix: '/api/users',
      methods: ['GET', 'POST'],
      headers: {},
    },
    upstream: {
      type: 'service',
      service_name: 'user-center',
      namespace: 'default',
      load_balance: 'random',
      healthy_only: true,
    },
    filters: [{ type: 'strip_prefix', parts: 2 }],
    timeout_ms: 20000,
    revision: 2,
    created_at: '2026-06-19T06:30:00.000Z',
    updated_at: '2026-06-24T10:00:00.000Z',
    last_status: 'healthy',
    request_count: 84210,
    error_rate: 0.011,
  },
  {
    namespace: 'prod',
    id: 'legacy-payment',
    name: '旧支付出口',
    enabled: false,
    priority: 90,
    match: {
      hosts: ['pay.eden.local'],
      path_prefix: '/legacy/pay',
      methods: ['POST'],
      headers: {},
    },
    upstream: {
      type: 'static',
      urls: ['http://10.0.8.21:8080', 'http://10.0.8.22:8080'],
      load_balance: 'weighted',
      healthy_only: false,
    },
    filters: [{ type: 'set_response_header', name: 'X-Route-State', value: 'legacy' }],
    timeout_ms: 45000,
    revision: 7,
    created_at: '2026-06-15T08:00:00.000Z',
    updated_at: '2026-06-23T14:00:00.000Z',
    last_status: 'disabled',
    request_count: 9280,
    error_rate: 0.034,
  },
]

function nextConfigRevision(configs: ConfigResource[]) {
  return configs.reduce((max, item) => Math.max(max, item.revision), 0) + 1
}

function nextRouteRevision(routes: GatewayRoute[]) {
  return routes.reduce((max, item) => Math.max(max, item.revision), 0) + 1
}

export async function listMockConfigs() {
  await delay()
  return readStorage(CONFIGS_KEY, initialConfigs)
}

export async function listMockConfigHistory(target?: Pick<ConfigResource, 'namespace' | 'group' | 'data_id'>) {
  await delay(80)
  const history = readStorage(CONFIG_HISTORY_KEY, initialHistory)
  if (!target) return history
  const key = identityOf(target)
  return history.filter((item) => identityOf(item) === key)
}

export async function saveMockConfig(input: Omit<ConfigResource, 'md5' | 'revision' | 'created_at' | 'updated_at' | 'created_by' | 'updated_by'> & Partial<Pick<ConfigResource, 'revision' | 'created_at' | 'created_by'>>) {
  await delay()
  const configs = readStorage(CONFIGS_KEY, initialConfigs)
  const history = readStorage(CONFIG_HISTORY_KEY, initialHistory)
  const normalized: ConfigResource = {
    namespace: input.namespace.trim() || 'default',
    group: input.group.trim() || 'DEFAULT_GROUP',
    data_id: input.data_id.trim(),
    content: input.content,
    type: input.type,
    md5: mockMD5(input.content),
    revision: nextConfigRevision(configs),
    description: input.description.trim(),
    tags: input.tags.map((tag) => tag.trim()).filter(Boolean),
    created_at: input.created_at || nowISO(),
    updated_at: nowISO(),
    created_by: input.created_by || 'demo-user',
    updated_by: 'demo-user',
  }
  const key = identityOf(normalized)
  const existingIndex = configs.findIndex((item) => identityOf(item) === key)

  if (existingIndex >= 0) {
    const previous = configs[existingIndex]!
    normalized.created_at = previous.created_at
    normalized.created_by = previous.created_by
    configs[existingIndex] = normalized
  } else {
    configs.unshift(normalized)
  }

  history.unshift({
    namespace: normalized.namespace,
    group: normalized.group,
    data_id: normalized.data_id,
    content: normalized.content,
    type: normalized.type,
    md5: normalized.md5,
    revision: normalized.revision,
    action: 'publish',
    operator: normalized.updated_by,
    summary: existingIndex >= 0 ? '发布新版本' : '创建配置',
    created_at: normalized.updated_at,
  })

  writeStorage(CONFIGS_KEY, configs)
  writeStorage(CONFIG_HISTORY_KEY, history)
  return copy(normalized)
}

export async function deleteMockConfig(target: Pick<ConfigResource, 'namespace' | 'group' | 'data_id'>) {
  await delay()
  const configs = readStorage(CONFIGS_KEY, initialConfigs)
  const history = readStorage(CONFIG_HISTORY_KEY, initialHistory)
  const key = identityOf(target)
  const existing = configs.find((item) => identityOf(item) === key)
  const nextConfigs = configs.filter((item) => identityOf(item) !== key)

  if (existing) {
    history.unshift({
      namespace: existing.namespace,
      group: existing.group,
      data_id: existing.data_id,
      content: existing.content,
      type: existing.type,
      md5: existing.md5,
      revision: existing.revision + 1,
      action: 'delete',
      operator: 'demo-user',
      summary: '删除配置',
      created_at: nowISO(),
    })
  }

  writeStorage(CONFIGS_KEY, nextConfigs)
  writeStorage(CONFIG_HISTORY_KEY, history)
}

export async function listMockRoutes() {
  await delay()
  return readStorage(ROUTES_KEY, initialRoutes)
}

export async function saveMockRoute(input: Omit<GatewayRoute, 'revision' | 'created_at' | 'updated_at' | 'last_status' | 'request_count' | 'error_rate'> & Partial<Pick<GatewayRoute, 'revision' | 'created_at' | 'request_count' | 'error_rate'>>) {
  await delay()
  const routes = readStorage(ROUTES_KEY, initialRoutes)
  const existingIndex = routes.findIndex((item) => item.id === input.id && item.namespace === input.namespace)
  const previous = existingIndex >= 0 ? routes[existingIndex] : undefined
  const route: GatewayRoute = {
    ...copy(input),
    revision: nextRouteRevision(routes),
    created_at: previous?.created_at || input.created_at || nowISO(),
    updated_at: nowISO(),
    last_status: input.enabled ? 'healthy' : 'disabled',
    request_count: previous?.request_count ?? input.request_count ?? 0,
    error_rate: previous?.error_rate ?? input.error_rate ?? 0,
  }

  if (existingIndex >= 0) {
    routes[existingIndex] = route
  } else {
    routes.unshift(route)
  }

  routes.sort((left, right) => left.priority - right.priority || left.id.localeCompare(right.id))
  writeStorage(ROUTES_KEY, routes)
  return copy(route)
}

export async function toggleMockRoute(namespace: string, id: string, enabled: boolean) {
  await delay(80)
  const routes = readStorage(ROUTES_KEY, initialRoutes)
  const next = routes.map((route) =>
    route.namespace === namespace && route.id === id
      ? {
          ...route,
          enabled,
          last_status: enabled ? 'healthy' : 'disabled',
          revision: nextRouteRevision(routes),
          updated_at: nowISO(),
        }
      : route,
  )
  writeStorage(ROUTES_KEY, next)
  return next.find((route) => route.namespace === namespace && route.id === id)
}

export async function deleteMockRoute(namespace: string, id: string) {
  await delay()
  const routes = readStorage(ROUTES_KEY, initialRoutes)
  writeStorage(ROUTES_KEY, routes.filter((route) => route.namespace !== namespace || route.id !== id))
}

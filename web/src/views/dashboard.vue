<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  getClusterStats,
  getEvents,
  getLogFiles,
  getLogs,
  getNamespaces,
  getRbacUsers,
  type ClusterStats,
  type RegistryEvent,
} from '../api/registry'
import { useI18n } from '../utils/i18n'

interface LogFileOption {
  name: string
  file: string
}

interface ParsedLogLine {
  raw: string
  parsed: boolean
  timestamp: string
  level: string
  levelClass: string
  thread?: string
  scope?: string
  message?: string
}

interface MemorySample {
  timestamp: number
  value: number
}

type DashboardPanel = 'events' | 'logs'
type EventTimeFilter = 'all' | '1h' | '24h' | '7d'

const AUTO_REFRESH_MS = 10000
const MEMORY_TREND_WINDOW_MS = 10 * 60 * 1000
const MEMORY_CHART_WIDTH = 320
const MEMORY_CHART_HEIGHT = 118
const MEMORY_CHART_PADDING_X = 10
const MEMORY_CHART_PADDING_TOP = 14
const MEMORY_CHART_PADDING_BOTTOM = 18
const KNOWN_EVENT_TYPES = [
  'service_register',
  'service_online',
  'service_offline',
  'service_heartbeat',
  'service_remove',
  'registry_node_sync',
  'health_change',
] as const

const { t, locale } = useI18n()
const stats = ref<ClusterStats | null>(null)
const events = ref<RegistryEvent[]>([])
const logFiles = ref<LogFileOption[]>([])
const namespaceCount = ref<number | null>(null)
const defaultNamespaceCount = ref(0)
const customNamespaceCount = ref(0)
const userCount = ref<number | null>(null)
const builtinUserCount = ref(0)
const customUserCount = ref(0)
const memoryHistory = ref<MemorySample[]>([])
const activeLogFile = ref('')
const logs = ref<string[]>([])
const loading = ref(true)
const logsLoading = ref(false)
const activePanel = ref<DashboardPanel>('events')
const activeEventType = ref('all')
const activeEventTime = ref<EventTimeFilter>('all')
const autoScrollLogs = ref(true)
const logsScrollRef = ref<HTMLDivElement | null>(null)
const logLineCount = ref(100)
const logSearchText = ref('')
const logSearchDate = ref<[Date, Date] | null>(null)
const logOffset = ref(0)
const logTotal = ref(0)
const logCurrentPage = ref(1)
const hasMoreLogs = ref(true)
const isFetchingMore = ref(false)
let timer: ReturnType<typeof setInterval> | null = null

const eventLineCount = ref(100)
const eventOffset = ref(0)
const eventTotal = ref(0)
const currentPage = ref(1)
const hasMoreEvents = ref(true)
const isFetchingMoreEvents = ref(false)
const eventSearchText = ref('')
const eventSearchDate = ref<[Date, Date] | null>(null)
const eventsScrollRef = ref<HTMLDivElement | null>(null)

const leaderNoteText = computed(() => {
  if (!stats.value) return '-'
  if (stats.value.is_leader) {
    return locale.value === 'zh' ? '当前节点为 Leader' : 'This node is the leader'
  }

  const leaderAddr = stats.value.leader_addr ? compactAddress(stats.value.leader_addr) : t.value.common.unknown
  return `${t.value.dashboard.leaderAddr}: ${leaderAddr}`
})

const eventsTabText = computed(() => (locale.value === 'zh' ? '事件' : 'Events'))
const logsTabText = computed(() => (locale.value === 'zh' ? '日志' : 'Logs'))
const logSelectPlaceholder = computed(() =>
  locale.value === 'zh' ? '选择日志文件' : 'Select log file',
)
const loadingLabel = computed(() => (locale.value === 'zh' ? '加载中...' : 'Loading...'))
const noLogFilesLabel = computed(() =>
  locale.value === 'zh' ? '当前未配置日志文件' : 'No log files configured',
)
const noLogsLabel = computed(() =>
  locale.value === 'zh' ? '暂无日志内容' : 'No log content',
)
const noMatchedEventsLabel = computed(() =>
  locale.value === 'zh' ? '没有符合筛选条件的事件' : 'No matching events',
)
const logLinesLabel = computed(() =>
  locale.value === 'zh' ? `已加载 ${logs.value.length} 行` : `Loaded ${logs.value.length} lines`,
)
const logLineOptions = computed(() => [
  { value: 100, label: locale.value === 'zh' ? '100 行' : '100 Lines' },
  { value: 500, label: locale.value === 'zh' ? '500 行' : '500 Lines' },
  { value: 1000, label: locale.value === 'zh' ? '1000 行' : '1000 Lines' },
  { value: 2000, label: locale.value === 'zh' ? '2000 行' : '2000 Lines' },
  { value: 5000, label: locale.value === 'zh' ? '5000 行' : '5000 Lines' },
])
const eventLineOptions = computed(() => [
  { value: 100, label: locale.value === 'zh' ? '100 条' : '100 Events' },
  { value: 500, label: locale.value === 'zh' ? '500 条' : '500 Events' },
  { value: 1000, label: locale.value === 'zh' ? '1000 条' : '1000 Events' },
  { value: 2000, label: locale.value === 'zh' ? '2000 条' : '2000 Events' },
  { value: 5000, label: locale.value === 'zh' ? '5000 条' : '5000 Events' },
])
const eventTypeFilterLabel = computed(() => (locale.value === 'zh' ? '事件类型' : 'Event Type'))
const eventTimeFilterLabel = computed(() => (locale.value === 'zh' ? '发生时间' : 'Occurred'))
const autoScrollLabel = computed(() => (locale.value === 'zh' ? '自动滚动' : 'Auto Scroll'))

const datePickerShortcuts = computed(() => [
  {
    text: locale.value === 'zh' ? '近 30 分钟' : 'Last 30m',
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setTime(start.getTime() - 30 * 60 * 1000)
      return [start, end]
    },
  },
  {
    text: locale.value === 'zh' ? '近 1 小时' : 'Last 1h',
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setTime(start.getTime() - 3600 * 1000)
      return [start, end]
    },
  },
  {
    text: locale.value === 'zh' ? '近 6 小时' : 'Last 6h',
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setTime(start.getTime() - 3600 * 1000 * 6)
      return [start, end]
    },
  },
  {
    text: locale.value === 'zh' ? '近 24 小时' : 'Last 24h',
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setTime(start.getTime() - 3600 * 1000 * 24)
      return [start, end]
    },
  },
  {
    text: locale.value === 'zh' ? '最近 3 天' : 'Last 3 days',
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setTime(start.getTime() - 3600 * 1000 * 24 * 3)
      return [start, end]
    },
  },
  {
    text: locale.value === 'zh' ? '最近 7 天' : 'Last 7 days',
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setTime(start.getTime() - 3600 * 1000 * 24 * 7)
      return [start, end]
    },
  },
])

const logPickerShortcuts = computed(() => datePickerShortcuts.value)

const eventTimeOptions = computed(() => [
  { value: 'all', label: locale.value === 'zh' ? '全部' : 'All' },
  { value: '30m', label: locale.value === 'zh' ? '近 30 分钟' : 'Last 30m' },
  { value: '1h', label: locale.value === 'zh' ? '近 1 小时' : 'Last 1h' },
  { value: '6h', label: locale.value === 'zh' ? '近 6 小时' : 'Last 6h' },
  { value: '24h', label: locale.value === 'zh' ? '近 24 小时' : 'Last 24h' },
])

const activeEventQuickTime = ref('all')

const localizedRoleText = computed(() => {
  const role = (stats.value?.role || '').toLowerCase()
  if (!role) return t.value.common.unknown

  const roleMap: Record<string, { zh: string; en: string }> = {
    leader: { zh: 'Leader', en: 'Leader' },
    follower: { zh: 'Follower', en: 'Follower' },
    candidate: { zh: 'Candidate', en: 'Candidate' },
    peer: { zh: '节点', en: 'Peer' },
    standalone: { zh: '单机', en: 'Standalone' },
  }

  const matched = roleMap[role]
  if (matched) {
    return locale.value === 'zh' ? matched.zh : matched.en
  }

  return stats.value?.role || t.value.common.unknown
})

const deploymentModeText = computed(() => {
  if (!stats.value) return '-'
  return stats.value.environment === 'cluster'
    ? (locale.value === 'zh' ? '集群部署' : 'Cluster deployment')
    : (locale.value === 'zh' ? '单机部署' : 'Standalone deployment')
})

const deploymentUnitText = computed(() =>
  locale.value === 'zh' ? '个节点' : 'nodes',
)

const serviceUnitText = computed(() =>
  locale.value === 'zh' ? '个服务' : 'services',
)

const namespaceUnitText = computed(() =>
  locale.value === 'zh' ? '个命名空间' : 'namespaces',
)

const serviceHealthRateValue = computed(() => {
  if (!stats.value?.instance_count) return 0
  return Math.min(100, Math.max(0, stats.value.health_rate))
})

const serviceHealthComparisonText = computed(() => {
  if (!stats.value) return '-'
  if (stats.value.instance_count === 0) {
    return locale.value === 'zh' ? '暂无可用健康对比' : 'No health comparison available'
  }

  return locale.value === 'zh'
    ? `健康率 ${stats.value.health_rate.toFixed(1)}%`
    : `${stats.value.health_rate.toFixed(1)}% health rate`
})

const namespaceLabelText = computed(() => {
  if (namespaceCount.value == null) return '-'
  if (namespaceCount.value === 0) {
    return locale.value === 'zh' ? '暂无命名空间' : 'No namespaces yet'
  }

  return locale.value === 'zh' ? '已创建的命名空间' : 'Created namespaces'
})

const memoryTrendModel = computed(() => {
  const samples = memoryHistory.value.filter((item) => Number.isFinite(item.value))
  const baselineY = MEMORY_CHART_HEIGHT - MEMORY_CHART_PADDING_BOTTOM

  if (samples.length === 0) {
    return {
      points: [] as Array<MemorySample & { x: number; y: number }>,
      polyline: '',
      areaPath: '',
      lastPoint: null as (MemorySample & { x: number; y: number }) | null,
      firstSample: null as MemorySample | null,
      lastSample: null as MemorySample | null,
      min: null as number | null,
      max: null as number | null,
      delta: null as number | null,
    }
  }

  const values = samples.map((item) => item.value)
  const min = Math.min(...values)
  const max = Math.max(...values)
  const innerWidth = MEMORY_CHART_WIDTH - MEMORY_CHART_PADDING_X * 2
  const innerHeight = MEMORY_CHART_HEIGHT - MEMORY_CHART_PADDING_TOP - MEMORY_CHART_PADDING_BOTTOM
  const valueRange = Math.max(1, max - min)

  const points = samples.map((sample, index) => {
    const ratio = samples.length === 1 ? 0.5 : index / (samples.length - 1)
    const normalized = (sample.value - min) / valueRange
    const x = MEMORY_CHART_PADDING_X + ratio * innerWidth
    const y = MEMORY_CHART_PADDING_TOP + (1 - normalized) * innerHeight
    return {
      ...sample,
      x: Number(x.toFixed(2)),
      y: Number(y.toFixed(2)),
    }
  })

  const polyline = points.map((point) => `${point.x},${point.y}`).join(' ')
  const areaPath = points.length === 0
    ? ''
    : `${points.map((point, index) => `${index === 0 ? 'M' : 'L'} ${point.x} ${point.y}`).join(' ')} L ${points.at(-1)?.x ?? 0} ${baselineY} L ${points[0]?.x ?? 0} ${baselineY} Z`

  return {
    points,
    polyline,
    areaPath,
    lastPoint: points.at(-1) ?? null,
    firstSample: samples[0] ?? null,
    lastSample: samples.at(-1) ?? null,
    min,
    max,
    delta: samples.length > 1 ? (samples.at(-1)?.value ?? 0) - (samples[0]?.value ?? 0) : null,
  }
})

const memoryTrendMinText = computed(() =>
  memoryTrendModel.value.min == null ? '-' : formatMemory(memoryTrendModel.value.min),
)

const memoryTrendMaxText = computed(() =>
  memoryTrendModel.value.max == null ? '-' : formatMemory(memoryTrendModel.value.max),
)

const memoryTrendDeltaText = computed(() => {
  const delta = memoryTrendModel.value.delta
  if (delta == null) {
    return locale.value === 'zh' ? '采样中...' : 'Collecting samples'
  }

  const prefix = delta > 0 ? '+' : delta < 0 ? '-' : '±'
  const value = formatMemory(Math.abs(delta))
  return locale.value === 'zh'
    ? `较 10 分钟前 ${prefix}${value}`
    : `${prefix}${value} vs 10m ago`
})

const memoryTrendEmptyText = computed(() =>
  locale.value === 'zh' ? '采样几次后将显示趋势' : 'The trend will appear after a few samples',
)

const memoryTrendPolyline = computed(() => memoryTrendModel.value.polyline)
const memoryTrendAreaPath = computed(() => memoryTrendModel.value.areaPath)
const memoryTrendLastPoint = computed(() => memoryTrendModel.value.lastPoint)

const localizedLogFiles = computed(() =>
  logFiles.value.map((item) => ({
    ...item,
    label: localizeLogFileName(item.name),
  })),
)

const parsedLogs = computed<ParsedLogLine[]>(() => {
  let currentLogs = logs.value
  if (logSearchText.value) {
    const searchLower = logSearchText.value.toLowerCase()
    currentLogs = currentLogs.filter(line => line.toLowerCase().includes(searchLower))
  }
  return currentLogs.map((line) => parseLogLine(line))
})

const eventTypeOptionsRaw = computed(() => {
  const seenTypes = new Set(events.value.map((event) => event.type).filter(Boolean))
  return [...new Set([...KNOWN_EVENT_TYPES, ...seenTypes])]
})

const eventSummaryText = computed(() => {
  if (eventTotal.value === 0) return '0 条'
  return `${eventTotal.value} 条`
})

const filteredEvents = computed(() => {
  return [...events.value]
})

async function fetchEvents(forceOffset = -1) {
  try {
    let startTime = ''
    let endTime = ''

    if (eventSearchDate.value && eventSearchDate.value.length === 2) {
      startTime = eventSearchDate.value[0].toISOString()
      endTime = eventSearchDate.value[1].toISOString()
    }

    const offset = forceOffset >= 0 ? forceOffset : (currentPage.value - 1) * eventLineCount.value
    const response = await getEvents(
      eventLineCount.value, 
      offset, 
      eventSearchText.value, 
      '', // Date is replaced by startTime/endTime range
      activeEventType.value !== 'all' ? activeEventType.value : '',
      '',
      startTime,
      endTime
    )
    events.value = response.data.data || []
    eventTotal.value = response.data.total || 0
  } catch (e) {
    console.error(e)
  }
}

function handlePageChange(page: number) {
  currentPage.value = page
  fetchEvents()
}

function handleSizeChange(size: number) {
  eventLineCount.value = size
  currentPage.value = 1
  fetchEvents()
}

async function handleEventScroll(e: Event) {
  // Infinite scroll disabled in favor of pagination
}

async function fetchLogs(file = activeLogFile.value, force = false, forceOffset = -1) {
  if (!file) {
    logs.value = []
    logTotal.value = 0
    return
  }

  if (!force && !autoScrollLogs.value && logs.value.length > 0) {
    return
  }

  logsLoading.value = true
  try {
    const offset = forceOffset >= 0 ? forceOffset : (logCurrentPage.value - 1) * logLineCount.value
    const response = await getLogs(file, logLineCount.value, offset, logSearchText.value)
    
    logs.value = response.data.data || []
    logTotal.value = response.data.total || 0
    
    if (force || autoScrollLogs.value) {
      await nextTick()
      syncLogsScroll()
    }
  } catch (error) {
    console.error('fetch dashboard logs failed', error)
    logs.value = []
    logTotal.value = 0
  } finally {
    logsLoading.value = false
  }
}

function handleLogPageChange(page: number) {
  logCurrentPage.value = page
  autoScrollLogs.value = false
  fetchLogs(activeLogFile.value, true)
}

function handleLogSizeChange(size: number) {
  logLineCount.value = size
  logCurrentPage.value = 1
  fetchLogs(activeLogFile.value, true)
}

async function handleLogScroll(e: Event) {
  // Infinite scroll disabled in favor of pagination
}

// Watchers
watch([activePanel, activeLogFile, logLineCount, logSearchText, logSearchDate], () => {
  if (activePanel.value === 'logs') {
    logCurrentPage.value = 1
    fetchLogs(activeLogFile.value, true)
  }
})

watch([eventSearchText, eventSearchDate, activeEventType, eventLineCount], () => {
  if (activePanel.value === 'events') {
    currentPage.value = 1
    fetchEvents()
  }
})

async function fetchData() {
  try {
    const dateStr = eventSearchDate.value ? new Date(eventSearchDate.value).toISOString().split('T')[0] : ''
    const [statsRes, eventsRes, logFilesRes, namespacesRes, usersRes] = await Promise.allSettled([
      getClusterStats(),
      getEvents(
        eventLineCount.value, 
        0, 
        eventSearchText.value, 
        dateStr,
        activeEventType.value !== 'all' ? activeEventType.value : ''
      ),
      getLogFiles(),
      getNamespaces(),
      getRbacUsers(),
    ])

    if (statsRes.status === 'fulfilled') {
      stats.value = statsRes.value.data
      recordMemorySample(statsRes.value.data.memory_usage)
    }

    if (eventsRes.status === 'fulfilled') {
      events.value = eventsRes.value.data.data || []
      eventTotal.value = eventsRes.value.data.total || 0
    }

    if (logFilesRes.status === 'fulfilled') {
      const files = logFilesRes.value.data || []
      logFiles.value = files

      const activeStillExists = files.some((item) => item.file === activeLogFile.value)
      const nextActiveFile = activeStillExists ? activeLogFile.value : files[0]?.file || ''

      if (nextActiveFile !== activeLogFile.value) {
        activeLogFile.value = nextActiveFile
        logCurrentPage.value = 1
      }
      
      await fetchLogs(activeLogFile.value)
    }

    if (namespacesRes.status === 'fulfilled') {
      const namespaces = namespacesRes.value.data || []
      const defaultCount = namespaces.filter((item) => item.name === 'default').length

      namespaceCount.value = namespaces.length
      defaultNamespaceCount.value = defaultCount
      customNamespaceCount.value = Math.max(0, namespaces.length - defaultCount)
    }

    if (usersRes.status === 'fulfilled') {
      const users = usersRes.value.data || []
      const builtinCount = users.filter((item) => item.is_builtin).length

      userCount.value = users.length
      builtinUserCount.value = builtinCount
      customUserCount.value = Math.max(0, users.length - builtinCount)
    }
  } catch (error) {
    console.error('fetch dashboard data failed', error)
  } finally {
    loading.value = false
  }
}

function compactAddress(addr: string) {
  if (!addr) return ''

  try {
    const url = new URL(addr)
    return url.host
  } catch {
    return addr.replace(/^https?:\/\//, '').replace(/\/$/, '')
  }
}

function formatTime(ts: string) {
  if (!ts) return '-'
  const date = new Date(ts)
  return date.toLocaleString(locale.value === 'zh' ? 'zh-CN' : 'en-US', {
    hour12: false,
  })
}

function formatMemory(bytes: number | undefined) {
  if (bytes == null || !Number.isFinite(bytes)) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`
}

function recordMemorySample(bytes: number | undefined) {
  if (bytes == null || !Number.isFinite(bytes)) return

  const now = Date.now()
  const cutoff = now - MEMORY_TREND_WINDOW_MS
  const preservedSamples = memoryHistory.value.filter((item) => item.timestamp >= cutoff)

  memoryHistory.value = [
    ...preservedSamples,
    {
      timestamp: now,
      value: bytes,
    },
  ]
}

function getEventTypeMeta(type: string) {
  const metas: Record<string, { zh: string; en: string; tone: string }> = {
    service_register: { zh: '服务注册', en: 'Service Register', tone: 'green' },
    service_online: { zh: '服务上线', en: 'Service Online', tone: 'green' },
    service_offline: { zh: '服务下线', en: 'Service Offline', tone: 'red' },
    service_heartbeat: { zh: '服务心跳', en: 'Service Heartbeat', tone: 'blue' },
    service_remove: { zh: '服务移除', en: 'Service Remove', tone: 'orange' },
    registry_node_sync: { zh: '节点同步', en: 'Node Sync', tone: 'cyan' },
    register: { zh: '注册', en: 'Register', tone: 'green' },
    deregister: { zh: '下线', en: 'Deregister', tone: 'red' },
    health_change: { zh: '健康变更', en: 'Health Change', tone: 'orange' },
  }

  return metas[type] || { zh: type || '事件', en: type || 'Event', tone: 'blue' }
}

function getEventTypeLabel(type: string) {
  const meta = getEventTypeMeta(type)
  return locale.value === 'zh' ? meta.zh : meta.en
}

function getEventToneClass(type: string) {
  return `is-${getEventTypeMeta(type).tone}`
}

function getEventMessage(event: RegistryEvent) {
  if (!event.message) return '-'
  if (locale.value !== 'zh') return event.message

  const exactMessages: Record<string, string> = {
    'Instance registered': '实例已注册',
    'Heartbeat received': '收到实例心跳',
    'Heartbeat recovered instance to online': '实例已恢复上线',
    'Instance manually set to offline': '实例已手动下线',
    'Instance manually set to online': '实例已手动上线',
    'Instance restored online': '实例已恢复上线',
    'Health checker marked instance offline after missed heartbeats': '心跳丢失，健康检查已将其标记为下线',
    'Instance removed after retention window': '实例在保留窗口结束后已移除',
    'Full sync completed': '节点全量同步完成',
  }

  return exactMessages[event.message] || event.message
}

function localizeLogFileName(name: string) {
  if (locale.value !== 'zh') return name

  if (/info/i.test(name)) return '信息日志'
  if (/error/i.test(name)) return '错误日志'
  if (/access/i.test(name)) return '访问日志'

  return name
}

function parseLogLine(line: string): ParsedLogLine {
  const match = line.match(/^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d+)\s+\[([A-Z]+)\](?:\s+\[(.*?)\])?(?:\s+\[(.*?)\])?\s*(.*)$/)
  if (!match) {
    return {
      raw: line,
      parsed: false,
      timestamp: '',
      level: '',
      levelClass: 'plain',
    }
  }

  const [, timestamp = '', level = '', thread, scope, message] = match
  return {
    raw: line,
    parsed: true,
    timestamp,
    level,
    levelClass: level.toLowerCase(),
    thread,
    scope,
    message: message?.trim() || '',
  }
}

function syncLogsScroll() {
  if (!autoScrollLogs.value || activePanel.value !== 'logs') {
    return
  }

  requestAnimationFrame(() => {
    const element = logsScrollRef.value
    if (!element) return
    element.scrollTop = element.scrollHeight
  })
}

watch(logLineCount, () => {
  fetchLogs(activeLogFile.value, true)
})

watch(eventLineCount, () => {
  eventOffset.value = 0
  fetchEvents()
})

watch([activeEventType, eventSearchDate, activeEventQuickTime], () => {
  currentPage.value = 1
  fetchEvents()
})

let searchTimer: ReturnType<typeof setTimeout> | null = null
watch([logSearchText, eventSearchText], () => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    logCurrentPage.value = 1
    fetchLogs(activeLogFile.value, true)
    currentPage.value = 1
    fetchEvents()
  }, 500)
})

watch(activeLogFile, (file, previousFile) => {
  if (!file) {
    logs.value = []
    return
  }

  if (file !== previousFile) {
    logCurrentPage.value = 1
    fetchLogs(file, true)
  }
})

watch(activePanel, async (panel) => {
  if (panel === 'logs') {
    await nextTick()
    syncLogsScroll()
  }
})

watch(autoScrollLogs, async (enabled) => {
  if (enabled) {
    await nextTick()
    syncLogsScroll()
  }
})

watch(logSearchText, async () => {
  await nextTick()
  syncLogsScroll()
})

onMounted(() => {
  fetchData()
  timer = setInterval(fetchData, AUTO_REFRESH_MS)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div class="dashboard-shell">
    <section class="metric-grid" data-guide="dashboard-metrics">
      <article class="metric-card is-blue">
        <div class="metric-card-head">
          <div class="metric-card-title">
            <div class="metric-icon"><el-icon><Connection /></el-icon></div>
            <h3 class="metric-heading">{{ locale === 'zh' ? '节点' : 'Nodes' }}</h3>
          </div>
          <span class="metric-badge">{{ stats?.mode?.toUpperCase() || '--' }}</span>
        </div>

        <div class="metric-main">
          <div class="metric-value-row">
            <div class="metric-value">{{ stats?.node_count ?? '-' }}</div>
            <small class="metric-unit">{{ deploymentUnitText }}</small>
          </div>
          <div class="metric-inline-meta">
            <span>{{ deploymentModeText }}</span>
            <span>{{ localizedRoleText }}</span>
          </div>
          <div class="metric-note" :title="leaderNoteText">{{ leaderNoteText }}</div>
        </div>
      </article>

      <article class="metric-card is-cyan">
        <div class="metric-card-head">
          <div class="metric-card-title">
            <div class="metric-icon"><el-icon><Grid /></el-icon></div>
            <h3 class="metric-heading">{{ locale === 'zh' ? '服务注册' : 'Services' }}</h3>
          </div>
          <span class="metric-badge">{{ `${stats?.healthy_count ?? 0}/${stats?.instance_count ?? 0}` }}</span>
        </div>

        <div class="metric-main">
          <div class="metric-value-row">
            <div class="metric-value">{{ stats?.service_count ?? '-' }}</div>
            <small class="metric-unit">{{ serviceUnitText }}</small>
          </div>
          <div class="metric-inline-meta">
            <span>{{ locale === 'zh' ? '实例' : 'Instances' }} {{ stats?.instance_count ?? '-' }}</span>
            <span>{{ locale === 'zh' ? '健康' : 'Healthy' }} {{ stats?.healthy_count ?? '-' }}</span>
          </div>
          <div class="metric-progress">
            <div class="metric-progress-track">
              <span class="metric-progress-fill" :style="{ width: `${serviceHealthRateValue}%` }"></span>
            </div>
            <span class="metric-progress-text">{{ serviceHealthComparisonText }}</span>
          </div>
        </div>
      </article>

      <article class="metric-card is-green">
        <div class="metric-card-head">
          <div class="metric-card-title">
            <div class="metric-icon"><el-icon><FolderOpened /></el-icon></div>
            <h3 class="metric-heading">{{ locale === 'zh' ? '命名空间' : 'Namespaces' }}</h3>
          </div>
          <span class="metric-badge">{{ `${customNamespaceCount}` }}</span>
        </div>

        <div class="metric-main">
          <div class="metric-value-row">
            <div class="metric-value">{{ namespaceCount ?? '-' }}</div>
            <small class="metric-unit">{{ namespaceUnitText }}</small>
          </div>
          <div class="metric-inline-meta">
            <span>{{ locale === 'zh' ? '默认' : 'Default' }} {{ defaultNamespaceCount }}</span>
            <span>{{ locale === 'zh' ? '自定义' : 'Custom' }} {{ customNamespaceCount }}</span>
          </div>
          <div class="metric-note" :title="namespaceLabelText">{{ namespaceLabelText }}</div>
        </div>
      </article>

      <article class="metric-card is-slate">
        <div class="metric-card-head">
          <div class="metric-card-title">
            <div class="metric-icon"><el-icon><Monitor /></el-icon></div>
            <h3 class="metric-heading">{{ locale === 'zh' ? '系统信息' : 'System' }}</h3>
          </div>
          <span class="metric-badge">10m</span>
        </div>

        <div class="metric-main metric-main--system">
          <div class="metric-system-copy">
            <div class="metric-value-row">
              <div class="metric-value metric-value--memory">{{ formatMemory(stats?.memory_usage) }}</div>
            </div>
            <div class="metric-inline-meta">
              <span>{{ locale === 'zh' ? '最低' : 'Min' }} {{ memoryTrendMinText }}</span>
              <span>{{ locale === 'zh' ? '最高' : 'Max' }} {{ memoryTrendMaxText }}</span>
            </div>
            <div class="metric-note" :title="memoryTrendDeltaText">{{ memoryTrendDeltaText }}</div>
          </div>
          <div class="metric-sparkline-shell metric-sparkline-shell--system">
            <svg class="metric-sparkline-svg" :viewBox="`0 0 ${MEMORY_CHART_WIDTH} ${MEMORY_CHART_HEIGHT}`" preserveAspectRatio="none">
              <path
                v-if="memoryTrendAreaPath"
                :d="memoryTrendAreaPath"
                fill="rgba(59, 130, 246, 0.12)"
              />
              <polyline
                v-if="memoryTrendPolyline"
                :points="memoryTrendPolyline"
                fill="none"
                stroke="#2563eb"
                stroke-width="3"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
              <circle
                v-if="memoryTrendLastPoint"
                :cx="memoryTrendLastPoint.x"
                :cy="memoryTrendLastPoint.y"
                r="4.5"
                fill="#ffffff"
                stroke="#2563eb"
                stroke-width="3"
              />
            </svg>
            <div v-if="!memoryTrendPolyline" class="metric-sparkline-empty">{{ memoryTrendEmptyText }}</div>
          </div>
        </div>
      </article>
    </section>

    <section class="activity-panel" data-guide="dashboard-activity">
      <header class="activity-head">
        <div class="panel-tabs">
          <button
            type="button"
            class="panel-tab"
            :class="{ active: activePanel === 'events' }"
            @click="activePanel = 'events'"
          >
            {{ eventsTabText }}
          </button>
          <button
            type="button"
            class="panel-tab"
            :class="{ active: activePanel === 'logs' }"
            @click="activePanel = 'logs'"
          >
            {{ logsTabText }}
          </button>
        </div>

        <div v-if="activePanel === 'events'" class="panel-actions panel-actions--events">
          <!-- 搜索内容 -->
          <el-input
            v-model="eventSearchText"
            :placeholder="locale === 'zh' ? '搜索内容...' : 'Search...'"
            clearable
            class="panel-input"
            style="width: 200px;"
          >
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>

          <!-- 发生时间范围 -->
          <div class="panel-filter">
            <span class="panel-filter-label">{{ eventTimeFilterLabel }}</span>
            <el-date-picker
              v-model="eventSearchDate"
              type="datetimerange"
              :start-placeholder="locale === 'zh' ? '开始时间' : 'Start'"
              :end-placeholder="locale === 'zh' ? '结束时间' : 'End'"
              size="default"
              class="panel-date-picker"
              style="width: 360px;"
              :shortcuts="datePickerShortcuts"
              @change="fetchEvents(0)"
            />
          </div>
          
          <!-- 事件类型 -->
          <div class="panel-filter">
            <span class="panel-filter-label">{{ eventTypeFilterLabel }}</span>
            <el-select v-model="activeEventType" class="panel-select" :teleported="false" style="width: 140px;">
              <el-option value="all" :label="locale === 'zh' ? '全部类型' : 'All Types'" />
              <el-option
                v-for="item in eventTypeOptionsRaw"
                :key="item"
                :label="getEventTypeLabel(item)"
                :value="item"
              />
            </el-select>
          </div>
        </div>

        <div v-else class="panel-actions panel-actions--logs">
          <!-- 自动滚动 -->
          <div class="panel-filter">
            <span class="panel-filter-label">{{ autoScrollLabel }}</span>
            <el-switch v-model="autoScrollLogs" size="default" />
          </div>

          <!-- 搜索内容 -->
          <el-input
            v-model="logSearchText"
            :placeholder="locale === 'zh' ? '搜索内容...' : 'Search...'"
            clearable
            class="panel-input"
            style="width: 200px;"
          >
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>

          <!-- 记录时间范围 -->
          <div class="panel-filter">
            <span class="panel-filter-label">{{ locale === 'zh' ? '记录时间' : 'Logged At' }}</span>
            <el-date-picker
              v-model="logSearchDate"
              type="datetimerange"
              :start-placeholder="locale === 'zh' ? '开始时间' : 'Start'"
              :end-placeholder="locale === 'zh' ? '结束时间' : 'End'"
              size="default"
              class="panel-date-picker"
              style="width: 320px;"
              :shortcuts="logPickerShortcuts"
            />
          </div>

          <!-- 日志文件 -->
          <div class="panel-filter">
            <span class="panel-filter-label">{{ locale === 'zh' ? '日志文件' : 'Log File' }}</span>
            <el-select
              v-model="activeLogFile"
              class="panel-select"
              :placeholder="logSelectPlaceholder"
              :teleported="false"
              :disabled="localizedLogFiles.length === 0"
              style="width: 160px;"
            >
              <el-option
                v-for="item in localizedLogFiles"
                :key="item.file"
                :label="item.label"
                :value="item.file"
              />
            </el-select>
          </div>

          <!-- 显示行数 -->
          <div class="panel-filter">
            <span class="panel-filter-label">{{ locale === 'zh' ? '行数' : 'Lines' }}</span>
            <el-select v-model="logLineCount" class="panel-select" :teleported="false" style="width: 100px;">
              <el-option v-for="item in logLineOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </div>
        </div>
      </header>

      <div v-if="activePanel === 'events'" class="panel-content-wrapper event-content-wrapper">
        <div class="panel-scroll event-scroll" ref="eventsScrollRef">
          <div v-if="loading" class="panel-empty">
            <el-icon><Loading /></el-icon>
            <p>{{ loadingLabel }}</p>
          </div>

          <div v-else-if="events.length === 0" class="panel-empty">
            <el-icon><DocumentDelete /></el-icon>
            <p>{{ t.dashboard.noEvents }}</p>
          </div>

          <div v-else class="event-feed">
            <div v-for="event in events" :key="event.id" class="event-node">
              <div class="event-node-aside">
                <div class="event-node-dot" :class="getEventToneClass(event.type)"></div>
                <div class="event-node-line"></div>
              </div>
              <div class="event-node-main">
                <div class="event-node-meta">
                  <span class="event-node-type" :class="getEventToneClass(event.type)">
                    {{ getEventTypeLabel(event.type) }}
                  </span>
                  <span class="event-node-id">{{ event.service }}</span>
                  <span class="event-node-scope">{{ event.instance }}</span>
                  <span class="event-node-time">{{ new Date(event.timestamp).toLocaleString() }}</span>
                </div>
                <div class="event-node-copy">
                  {{ getEventMessage(event) }}
                </div>
              </div>
            </div>
          </div>
        </div>
        
        <div class="panel-pagination">
          <div class="pagination-info">
            {{ locale === 'zh' ? `共计 ${eventTotal} 条` : `Total ${eventTotal}` }}
          </div>
          <el-pagination
            v-model:current-page="currentPage"
            v-model:page-size="eventLineCount"
            :page-sizes="[100, 500, 1000, 2000, 5000]"
            layout="sizes, prev, pager, next, jumper"
            :total="eventTotal"
            @size-change="handleSizeChange"
            @current-change="handlePageChange"
            size="small"
            background
          >
          </el-pagination>
        </div>
      </div>

      <div v-else class="panel-content-wrapper log-content-wrapper">
        <div ref="logsScrollRef" class="panel-scroll log-scroll" @scroll="handleLogScroll">
          <div v-if="loading || logsLoading" class="panel-empty">
            <el-icon><Loading /></el-icon>
            <p>{{ loadingLabel }}</p>
          </div>

          <div v-else-if="localizedLogFiles.length === 0" class="panel-empty">
            <el-icon><Document /></el-icon>
            <p>{{ noLogFilesLabel }}</p>
          </div>

          <div v-else-if="logs.length === 0" class="panel-empty">
            <el-icon><Tickets /></el-icon>
            <p>{{ noLogsLabel }}</p>
          </div>

          <template v-else>
            <div class="log-toolbar-note">
              <el-icon><InfoFilled /></el-icon>
              <span>{{ locale === 'zh' ? '正在查看日志文件...' : 'Viewing log file...' }}</span>
            </div>

            <div class="log-viewer">
              <div v-for="(entry, index) in parsedLogs" :key="index" class="log-line" :class="`is-${entry.levelClass}`">
                <span class="log-index">{{ (logCurrentPage - 1) * logLineCount + index + 1 }}</span>
                
                <div class="log-main">
                  <template v-if="entry.parsed">
                    <div class="log-meta">
                       <span class="log-time">{{ entry.timestamp }}</span>
                       <span class="log-level" :class="`is-${entry.levelClass}`">{{ entry.level }}</span>
                    </div>
                    <span v-if="entry.thread" class="log-thread">#{{ entry.thread }}</span>
                    <span v-if="entry.scope" class="log-scope">{{ entry.scope }}</span>
                    <span class="log-text">{{ entry.message }}</span>
                  </template>
                  <code v-else class="log-raw">{{ entry.raw }}</code>
                </div>
              </div>
            </div>
          </template>
        </div>

        <div class="panel-pagination">
          <div class="pagination-info">
            {{ locale === 'zh' ? `共计 ${logTotal} 条` : `Total ${logTotal}` }}
          </div>
          <el-pagination
            v-model:current-page="logCurrentPage"
            v-model:page-size="logLineCount"
            :page-sizes="[100, 500, 1000, 2000, 5000]"
            layout="sizes, prev, pager, next, jumper"
            :total="logTotal"
            @size-change="handleLogSizeChange"
            @current-change="handleLogPageChange"
            size="small"
            background
          />
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.dashboard-shell {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
  overflow: hidden;
}

.metric-grid {
  flex: 0 0 25%;
  min-height: 0;
  max-height: 25%;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  align-items: stretch;
}

.metric-card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-height: 0;
  height: 100%;
  padding: 12px 14px 10px;
  overflow: hidden;
  border-radius: 16px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  background: var(--bg-secondary);
  box-shadow: 0 8px 18px rgba(15, 23, 42, 0.05);
  transition: box-shadow 0.2s ease;
}

.metric-card::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 3px;
  opacity: 0.9;
}

.metric-card::after {
  display: none;
}

.metric-card:hover {
  box-shadow: 0 12px 22px rgba(15, 23, 42, 0.08);
}

.metric-card.is-blue::before {
  background: linear-gradient(90deg, #3b82f6, #38bdf8);
}

.metric-card.is-cyan::before {
  background: linear-gradient(90deg, #06b6d4, #14b8a6);
}

.metric-card.is-purple::before {
  background: linear-gradient(90deg, #8b5cf6, #ec4899);
}

.metric-card.is-green::before {
  background: linear-gradient(90deg, #10b981, #22c55e);
}

.metric-card.is-slate::before {
  background: linear-gradient(90deg, #475569, #94a3b8);
}

.metric-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-height: 30px;
}

.metric-card-title {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.metric-icon {
  width: 30px;
  height: 30px;
  border-radius: 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.65);
}

.metric-heading {
  margin: 0;
  min-width: 0;
  font-size: 14px;
  font-weight: 800;
  line-height: 1.2;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.metric-card.is-blue .metric-icon {
  color: #2563eb;
  background: rgba(59, 130, 246, 0.12);
}

.metric-card.is-cyan .metric-icon {
  color: #0891b2;
  background: rgba(6, 182, 212, 0.12);
}

.metric-card.is-purple .metric-icon {
  color: #9333ea;
  background: rgba(147, 51, 234, 0.12);
}

.metric-card.is-green .metric-icon {
  color: #059669;
  background: rgba(16, 185, 129, 0.12);
}

.metric-card.is-slate .metric-icon {
  color: #475569;
  background: rgba(100, 116, 139, 0.12);
}

.metric-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 4px;
  flex: 1;
}

.metric-main--system {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 120px;
  align-items: stretch;
  gap: 8px 10px;
}

.metric-system-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 4px;
}

.metric-value-row {
  display: flex;
  align-items: flex-end;
  gap: 6px;
}

.metric-value {
  font-size: clamp(20px, 1.55vw, 28px);
  font-weight: 800;
  line-height: 1;
  letter-spacing: -0.03em;
  color: var(--text-primary);
  white-space: nowrap;
}

.metric-value--memory {
  font-size: clamp(18px, 1.4vw, 24px);
}

.metric-unit {
  font-size: 12px;
  font-weight: 700;
  color: var(--text-muted);
}

.metric-badge {
  display: inline-flex;
  align-items: center;
  min-height: 22px;
  max-width: 44%;
  padding: 0 7px;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.1);
  color: var(--text-secondary);
  font-size: 10px;
  font-weight: 700;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.metric-inline-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.metric-inline-meta span {
  display: inline-flex;
  align-items: center;
  min-height: 18px;
  padding: 0 6px;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.08);
  color: var(--text-secondary);
  font-size: 10px;
  font-weight: 700;
  white-space: nowrap;
}

.metric-note {
  font-size: 11px;
  line-height: 1.3;
  color: var(--text-muted);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.metric-progress {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.metric-progress-track {
  position: relative;
  width: 100%;
  height: 5px;
  overflow: hidden;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.16);
}

.metric-progress-fill {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, #06b6d4, #10b981);
  box-shadow: 0 6px 14px rgba(16, 185, 129, 0.2);
}

.metric-progress-text {
  font-size: 10px;
  font-weight: 700;
  color: var(--text-secondary);
  display: -webkit-box;
  -webkit-line-clamp: 1;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.system-panel {
  flex: none;
  display: grid;
  grid-template-columns: minmax(220px, 0.9fr) minmax(240px, 1fr) minmax(320px, 1.15fr);
  gap: 18px;
  padding: 20px 22px;
  border-radius: 24px;
  border: 1px solid rgba(148, 163, 184, 0.14);
  background:
    radial-gradient(circle at 100% 0%, rgba(191, 219, 254, 0.35), transparent 34%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.94), rgba(248, 250, 252, 0.98));
  box-shadow: 0 14px 30px rgba(15, 23, 42, 0.05);
}

.system-summary {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}

.system-icon {
  color: #2563eb;
  background: rgba(59, 130, 246, 0.12);
}

.system-value {
  font-size: clamp(34px, 3vw, 44px);
  font-weight: 800;
  line-height: 1;
  letter-spacing: -0.04em;
  color: var(--text-primary);
}

.system-label {
  font-size: 14px;
  font-weight: 700;
  color: var(--text-secondary);
}

.system-note {
  margin: 0;
  font-size: 12px;
  line-height: 1.55;
  color: var(--text-muted);
}

.system-stat-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  align-content: start;
}

.panel-content-wrapper {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.panel-scroll.event-scroll {
  flex: 1;
  padding: 16px;
  overflow-y: auto;
}

.panel-actions--logs,
.panel-actions--events {
  display: flex;
  align-items: center;
  gap: 12px;
  flex: 1;
  justify-content: flex-end;
}

.panel-filter {
  display: flex;
  align-items: center;
  background: rgba(148, 163, 184, 0.06);
  border: 1px solid rgba(148, 163, 184, 0.12);
  border-radius: 10px;
  padding: 2px 4px 2px 10px;
  height: 32px;
  transition: all 0.2s ease;
}

.panel-filter:hover {
  background: rgba(148, 163, 184, 0.1);
  border-color: rgba(148, 163, 184, 0.2);
}

.panel-filter-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
  margin-right: 8px;
  white-space: nowrap;
}

.panel-filter :deep(.el-input__wrapper),
.panel-filter :deep(.el-select__wrapper) {
  background: transparent !important;
  box-shadow: none !important;
}

.panel-filter :deep(.el-input__inner) {
  font-size: 13px;
  font-weight: 500;
}

.panel-pagination {
  padding: 12px 20px;
  background: var(--surface-color);
  border-top: 1px solid var(--border-color);
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 16px;
}

.pagination-info {
  font-size: 12px;
  font-weight: 700;
  color: var(--text-muted);
}

:deep(.el-pagination__jump) {
  margin-left: 0;
  font-size: 12px;
  font-weight: 700;
  color: var(--text-muted);
}

:deep(.el-pagination__jump-prev),
:deep(.el-pagination__jump-next) {
  display: none !important;
}

:deep(.el-pagination__goto) {
  display: none !important;
}

:deep(.el-pagination__jump)::before {
  content: v-bind("locale === 'zh' ? '跳转至' : 'Go to'");
  margin-right: 8px;
}

.event-feed {
  display: flex;
  flex-direction: column;
}

.event-node {
  display: flex;
  gap: 16px;
  margin-bottom: 24px;
}

.event-node-aside {
  display: flex;
  flex-direction: column;
  align-items: center;
  position: relative;
}

.event-node-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  border: 2px solid currentColor;
  background: #fff;
  z-index: 1;
}

.event-node-line {
  position: absolute;
  top: 10px;
  bottom: -24px;
  width: 1px;
  background: var(--border-color);
}

.event-node:last-child .event-node-line {
  display: none;
}

.panel-pill-group :deep(.el-radio-button__inner) {
  border-radius: 12px !important;
  border: 1px solid rgba(148, 163, 184, 0.16) !important;
  background: rgba(148, 163, 184, 0.04) !important;
  margin-right: 6px;
  font-size: 13px;
  min-width: 60px;
  padding: 8px 12px;
  box-shadow: none !important;
}

.panel-pill-group :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) {
  background: var(--accent-blue) !important;
  color: #fff !important;
  border-color: var(--accent-blue) !important;
}

.event-node:last-child .event-node-line {
  display: none;
}

.event-node-main {
  flex: 1;
}

.event-node-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
  margin-bottom: 6px;
  flex-wrap: wrap;
}

.event-node-type {
  font-weight: 600;
  text-transform: uppercase;
  font-size: 11px;
  padding: 2px 6px;
  border-radius: 4px;
}

.event-node-id {
  font-weight: 500;
  color: var(--text-heading);
}

.event-node-scope {
  color: var(--text-dim);
  font-family: var(--font-mono);
}

.event-node-time {
  margin-left: auto;
  color: var(--text-dim);
}

.event-node-copy {
  color: var(--text-body);
  line-height: 1.5;
  font-size: 12px;
}

.event-node-dot.is-green { color: #10b981; }
.event-node-dot.is-red { color: #ef4444; }
.event-node-dot.is-blue { color: #3b82f6; }
.event-node-dot.is-orange { color: #f59e0b; }
.event-node-dot.is-cyan { color: #06b6d4; }

.event-node-type.is-green { background: rgba(16, 185, 129, 0.1); color: #10b981; }
.event-node-type.is-red { background: rgba(239, 68, 68, 0.1); color: #ef4444; }
.event-node-type.is-blue { background: rgba(59, 130, 246, 0.1); color: #3b82f6; }
.event-node-type.is-orange { background: rgba(245, 158, 11, 0.1); color: #f59e0b; }
.event-node-type.is-cyan { background: rgba(6, 182, 212, 0.1); color: #06b6d4; }


.system-stat-label {
  font-size: 12px;
  font-weight: 700;
  color: var(--text-muted);
}

.system-stat-value {
  font-size: 18px;
  font-weight: 800;
  color: var(--text-primary);
}

.system-chart-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}

.system-chart-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.system-chart-copy {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.system-chart-kicker {
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.system-chart-title {
  font-size: 16px;
  font-weight: 800;
  color: var(--text-primary);
}

.system-chart-delta {
  display: inline-flex;
  align-items: center;
  min-height: 32px;
  padding: 0 12px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 800;
  white-space: nowrap;
}

.system-chart-delta.is-up {
  color: #b91c1c;
  background: rgba(239, 68, 68, 0.12);
}

.system-chart-delta.is-down {
  color: #0f766e;
  background: rgba(16, 185, 129, 0.12);
}

.system-chart-delta.is-flat {
  color: var(--text-secondary);
  background: rgba(148, 163, 184, 0.12);
}

.system-chart-shell {
  position: relative;
  flex: 1;
  min-height: 134px;
  border-radius: 18px;
  overflow: hidden;
  border: 1px solid rgba(148, 163, 184, 0.12);
  background:
    linear-gradient(180deg, rgba(59, 130, 246, 0.03), rgba(59, 130, 246, 0)),
    repeating-linear-gradient(
      to top,
      rgba(148, 163, 184, 0.08),
      rgba(148, 163, 184, 0.08) 1px,
      transparent 1px,
      transparent 34px
    );
}

.system-chart-svg {
  width: 100%;
  height: 100%;
  display: block;
}

.system-chart-empty {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 18px;
  font-size: 13px;
  font-weight: 700;
  color: var(--text-muted);
  text-align: center;
}

.system-chart-axis {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
  font-weight: 700;
  color: var(--text-muted);
}

.metric-sparkline-shell {
  position: relative;
  flex: 0 0 24px;
  height: 24px;
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid rgba(148, 163, 184, 0.12);
  background:
    linear-gradient(180deg, rgba(59, 130, 246, 0.04), rgba(59, 130, 246, 0)),
    repeating-linear-gradient(
      to top,
      rgba(148, 163, 184, 0.08),
      rgba(148, 163, 184, 0.08) 1px,
      transparent 1px,
      transparent 12px
    );
}

.metric-sparkline-shell--system {
  flex: none;
  height: auto;
  min-height: 64px;
}

.metric-sparkline-svg {
  width: 100%;
  height: 100%;
  display: block;
}

.metric-sparkline-empty {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 4px;
  font-size: 9px;
  font-weight: 700;
  color: var(--text-muted);
  text-align: center;
}

.activity-panel {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-radius: 24px;
  border: 1px solid rgba(148, 163, 184, 0.14);
  background: var(--bg-secondary);
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.05);
}

.activity-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 16px 20px 14px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.12);
  flex-shrink: 0;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.68), rgba(255, 255, 255, 0));
}

.panel-tabs {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px;
  border-radius: 18px;
  border: 1px solid rgba(148, 163, 184, 0.14);
  background: rgba(148, 163, 184, 0.06);
}

.panel-tab {
  min-height: 42px;
  padding: 0 20px;
  border: none;
  border-radius: 14px;
  background: transparent;
  color: var(--text-secondary);
  font-size: 14px;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.2s ease;
}

.panel-tab:hover {
  color: var(--text-primary);
}

.panel-tab.active {
  background: linear-gradient(135deg, #3b82f6, #60a5fa);
  color: #fff;
  box-shadow: 0 10px 22px rgba(59, 130, 246, 0.22);
}

.panel-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  flex-shrink: 0;
}

.panel-actions--events,
.panel-actions--logs {
  flex-wrap: wrap;
  justify-content: flex-end;
}

.panel-filter {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.panel-filter-label {
  flex-shrink: 0;
  font-size: 12px;
  font-weight: 700;
  color: var(--text-muted);
}

.panel-summary {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 40px;
  padding: 0 14px;
  border-radius: 14px;
  border: 1px solid rgba(59, 130, 246, 0.16);
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.08), rgba(96, 165, 250, 0.14));
}

.panel-summary-kicker {
  font-size: 12px;
  font-weight: 700;
  color: var(--accent-blue);
}

.panel-summary-value {
  font-size: 13px;
  font-weight: 800;
  color: var(--text-primary);
  white-space: nowrap;
}

.panel-scroll {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 14px 18px 18px;
}

.panel-scroll::-webkit-scrollbar {
  width: 6px;
  height: 6px;
}

.panel-scroll::-webkit-scrollbar-track {
  background: transparent;
}

.panel-scroll::-webkit-scrollbar-thumb {
  background: rgba(148, 163, 184, 0.28);
  border-radius: 999px;
}

.panel-empty {
  height: 100%;
  min-height: 220px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 20px;
  border-radius: 18px;
  border: 1px dashed rgba(148, 163, 184, 0.2);
  background: rgba(148, 163, 184, 0.05);
  color: var(--text-muted);
  text-align: center;
}

.panel-empty .el-icon {
  font-size: 28px;
}

.event-entry {
  display: flex;
  gap: 14px;
  padding: 14px 16px;
  border-radius: 18px;
  border: 1px solid rgba(148, 163, 184, 0.12);
  background: rgba(148, 163, 184, 0.05);
}

.event-entry + .event-entry {
  margin-top: 10px;
}

.event-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  margin-top: 8px;
  flex-shrink: 0;
  background: var(--accent-blue);
  box-shadow: 0 0 0 5px rgba(59, 130, 246, 0.08);
}

.event-dot.is-green {
  background: var(--accent-green);
  box-shadow: 0 0 0 5px rgba(16, 185, 129, 0.08);
}

.event-dot.is-red {
  background: var(--accent-red);
  box-shadow: 0 0 0 5px rgba(239, 68, 68, 0.08);
}

.event-dot.is-orange {
  background: var(--accent-orange);
  box-shadow: 0 0 0 5px rgba(245, 158, 11, 0.08);
}

.event-dot.is-cyan {
  background: var(--accent-cyan);
  box-shadow: 0 0 0 5px rgba(6, 182, 212, 0.08);
}

.event-body {
  flex: 1;
  min-width: 0;
}

.event-meta {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  margin-bottom: 8px;
}

.event-meta-main {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.event-type {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  padding: 0 12px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 700;
}

.event-type.is-blue {
  background: rgba(59, 130, 246, 0.08);
  color: #2563eb;
}

.event-type.is-green {
  background: rgba(16, 185, 129, 0.1);
  color: #059669;
}

.event-type.is-red {
  background: rgba(239, 68, 68, 0.1);
  color: #dc2626;
}

.event-type.is-orange {
  background: rgba(245, 158, 11, 0.12);
  color: #d97706;
}

.event-type.is-cyan {
  background: rgba(6, 182, 212, 0.1);
  color: #0f766e;
}

.event-time {
  flex-shrink: 0;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
}

.event-title {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  font-size: 14px;
  color: var(--text-primary);
}

.event-title strong {
  font-size: 15px;
  font-weight: 800;
}

.event-title code {
  max-width: 100%;
  padding: 4px 8px;
  border-radius: 9px;
  background: rgba(148, 163, 184, 0.08);
  color: var(--text-secondary);
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.event-message {
  font-size: 12px;
  line-height: 1.55;
  color: var(--text-secondary);
}

.log-scroll {
  background:
    linear-gradient(180deg, rgba(15, 23, 42, 0.02), transparent 80%),
    var(--bg-secondary);
}

.log-toolbar-note {
  margin-bottom: 10px;
  font-size: 12px;
  font-weight: 700;
  color: var(--text-muted);
}

.log-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.log-line {
  display: flex;
  gap: 12px;
  padding: 6px 12px;
  border-radius: 8px;
  transition: background 0.18s ease;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 12px;
  line-height: 1.6;
}

.log-line:hover {
  background: rgba(148, 163, 184, 0.06);
}

.log-line.is-info {
  background: rgba(59, 130, 246, 0.03);
}

.log-line.is-warn {
  background: rgba(245, 158, 11, 0.04);
}

.log-line.is-error {
  background: rgba(239, 68, 68, 0.04);
}

.log-line.is-fatal {
  background: rgba(159, 18, 57, 0.04);
}

.log-line.is-debug {
  background: rgba(6, 182, 212, 0.04);
}

.log-index {
  padding-top: 2px;
  font-size: 12px;
  font-weight: 700;
  color: var(--text-muted);
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.log-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 12px;
}

.log-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.log-time {
  color: var(--text-muted);
  white-space: nowrap;
}

.log-level {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 54px;
  height: 22px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 800;
  text-transform: uppercase;
}

.log-level.is-info {
  color: #1d4ed8;
  background: rgba(59, 130, 246, 0.12);
}

.log-level.is-warn {
  color: #b45309;
  background: rgba(245, 158, 11, 0.14);
}

.log-level.is-error {
  color: #b91c1c;
  background: rgba(239, 68, 68, 0.14);
}

.log-level.is-fatal {
  color: #9f1239;
  background: rgba(159, 18, 57, 0.14);
}

.log-level.is-debug {
  color: #0f766e;
  background: rgba(6, 182, 212, 0.12);
}

.log-thread {
  color: var(--text-muted);
}

.log-scope {
  padding: 1px 7px;
  border-radius: 7px;
  background: rgba(148, 163, 184, 0.08);
  color: var(--text-secondary);
}

.log-text {
  color: var(--text-primary);
}

.log-raw {
  display: block;
  color: var(--text-primary);
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.panel-select {
  width: 180px;
}

.panel-select--time {
  width: 156px;
}

.log-file-select {
  width: 180px;
}

.log-lines-select {
  width: 120px;
}

.log-search-input {
  width: 200px;
}

.log-search-input :deep(.el-input__wrapper) {
  min-height: 38px;
  border-radius: 12px;
  background: rgba(148, 163, 184, 0.06) !important;
  border: 1px solid rgba(148, 163, 184, 0.16) !important;
  box-shadow: none !important;
}

.log-search-input :deep(.el-input__inner) {
  color: var(--text-primary);
  font-size: 13px;
}

.log-search-input :deep(.el-input__inner::placeholder) {
  color: var(--text-muted);
}

.log-scroll-toggle {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

:deep(.panel-select .el-select__wrapper) {
  min-height: 38px;
  border-radius: 12px;
  background: rgba(148, 163, 184, 0.06) !important;
  border: 1px solid rgba(148, 163, 184, 0.16) !important;
  box-shadow: none !important;
}

:deep(.panel-select .el-select__placeholder),
:deep(.panel-select .el-select__selected-item) {
  color: var(--text-primary) !important;
  font-size: 13px;
}

:deep(.log-scroll-toggle .el-switch) {
  --el-switch-on-color: #3b82f6;
  --el-switch-off-color: rgba(148, 163, 184, 0.3);
}

@media (max-width: 1360px) {
  .metric-grid {
    flex: none;
    max-height: none;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .metric-main--system {
    grid-template-columns: minmax(0, 1fr) 108px;
  }

  .system-panel {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .system-chart-panel {
    grid-column: 1 / -1;
  }
}
@media (max-width: 980px) {
  .system-panel {
    grid-template-columns: 1fr;
  }

  .system-chart-head {
    flex-direction: column;
    align-items: flex-start;
  }

  .activity-head {
    flex-direction: column;
    align-items: stretch;
  }

  .panel-actions {
    justify-content: flex-start;
  }

  .panel-summary {
    align-self: flex-start;
  }
}

@media (max-width: 900px) {
  .metric-grid {
    flex: none;
    max-height: none;
    grid-template-columns: 1fr;
  }

  .metric-main--system {
    grid-template-columns: 1fr;
  }

  .metric-sparkline-shell--system {
    min-height: 54px;
  }

  .metric-card {
    min-height: 0;
  }

  .metric-stats,
  .system-stat-grid {
    grid-template-columns: 1fr;
  }

  .panel-actions--events,
  .panel-actions--logs {
    width: 100%;
    flex-direction: column;
    align-items: stretch;
  }

  .panel-summary {
    width: 100%;
    justify-content: space-between;
  }

  .panel-filter {
    width: 100%;
    flex-direction: column;
    align-items: stretch;
  }

  .panel-select,
  .panel-select--time,
  .log-file-select {
    width: 100%;
  }

  .log-scroll-toggle {
    justify-content: space-between;
  }

  .event-meta {
    flex-direction: column;
  }

  .event-title {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>

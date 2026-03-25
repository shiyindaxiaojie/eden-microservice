<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  getClusterStats,
  getEvents,
  getLogFiles,
  getLogs,
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
  timestamp?: string
  level?: string
  levelClass: string
  thread?: string
  scope?: string
  message?: string
}

type DashboardPanel = 'events' | 'logs'
type EventTimeFilter = 'all' | '1h' | '24h' | '7d'

const AUTO_REFRESH_MS = 10000
const LOG_LINE_COUNT = 160
const KNOWN_EVENT_TYPES = [
  'service_register',
  'service_online',
  'service_offline',
  'service_heartbeat',
  'service_remove',
  'registry_node_sync',
  'register',
  'deregister',
  'health_change',
] as const

const { t, locale } = useI18n()
const stats = ref<ClusterStats | null>(null)
const events = ref<RegistryEvent[]>([])
const logFiles = ref<LogFileOption[]>([])
const activeLogFile = ref('')
const logs = ref<string[]>([])
const loading = ref(true)
const logsLoading = ref(false)
const activePanel = ref<DashboardPanel>('events')
const activeEventType = ref('all')
const activeEventTime = ref<EventTimeFilter>('all')
const autoScrollLogs = ref(true)
const logsScrollRef = ref<HTMLDivElement | null>(null)
let timer: ReturnType<typeof setInterval> | null = null

const serviceHealthText = computed(() => {
  if (!stats.value) return '-'
  return `${locale.value === 'zh' ? '健康实例' : 'Healthy'} ${stats.value.healthy_count}/${stats.value.instance_count}`
})

const healthStatusText = computed(() => {
  if (!stats.value) return '-'
  const unhealthyCount = Math.max(0, stats.value.instance_count - stats.value.healthy_count)
  if (unhealthyCount === 0) {
    return locale.value === 'zh' ? '全部健康' : 'All healthy'
  }

  return locale.value === 'zh' ? `${unhealthyCount} 个异常` : `${unhealthyCount} unhealthy`
})

const clusterModeText = computed(() => {
  if (!stats.value) return ''

  const environment = stats.value.environment === 'cluster'
    ? (locale.value === 'zh' ? '集群模式' : 'Cluster')
    : (locale.value === 'zh' ? '单机模式' : 'Standalone')

  return `${environment} / ${stats.value.mode.toUpperCase()}`
})

const refreshBadgeText = computed(() =>
  locale.value === 'zh' ? '每 10 秒自动刷新' : 'Auto refresh every 10s',
)

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
const currentNodeText = computed(() => (locale.value === 'zh' ? '当前节点' : 'Current node'))
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
  locale.value === 'zh' ? `最近 ${LOG_LINE_COUNT} 行` : `Last ${LOG_LINE_COUNT} lines`,
)
const eventTypeFilterLabel = computed(() => (locale.value === 'zh' ? '事件类型' : 'Event Type'))
const eventTimeFilterLabel = computed(() => (locale.value === 'zh' ? '发生时间' : 'Occurred'))
const autoScrollLabel = computed(() => (locale.value === 'zh' ? '自动滚动' : 'Auto Scroll'))

const localizedLogFiles = computed(() =>
  logFiles.value.map((item) => ({
    ...item,
    label: localizeLogFileName(item.name),
  })),
)

const parsedLogs = computed<ParsedLogLine[]>(() => logs.value.map((line) => parseLogLine(line)))

const eventTypeOptions = computed(() => {
  const seenTypes = new Set(events.value.map((event) => event.type).filter(Boolean))
  const orderedTypes = KNOWN_EVENT_TYPES.filter((type) => seenTypes.has(type))
  const extraTypes = [...seenTypes].filter((type) => !KNOWN_EVENT_TYPES.includes(type as typeof KNOWN_EVENT_TYPES[number])).sort()
  const allTypes = [...orderedTypes, ...extraTypes]

  return [
    {
      value: 'all',
      label: locale.value === 'zh' ? '全部类型' : 'All Types',
    },
    ...allTypes.map((type) => ({
      value: type,
      label: getEventTypeLabel(type),
    })),
  ]
})

const eventTimeOptions = computed(() => [
  {
    value: 'all',
    label: locale.value === 'zh' ? '全部时间' : 'All Time',
  },
  {
    value: '1h',
    label: locale.value === 'zh' ? '近 1 小时' : 'Last 1 hour',
  },
  {
    value: '24h',
    label: locale.value === 'zh' ? '近 24 小时' : 'Last 24 hours',
  },
  {
    value: '7d',
    label: locale.value === 'zh' ? '近 7 天' : 'Last 7 days',
  },
])

const filteredEvents = computed(() => {
  const now = Date.now()

  return [...events.value]
    .filter((event) => {
      if (activeEventType.value !== 'all' && event.type !== activeEventType.value) {
        return false
      }

      if (activeEventTime.value === 'all') {
        return true
      }

      const eventTime = new Date(event.timestamp).getTime()
      if (!Number.isFinite(eventTime)) {
        return false
      }

      const elapsed = now - eventTime
      switch (activeEventTime.value) {
        case '1h':
          return elapsed <= 60 * 60 * 1000
        case '24h':
          return elapsed <= 24 * 60 * 60 * 1000
        case '7d':
          return elapsed <= 7 * 24 * 60 * 60 * 1000
        default:
          return true
      }
    })
    .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
})

async function fetchLogs(file = activeLogFile.value) {
  if (!file) {
    logs.value = []
    return
  }

  logsLoading.value = true
  try {
    const response = await getLogs(file, LOG_LINE_COUNT)
    logs.value = response.data || []
    await nextTick()
    syncLogsScroll()
  } catch (error) {
    console.error('fetch dashboard logs failed', error)
    logs.value = []
  } finally {
    logsLoading.value = false
  }
}

async function fetchData() {
  try {
    const [statsRes, eventsRes, logFilesRes] = await Promise.all([
      getClusterStats(),
      getEvents(),
      getLogFiles(),
    ])

    stats.value = statsRes.data
    events.value = eventsRes.data || []

    const files = logFilesRes.data || []
    logFiles.value = files

    const activeStillExists = files.some((item) => item.file === activeLogFile.value)
    const nextActiveFile = activeStillExists ? activeLogFile.value : files[0]?.file || ''

    if (nextActiveFile !== activeLogFile.value) {
      activeLogFile.value = nextActiveFile
    } else {
      await fetchLogs(nextActiveFile)
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
  if (!bytes) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`
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
    'Heartbeat recovered instance to online': '心跳恢复，实例已重新上线',
    'Instance manually set to offline': '实例已手动下线',
    'Instance manually set to online': '实例已手动上线',
    'Health checker marked instance offline after missed heartbeats': '健康检查发现心跳超时，实例已标记为下线',
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
      levelClass: 'plain',
    }
  }

  const [, timestamp, level, thread, scope, message] = match
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

  const element = logsScrollRef.value
  if (!element) return
  element.scrollTop = element.scrollHeight
}

watch(activeLogFile, (file, previousFile) => {
  if (!file) {
    logs.value = []
    return
  }

  if (file !== previousFile) {
    fetchLogs(file)
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
    <section class="metric-grid">
      <article class="metric-card is-blue">
        <div class="metric-card-head">
          <div class="metric-icon"><el-icon><Connection /></el-icon></div>
          <span class="metric-side-label">{{ stats?.role || t.common.unknown }}</span>
        </div>

        <div class="metric-main">
          <div class="metric-value">{{ stats?.node_count ?? '-' }}</div>
          <div class="metric-label">{{ t.dashboard.nodes }}</div>
          <div class="metric-note">{{ leaderNoteText }}</div>
        </div>
      </article>

      <article class="metric-card is-cyan">
        <div class="metric-card-head">
          <div class="metric-icon"><el-icon><Grid /></el-icon></div>
          <span class="metric-side-label">{{ serviceHealthText }}</span>
        </div>

        <div class="metric-main">
          <div class="metric-value">
            <span>{{ stats?.service_count ?? '-' }}</span>
            <small>/ {{ stats?.instance_count ?? '-' }}</small>
          </div>
          <div class="metric-label">{{ t.dashboard.services }} / {{ t.dashboard.instances }}</div>
          <div class="metric-note">{{ clusterModeText || '-' }}</div>
        </div>
      </article>

      <article class="metric-card is-purple">
        <div class="metric-card-head">
          <div class="metric-icon"><el-icon><Monitor /></el-icon></div>
          <span class="metric-side-label">{{ currentNodeText }}</span>
        </div>

        <div class="metric-main">
          <div class="metric-value">{{ formatMemory(stats?.memory_usage) }}</div>
          <div class="metric-label">{{ t.dashboard.memoryUsage }}</div>
          <div class="metric-note">{{ t.dashboard.memoryDesc }}</div>
        </div>
      </article>

      <article class="metric-card is-green">
        <div class="metric-card-head">
          <div class="metric-icon"><el-icon><CircleCheck /></el-icon></div>
          <span class="metric-side-label">{{ healthStatusText }}</span>
        </div>

        <div class="metric-main">
          <div class="metric-value">{{ stats ? `${stats.health_rate.toFixed(1)}%` : '-' }}</div>
          <div class="metric-label">{{ t.dashboard.healthRate }}</div>
          <div class="metric-note">{{ refreshBadgeText }}</div>
        </div>
      </article>
    </section>

    <section class="activity-panel">
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
          <div class="panel-filter">
            <span class="panel-filter-label">{{ eventTypeFilterLabel }}</span>
            <el-select v-model="activeEventType" class="panel-select" :teleported="false">
              <el-option
                v-for="item in eventTypeOptions"
                :key="item.value"
                :label="item.label"
                :value="item.value"
              />
            </el-select>
          </div>

          <div class="panel-filter">
            <span class="panel-filter-label">{{ eventTimeFilterLabel }}</span>
            <el-select v-model="activeEventTime" class="panel-select panel-select--time" :teleported="false">
              <el-option
                v-for="item in eventTimeOptions"
                :key="item.value"
                :label="item.label"
                :value="item.value"
              />
            </el-select>
          </div>

          <span class="panel-count">{{ filteredEvents.length }}</span>
        </div>

        <div v-else class="panel-actions panel-actions--logs">
          <label class="log-scroll-toggle">
            <span class="panel-filter-label">{{ autoScrollLabel }}</span>
            <el-switch v-model="autoScrollLogs" size="small" />
          </label>

          <el-select
            v-model="activeLogFile"
            class="panel-select log-file-select"
            :placeholder="logSelectPlaceholder"
            :teleported="false"
            :disabled="localizedLogFiles.length === 0"
          >
            <el-option
              v-for="item in localizedLogFiles"
              :key="item.file"
              :label="item.label"
              :value="item.file"
            />
          </el-select>
        </div>
      </header>

      <div v-if="activePanel === 'events'" class="panel-scroll">
        <div v-if="loading" class="panel-empty">
          <el-icon><Loading /></el-icon>
          <p>{{ loadingLabel }}</p>
        </div>

        <div v-else-if="events.length === 0" class="panel-empty">
          <el-icon><DocumentDelete /></el-icon>
          <p>{{ t.dashboard.noEvents }}</p>
        </div>

        <div v-else-if="filteredEvents.length === 0" class="panel-empty">
          <el-icon><Filter /></el-icon>
          <p>{{ noMatchedEventsLabel }}</p>
        </div>

        <template v-else>
          <div v-for="evt in filteredEvents" :key="evt.id" class="event-entry">
            <div class="event-dot" :class="getEventToneClass(evt.type)"></div>

            <div class="event-body">
              <div class="event-meta">
                <div class="event-meta-main">
                  <span class="event-type" :class="getEventToneClass(evt.type)">{{ getEventTypeLabel(evt.type) }}</span>

                  <div class="event-title">
                    <strong>{{ evt.service || '-' }}</strong>
                    <code>{{ evt.instance || '-' }}</code>
                  </div>
                </div>

                <span class="event-time">{{ formatTime(evt.timestamp) }}</span>
              </div>

              <p class="event-message">{{ getEventMessage(evt) }}</p>
            </div>
          </div>
        </template>
      </div>

      <div v-else ref="logsScrollRef" class="panel-scroll log-scroll">
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
          <div class="log-toolbar-note">{{ logLinesLabel }}</div>

          <div class="log-list">
            <div
              v-for="(entry, index) in parsedLogs"
              :key="`${activeLogFile}-${index}-${entry.raw}`"
              class="log-line"
              :class="`is-${entry.levelClass}`"
            >
              <span class="log-index">{{ index + 1 }}</span>

              <div class="log-main">
                <template v-if="entry.parsed">
                  <span class="log-time">{{ entry.timestamp }}</span>
                  <span class="log-level" :class="`is-${entry.levelClass}`">{{ entry.level }}</span>
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
  flex: 0 0 20%;
  min-height: 120px;
  max-height: 160px;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  align-items: stretch;
}

.metric-card {
  position: relative;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 10px;
  min-height: 0;
  height: 100%;
  padding: 12px 18px 14px;
  overflow: hidden;
  border-radius: 20px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  background: var(--bg-secondary);
  box-shadow: none;
}

.metric-card::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 3px;
  opacity: 0.9;
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

.metric-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.metric-icon {
  width: 38px;
  height: 38px;
  border-radius: 12px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 17px;
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

.metric-side-label {
  max-width: 58%;
  padding: 4px 10px;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.08);
  color: var(--text-secondary);
  font-size: 11px;
  font-weight: 700;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.metric-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.metric-value {
  display: flex;
  align-items: baseline;
  gap: 6px;
  font-size: clamp(28px, 2.3vw, 34px);
  font-weight: 800;
  line-height: 1;
  letter-spacing: -0.03em;
  color: var(--text-primary);
  white-space: nowrap;
}

.metric-value small {
  font-size: 16px;
  font-weight: 700;
  color: var(--text-muted);
}

.metric-label {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-secondary);
}

.metric-note {
  font-size: 12px;
  line-height: 1.35;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
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
  gap: 14px;
  padding: 14px 18px 12px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.12);
  flex-shrink: 0;
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
  gap: 10px;
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

.panel-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 36px;
  height: 36px;
  padding: 0 12px;
  border-radius: 999px;
  background: rgba(59, 130, 246, 0.08);
  color: var(--accent-blue);
  font-size: 13px;
  font-weight: 700;
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
  font-size: 14px;
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
  gap: 4px;
}

.log-line {
  display: grid;
  grid-template-columns: 34px minmax(0, 1fr);
  gap: 12px;
  align-items: start;
  padding: 8px 10px;
  border-radius: 14px;
  transition: background 0.18s ease;
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
  min-width: 0;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 12px;
  line-height: 1.6;
  color: var(--text-primary);
  word-break: break-word;
}

.log-time,
.log-level,
.log-thread,
.log-scope {
  display: inline-block;
  margin-right: 8px;
  margin-bottom: 2px;
}

.log-time {
  color: var(--text-muted);
}

.log-level {
  padding: 1px 7px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 700;
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
    min-height: 0;
    max-height: none;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .metric-card {
    min-height: 124px;
  }
}

@media (max-width: 980px) {
  .activity-head {
    flex-direction: column;
    align-items: stretch;
  }

  .panel-actions {
    justify-content: flex-start;
  }
}

@media (max-width: 900px) {
  .metric-grid {
    grid-template-columns: 1fr;
  }

  .panel-actions--events,
  .panel-actions--logs {
    width: 100%;
    flex-direction: column;
    align-items: stretch;
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

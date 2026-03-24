<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
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

const AUTO_REFRESH_MS = 10000
const LOG_LINE_COUNT = 160

const { t, locale } = useI18n()
const stats = ref<ClusterStats | null>(null)
const events = ref<RegistryEvent[]>([])
const logFiles = ref<LogFileOption[]>([])
const activeLogFile = ref('')
const logs = ref<string[]>([])
const loading = ref(true)
const logsLoading = ref(false)
let timer: ReturnType<typeof setInterval> | null = null

const overviewDescription = computed(() =>
  locale.value === 'zh'
    ? '集中查看节点规模、服务健康、资源占用与最近运行动态。'
    : 'Track cluster size, service health, resource usage, and recent activity in one place.',
)

const leaderBadgeText = computed(() => {
  if (!stats.value) return ''
  if (stats.value.is_leader) {
    return locale.value === 'zh' ? '当前节点是 Leader' : 'Current node is Leader'
  }

  return `${t.value.dashboard.leaderAddr}: ${stats.value.leader_addr || t.value.common.unknown}`
})

const modeBadgeText = computed(() => {
  if (!stats.value) return ''

  const environment = stats.value.environment === 'cluster'
    ? (locale.value === 'zh' ? '集群模式' : 'Cluster')
    : (locale.value === 'zh' ? '单机模式' : 'Standalone')

  return `${environment} / ${stats.value.mode.toUpperCase()}`
})

const refreshBadgeText = computed(() =>
  locale.value === 'zh' ? '每 10 秒自动刷新' : 'Auto refresh every 10s',
)

const eventsHintText = computed(() =>
  locale.value === 'zh'
    ? '事件列表在面板内部滚动，不再撑高外层页面。'
    : 'The event stream scrolls inside the panel instead of stretching the page.',
)

const logsTitle = computed(() => (locale.value === 'zh' ? '运行日志' : 'Runtime Logs'))
const logsHintText = computed(() =>
  locale.value === 'zh'
    ? '从 config.yaml 配置的日志文件中读取内容。'
    : 'Read from the log files configured in config.yaml.',
)
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
const logLinesLabel = computed(() =>
  locale.value === 'zh' ? `最近 ${LOG_LINE_COUNT} 行` : `Last ${LOG_LINE_COUNT} lines`,
)
const healthyInstancesLabel = computed(() =>
  locale.value === 'zh' ? '健康实例' : 'Healthy instances',
)

const activeLogName = computed(
  () => logFiles.value.find((item) => item.file === activeLogFile.value)?.name || '',
)

const healthNote = computed(() => {
  if (!stats.value) return '-'
  return `${healthyInstancesLabel.value} ${stats.value.healthy_count}/${stats.value.instance_count}`
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

function formatTime(ts: string) {
  if (!ts) return '-'
  const date = new Date(ts)
  return date.toLocaleString(locale.value === 'zh' ? 'zh-CN' : 'en-US')
}

function formatMemory(bytes: number | undefined) {
  if (!bytes) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`
}

function getEventTypeLabel(type: string) {
  switch (type) {
    case 'register':
      return locale.value === 'zh' ? '注册' : 'Register'
    case 'deregister':
      return locale.value === 'zh' ? '下线' : 'Deregister'
    case 'health_change':
      return locale.value === 'zh' ? '健康变更' : 'Health Change'
    default:
      return type || (locale.value === 'zh' ? '事件' : 'Event')
  }
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
    <section class="overview-banner">
      <div class="banner-copy">
        <p class="banner-kicker">{{ t.nav.dashboard }}</p>
        <h2 class="banner-title">{{ locale === 'zh' ? '注册中心运行总览' : 'Registry Runtime Overview' }}</h2>
        <p class="banner-desc">{{ overviewDescription }}</p>
      </div>

      <div class="banner-badges">
        <span v-if="stats" class="banner-badge is-primary">{{ leaderBadgeText }}</span>
        <span v-if="stats" class="banner-badge is-subtle">{{ modeBadgeText }}</span>
        <span class="banner-badge is-subtle">{{ refreshBadgeText }}</span>
      </div>
    </section>

    <section class="metric-grid">
      <article class="metric-card is-blue">
        <div class="metric-card-head">
          <div class="metric-icon"><el-icon><Connection /></el-icon></div>
          <span class="metric-side-label">{{ stats?.role || t.common.unknown }}</span>
        </div>
        <div class="metric-value">{{ stats?.node_count ?? '-' }}</div>
        <div class="metric-label">{{ t.dashboard.nodes }}</div>
        <div class="metric-note">{{ leaderBadgeText || '-' }}</div>
      </article>

      <article class="metric-card is-cyan">
        <div class="metric-card-head">
          <div class="metric-icon"><el-icon><Grid /></el-icon></div>
          <span class="metric-side-label">{{ healthNote }}</span>
        </div>
        <div class="metric-value">
          <span>{{ stats?.service_count ?? '-' }}</span>
          <small>/ {{ stats?.instance_count ?? '-' }}</small>
        </div>
        <div class="metric-label">{{ t.dashboard.services }} / {{ t.dashboard.instances }}</div>
        <div class="metric-note">{{ modeBadgeText || '-' }}</div>
      </article>

      <article class="metric-card is-purple">
        <div class="metric-card-head">
          <div class="metric-icon"><el-icon><Monitor /></el-icon></div>
          <span class="metric-side-label">{{ locale === 'zh' ? '当前节点' : 'Current node' }}</span>
        </div>
        <div class="metric-value">{{ formatMemory(stats?.memory_usage) }}</div>
        <div class="metric-label">{{ t.dashboard.memoryUsage }}</div>
        <div class="metric-note">{{ t.dashboard.memoryDesc }}</div>
      </article>

      <article class="metric-card is-green">
        <div class="metric-card-head">
          <div class="metric-icon"><el-icon><CircleCheck /></el-icon></div>
          <span class="metric-side-label">{{ healthNote }}</span>
        </div>
        <div class="metric-value">{{ stats ? `${stats.health_rate.toFixed(1)}%` : '-' }}</div>
        <div class="metric-label">{{ t.dashboard.healthRate }}</div>
        <div class="metric-note">{{ refreshBadgeText }}</div>
      </article>
    </section>

    <section class="activity-grid">
      <article class="panel-card">
        <header class="panel-head">
          <div class="panel-copy">
            <h3 class="panel-title">{{ t.dashboard.recentEvents }}</h3>
            <p class="panel-subtitle">{{ eventsHintText }}</p>
          </div>
          <span class="panel-count">{{ events.length }}</span>
        </header>

        <div class="panel-scroll">
          <div v-if="loading" class="panel-empty">
            <el-icon><Loading /></el-icon>
            <p>{{ loadingLabel }}</p>
          </div>

          <div v-else-if="events.length === 0" class="panel-empty">
            <el-icon><DocumentDelete /></el-icon>
            <p>{{ t.dashboard.noEvents }}</p>
          </div>

          <template v-else>
            <div v-for="evt in events" :key="evt.id" class="event-entry">
              <div class="event-dot" :class="evt.type"></div>

              <div class="event-body">
                <div class="event-meta">
                  <span class="event-type">{{ getEventTypeLabel(evt.type) }}</span>
                  <span class="event-time">{{ formatTime(evt.timestamp) }}</span>
                </div>

                <div class="event-title">
                  <strong>{{ evt.service || '-' }}</strong>
                  <code>{{ evt.instance || '-' }}</code>
                </div>

                <p class="event-message">{{ evt.message || '-' }}</p>
              </div>
            </div>
          </template>
        </div>
      </article>

      <article class="panel-card">
        <header class="panel-head">
          <div class="panel-copy">
            <h3 class="panel-title">{{ logsTitle }}</h3>
            <p class="panel-subtitle">{{ logsHintText }}</p>
          </div>

          <div class="logs-head-actions">
            <span class="panel-mini-text">{{ activeLogName || logLinesLabel }}</span>
            <el-select
              v-model="activeLogFile"
              class="log-file-select"
              :placeholder="logSelectPlaceholder"
              clearable
              :disabled="logFiles.length === 0"
            >
              <el-option
                v-for="item in logFiles"
                :key="item.file"
                :label="item.name"
                :value="item.file"
              />
            </el-select>
          </div>
        </header>

        <div class="panel-scroll log-scroll">
          <div v-if="loading || logsLoading" class="panel-empty">
            <el-icon><Loading /></el-icon>
            <p>{{ loadingLabel }}</p>
          </div>

          <div v-else-if="logFiles.length === 0" class="panel-empty">
            <el-icon><Document /></el-icon>
            <p>{{ noLogFilesLabel }}</p>
          </div>

          <div v-else-if="logs.length === 0" class="panel-empty">
            <el-icon><Tickets /></el-icon>
            <p>{{ noLogsLabel }}</p>
          </div>

          <template v-else>
            <div
              v-for="(line, index) in logs"
              :key="`${activeLogFile}-${index}-${line}`"
              class="log-line"
            >
              <span class="log-index">{{ index + 1 }}</span>
              <code class="log-content">{{ line }}</code>
            </div>
          </template>
        </div>
      </article>
    </section>
  </div>
</template>

<style scoped>
.dashboard-shell {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
  overflow: hidden;
}

.overview-banner {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  padding: 24px 26px;
  border-radius: 24px;
  border: 1px solid rgba(96, 165, 250, 0.16);
  background:
    radial-gradient(circle at top right, rgba(14, 165, 233, 0.14), transparent 32%),
    linear-gradient(135deg, rgba(59, 130, 246, 0.08), rgba(16, 185, 129, 0.05)),
    var(--bg-secondary);
  box-shadow: 0 18px 40px rgba(15, 23, 42, 0.08);
  flex-shrink: 0;
}

.banner-copy {
  min-width: 0;
}

.banner-kicker {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--accent-blue);
  margin-bottom: 10px;
}

.banner-title {
  font-size: 28px;
  line-height: 1.1;
  font-weight: 700;
  letter-spacing: -0.03em;
  color: var(--text-primary);
  margin: 0 0 8px;
}

.banner-desc {
  max-width: 620px;
  font-size: 14px;
  line-height: 1.6;
  color: var(--text-secondary);
}

.banner-badges {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
}

.banner-badge {
  display: inline-flex;
  align-items: center;
  min-height: 34px;
  padding: 0 14px;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  background: rgba(255, 255, 255, 0.06);
  font-size: 13px;
  font-weight: 600;
  white-space: nowrap;
}

.banner-badge.is-primary {
  color: #0f766e;
  background: rgba(16, 185, 129, 0.12);
  border-color: rgba(16, 185, 129, 0.18);
}

.banner-badge.is-subtle {
  color: var(--text-secondary);
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  flex-shrink: 0;
}

.metric-card {
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-height: 164px;
  padding: 18px 18px 16px;
  border-radius: 22px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  background: var(--bg-secondary);
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.06);
}

.metric-card::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 3px;
  opacity: 0.8;
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
  width: 44px;
  height: 44px;
  border-radius: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
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
  max-width: 65%;
  padding: 4px 10px;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.08);
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.metric-value {
  display: flex;
  align-items: baseline;
  gap: 6px;
  font-size: 38px;
  font-weight: 700;
  line-height: 1;
  letter-spacing: -0.03em;
  color: var(--text-primary);
  margin-top: auto;
}

.metric-value small {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-muted);
}

.metric-label {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-secondary);
}

.metric-note {
  min-height: 20px;
  font-size: 12px;
  color: var(--text-muted);
}

.activity-grid {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(0, 1.12fr) minmax(360px, 0.88fr);
  gap: 16px;
}

.panel-card {
  min-height: 0;
  display: flex;
  flex-direction: column;
  border-radius: 24px;
  border: 1px solid rgba(148, 163, 184, 0.14);
  background: var(--bg-secondary);
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.05);
  overflow: hidden;
}

.panel-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  padding: 18px 18px 14px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.12);
  flex-shrink: 0;
}

.panel-copy {
  min-width: 0;
}

.panel-title {
  font-size: 18px;
  font-weight: 700;
  line-height: 1.2;
  color: var(--text-primary);
  margin: 0 0 6px;
}

.panel-subtitle {
  font-size: 13px;
  line-height: 1.5;
  color: var(--text-secondary);
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

.panel-mini-text {
  font-size: 12px;
  color: var(--text-muted);
  white-space: nowrap;
}

.logs-head-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.panel-scroll {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 12px 14px 16px;
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
  min-height: 180px;
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
  gap: 12px;
  padding: 14px 16px;
  border-radius: 18px;
  border: 1px solid rgba(148, 163, 184, 0.12);
  background: linear-gradient(135deg, rgba(148, 163, 184, 0.05), rgba(255, 255, 255, 0.02));
}

.event-entry + .event-entry {
  margin-top: 10px;
}

.event-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  margin-top: 6px;
  flex-shrink: 0;
  background: var(--accent-blue);
  box-shadow: 0 0 0 4px rgba(59, 130, 246, 0.08);
}

.event-dot.register {
  background: var(--accent-green);
  box-shadow: 0 0 0 4px rgba(16, 185, 129, 0.08);
}

.event-dot.deregister {
  background: var(--accent-red);
  box-shadow: 0 0 0 4px rgba(239, 68, 68, 0.08);
}

.event-dot.health_change {
  background: var(--accent-orange);
  box-shadow: 0 0 0 4px rgba(245, 158, 11, 0.08);
}

.event-body {
  flex: 1;
  min-width: 0;
}

.event-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.event-type {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  padding: 0 10px;
  border-radius: 999px;
  background: rgba(59, 130, 246, 0.08);
  color: var(--accent-blue);
  font-size: 12px;
  font-weight: 700;
}

.event-time {
  flex-shrink: 0;
  font-size: 12px;
  color: var(--text-muted);
}

.event-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  color: var(--text-primary);
  margin-bottom: 6px;
}

.event-title code {
  max-width: 60%;
  padding: 3px 8px;
  border-radius: 8px;
  background: rgba(148, 163, 184, 0.08);
  color: var(--text-secondary);
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.event-message {
  font-size: 13px;
  line-height: 1.6;
  color: var(--text-secondary);
}

.log-scroll {
  background:
    linear-gradient(180deg, rgba(15, 23, 42, 0.02), transparent 80%),
    var(--bg-secondary);
}

.log-line {
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr);
  gap: 12px;
  padding: 10px 0;
  border-bottom: 1px dashed rgba(148, 163, 184, 0.12);
}

.log-line:last-child {
  border-bottom: none;
}

.log-index {
  padding-top: 1px;
  font-size: 12px;
  font-weight: 700;
  color: var(--text-muted);
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.log-content {
  display: block;
  color: var(--text-primary);
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 12px;
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-word;
}

:deep(.log-file-select .el-select__wrapper) {
  min-height: 38px;
  border-radius: 12px;
  background: rgba(148, 163, 184, 0.06) !important;
  border: 1px solid rgba(148, 163, 184, 0.16) !important;
  box-shadow: none !important;
}

:deep(.log-file-select .el-select__placeholder),
:deep(.log-file-select .el-select__selected-item) {
  color: var(--text-primary) !important;
  font-size: 13px;
}

@media (max-width: 1360px) {
  .metric-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .activity-grid {
    grid-template-columns: 1fr;
    grid-template-rows: minmax(0, 1fr) minmax(0, 1fr);
  }
}

@media (max-width: 900px) {
  .dashboard-shell {
    gap: 14px;
  }

  .overview-banner {
    flex-direction: column;
    padding: 20px;
  }

  .banner-title {
    font-size: 24px;
  }

  .banner-badges {
    justify-content: flex-start;
  }

  .metric-grid {
    grid-template-columns: 1fr;
  }

  .panel-head {
    flex-direction: column;
  }

  .logs-head-actions {
    width: 100%;
    flex-direction: column;
    align-items: stretch;
  }

  .panel-mini-text {
    white-space: normal;
  }

  .event-meta {
    flex-direction: column;
    align-items: flex-start;
  }

  .event-title {
    flex-direction: column;
    align-items: flex-start;
  }

  .event-title code {
    max-width: 100%;
  }
}
</style>

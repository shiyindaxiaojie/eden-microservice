<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { getClusterStats, getEvents, type ClusterStats, type RegistryEvent } from '../api/registry'
import { useI18n } from '../utils/i18n'

const { t, locale } = useI18n()
const stats = ref<ClusterStats | null>(null)
const events = ref<RegistryEvent[]>([])
const loading = ref(true)
let timer: ReturnType<typeof setInterval> | null = null

async function fetchData() {
  try {
    const [statsRes, eventsRes] = await Promise.all([getClusterStats(), getEvents()])
    stats.value = statsRes.data
    events.value = eventsRes.data || []
  } catch (e) {
    console.error('fetch dashboard data failed', e)
  } finally {
    loading.value = false
  }
}

function formatTime(ts: string) {
  if (!ts) return '-'
  const d = new Date(ts)
  return d.toLocaleString(locale.value === 'zh' ? 'zh-CN' : 'en-US')
}

function formatMemory(bytes: number | undefined) {
  if (!bytes) return '-'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
  return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB'
}

onMounted(() => {
  fetchData()
  timer = setInterval(fetchData, 10000)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div>
    <!-- Stat Cards -->
    <div class="stat-cards">
      <div class="stat-card">
        <div class="stat-icon blue"><el-icon><Connection /></el-icon></div>
        <div class="stat-value">{{ stats?.node_count ?? '-' }}</div>
        <div class="stat-label">{{ t.dashboard.nodes }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon cyan"><el-icon><Grid /></el-icon></div>
        <div class="stat-value">
          <span class="main-val">{{ stats?.service_count ?? '-' }}</span>
          <span class="sub-val">/ {{ stats?.instance_count ?? '-' }}</span>
        </div>
        <div class="stat-label">{{ t.dashboard.services }} / {{ t.dashboard.instances }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon purple"><el-icon><Monitor /></el-icon></div>
        <div class="stat-value">{{ formatMemory(stats?.memory_usage) }}</div>
        <div class="stat-label">{{ t.dashboard.memoryUsage }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon green"><el-icon><CircleCheck /></el-icon></div>
        <div class="stat-value">{{ stats ? stats.health_rate.toFixed(1) + '%' : '-' }}</div>
        <div class="stat-label">{{ t.dashboard.healthRate }}</div>
      </div>
    </div>

    <div v-if="stats" style="margin-bottom: 24px; font-size: 13px; color: var(--text-secondary);">
      <el-tag v-if="stats.is_leader" type="success" size="small" effect="dark">{{ t.dashboard.isLeader }}</el-tag>
      <el-tag v-else type="info" size="small" effect="dark">{{ t.dashboard.leaderAddr }}: {{ stats.leader_addr || t.common.unknown }}</el-tag>
    </div>

    <!-- Recent Events -->
    <h3 class="section-title">{{ t.dashboard.recentEvents }}</h3>
    <div v-if="events.length === 0 && !loading" class="empty-state">
      <el-icon><DocumentDelete /></el-icon>
      <p>{{ t.dashboard.noEvents }}</p>
    </div>
    <div class="event-list">
      <div v-for="evt in events" :key="evt.id" class="event-item">
        <div class="event-dot" :class="evt.type"></div>
        <div class="event-body">
          <div class="event-msg">
            <strong>{{ evt.service }}</strong> · {{ evt.instance }} — {{ evt.message }}
          </div>
          <div class="event-time">{{ formatTime(evt.timestamp) }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

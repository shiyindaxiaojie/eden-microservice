<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { getClusterStats, getEvents, type ClusterStats, type RegistryEvent } from '../api/registry'

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
  return d.toLocaleString('zh-CN')
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
        <div class="stat-label">集群节点</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon purple"><el-icon><Grid /></el-icon></div>
        <div class="stat-value">{{ stats?.service_count ?? '-' }}</div>
        <div class="stat-label">注册服务</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon orange"><el-icon><Monitor /></el-icon></div>
        <div class="stat-value">{{ stats?.instance_count ?? '-' }}</div>
        <div class="stat-label">服务实例</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon green"><el-icon><CircleCheck /></el-icon></div>
        <div class="stat-value">{{ stats ? stats.health_rate.toFixed(1) + '%' : '-' }}</div>
        <div class="stat-label">健康率</div>
      </div>
    </div>

    <!-- Leader Info -->
    <div v-if="stats" style="margin-bottom: 24px; font-size: 13px; color: var(--text-secondary);">
      <el-tag v-if="stats.is_leader" type="success" size="small" effect="dark">当前节点为 Leader</el-tag>
      <el-tag v-else type="info" size="small" effect="dark">Leader: {{ stats.leader_addr || '未知' }}</el-tag>
    </div>

    <!-- Recent Events -->
    <h3 class="section-title">近期事件</h3>
    <div v-if="events.length === 0 && !loading" class="empty-state">
      <el-icon><DocumentDelete /></el-icon>
      <p>暂无事件</p>
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

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getServiceInstances, getSubscribers, setInstanceStatus, type Instance } from '../api/registry'
import { useI18n } from '../utils/i18n'
import { ArrowLeft, Refresh, Grid, Menu, Connection, Monitor } from '@element-plus/icons-vue'

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()

const serviceName = ref(route.params.name as string)
const namespace = ref((route.query.namespace as string) || 'default')
const instances = ref<Instance[]>([])
const subscriberCount = ref(0)
const loading = ref(true)
const userRole = ref(localStorage.getItem('user_role') || 'admin')
const canManage = computed(() => userRole.value === 'admin' || userRole.value === 'developer')

const searchIP = ref('')
const filterHealth = ref('')
const viewMode = ref<'grid' | 'list'>('list')

const filteredInstances = computed(() => {
  let res = instances.value
  if (searchIP.value) {
    const q = searchIP.value.toLowerCase()
    res = res.filter((i) => `${i.host}:${i.port}`.toLowerCase().includes(q))
  }
  if (filterHealth.value) {
    res = res.filter((i) => i.status === filterHealth.value)
  }
  return res
})

async function fetchInstances() {
  loading.value = true
  try {
    const [instancesRes, subscribersRes] = await Promise.all([
      getServiceInstances(serviceName.value, namespace.value),
      getSubscribers(serviceName.value, namespace.value)
    ])
    instances.value = instancesRes.data || []
    subscriberCount.value = (subscribersRes.data || []).length
  } catch (e) {
    console.error('fetch instances failed', e)
  } finally {
    loading.value = false
  }
}

function formatTime(ts: string) {
  if (!ts) return '-'
  return new Date(ts).toLocaleString(locale.value === 'zh' ? 'zh-CN' : 'en-US')
}

function formatMeta(meta: Record<string, string>) {
  if (!meta || Object.keys(meta).length === 0) return '-'
  return JSON.stringify(meta)
}

async function handleSetStatus(inst: Instance, status: 'online' | 'offline') {
  try {
    const msg = locale.value === 'zh'
      ? `确定将实例 ${inst.host}:${inst.port} 标记为${status === 'online' ? '上线' : '下线'}吗？`
      : `Are you sure to put instance ${inst.host}:${inst.port} ${status}?`
    await ElMessageBox.confirm(
      msg,
      status === 'online' ? 'Online Confirmation' : 'Offline Confirmation',
      {
        confirmButtonText: t.value.common.confirm,
        cancelButtonText: t.value.common.cancel,
        type: status === 'offline' ? 'warning' : 'info'
      }
    )
    await setInstanceStatus(inst.namespace || namespace.value, inst.service_name, inst.id, status)
    ElMessage.success(status === 'online' ? 'Instance Online' : 'Instance Offline')
    fetchInstances()
  } catch {
    // cancelled
  }
}

onMounted(fetchInstances)
</script>

<template>
  <div>
    <div style="margin-bottom: 20px;">
      <el-button text @click="router.push({ path: '/services', query: { namespace } })" style="color: var(--text-secondary)">
        <el-icon><ArrowLeft /></el-icon> {{ t.detail.backToServices }}
      </el-button>
      <el-button text @click="fetchInstances" style="color: var(--text-secondary); margin-left: 8px;">
        <el-icon><Refresh /></el-icon> {{ t.common.refresh }}
      </el-button>
    </div>

    <h3 class="section-title" style="margin-bottom: 24px;">
      {{ t.services.service }}: <span style="color: var(--accent-blue);">{{ serviceName }}</span>
      <el-tag size="small" effect="dark" style="margin-left: 12px;">{{ filteredInstances.length }} {{ t.detail.instanceCount }}</el-tag>
      <el-tag size="small" type="info" effect="plain" style="margin-left: 12px;">{{ namespace }}</el-tag>
      <el-tag size="small" type="warning" effect="plain" style="margin-left: 12px;">{{ subscriberCount }} {{ t.services.subscribers || 'Subscribers' }}</el-tag>
    </h3>

    <div class="search-bar" style="display: flex; gap: 16px; margin-bottom: 24px;">
      <el-input
        v-model="searchIP"
        placeholder="Filter by IP..."
        size="large"
        clearable
        style="width: 300px;"
      />
      <el-select v-model="filterHealth" placeholder="All Status" size="large" clearable style="width: 150px;">
        <el-option label="Passing" value="passing" />
        <el-option label="Critical" value="critical" />
      </el-select>
      <div style="flex: 1;"></div>
      <el-radio-group v-model="viewMode" size="large">
        <el-radio-button value="grid"><el-icon><Menu /></el-icon></el-radio-button>
        <el-radio-button value="list"><el-icon><Grid /></el-icon></el-radio-button>
      </el-radio-group>
    </div>

    <div v-if="!loading && filteredInstances.length === 0" class="empty-state">
      <el-icon><Monitor /></el-icon>
      <p>No instances found</p>
    </div>

    <div v-if="viewMode === 'grid'" class="service-grid" style="grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));">
      <div
        v-for="(inst, idx) in filteredInstances"
        :key="inst.id"
        class="service-card"
        :style="{ animationDelay: `${idx * 0.05}s` }"
      >
        <div class="svc-name" style="font-size: 16px;">
          <el-icon><Connection /></el-icon> {{ inst.host }}:{{ inst.port }}
        </div>
        <div class="svc-stats" style="flex-direction: column; align-items: flex-start; gap: 8px; margin-bottom: 16px;">
          <div style="font-size: 13px; color: var(--text-muted);">
            ID: <span style="color: var(--text-primary);">{{ inst.id }}</span>
          </div>
          <div style="font-size: 13px; color: var(--text-muted);">
            Weight: <span style="color: var(--text-primary);">{{ inst.weight }}</span> | DC: <span style="color: var(--text-primary);">{{ inst.datacenter || '-' }}</span>
          </div>
          <div style="font-size: 13px; color: var(--text-muted);">
            Namespace: <span style="color: var(--text-primary);">{{ inst.namespace || namespace }}</span>
          </div>
          <div>
            <el-tag v-if="inst.status === 'passing'" type="success" size="small" effect="dark">Passing</el-tag>
            <el-tag v-else type="danger" size="small" effect="dark">Critical</el-tag>
          </div>
        </div>
        <div v-if="canManage" class="card-actions" style="display: flex; gap: 8px; margin-top: auto; padding-top: 16px; border-top: 1px solid var(--border-color);">
          <el-button
            v-if="inst.status !== 'passing'"
            type="success"
            size="small"
            plain
            @click="handleSetStatus(inst, 'online')"
            style="flex: 1;"
          >
            Online
          </el-button>
          <el-button
            v-if="inst.status !== 'critical'"
            type="danger"
            size="small"
            plain
            @click="handleSetStatus(inst, 'offline')"
            style="flex: 1;"
          >
            Offline
          </el-button>
        </div>
      </div>
    </div>

    <div v-else class="glass-table-wrapper">
      <el-table :data="filteredInstances" v-loading="loading" height="100%" style="width: 100%">
        <el-table-column :label="t.detail.instanceId" prop="id" min-width="160" />
        <el-table-column :label="t.common.address" min-width="160">
          <template #default="{ row }"><span class="ip-address">{{ row.host }}:{{ row.port }}</span></template>
        </el-table-column>
        <el-table-column label="Namespace" min-width="120">
          <template #default="{ row }">{{ row.namespace || namespace }}</template>
        </el-table-column>
        <el-table-column :label="t.detail.healthStatus" min-width="100">
          <template #default="{ row }">
            <el-tag v-if="row.status === 'passing'" type="success" size="small" effect="dark">{{ t.services.healthy }}</el-tag>
            <el-tag v-else type="danger" size="small" effect="dark">{{ t.common.warning }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t.common.weight" prop="weight" min-width="80" />
        <el-table-column :label="t.common.metadata" min-width="200">
          <template #default="{ row }">
            <span style="font-size: 12px; color: var(--text-muted);">{{ formatMeta(row.metadata) }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t.detail.lastHeartbeat" min-width="180">
          <template #default="{ row }">{{ formatTime(row.last_heartbeat) }}</template>
        </el-table-column>
        <el-table-column :label="t.detail.registeredAt" min-width="180">
          <template #default="{ row }">{{ formatTime(row.registered_at) }}</template>
        </el-table-column>
        <el-table-column v-if="canManage" :label="t.common.actions" width="160" fixed="right">
          <template #default="{ row }">
            <el-button v-if="row.status !== 'passing'" type="success" size="small" plain @click="handleSetStatus(row, 'online')">Online</el-button>
            <el-button v-if="row.status !== 'critical'" type="danger" size="small" plain @click="handleSetStatus(row, 'offline')">Offline</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
  </div>
</template>

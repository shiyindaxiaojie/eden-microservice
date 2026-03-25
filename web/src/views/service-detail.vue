<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getServiceInstances, getSubscribers, setInstanceStatus, type Instance } from '../api/registry'
import { useI18n } from '../utils/i18n'
import { ArrowLeft, Refresh, Monitor, Menu, Grid } from '@element-plus/icons-vue'

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

const detailCurrentPage = ref(1)
const detailPageSize = ref(10)

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

const pagedInstances = computed(() => {
  const start = (detailCurrentPage.value - 1) * detailPageSize.value
  return filteredInstances.value.slice(start, start + detailPageSize.value)
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
  <div style="display: flex; flex-direction: column; height: 100%;">
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px; flex-shrink: 0;">
      <h3 class="section-title" style="margin: 0;">
        {{ t.services.service }}: <span style="color: var(--accent-blue);">{{ serviceName }}</span>
        <el-tag size="small" effect="dark" style="margin-left: 12px;">{{ filteredInstances.length }} {{ t.detail.instanceCount }}</el-tag>
        <el-tag size="small" type="warning" effect="plain" style="margin-left: 12px;">{{ subscriberCount }} {{ t.services.subscribers || 'Subscribers' }}</el-tag>
        <el-tag size="small" type="info" effect="plain" style="margin-left: 12px;">{{ namespace }}</el-tag>
      </h3>
      <div style="display: flex; gap: 8px;">
        <el-button @click="fetchInstances">
          <el-icon><Refresh /></el-icon> <span style="margin-left: 4px;">刷新</span>
        </el-button>
        <el-button @click="router.push({ path: '/services', query: { namespace } })">
          <el-icon><ArrowLeft /></el-icon> <span style="margin-left: 4px;">返回服务列表</span>
        </el-button>
      </div>
    </div>

    <div class="search-bar" style="display: flex; align-items: center; gap: 16px; margin-bottom: 16px; flex-shrink: 0;">
      <div style="display: flex; align-items: center; gap: 8px;">
        <span style="font-size: 13px; color: var(--text-secondary); white-space: nowrap;">IP</span>
        <el-input
          v-model="searchIP"
          placeholder="输入 IP"
          size="default"
          clearable
          style="width: 240px;"
        />
      </div>
      <div style="display: flex; align-items: center; gap: 8px;">
        <span style="font-size: 13px; color: var(--text-secondary); white-space: nowrap;">状态</span>
        <el-select v-model="filterHealth" placeholder="全部状态" size="default" clearable style="width: 140px;">
          <el-option label="健康" value="passing" />
          <el-option label="异常" value="critical" />
        </el-select>
      </div>
      <div style="flex: 1;"></div>
      <el-radio-group v-model="viewMode" size="default">
        <el-tooltip content="列表视图" placement="top"><el-radio-button value="list"><el-icon><Menu /></el-icon></el-radio-button></el-tooltip>
        <el-tooltip content="网格视图" placement="top"><el-radio-button value="grid"><el-icon><Grid /></el-icon></el-radio-button></el-tooltip>
      </el-radio-group>
    </div>

    <div v-if="!loading && filteredInstances.length === 0" class="empty-state" style="flex: 1;">
      <el-icon><Monitor /></el-icon>
      <p>No instances found</p>
    </div>

    <div v-else-if="viewMode === 'grid'" class="service-grid" style="display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 20px; flex: 1; overflow-y: auto; padding: 4px;">
      <div
        v-for="(inst, idx) in pagedInstances"
        :key="inst.id"
        class="inst-card glass-panel"
        :style="{ animationDelay: `${idx * 0.05}s` }"
      >
        <div class="inst-header">
          <div class="inst-title">
             <div class="status-indicator" :class="inst.status === 'passing' ? 'passing' : 'critical'"></div>
             <strong>{{ inst.host }}:{{ inst.port }}</strong>
          </div>
          <el-tag v-if="inst.status === 'passing'" type="success" size="small" effect="light" class="health-tag">健康</el-tag>
          <el-tag v-else type="danger" size="small" effect="light" class="health-tag">异常</el-tag>
        </div>
        
        <div class="inst-body">
          <div class="info-kv">
            <span class="info-k">实例 ID</span>
            <span class="info-v">{{ inst.id }}</span>
          </div>
          <div class="info-kv-group">
            <div class="info-kv">
              <span class="info-k">权重</span>
              <span class="info-v">{{ inst.weight }}</span>
            </div>
            <div class="info-kv">
              <span class="info-k">数据中心</span>
              <span class="info-v">{{ inst.datacenter || '-' }}</span>
            </div>
            <div class="info-kv">
              <span class="info-k">命名空间</span>
              <span class="info-v">{{ inst.namespace || namespace }}</span>
            </div>
          </div>
        </div>

        <div v-if="canManage" class="inst-footer">
          <button
            v-if="inst.status !== 'passing'"
            type="button"
            class="action-btn success-btn"
            @click="handleSetStatus(inst, 'online')"
          >
            上线实例
          </button>
          <button
            v-if="inst.status !== 'critical'"
            type="button"
            class="action-btn danger-btn"
            @click="handleSetStatus(inst, 'offline')"
          >
            下线实例
          </button>
        </div>
      </div>
    </div>

    <div v-else-if="viewMode === 'list'" class="glass-table-wrapper" style="flex: 1; display: flex; flex-direction: column; overflow: hidden;">
      <el-table :data="pagedInstances" v-loading="loading" height="100%" style="width: 100%; flex: 1;">
        <el-table-column type="index" label="序号" width="60" align="center" />
        <el-table-column :label="t.detail.instanceId" prop="id" min-width="150" />
        <el-table-column :label="t.common.address" min-width="120">
          <template #default="{ row }"><span style="font-family: inherit;">{{ row.host }}:{{ row.port }}</span></template>
        </el-table-column>
        <el-table-column label="命名空间" min-width="100">
          <template #default="{ row }">{{ row.namespace || namespace }}</template>
        </el-table-column>
        <el-table-column :label="t.detail.healthStatus" min-width="80">
          <template #default="{ row }">
            <el-tag v-if="row.status === 'passing'" type="success" size="small" effect="dark">{{ t.services.healthy }}</el-tag>
            <el-tag v-else type="danger" size="small" effect="dark">{{ t.common.warning }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t.common.weight" prop="weight" min-width="80" />
        <el-table-column :label="t.common.metadata" min-width="150">
          <template #default="{ row }">
            <span style="font-size: 12px; color: var(--text-muted);">{{ formatMeta(row.metadata) }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t.detail.lastHeartbeat" min-width="160">
          <template #default="{ row }">{{ formatTime(row.last_heartbeat) }}</template>
        </el-table-column>
        <el-table-column :label="t.detail.registeredAt" min-width="160">
          <template #default="{ row }">{{ formatTime(row.registered_at) }}</template>
        </el-table-column>
        <el-table-column v-if="canManage" :label="t.common.actions" width="120" fixed="right">
          <template #default="{ row }">
            <el-button v-if="row.status !== 'passing'" type="success" size="small" text @click="handleSetStatus(row, 'online')">上线</el-button>
            <el-button v-if="row.status !== 'critical'" type="danger" size="small" text @click="handleSetStatus(row, 'offline')">下线</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div style="display: flex; justify-content: flex-end; padding: 16px 24px; border-top: 1px solid var(--border-color); background: var(--bg-primary); border-radius: 0 0 12px 12px; flex-shrink: 0;">
        <el-pagination
          v-model:current-page="detailCurrentPage"
          v-model:page-size="detailPageSize"
          :total="filteredInstances.length"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next"
          size="small"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.inst-card {
  display: flex;
  flex-direction: column;
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  overflow: hidden;
  transition: border-color 0.2s ease, background-color 0.2s ease, box-shadow 0.2s ease;
  box-shadow: none;
}

.inst-card:hover {
  border-color: rgba(59, 130, 246, 0.3);
  background: rgba(59, 130, 246, 0.03);
  box-shadow: inset 0 0 0 1px rgba(59, 130, 246, 0.08);
}

.inst-header {
  padding: 16px 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid var(--border-color);
  background: var(--bg-glass);
}

.inst-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.inst-title strong {
  font-family: inherit;
  font-size: 15px;
  color: var(--text-primary);
  font-weight: 600;
  letter-spacing: 0.5px;
}

.status-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #cbd5e1;
}

.status-indicator.passing {
  background: #10b981;
  box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.15);
}

.status-indicator.critical {
  background: #ef4444;
  box-shadow: 0 0 0 3px rgba(239, 68, 68, 0.15);
}

.health-tag {
  font-weight: 600;
}

.inst-body {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  flex: 1;
}

.info-kv-group {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}

.info-kv {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.info-k {
  font-size: 11px;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  font-weight: 500;
}

.info-v {
  font-size: 13px;
  color: var(--text-primary);
  font-weight: 500;
}

.inst-footer {
  padding: 12px 20px;
  background: rgba(0,0,0,0.01);
  border-top: 1px solid var(--border-color);
  display: flex;
  gap: 12px;
}

.action-btn {
  flex: 1;
  padding: 8px 0;
  border: 0;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
  font-family: inherit;
}

.success-btn {
  background: rgba(16, 185, 129, 0.1);
  color: #10b981;
}
.success-btn:hover {
  background: #10b981;
  color: #fff;
}

.danger-btn {
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
}
.danger-btn:hover {
  background: #ef4444;
  color: #fff;
}
</style>

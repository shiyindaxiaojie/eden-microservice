<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft, RefreshRight, Search } from '@element-plus/icons-vue'
import { getServiceInstances, setInstanceStatus, type Instance } from '../api/registry'
import { useI18n } from '../utils/i18n'

type StatusFilter = 'all' | 'passing' | 'critical' | 'manual_offline'

const route = useRoute()
const router = useRouter()
const { locale } = useI18n()

const isZh = computed(() => locale.value === 'zh')
const text = (zh: string, en: string) => (isZh.value ? zh : en)

const instances = ref<Instance[]>([])
const loading = ref(false)
const actionTarget = ref('')
const search = ref('')
const statusFilter = ref<StatusFilter>('all')

function decodeName(value: string) {
  try {
    return decodeURIComponent(value)
  } catch {
    return value
  }
}

const serviceName = computed(() => decodeName(String(route.params.name || '')))
const namespace = computed(() => {
  const raw = route.query.namespace
  return typeof raw === 'string' && raw.trim() ? raw : 'default'
})

const stats = computed(() => {
  const total = instances.value.length
  const passing = instances.value.filter((item) => !item.manual_offline && item.status === 'passing').length
  const critical = instances.value.filter((item) => !item.manual_offline && item.status === 'critical').length
  const manualOffline = instances.value.filter((item) => item.manual_offline).length
  return { total, passing, critical, manualOffline }
})

const filteredInstances = computed(() => {
  const query = search.value.trim().toLowerCase()
  return instances.value.filter((item) => {
    const addr = `${item.host}:${item.port}`.toLowerCase()
    const matchesSearch = !query || item.id.toLowerCase().includes(query) || addr.includes(query)

    switch (statusFilter.value) {
      case 'passing':
        return matchesSearch && !item.manual_offline && item.status === 'passing'
      case 'critical':
        return matchesSearch && !item.manual_offline && item.status === 'critical'
      case 'manual_offline':
        return matchesSearch && !!item.manual_offline
      default:
        return matchesSearch
    }
  })
})

function formatTime(value?: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString(isZh.value ? 'zh-CN' : 'en-US')
}

function instanceAddress(instance: Instance) {
  return `${instance.host}:${instance.port}`
}

function instanceStateText(instance: Instance) {
  if (instance.manual_offline) return text('手动下线', 'Manual Offline')
  return instance.status === 'passing' ? text('健康', 'Passing') : text('异常', 'Critical')
}

function instanceStateType(instance: Instance) {
  if (instance.manual_offline) return 'warning'
  return instance.status === 'passing' ? 'success' : 'danger'
}

function actionLabel(instance: Instance) {
  return instance.manual_offline ? text('上线', 'Online') : text('下线', 'Offline')
}

async function loadInstances() {
  if (!serviceName.value) {
    instances.value = []
    return
  }

  loading.value = true
  try {
    const response = await getServiceInstances(serviceName.value, namespace.value)
    instances.value = response.data || []
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || text('加载服务详情失败', 'Failed to load service detail'))
  } finally {
    loading.value = false
  }
}

function goBack() {
  router.push({
    path: '/services',
    query: { namespace: namespace.value },
  })
}

async function toggleStatus(instance: Instance) {
  const nextStatus = instance.manual_offline ? 'online' : 'offline'
  const addr = instanceAddress(instance)

  try {
    await ElMessageBox.confirm(
      nextStatus === 'online'
        ? text(`确定要将实例 ${addr} 恢复上线吗？`, `Bring instance ${addr} online?`)
        : text(`确定要将实例 ${addr} 手动下线吗？`, `Take instance ${addr} offline?`),
      text('状态确认', 'Status Confirmation'),
      { type: 'warning' },
    )
  } catch {
    return
  }

  actionTarget.value = instance.id
  try {
    await setInstanceStatus(namespace.value, serviceName.value, instance.id, nextStatus)
    ElMessage.success(
      nextStatus === 'online'
        ? text('实例已恢复上线', 'Instance is back online')
        : text('实例已手动下线', 'Instance is offline'),
    )
    await loadInstances()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || text('实例状态更新失败', 'Failed to update instance status'))
  } finally {
    actionTarget.value = ''
  }
}

watch(
  () => [route.params.name, route.query.namespace],
  () => {
    loadInstances().catch((error) => console.error('load service detail failed', error))
  },
)

onMounted(() => {
  loadInstances().catch((error) => console.error('load service detail failed', error))
})
</script>

<template>
  <div class="detail-shell">
    <section class="detail-hero">
      <button type="button" class="back-btn" @click="goBack">
        <el-icon><ArrowLeft /></el-icon>
        <span>{{ text('返回服务列表', 'Back to Services') }}</span>
      </button>

      <div class="hero-head">
        <div>
          <p class="eyebrow">{{ text('服务详情', 'Service Detail') }}</p>
          <h1>{{ serviceName }}</h1>
          <div class="hero-meta">
            <span class="hero-badge">{{ text('命名空间', 'Namespace') }}: {{ namespace }}</span>
            <span class="hero-note">
              {{ stats.total }} {{ text('个实例', 'instances') }}
            </span>
          </div>
        </div>

        <button type="button" class="refresh-btn" :disabled="loading" @click="loadInstances()">
          <el-icon><RefreshRight /></el-icon>
          <span>{{ text('刷新', 'Refresh') }}</span>
        </button>
      </div>

      <div class="stat-grid">
        <article class="stat-card">
          <span>{{ text('总实例数', 'Total Instances') }}</span>
          <strong>{{ stats.total }}</strong>
        </article>
        <article class="stat-card ok">
          <span>{{ text('健康实例', 'Passing') }}</span>
          <strong>{{ stats.passing }}</strong>
        </article>
        <article class="stat-card danger">
          <span>{{ text('异常实例', 'Critical') }}</span>
          <strong>{{ stats.critical }}</strong>
        </article>
        <article class="stat-card warn">
          <span>{{ text('手动下线', 'Manual Offline') }}</span>
          <strong>{{ stats.manualOffline }}</strong>
        </article>
      </div>
    </section>

    <section class="detail-panel">
      <div class="panel-toolbar">
        <el-input
          v-model="search"
          clearable
          :prefix-icon="Search"
          :placeholder="text('按实例 ID 或 IP 过滤', 'Filter by instance ID or IP')"
          class="search-box"
        />

        <el-select v-model="statusFilter" class="status-select">
          <el-option :label="text('全部状态', 'All Status')" value="all" />
          <el-option :label="text('健康', 'Passing')" value="passing" />
          <el-option :label="text('异常', 'Critical')" value="critical" />
          <el-option :label="text('手动下线', 'Manual Offline')" value="manual_offline" />
        </el-select>
      </div>

      <div class="table-shell" v-loading="loading">
        <div v-if="!loading && filteredInstances.length === 0" class="empty-state">
          <strong>{{ text('当前没有可展示的实例', 'No instances to display') }}</strong>
          <p>{{ text('可以尝试切换筛选条件或返回服务列表查看其他服务。', 'Try another filter or go back to the service list.') }}</p>
        </div>

        <el-table v-else :data="filteredInstances" stripe>
          <el-table-column prop="id" :label="text('实例 ID', 'Instance ID')" min-width="240" />

          <el-table-column :label="text('地址', 'Address')" min-width="180">
            <template #default="{ row }">
              <span class="mono">{{ instanceAddress(row) }}</span>
            </template>
          </el-table-column>

          <el-table-column :label="text('状态', 'Status')" width="150">
            <template #default="{ row }">
              <el-tag :type="instanceStateType(row)" effect="light">
                {{ instanceStateText(row) }}
              </el-tag>
            </template>
          </el-table-column>

          <el-table-column prop="datacenter" :label="text('数据中心', 'Datacenter')" width="140" />

          <el-table-column prop="weight" :label="text('权重', 'Weight')" width="110" align="center" />

          <el-table-column :label="text('最近心跳', 'Last Heartbeat')" min-width="180">
            <template #default="{ row }">
              {{ formatTime(row.last_heartbeat) }}
            </template>
          </el-table-column>

          <el-table-column :label="text('注册时间', 'Registered At')" min-width="180">
            <template #default="{ row }">
              {{ formatTime(row.registered_at) }}
            </template>
          </el-table-column>

          <el-table-column :label="text('操作', 'Actions')" width="140" fixed="right">
            <template #default="{ row }">
              <el-button
                size="small"
                :type="row.manual_offline ? 'success' : 'warning'"
                :loading="actionTarget === row.id"
                @click="toggleStatus(row)"
              >
                {{ actionLabel(row) }}
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </section>
  </div>
</template>

<style scoped>
.detail-shell {
  display: flex;
  flex-direction: column;
  gap: 24px;
  padding: 24px;
  min-height: 100%;
  background:
    radial-gradient(circle at top right, rgba(34, 197, 94, 0.12), transparent 30%),
    radial-gradient(circle at left center, rgba(14, 165, 233, 0.12), transparent 28%),
    linear-gradient(180deg, #f8fbff 0%, #f4f7fb 100%);
}

.detail-hero,
.detail-panel {
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 24px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 18px 60px rgba(15, 23, 42, 0.08);
  backdrop-filter: blur(18px);
}

.detail-hero {
  padding: 28px;
}

.back-btn,
.refresh-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 999px;
  background: #ffffff;
  color: #0f172a;
  cursor: pointer;
  transition: transform 0.18s ease, box-shadow 0.18s ease, border-color 0.18s ease;
}

.back-btn:hover,
.refresh-btn:hover {
  transform: translateY(-1px);
  border-color: rgba(37, 99, 235, 0.25);
  box-shadow: 0 12px 30px rgba(37, 99, 235, 0.12);
}

.back-btn {
  padding: 10px 16px;
  margin-bottom: 20px;
}

.refresh-btn {
  padding: 10px 16px;
}

.refresh-btn:disabled {
  cursor: not-allowed;
  opacity: 0.7;
}

.hero-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.eyebrow {
  margin: 0 0 8px;
  font-size: 12px;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: #64748b;
}

.hero-head h1 {
  margin: 0;
  font-size: clamp(28px, 4vw, 40px);
  line-height: 1.05;
  color: #0f172a;
}

.hero-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin-top: 14px;
}

.hero-badge,
.hero-note {
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  padding: 8px 12px;
  font-size: 13px;
}

.hero-badge {
  background: rgba(37, 99, 235, 0.12);
  color: #1d4ed8;
}

.hero-note {
  background: rgba(148, 163, 184, 0.12);
  color: #475569;
}

.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin-top: 22px;
}

.stat-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 18px 20px;
  border-radius: 18px;
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.96), rgba(241, 245, 249, 0.96));
  border: 1px solid rgba(148, 163, 184, 0.18);
}

.stat-card span {
  color: #64748b;
  font-size: 13px;
}

.stat-card strong {
  color: #0f172a;
  font-size: 30px;
  line-height: 1;
}

.stat-card.ok {
  background: linear-gradient(180deg, rgba(236, 253, 245, 1), rgba(220, 252, 231, 0.95));
}

.stat-card.ok strong {
  color: #047857;
}

.stat-card.danger {
  background: linear-gradient(180deg, rgba(254, 242, 242, 1), rgba(254, 226, 226, 0.95));
}

.stat-card.danger strong {
  color: #b91c1c;
}

.stat-card.warn {
  background: linear-gradient(180deg, rgba(255, 251, 235, 1), rgba(254, 243, 199, 0.95));
}

.stat-card.warn strong {
  color: #b45309;
}

.detail-panel {
  padding: 22px;
}

.panel-toolbar {
  display: flex;
  gap: 14px;
  margin-bottom: 18px;
}

.search-box {
  flex: 1;
}

.status-select {
  width: 200px;
}

.table-shell {
  overflow: hidden;
  border-radius: 18px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  background: #ffffff;
}

.empty-state {
  padding: 48px 24px;
  text-align: center;
  color: #475569;
}

.empty-state strong {
  display: block;
  margin-bottom: 8px;
  color: #0f172a;
  font-size: 16px;
}

.empty-state p {
  margin: 0;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
}

@media (max-width: 1080px) {
  .stat-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .detail-shell {
    padding: 16px;
  }

  .detail-hero,
  .detail-panel {
    border-radius: 20px;
  }

  .hero-head,
  .panel-toolbar {
    flex-direction: column;
  }

  .status-select {
    width: 100%;
  }

  .stat-grid {
    grid-template-columns: 1fr;
  }
}
</style>

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
const { text, elementLocale, localeTag } = useI18n()


const instances = ref<Instance[]>([])
const loading = ref(false)
const actionTarget = ref('')
const searchId = ref('')
const searchIp = ref('')
const statusFilter = ref<StatusFilter>('all')
const currentPage = ref(1)
const pageSize = ref(12)

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
  const passing = instances.value.filter((item) => item.status === 'passing').length
  const critical = instances.value.filter((item) => item.status === 'critical').length
  const manualOffline = instances.value.filter((item) => item.manual_offline).length
  return {
    total,
    passing,
    critical,
    manualOffline,
    online: total - manualOffline,
  }
})

const filteredInstances = computed(() => {
  const idQuery = searchId.value.trim().toLowerCase()
  const ipQuery = searchIp.value.trim().toLowerCase()

  return instances.value.filter((item) => {
    const addr = `${item.host}:${item.port}`.toLowerCase()
    const matchesId = !idQuery || item.id.toLowerCase().includes(idQuery)
    const matchesIp = !ipQuery || addr.includes(ipQuery)
    const matchesSearch = matchesId && matchesIp

    const matchesStatus =
      statusFilter.value === 'all' ||
      (statusFilter.value === 'passing' && item.status === 'passing' && !item.manual_offline) ||
      (statusFilter.value === 'critical' && item.status === 'critical' && !item.manual_offline) ||
      (statusFilter.value === 'manual_offline' && !!item.manual_offline)

    return matchesSearch && matchesStatus
  })
})

const pagedInstances = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredInstances.value.slice(start, start + pageSize.value)
})

const totalPages = computed(() => Math.max(1, Math.ceil(filteredInstances.value.length / pageSize.value)))

function formatTime(value?: string) {
  if (!value || value.startsWith('0001')) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString(localeTag.value)
}

function instanceAddress(instance: Instance) {
  return `${instance.host}:${instance.port}`
}

function isManuallyOffline(instance: Instance) {
  return !!instance.manual_offline
}

function instanceStateText(instance: Instance) {
  return isManuallyOffline(instance) ? text('已下线', 'Offline') : text('在线', 'Online')
}

function instanceStateType(instance: Instance) {
  return isManuallyOffline(instance) ? 'warning' : 'success'
}

function actionLabel(instance: Instance) {
  return isManuallyOffline(instance) ? text('恢复上线', 'Restore Online') : text('下线', 'Offline')
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
  const isRestore = isManuallyOffline(instance)
  const nextStatus = isRestore ? 'online' : 'offline'
  const addr = instanceAddress(instance)

  try {
    await ElMessageBox.confirm(
      isRestore
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
      isRestore
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

watch([searchId, searchIp, statusFilter], () => {
  currentPage.value = 1
})

watch(pageSize, () => {
  currentPage.value = 1
})

watch(filteredInstances, () => {
  if (currentPage.value > totalPages.value) {
    currentPage.value = totalPages.value
  }
})

onMounted(() => {
  loadInstances().catch((error) => console.error('load service detail failed', error))
})
</script>

<template>
  <div class="svc-shell">
    <div class="svc-main glass-card">
      <div class="svc-toolbar">
        <div class="toolbar-row">
          <div class="toolbar-group">
            <button type="button" class="back-link-btn" @click="goBack" :title="text('返回服务列表', 'Back to Services')">
              <el-icon><ArrowLeft /></el-icon>
            </button>
            <div class="title-wrap">
              <h1 class="page-title">{{ serviceName }}</h1>
              <span class="ns-badge">{{ namespace }}</span>
            </div>
          </div>

          <div class="toolbar-group">
            <div class="field-item">
              <span class="field-label">{{ text('实例 ID', 'Instance ID') }}</span>
              <el-input
                v-model="searchId"
                size="default"
                clearable
                :prefix-icon="Search"
                :placeholder="text('输入 ID', 'Filter ID')"
                class="search-input"
              />
            </div>
          </div>

          <div class="toolbar-group">
            <div class="field-item">
              <span class="field-label">{{ text('IP 地址', 'IP Address') }}</span>
              <el-input
                v-model="searchIp"
                size="default"
                clearable
                :prefix-icon="Search"
                :placeholder="text('输入 IP', 'Filter IP')"
                class="search-input"
              />
            </div>
          </div>

          <div class="toolbar-group">
            <button type="button" class="refresh-btn-minimal" :disabled="loading" @click="loadInstances()">
              <el-icon :class="{ 'is-loading': loading }"><RefreshRight /></el-icon>
            </button>
          </div>

          <div class="toolbar-group right-align">
            <div class="pill-group">
              <button
                type="button"
                :class="{ active: statusFilter === 'all' }"
                @click="statusFilter = 'all'"
              >
                {{ text('全部', 'All') }}
                <span class="pill-count">{{ stats.total }}</span>
              </button>
              <button
                type="button"
                :class="{ active: statusFilter === 'passing' }"
                @click="statusFilter = 'passing'"
              >
                {{ text('健康', 'OK') }}
                <span class="pill-count">{{ stats.passing }}</span>
              </button>
              <button
                type="button"
                :class="{ active: statusFilter === 'critical' }"
                @click="statusFilter = 'critical'"
              >
                {{ text('异常', 'Err') }}
                <span class="pill-count" :class="{ 'err-count': stats.critical > 0 }">{{ stats.critical }}</span>
              </button>
              <button
                type="button"
                :class="{ active: statusFilter === 'manual_offline' }"
                @click="statusFilter = 'manual_offline'"
              >
                {{ text('下线', 'Off') }}
                <span class="pill-count">{{ stats.manualOffline }}</span>
              </button>
            </div>
          </div>
        </div>
      </div>

      <section class="svc-content" v-loading="loading">
        <div class="table-wrap">
          <div v-if="!loading && filteredInstances.length === 0" class="empty-state">
            <strong>{{ text('当前没有可展示的实例', 'No instances to display') }}</strong>
            <p>{{ text('可以尝试切换筛选条件或返回服务列表查看其他服务。', 'Try another filter or go back to the service list.') }}</p>
          </div>

          <el-table
            v-else
            :data="pagedInstances"
            height="100%"
            style="width: 100%; font-size: 14px;"
          >
            <el-table-column type="index" :label="text('序号', 'No.')" width="60" align="center" />
            <el-table-column prop="id" :label="text('实例 ID', 'Instance ID')" min-width="200" />

            <el-table-column :label="text('地址', 'Address')" min-width="150">
              <template #default="{ row }">
                <span class="mono">{{ instanceAddress(row) }}</span>
              </template>
            </el-table-column>

            <el-table-column :label="text('状态', 'Status')" width="100">
              <template #default="{ row }">
                <el-tag :type="instanceStateType(row)" effect="light">
                  {{ instanceStateText(row) }}
                </el-tag>
              </template>
            </el-table-column>

            <el-table-column prop="datacenter" :label="text('数据中心', 'Datacenter')" width="100" />

            <el-table-column prop="weight" :label="text('权重', 'Weight')" width="100" align="center" />

            <el-table-column :label="text('最近心跳', 'Last Heartbeat')" min-width="150">
              <template #default="{ row }">
                {{ formatTime(row.last_heartbeat || row.lastHeartbeat) }}
              </template>
            </el-table-column>

            <el-table-column :label="text('注册时间', 'Registered At')" min-width="150">
              <template #default="{ row }">
                {{ formatTime(row.registered_at || row.registeredAt) }}
              </template>
            </el-table-column>

            <el-table-column :label="text('操作', 'Actions')" width="100" fixed="right" align="center">
              <template #default="{ row }">
                <el-button
                  link
                  style="font-weight: 500;"
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

        <footer class="svc-footer">
          <span class="footer-info">{{ filteredInstances.length }} {{ text('条', 'records') }}</span>
          <el-config-provider :locale="elementLocale">
            <el-pagination
              v-model:current-page="currentPage"
              v-model:page-size="pageSize"
              :page-sizes="[12, 20, 50]"
              :total="filteredInstances.length"
              layout="sizes, prev, pager, next"
              background
            />
          </el-config-provider>
        </footer>
      </section>
    </div>
  </div>
</template>

<style scoped>
.svc-shell {
  display: flex;
  flex-direction: column;
  height: 100%;
  padding: 24px;
}

.svc-main {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  border-radius: 12px;
  overflow: hidden;
  background: var(--bg-card);
}

.svc-toolbar {
  padding: 16px 24px;
  border-bottom: 1px solid var(--border-color);
}

.toolbar-row {
  display: flex;
  align-items: center;
  gap: 0;
  flex-wrap: nowrap;
  min-width: 0;
}

.toolbar-group {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 12px;
  min-width: 0;
  flex-shrink: 0;
}

.toolbar-group:first-child {
  padding-left: 0;
}

.toolbar-group:last-child {
  padding-right: 0;
}

.toolbar-group.right-align {
  margin-left: auto;
}

.back-link-btn {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s;
}

.back-link-btn:hover {
  border-color: var(--accent-blue);
  color: var(--accent-blue);
  background: rgba(59, 130, 246, 0.05);
}

.title-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
}

.page-title {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: -0.01em;
}

.ns-badge {
  padding: 2px 8px;
  background: rgba(148, 163, 184, 0.1);
  color: var(--text-muted);
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
}

.field-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.field-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-muted);
  white-space: nowrap;
}

.search-input {
  width: 160px;
}

.refresh-btn-minimal {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.2s;
}

.refresh-btn-minimal:hover:not(:disabled) {
  border-color: var(--accent-blue);
  color: var(--accent-blue);
}

.refresh-btn-minimal:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.is-loading {
  animation: rotating 1.5s linear infinite;
}

@keyframes rotating {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* Pill Group styles aligned with services.vue */
.pill-group {
  display: flex;
  gap: 2px;
  padding: 2px;
  background: rgba(148, 163, 184, 0.1);
  border-radius: 10px;
}

.pill-group button {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 12px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--text-muted);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.pill-group button:hover {
  color: var(--text-secondary);
}

.pill-group button.active {
  background: var(--bg-card);
  color: var(--accent-blue);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
}

.pill-count {
  font-size: 11px;
  padding: 1px 6px;
  background: rgba(148, 163, 184, 0.15);
  border-radius: 6px;
  color: var(--text-muted);
}

.err-count {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
}

.svc-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 0 0;
  flex-shrink: 0;
  background: transparent;
}

.footer-info {
  font-size: 14px;
  color: var(--text-muted);
}

.svc-content {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding: 12px 24px 12px;
}

.table-wrap {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  background: transparent;
  display: flex;
  flex-direction: column;
}

.empty-state {
  flex: 1;
  display: grid;
  place-items: center;
  min-height: 200px;
  text-align: center;
}

.empty-state strong {
  display: block;
  color: var(--text-secondary);
  font-weight: 600;
  margin-bottom: 6px;
}

.empty-state p {
  color: var(--text-muted);
  opacity: 0.6;
}

:deep(.table-wrap .el-table) {
  height: 100%;
  --el-table-bg-color: transparent;
  --el-table-tr-bg-color: transparent;
  --el-table-row-hover-bg-color: rgba(255, 255, 255, 0.03);
  --el-table-border-color: rgba(255, 255, 255, 0.04);
  --el-table-header-bg-color: transparent;
  --el-table-header-text-color: var(--text-muted);
  --el-table-text-color: var(--text-primary);
}

:deep(.table-wrap .el-table__inner-wrapper::before) {
  display: none;
}

:deep(.table-wrap .el-table th.el-table__cell) {
  background: transparent;
  color: var(--text-muted);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  font-size: 13px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

:deep(.table-wrap .el-table td.el-table__cell) {
  padding-top: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.03);
}

:deep(.table-wrap .el-table tr:last-child td.el-table__cell) {
  border-bottom: none;
}

:deep(.table-wrap .el-table tr:hover > td.el-table__cell) {
  background: rgba(255, 255, 255, 0.02) !important;
}

:deep(.table-wrap .el-table__fixed-right) {
  box-shadow: none;
}

:deep(.svc-footer .el-pagination) {
  flex-wrap: wrap;
  justify-content: flex-end;
  --el-pagination-font-size: 14px;
}

:deep(.svc-footer .el-pagination *) {
  font-size: 14px !important;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
}

@media (max-width: 1180px) {
  .toolbar-row {
    flex-wrap: wrap;
    gap: 8px;
  }

  .toolbar-group {
    padding: 0;
  }

  .toolbar-group:first-child {
    width: 100%;
  }

  .toolbar-group.right-align {
    margin-left: 0;
  }
}

@media (max-width: 900px) {
  .search-input { width: 140px; }
  .toolbar-group.right-align { margin-left: 0; width: 100%; justify-content: flex-end; }
}

@media (max-width: 768px) {
  .svc-toolbar {
    padding: 10px 16px;
  }

  .svc-content {
    padding: 12px 16px 8px;
  }

  .toolbar-group {
    flex-wrap: wrap;
  }

  .search-input {
    width: 100% !important;
  }

  .pill-group {
    width: 100%;
  }

  .pill-group button {
    flex: 1;
    justify-content: center;
  }

  .svc-footer {
    flex-direction: column;
    align-items: stretch;
  }
}

</style>

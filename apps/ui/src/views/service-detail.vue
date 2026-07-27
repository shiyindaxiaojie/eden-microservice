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
const serviceGroup = computed(() => {
  const raw = route.query.group
  return typeof raw === 'string' && raw.trim() ? raw : 'default'
})

const stats = computed(() => {
  const total = instances.value.length
  const passing = instances.value.filter((item) => item.status === 'passing' && !isManuallyOffline(item)).length
  const critical = instances.value.filter((item) => item.status === 'critical' && !isManuallyOffline(item)).length
  const manualOffline = instances.value.filter((item) => isManuallyOffline(item)).length
  return { total, passing, critical, manualOffline }
})

const filteredInstances = computed(() => {
  const idQuery = searchId.value.trim().toLowerCase()
  const ipQuery = searchIp.value.trim().toLowerCase()

  return instances.value.filter((item) => {
    const address = `${item.host}:${item.port}`.toLowerCase()
    const matchesId = !idQuery || item.id.toLowerCase().includes(idQuery)
    const matchesIp = !ipQuery || address.includes(ipQuery)
    const matchesStatus =
      statusFilter.value === 'all' ||
      (statusFilter.value === 'passing' && item.status === 'passing' && !isManuallyOffline(item)) ||
      (statusFilter.value === 'critical' && item.status === 'critical' && !isManuallyOffline(item)) ||
      (statusFilter.value === 'manual_offline' && isManuallyOffline(item))

    return matchesId && matchesIp && matchesStatus
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
  return !!instance.manual_offline || instance.status === 'offline'
}

function instanceStateText(instance: Instance) {
  if (isManuallyOffline(instance)) return text('已下线', 'Offline')
  return instance.status === 'passing' ? text('健康', 'Healthy') : text('异常', 'Critical')
}

function instanceStateType(instance: Instance) {
  if (isManuallyOffline(instance)) return 'warning'
  return instance.status === 'passing' ? 'success' : 'danger'
}

function actionLabel(instance: Instance) {
  return isManuallyOffline(instance) ? text('恢复上线', 'Restore') : text('下线', 'Offline')
}

function instanceIdentifier(instance: Instance) {
  const segments = instance.id.split('#')
  if (segments.length >= 4) {
    return `${segments[1] || text('默认', 'Default')} · ${segments[3] || instance.port}`
  }
  return instance.id
}

function dataCenter(instance: Instance) {
  return instance.datacenter || text('未设置', 'Not set')
}

async function loadInstances() {
  if (!serviceName.value) {
    instances.value = []
    return
  }

  loading.value = true
  try {
    const response = await getServiceInstances(serviceName.value, namespace.value, serviceGroup.value)
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
    query: { namespace: namespace.value, group: serviceGroup.value },
  })
}

async function toggleStatus(instance: Instance) {
  const isRestore = isManuallyOffline(instance)
  const nextStatus = isRestore ? 'online' : 'offline'
  const address = instanceAddress(instance)

  try {
    await ElMessageBox.confirm(
      isRestore
        ? text(`确定要将实例 ${address} 恢复上线吗？`, `Bring instance ${address} online?`)
        : text(`确定要将实例 ${address} 手动下线吗？`, `Take instance ${address} offline?`),
      text('状态确认', 'Status confirmation'),
      { type: 'warning' },
    )
  } catch {
    return
  }

  actionTarget.value = instance.id
  try {
    await setInstanceStatus(namespace.value, serviceName.value, serviceGroup.value, instance.id, nextStatus)
    ElMessage.success(isRestore ? text('实例已恢复上线', 'Instance restored') : text('实例已下线', 'Instance offline'))
    await loadInstances()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || text('实例状态更新失败', 'Failed to update instance status'))
  } finally {
    actionTarget.value = ''
  }
}

watch(
  () => [route.params.name, route.query.namespace, route.query.group],
  () => loadInstances().catch((error) => console.error('load service detail failed', error)),
)

watch([searchId, searchIp, statusFilter], () => {
  currentPage.value = 1
})

watch(pageSize, () => {
  currentPage.value = 1
})

watch(filteredInstances, () => {
  if (currentPage.value > totalPages.value) currentPage.value = totalPages.value
})

onMounted(() => {
  loadInstances().catch((error) => console.error('load service detail failed', error))
})
</script>

<template>
  <div class="service-detail-shell">
    <section class="service-hero">
      <div class="service-hero-main">
        <div class="service-heading">
          <button type="button" class="back-link" @click="goBack">
            <el-icon><ArrowLeft /></el-icon>
            <span>{{ text('返回服务列表', 'Back to services') }}</span>
          </button>
          <h1>{{ serviceName }}</h1>
          <dl class="service-meta">
            <div>
              <dt>{{ text('分组', 'Group') }}</dt>
              <dd>{{ serviceGroup }}</dd>
            </div>
            <div>
              <dt>{{ text('命名空间', 'Namespace') }}</dt>
              <dd>{{ namespace }}</dd>
            </div>
          </dl>
        </div>
      </div>

      <div class="service-health" aria-label="服务实例统计">
        <button type="button" class="health-stat" :class="{ active: statusFilter === 'all' }" @click="statusFilter = 'all'">
          <strong>{{ stats.total }}</strong>
          <span>{{ text('全部实例', 'All instances') }}</span>
        </button>
        <button type="button" class="health-stat is-healthy" :class="{ active: statusFilter === 'passing' }" @click="statusFilter = 'passing'">
          <strong>{{ stats.passing }}</strong>
          <span>{{ text('健康', 'Healthy') }}</span>
        </button>
        <button type="button" class="health-stat is-critical" :class="{ active: statusFilter === 'critical' }" @click="statusFilter = 'critical'">
          <strong>{{ stats.critical }}</strong>
          <span>{{ text('异常', 'Critical') }}</span>
        </button>
        <button type="button" class="health-stat is-offline" :class="{ active: statusFilter === 'manual_offline' }" @click="statusFilter = 'manual_offline'">
          <strong>{{ stats.manualOffline }}</strong>
          <span>{{ text('已下线', 'Offline') }}</span>
        </button>
      </div>
    </section>

    <section class="instance-directory" v-loading="loading">
      <header class="directory-header">
        <div>
          <span class="directory-kicker">{{ text('实例目录', 'Instance directory') }}</span>
          <h2>{{ text('运行实例', 'Running instances') }}</h2>
        </div>
        <button type="button" class="refresh-button" :disabled="loading" @click="loadInstances">
          <el-icon :class="{ 'is-loading': loading }"><RefreshRight /></el-icon>
          <span>{{ text('刷新', 'Refresh') }}</span>
        </button>
      </header>

      <div class="directory-controls">
        <label class="filter-field">
          <span>{{ text('实例 ID', 'Instance ID') }}</span>
          <el-input v-model="searchId" clearable :prefix-icon="Search" :placeholder="text('筛选实例 ID', 'Filter instance ID')" />
        </label>
        <label class="filter-field">
          <span>{{ text('IP 地址', 'IP address') }}</span>
          <el-input v-model="searchIp" clearable :prefix-icon="Search" :placeholder="text('筛选 IP 地址', 'Filter IP address')" />
        </label>
        <div class="status-filter" role="group" :aria-label="text('实例状态', 'Instance status')">
          <button type="button" :class="{ active: statusFilter === 'all' }" @click="statusFilter = 'all'">{{ text('全部', 'All') }}</button>
          <button type="button" :class="{ active: statusFilter === 'passing' }" @click="statusFilter = 'passing'">{{ text('健康', 'Healthy') }}</button>
          <button type="button" :class="{ active: statusFilter === 'critical' }" @click="statusFilter = 'critical'">{{ text('异常', 'Critical') }}</button>
          <button type="button" :class="{ active: statusFilter === 'manual_offline' }" @click="statusFilter = 'manual_offline'">{{ text('下线', 'Offline') }}</button>
        </div>
      </div>

      <div class="instance-table-wrap">
        <div v-if="!loading && filteredInstances.length === 0" class="empty-state">
          <strong>{{ text('没有匹配的实例', 'No matching instances') }}</strong>
          <p>{{ text('调整筛选条件，或返回服务列表查看其他服务。', 'Adjust the filters or return to the service list.') }}</p>
        </div>

        <el-table v-else :data="pagedInstances" height="100%" class="instance-table">
          <el-table-column type="index" :label="text('序号', 'No. ')" width="64" align="center" />

          <el-table-column :label="text('实例标识', 'Instance')" min-width="160">
            <template #default="{ row }">
              <el-tooltip :content="row.id" placement="top-start">
                <div class="instance-id-cell">
                  <strong>{{ instanceIdentifier(row) }}</strong>
                  <span>{{ text('完整实例 ID', 'Full instance ID') }}</span>
                </div>
              </el-tooltip>
            </template>
          </el-table-column>

          <el-table-column :label="text('地址', 'Address')" min-width="160">
            <template #default="{ row }">
              <code class="address-code">{{ instanceAddress(row) }}</code>
            </template>
          </el-table-column>

          <el-table-column :label="text('状态', 'Status')" width="108">
            <template #default="{ row }">
              <el-tag :type="instanceStateType(row)" effect="dark" class="instance-state-tag">
                {{ instanceStateText(row) }}
              </el-tag>
            </template>
          </el-table-column>

          <el-table-column :label="text('数据中心', 'Datacenter')" min-width="110">
            <template #default="{ row }">
              <span class="muted-cell">{{ dataCenter(row) }}</span>
            </template>
          </el-table-column>

          <el-table-column prop="weight" :label="text('权重', 'Weight')" width="84" align="center" />

          <el-table-column :label="text('最近心跳', 'Last heartbeat')" min-width="170">
            <template #default="{ row }">
              <span class="time-cell">{{ formatTime(row.last_heartbeat || row.lastHeartbeat) }}</span>
            </template>
          </el-table-column>

          <el-table-column :label="text('注册时间', 'Registered')" min-width="170">
            <template #default="{ row }">
              <span class="time-cell">{{ formatTime(row.registered_at || row.registeredAt) }}</span>
            </template>
          </el-table-column>

          <el-table-column :label="text('操作', 'Actions')" width="104" fixed="right" align="right">
            <template #default="{ row }">
              <el-button
                link
                :type="isManuallyOffline(row) ? 'success' : 'warning'"
                :loading="actionTarget === row.id"
                @click="toggleStatus(row)"
              >
                {{ actionLabel(row) }}
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <footer class="directory-footer">
        <span>{{ text(`共 ${filteredInstances.length} 个实例`, `${filteredInstances.length} instances`) }}</span>
        <el-config-provider :locale="elementLocale">
          <el-pagination
            v-model:current-page="currentPage"
            v-model:page-size="pageSize"
            :page-sizes="[12, 20, 50]"
            :total="filteredInstances.length"
            layout="total, sizes, prev, pager, next"
            background
          />
        </el-config-provider>
      </footer>
    </section>
  </div>
</template>

<style scoped lang="scss">
.service-detail-shell {
  display: flex;
  height: 100%;
  flex: 1;
  min-height: 0;
  flex-direction: column;
  gap: 14px;
}

.service-hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 28px;
  min-height: 96px;
  padding: 16px 0 18px;
  border-bottom: 1px solid var(--border-color);
}

.service-hero-main,
.service-heading,
.service-health,
.directory-controls,
.status-filter,
.directory-footer {
  display: flex;
  align-items: center;
}

.service-hero-main {
  min-width: 0;
}

.back-link,
.refresh-button {
  display: inline-flex;
  align-items: center;
  cursor: pointer;
  transition: color 0.2s ease;
}

.back-link {
  gap: 5px;
  width: fit-content;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--text-muted);
  font-size: 12px;
  font-weight: 650;
}

.back-link:hover,
.refresh-button:hover:not(:disabled) {
  color: var(--text-primary);
}

.service-heading {
  min-width: 0;
  flex-direction: column;
  align-items: flex-start;
  gap: 7px;
}

.directory-kicker {
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.09em;
  text-transform: uppercase;
}

.service-heading h1,
.directory-header h2 {
  margin: 0;
  color: var(--text-primary);
}

.service-heading h1 {
  overflow: hidden;
  max-width: 560px;
  font-size: clamp(22px, 2vw, 30px);
  font-weight: 720;
  letter-spacing: -0.025em;
  line-height: 1.15;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.service-meta {
  display: flex;
  gap: 18px;
  margin: 0;
}

.service-meta div {
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.service-meta dt,
.service-meta dd {
  margin: 0;
}

.service-meta dt {
  color: var(--text-muted);
  font-size: 12px;
  font-weight: 600;
}

.service-meta dd {
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 650;
}

.service-health {
  flex: 0 0 auto;
  min-height: 56px;
  gap: 24px;
}

.health-stat {
  position: relative;
  display: flex;
  min-width: 58px;
  height: 56px;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
  gap: 3px;
  padding: 0 0 7px;
  border: 0;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  text-align: left;
  transition: color 0.2s ease;
}

.health-stat:hover,
.health-stat.active {
  color: var(--text-primary);
}

.health-stat.active::after {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  height: 2px;
  background: var(--accent-blue);
  content: '';
}

.health-stat strong {
  color: inherit;
  font-size: 17px;
  line-height: 1;
}

.health-stat span {
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
}

.health-stat.is-healthy strong { color: var(--accent-green); }
.health-stat.is-critical strong { color: var(--accent-red); }
.health-stat.is-offline strong { color: var(--accent-orange); }

.instance-directory {
  display: flex;
  min-height: 0;
  flex: 1;
  flex-direction: column;
  overflow: hidden;
  background: transparent;
}

.directory-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 20px;
  padding: 12px 0 14px;
}

.directory-header h2 {
  margin-top: 4px;
  font-size: 17px;
  font-weight: 700;
}

.refresh-button {
  height: 32px;
  gap: 6px;
  padding: 0 10px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: var(--text-secondary);
  font-size: 13px;
  font-weight: 650;
}

.refresh-button:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.is-loading {
  animation: rotating 1.2s linear infinite;
}

@keyframes rotating {
  to { transform: rotate(360deg); }
}

.directory-controls {
  flex-wrap: wrap;
  gap: 12px;
  padding: 12px 0;
  border-bottom: 1px solid var(--border-color);
  background: transparent;
}

.filter-field {
  display: flex;
  align-items: center;
  gap: 8px;
}

.filter-field > span {
  color: var(--text-muted);
  font-size: 12px;
  font-weight: 650;
  white-space: nowrap;
}

.filter-field :deep(.el-input) {
  width: 190px;
}

.filter-field :deep(.el-input__wrapper) {
  box-shadow: 0 0 0 1px var(--border-color) inset;
  background: var(--control-bg);
}

.status-filter {
  gap: 18px;
  margin-left: auto;
}

.status-filter button {
  position: relative;
  padding: 5px 0;
  border: 0;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 12px;
  font-weight: 650;
  transition: color 0.2s ease;
}

.status-filter button:hover,
.status-filter button.active {
  color: var(--text-primary);
}

.status-filter button.active::after {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  height: 2px;
  background: var(--accent-blue);
  content: '';
}

.instance-table-wrap {
  display: flex;
  min-height: 0;
  flex: 1;
}

.instance-table {
  --el-table-bg-color: transparent;
  --el-table-tr-bg-color: transparent;
  --el-table-row-hover-bg-color: rgba(109, 158, 193, 0.065);
  --el-table-border-color: rgba(148, 181, 207, 0.1);
  --el-table-header-bg-color: transparent;
  --el-table-header-text-color: var(--text-muted);
  --el-table-text-color: var(--text-primary);
}

:deep(.instance-table .el-table__inner-wrapper::before) {
  display: none;
}

:deep(.instance-table th.el-table__cell) {
  height: 42px;
  background: rgba(7, 17, 31, 0.22);
  color: var(--text-muted);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.05em;
}

:deep(.instance-table td.el-table__cell) {
  height: 62px;
  padding-top: 7px;
  padding-bottom: 7px;
}

:deep(.instance-table tr:hover > td.el-table__cell) {
  background: rgba(109, 158, 193, 0.055) !important;
}

:deep(.instance-table .el-table__fixed-right) {
  box-shadow: -8px 0 16px rgba(3, 10, 20, 0.14);
}

.instance-id-cell {
  display: flex;
  max-width: 180px;
  flex-direction: column;
  gap: 4px;
}

.instance-id-cell strong {
  overflow: hidden;
  color: var(--text-primary);
  font-size: 13px;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.instance-id-cell span,
.muted-cell {
  color: var(--text-muted);
  font-size: 12px;
}

.address-code,
.time-cell {
  color: var(--text-secondary);
  font-size: 13px;
  white-space: nowrap;
}

.address-code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.instance-state-tag {
  border: 0;
  font-weight: 700;
}

.empty-state {
  display: grid;
  width: 100%;
  min-height: 220px;
  place-content: center;
  color: var(--text-muted);
  text-align: center;
}

.empty-state strong {
  color: var(--text-secondary);
  font-size: 15px;
}

.empty-state p {
  margin: 8px 0 0;
  font-size: 13px;
}

.directory-footer {
  justify-content: space-between;
  gap: 16px;
  padding: 12px 0 0;
  border-top: 1px solid var(--border-color);
  color: var(--text-muted);
  font-size: 12px;
}

:deep(.directory-footer .el-pagination) {
  --el-pagination-font-size: 13px;
}

@media (max-width: 1180px) {
  .service-hero {
    align-items: flex-start;
    flex-direction: column;
  }

  .service-health {
    width: 100%;
    gap: 20px;
  }

  .health-stat {
    flex: 0 1 auto;
  }

  .status-filter {
    margin-left: 0;
  }
}

@media (max-width: 760px) {
  .service-detail-shell {
    gap: 14px;
  }

  .service-hero {
    padding-bottom: 14px;
  }

  .service-heading h1 {
    max-width: 100%;
    font-size: 22px;
  }

  .service-health {
    display: flex;
    flex-wrap: wrap;
    gap: 12px 20px;
  }

  .health-stat {
    min-width: calc(50% - 10px);
  }

  .directory-header,
  .directory-footer {
    padding-right: 0;
    padding-left: 0;
  }

  .directory-controls {
    align-items: stretch;
    flex-direction: column;
    padding: 12px 0;
  }

  .filter-field,
  .filter-field :deep(.el-input) {
    width: 100%;
  }

  .status-filter {
    width: 100%;
    gap: 0;
  }

  .status-filter button {
    flex: 1;
    text-align: left;
  }

  .directory-footer {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>

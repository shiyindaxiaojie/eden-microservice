<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Grid, List as ListIcon, Plus, RefreshLeft, Search } from '@element-plus/icons-vue'
import {
  addClusterMember,
  getClusterMembers,
  getClusterStats,
  removeClusterMember,
  type ClusterMember,
  type ClusterStats,
} from '../api/registry'
import { useI18n } from '../utils/i18n'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import en from 'element-plus/es/locale/lang/en'

const { t, locale } = useI18n()
const members = ref<ClusterMember[]>([])
const stats = ref<ClusterStats | null>(null)
const loading = ref(true)
const showAddDialog = ref(false)
const searchID = ref('')
const searchIP = ref('')
const addForms = ref([{ protocol: 'http://', host: '', port: '' }])
const viewMode = ref<'grid' | 'list'>('list')
const filterMode = ref<'all' | 'online' | 'offline'>('all')
const currentPage = ref(1)
const pageSize = ref(12)

const nodeStats = computed(() => ({
  total: members.value.length,
  online: members.value.filter((n) => n.status === 'Online').length,
  offline: members.value.filter((n) => n.status !== 'Online').length,
}))

const filteredMembers = computed(() => {
  const qID = searchID.value.trim().toLowerCase()
  const qIP = searchIP.value.trim().toLowerCase()
  let list = members.value

  if (filterMode.value === 'online') list = list.filter((n) => n.status === 'Online')
  if (filterMode.value === 'offline') list = list.filter((n) => n.status !== 'Online')

  if (qID) {
    list = list.filter((m) => m.id.toLowerCase().includes(qID))
  }

  if (qIP) {
    list = list.filter((m) => {
      const addrs = [m.address, m.http_addr, m.grpc_addr, m.raft_addr, m.quic_addr].filter((a): a is string => !!a)
      return addrs.some((a) => a.toLowerCase().includes(qIP))
    })
  }

  return list
})

const pagedMembers = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredMembers.value.slice(start, start + pageSize.value)
})

const clusterHint = computed(() =>
  locale.value === 'zh'
    ? '统一查看节点状态、协议地址和当前集群角色。'
    : 'Inspect node status, protocol endpoints, and the current cluster role in one place.',
)
const invalidAddressMessage = computed(() =>
  locale.value === 'zh' ? '请填写有效的节点地址信息' : 'Please enter valid address information',
)
const addFailedMessage = computed(() => (locale.value === 'zh' ? '添加失败' : 'Add failed'))
const removeConfirm = (row: ClusterMember) =>
  locale.value === 'zh'
    ? `确定移除节点 ${row.id} (${row.address}) 吗？`
    : `Are you sure you want to remove node ${row.id} (${row.address})?`

const emptyNodesLabel = computed(() => (locale.value === 'zh' ? '暂无节点数据' : 'No nodes found'))

function addNodeRow() {
  addForms.value.push({ protocol: 'http://', host: '', port: '' })
}

function removeNodeRow(index: number) {
  addForms.value.splice(index, 1)
  if (addForms.value.length === 0) {
    addNodeRow()
  }
}

function resetFilters() {
  searchID.value = ''
  searchIP.value = ''
  filterMode.value = 'all'
  fetchCluster()
}

async function fetchCluster() {
  loading.value = true
  try {
    const [membersRes, statsRes] = await Promise.all([getClusterMembers(), getClusterStats()])
    members.value = membersRes.data || []
    stats.value = statsRes.data
  } catch (error) {
    console.error('fetch cluster failed', error)
  } finally {
    loading.value = false
  }
}

async function handleAdd() {
  const addresses = addForms.value.filter((form) => form.host && form.port).map((form) => `${form.protocol}${form.host}:${form.port}`)

  if (addresses.length === 0) {
    ElMessage.warning(invalidAddressMessage.value)
    return
  }

  try {
    await addClusterMember({ addresses })
    ElMessage.success(t.value.common.success)
    showAddDialog.value = false
    addForms.value = [{ protocol: 'http://', host: '', port: '' }]
    await fetchCluster()
  } catch (error: any) {
    const data = error.response?.data
    if (data?.error === 'not leader' && data?.leader) {
      ElMessage.warning(t.value.settings.notLeaderTip.replace('{addr}', data.leader))
    } else {
      ElMessage.error(data?.error || addFailedMessage.value)
    }
  }
}

async function handleRemove(row: ClusterMember) {
  try {
    await ElMessageBox.confirm(removeConfirm(row), t.value.common.warning, { type: 'warning' })
    await removeClusterMember(row.address, row.id)
    ElMessage.success(t.value.common.success)
    await fetchCluster()
  } catch {
    // ignore cancel
  }
}

function roleTagType(role: string) {
  switch (role) {
    case 'Leader':
      return 'danger'
    case 'Follower':
      return 'primary'
    case 'Candidate':
      return 'warning'
    case 'Local':
    case 'Standalone':
      return 'success'
    default:
      return 'info'
  }
}

function getRoleName(role: string) {
  const normalized = role.toLowerCase()
  if (normalized.includes('leader')) return t.value.cluster.roles.leader
  if (normalized.includes('follower')) return t.value.cluster.roles.follower
  if (normalized.includes('candidate')) return t.value.cluster.roles.candidate
  if (normalized.includes('local')) return t.value.cluster.roles.local
  if (normalized.includes('standalone')) return t.value.cluster.roles.standalone
  if (normalized.includes('peer')) return t.value.cluster.roles.peer
  return role
}

onMounted(fetchCluster)
</script>

<template>
  <div class="svc-shell">
    <!-- Toolbar -->
    <div class="svc-toolbar">
      <div class="toolbar-row">
        <div class="toolbar-group">
          <div class="field-item">
            <span class="field-label">{{ locale === 'zh' ? '节点 ID' : 'Node ID' }}</span>
            <el-input
              v-model="searchID"
              :prefix-icon="Search"
              :placeholder="locale === 'zh' ? '输入节点 ID' : 'Node ID'"
              clearable
              class="search-input"
              style="width: 180px;"
            />
          </div>
        </div>

        <div class="toolbar-group">
          <div class="field-item">
            <span class="field-label">{{ locale === 'zh' ? 'IP 地址' : 'IP Address' }}</span>
            <el-input
              v-model="searchIP"
              :prefix-icon="Search"
              :placeholder="locale === 'zh' ? '输入 IP 地址' : 'IP Address'"
              clearable
              class="search-input"
              style="width: 180px;"
            />
          </div>
        </div>

        <div class="toolbar-group">
          <button
            type="button"
            class="refresh-btn"
            :title="t.common.refresh"
            @click="resetFilters()"
          >
            <el-icon><RefreshLeft /></el-icon>
          </button>
        </div>

        <div class="toolbar-group right-align">
          <div class="pill-group">
            <button type="button" :class="{ active: filterMode === 'all' }" @click="filterMode = 'all'">
              {{ t.common.all || '全部' }}
              <span class="pill-count">{{ nodeStats.total }}</span>
            </button>
            <button type="button" :class="{ active: filterMode === 'online' }" @click="filterMode = 'online'">
              {{ locale === 'zh' ? '在线' : 'Online' }}
              <span class="pill-count">{{ nodeStats.online }}</span>
            </button>
            <button type="button" :class="{ active: filterMode === 'offline' }" @click="filterMode = 'offline'">
              {{ locale === 'zh' ? '离线' : 'Offline' }}
              <span class="pill-count">{{ nodeStats.offline }}</span>
            </button>
          </div>

          <div class="toolbar-sep"></div>

          <div class="pill-group icon-pills">
            <button type="button" :class="{ active: viewMode === 'list' }" @click="viewMode = 'list'">
              <el-icon><ListIcon /></el-icon>
            </button>
            <button type="button" :class="{ active: viewMode === 'grid' }" @click="viewMode = 'grid'">
              <el-icon><Grid /></el-icon>
            </button>
          </div>

          <div class="toolbar-sep"></div>

          <el-button
            v-if="stats?.environment === 'cluster' || stats?.mode === 'cp'"
            type="primary"
            class="add-btn"
            :icon="Plus"
            @click="showAddDialog = true"
          >
            {{ t.cluster.addNode }}
          </el-button>
        </div>
      </div>
    </div>

    <!-- Content -->
    <section class="svc-content" v-loading="loading">
      <div v-if="!loading && filteredMembers.length === 0" class="empty-panel">
        <div class="empty-inner">
          <strong>{{ emptyNodesLabel }}</strong>
          <p>{{ clusterHint }}</p>
        </div>
      </div>

      <template v-else>
        <div v-if="viewMode === 'list'" class="table-wrap">
          <el-table :data="pagedMembers" height="100%" style="width: 100%; font-size: 14px;">
            <el-table-column type="index" :label="locale === 'zh' ? '序号' : 'No.'" width="60" align="center" />
            <el-table-column :label="t.cluster.nodeId" min-width="100">
              <template #default="{ row }">
                <div class="node-id-cell">
                  <div class="node-main">
                    <div class="node-main-row">
                      <strong class="node-name">{{ row.id }}</strong>
                      <el-tag v-if="row.is_local" size="small" effect="plain" type="success">{{ t.cluster.currentNode }}</el-tag>
                    </div>
                  </div>
                </div>
              </template>
            </el-table-column>

            <el-table-column :label="t.cluster.address" min-width="400">
              <template #default="{ row }">
                <div class="address-tags">
                  <span v-if="row.http_addr" class="addr-chip"><i class="p-tag">HTTP</i>{{ row.http_addr }}</span>
                  <span v-if="row.grpc_addr" class="addr-chip grpc"><i class="p-tag">GRPC</i>{{ row.grpc_addr }}</span>
                  <span v-if="row.raft_addr" class="addr-chip raft"><i class="p-tag">RAFT</i>{{ row.raft_addr }}</span>
                  <span v-if="row.quic_addr" class="addr-chip quic"><i class="p-tag">QUIC</i>{{ row.quic_addr }}</span>
                  <span v-if="!row.http_addr" class="mono-addr">{{ row.address }}</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column :label="t.cluster.status" width="100">
              <template #default="{ row }">
                <div class="status-cell">
                  <span class="status-dot" :class="row.status === 'Online' ? 'active' : 'inactive'"></span>
                  <span class="status-text">{{ row.status === 'Online' ? t.cluster.online : t.cluster.offline }}</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column :label="t.cluster.role" width="100">
              <template #default="{ row }">
                <el-tag :type="roleTagType(row.role)" size="small" effect="dark" class="role-tag">{{ getRoleName(row.role) }}</el-tag>
              </template>
            </el-table-column>

            <el-table-column :label="t.common.actions" width="100" fixed="right" align="center">
              <template #default="{ row }">
                <el-button
                  v-if="!row.is_local && row.role !== 'Leader' && (stats?.mode !== 'cp' || stats?.role === 'Leader')"
                  type="danger"
                  size="small"
                  link
                  :icon="Delete"
                  @click="handleRemove(row)"
                >
                  {{ t.common.delete }}
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <div v-else class="card-grid">
          <div
            v-for="member in pagedMembers"
            :key="member.id"
            class="node-card"
            :class="{ 'is-local': member.is_local }"
          >
            <div class="card-glow" :style="{ background: member.status === 'Online' ? 'rgba(16, 185, 129, 0.1)' : 'rgba(239, 68, 68, 0.1)' }" />
            
            <div class="card-head">
              <div class="card-id-row">
                <h3 class="node-id-text">{{ member.id }}</h3>
                <el-tag v-if="member.is_local" size="small" type="success" effect="plain">{{ t.cluster.currentNode }}</el-tag>
              </div>
              <el-tag :type="roleTagType(member.role)" size="small" effect="dark">{{ getRoleName(member.role) }}</el-tag>
            </div>

            <div class="card-body">
              <div class="status-line">
                <span class="status-dot" :class="member.status === 'Online' ? 'active' : 'inactive'"></span>
                <span class="status-label">{{ member.status === 'Online' ? t.cluster.online : t.cluster.offline }}</span>
              </div>

              <div class="addr-group">
                <div v-if="member.http_addr" class="addr-box">
                  <span class="p-tag sm">HTTP</span>
                  <code>{{ member.http_addr }}</code>
                </div>
                <div v-if="member.grpc_addr" class="addr-box">
                  <span class="p-tag sm">GRPC</span>
                  <code>{{ member.grpc_addr }}</code>
                </div>
                <div v-if="member.raft_addr" class="addr-box">
                  <span class="p-tag sm">RAFT</span>
                  <code>{{ member.raft_addr }}</code>
                </div>
              </div>
            </div>

            <div class="card-footer">
              <div class="footer-spacer"></div>
              <el-button
                v-if="!member.is_local && member.role !== 'Leader' && (stats?.mode !== 'cp' || stats?.role === 'Leader')"
                type="danger"
                size="small"
                link
                :icon="Delete"
                @click="handleRemove(member)"
              >
                {{ t.common.delete }}
              </el-button>
            </div>
          </div>
        </div>

        <footer class="svc-footer">
          <span class="footer-info">{{ filteredMembers.length }} {{ locale === 'zh' ? '条' : 'records' }}</span>
          <el-config-provider :locale="locale === 'zh' ? zhCn : en">
            <el-pagination
              v-model:current-page="currentPage"
              v-model:page-size="pageSize"
              :page-sizes="[12, 20, 50]"
              :total="filteredMembers.length"
              layout="sizes, prev, pager, next"
              background
            />
          </el-config-provider>
        </footer>
      </template>
    </section>

    <!-- Add Node Dialog -->
    <el-dialog
      v-model="showAddDialog"
      :title="t.cluster.addNode"
      width="560px"
      append-to-body
      @close="addForms = [{ protocol: 'http://', host: '', port: '' }]"
      class="glass-dialog"
    >
      <div class="add-node-form">
        <div v-for="(form, index) in addForms" :key="index" class="node-input-row">
          <el-select v-model="form.protocol" class="protocol-select">
            <el-option label="http://" value="http://" />
            <el-option label="https://" value="https://" />
          </el-select>
          <el-input v-model="form.host" placeholder="127.0.0.1" class="host-input" />
          <span class="colon-separator">:</span>
          <el-input v-model="form.port" placeholder="8501" class="port-input" />
          <div class="row-actions">
            <el-button v-if="index === addForms.length - 1" type="primary" circle :icon="Plus" size="small" plain @click="addNodeRow" />
            <el-button v-if="addForms.length > 1" type="danger" circle :icon="Delete" size="small" plain @click="removeNodeRow(index)" />
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="showAddDialog = false">{{ t.common.cancel }}</el-button>
        <el-button type="primary" @click="handleAdd">{{ t.common.confirm }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
/* ===== Shell ===== */
.svc-shell {
  display: flex;
  flex-direction: column;
  height: calc(100vh - var(--header-height) - 48px);
  min-height: 0;
  overflow: hidden;
  gap: 0;
  padding: 0;
}

.svc-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  padding: 16px 24px 12px;
}

.table-wrap {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  margin-bottom: 8px;
}

.card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  align-content: start;
  padding: 2px;
}

/* ===== Toolbar ===== */
.svc-toolbar {
  flex-shrink: 0;
  padding: 12px 24px;
  border-bottom: 1px solid var(--border-color);
}

.toolbar-row {
  display: flex;
  align-items: center;
  gap: 0;
}

.toolbar-group {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 16px;
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

.field-item {
  display: flex;
  align-items: center;
  gap: 12px;
}

.field-label {
  font-size: 14px;
  color: var(--text-secondary);
  white-space: nowrap;
}

.toolbar-sep {
  width: 1px;
  height: 24px;
  background: var(--border-color);
  flex-shrink: 0;
  margin: 0 4px;
  opacity: 0.5;
}

.refresh-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.15s ease;
  margin-left: -20px;
}

.refresh-btn:hover {
  color: var(--accent-blue);
  background: rgba(59, 130, 246, 0.08);
}

/* -- Pill groups -- */
.pill-group {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 2px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.03);
}

.pill-group button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--text-muted);
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;
  white-space: nowrap;
  font-family: inherit;
  font-size: 14px;
}

.pill-group button:hover {
  color: var(--text-secondary);
  background: rgba(255, 255, 255, 0.04);
}

.pill-count {
  font-weight: 600;
  opacity: 0.5;
  font-variant-numeric: tabular-nums;
  font-size: 14px;
}

.pill-group button.active {
  background: rgba(59, 130, 246, 0.12);
  color: var(--accent-blue);
  font-weight: 600;
}

.pill-group button.active .pill-count {
  opacity: 0.8;
}

.icon-pills button {
  padding: 6px 10px;
}

/* ── Content ── */
.svc-content {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding: 16px 24px 12px;
}

.empty-panel {
  display: grid;
  place-items: center;
  height: 100%;
}

.empty-inner {
  text-align: center;
  color: var(--text-muted);
}
.empty-inner strong {
  display: block;
  font-size: 16px;
  color: var(--text-primary);
  margin-bottom: 8px;
}

:deep(.search-input .el-input__wrapper) {
  background: rgba(255, 255, 255, 0.04) !important;
  border: 1px solid var(--border-color) !important;
  box-shadow: none !important;
  border-radius: 6px;
  color: var(--text-primary);
  height: 32px;
}
:deep(.search-input .el-input__wrapper:hover),
:deep(.search-input .el-input__wrapper:focus-within) {
  border-color: rgba(59, 130, 246, 0.4) !important;
  background: rgba(255, 255, 255, 0.06) !important;
}

.pill-btn {
  border-radius: 8px;
  height: 32px;
}
.add-btn {
  height: 32px;
  border-radius: 8px;
  padding: 0 16px;
}

.table-wrap {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  background: transparent;
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
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

:deep(.table-wrap .el-table td.el-table__cell) {
  padding-top: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.03);
}

:deep(.table-wrap .el-table tr:hover > td.el-table__cell) {
  background: rgba(255, 255, 255, 0.02) !important;
}

:deep(.table-wrap .el-table__fixed-right) {
  box-shadow: none;
}

.node-id-cell {
  display: flex;
  align-items: center;
  gap: 14px;
}

.node-badge {
  width: 40px;
  height: 40px;
  display: grid;
  place-items: center;
  border-radius: 10px;
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.1), rgba(6, 182, 212, 0.1));
  color: var(--accent-blue);
  font-weight: 700;
  font-size: 14px;
}

.node-main {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.node-main-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.node-name {
  font-size: 14px;
}

.node-sub {
  font-size: 12px;
  color: var(--text-muted);
}

.address-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.addr-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 12px;
  color: var(--text-primary);
}

.addr-chip.grpc { color: var(--accent-green); }
.addr-chip.raft { color: var(--accent-orange); }
.addr-chip.quic { color: var(--accent-blue); }

.p-tag {
  font-style: normal;
  font-weight: 800;
  font-size: 10px;
  opacity: 0.6;
}

.status-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.status-dot.active {
  background: var(--accent-green);
  box-shadow: 0 0 10px var(--accent-green);
}

.status-dot.inactive {
  background: var(--text-muted);
}

.status-text {
  font-size: 13px;
}

/* -- Card Grid -- */
.card-grid {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 24px;
  padding: 2px 2px 24px;
}

/* Hide scrollbar for Chrome/Safari */
.card-grid::-webkit-scrollbar {
  width: 6px;
}
.card-grid::-webkit-scrollbar-thumb {
  background: rgba(255,255,255,0.05);
  border-radius: 10px;
}

.node-card {
  display: flex;
  flex-direction: column;
  padding: 24px;
  border-radius: 12px;
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  position: relative;
  overflow: hidden;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  min-height: 220px;
}

.node-card:hover {
  transform: translateY(-4px);
  border-color: var(--accent-blue);
  box-shadow: 0 12px 24px -8px rgba(0, 0, 0, 0.3), 0 0 0 1px var(--accent-blue);
  background: var(--bg-glass);
}

.card-glow {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 4px;
  opacity: 0.6;
}

.card-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
}

.card-id-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.node-id-text {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
  color: var(--text-primary);
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  letter-spacing: -0.02em;
}

.card-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-bottom: 20px;
}

.status-line {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 500;
}

.status-label {
  color: var(--text-secondary);
}

.addr-group {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.addr-box {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
  color: var(--text-secondary);
}

.addr-box .p-tag {
  min-width: 44px;
  text-align: center;
  font-size: 10px;
  font-weight: 800;
  opacity: 0.6;
}

.addr-box code {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.03);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: 'JetBrains Mono', monospace;
}

.card-footer {
  display: flex;
  align-items: center;
  padding-top: 16px;
  border-top: 1px solid rgba(255, 255, 255, 0.03);
}

.footer-spacer {
  flex: 1;
}

.add-node-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.node-input-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px;
  background: rgba(255, 255, 255, 0.02);
  border-radius: 10px;
  border: 1px solid var(--border-color);
}

.protocol-select { width: 100px; }
.host-input { flex: 1; }
.port-input { width: 90px; }

/* ===== Footer ===== */
.svc-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 0 0;
  flex-shrink: 0;
}

.footer-info {
  font-size: 14px;
  color: var(--text-muted);
  opacity: 0.8;
}

:deep(.svc-footer .el-pagination) {
  flex-wrap: wrap;
  justify-content: flex-end;
}

:deep(.svc-footer .el-pagination *) {
  font-size: 14px !important;
}


</style>




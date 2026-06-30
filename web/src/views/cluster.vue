<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Connection, Delete, Grid, List as ListIcon, Plus, RefreshLeft, Search } from '@element-plus/icons-vue'
import {
  addClusterMember,
  getClusterMembers,
  getClusterStats,
  removeClusterMember,
  type ClusterMember,
  type ClusterStats,
} from '../api/registry'
import { useI18n } from '../utils/i18n'

const { t, text, elementLocale } = useI18n()
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
const pageSize = ref(8)

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
  text('统一查看节点状态、协议地址和当前集群角色。', 'Inspect node status, protocol endpoints, and the current cluster role in one place.'),
)
const invalidAddressMessage = computed(() =>
  text('请填写有效的节点地址信息', 'Please enter valid address information'),
)
const addFailedMessage = computed(() => (text('添加失败', 'Add failed')))
const removeConfirm = (row: ClusterMember) =>
  text(`确定移除节点 ${row.id} (${row.address}) 吗？`, `Are you sure you want to remove node ${row.id} (${row.address})?`)

const emptyNodesLabel = computed(() => (text('暂无节点数据', 'No nodes found')))

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

function canRemoveMember(member: ClusterMember) {
  return !member.is_local && member.role !== 'Leader' && (stats.value?.mode !== 'cp' || stats.value?.role === 'Leader')
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

interface MemberPortItem {
  key: string
  label: string
  port: string
  className: string
}

function stripProtocolPrefix(value = '') {
  return value.replace(/^[a-z]+:\/\//i, '')
}

function splitEndpoint(value?: string) {
  const raw = stripProtocolPrefix(value || '')
  if (!raw) {
    return { host: '', port: '' }
  }

  const lastColonIndex = raw.lastIndexOf(':')
  if (lastColonIndex === -1) {
    return { host: raw, port: '' }
  }

  return {
    host: raw.slice(0, lastColonIndex),
    port: raw.slice(lastColonIndex + 1),
  }
}

function getMemberPrimaryHost(member: ClusterMember) {
  const addresses = [member.address, member.http_addr, member.grpc_addr, member.raft_addr, member.quic_addr].filter(
    (item): item is string => !!item,
  )

  for (const address of addresses) {
    const { host } = splitEndpoint(address)
    if (host) {
      return host
    }
  }

  return text('未配置', 'Not configured')
}

function getMemberDisplayAddress(member: ClusterMember) {
  const addresses = [member.address, member.http_addr, member.grpc_addr, member.raft_addr, member.quic_addr].filter(
    (item): item is string => !!item,
  )

  for (const address of addresses) {
    const normalized = stripProtocolPrefix(address)
    if (normalized) {
      return normalized
    }
  }

  return getMemberPrimaryHost(member)
}

function getMemberPortItems(member: ClusterMember): MemberPortItem[] {
  const items = [
    member.http_addr ? { key: 'http', label: 'HTTP', ...splitEndpoint(member.http_addr), className: 'http' } : null,
    member.grpc_addr ? { key: 'grpc', label: 'GRPC', ...splitEndpoint(member.grpc_addr), className: 'grpc' } : null,
    member.raft_addr ? { key: 'raft', label: 'RAFT', ...splitEndpoint(member.raft_addr), className: 'raft' } : null,
    member.quic_addr ? { key: 'quic', label: 'QUIC', ...splitEndpoint(member.quic_addr), className: 'quic' } : null,
  ]
    .filter((item): item is NonNullable<typeof item> => item !== null)
    .map(({ key, label, port, className }) => ({
      key,
      label,
      port: port || '--',
      className,
    }))

  if (items.length > 0) {
    return items
  }

  const fallback = splitEndpoint(member.address)
  return [{ key: 'address', label: 'ADDR', port: fallback.port || '--', className: 'mono' }]
}

function roleChipClass(role: string) {
  const normalized = role.toLowerCase()
  if (normalized.includes('leader')) return 'leader'
  if (normalized.includes('follower')) return 'follower'
  if (normalized.includes('candidate')) return 'candidate'
  if (normalized.includes('local') || normalized.includes('standalone')) return 'local'
  return 'neutral'
}

onMounted(fetchCluster)
</script>

<template>
  <div class="svc-shell">
    <div class="svc-main glass-card">
    <!-- Toolbar -->
    <div class="svc-toolbar" data-guide="cluster-toolbar">
      <div class="toolbar-row">
        <div class="toolbar-group">
          <div class="field-item">
            <span class="field-label">{{ text('节点 ID', 'Node ID') }}</span>
            <el-input
              v-model="searchID"
              :prefix-icon="Search"
              :placeholder="text('输入节点 ID', 'Node ID')"
              clearable
                class="search-input"
              />
          </div>
        </div>

        <div class="toolbar-group">
          <div class="field-item">
            <span class="field-label">{{ text('IP 地址', 'IP Address') }}</span>
            <el-input
              v-model="searchIP"
              :prefix-icon="Search"
              :placeholder="text('输入 IP 地址', 'IP Address')"
              clearable
                class="search-input"
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
              {{ text('在线', 'Online') }}
              <span class="pill-count">{{ nodeStats.online }}</span>
            </button>
            <button type="button" :class="{ active: filterMode === 'offline' }" @click="filterMode = 'offline'">
              {{ text('离线', 'Offline') }}
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
            <el-table-column type="index" :label="text('序号', 'No.')" width="60" align="center" />
            <el-table-column :label="t.cluster.nodeId" min-width="120">
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

            <el-table-column :label="t.cluster.role" min-width="110">
              <template #default="{ row }">
                <el-tag :type="roleTagType(row.role)" size="small" effect="dark" class="role-tag">{{ getRoleName(row.role) }}</el-tag>
              </template>
            </el-table-column>

            <el-table-column :label="t.common.actions" width="100" fixed="right" align="center">
              <template #default="{ row }">
                <el-button
                  v-if="canRemoveMember(row)"
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
          <article
            v-for="member in pagedMembers"
            :key="member.id"
            class="node-card"
            :title="getMemberDisplayAddress(member)"
            :class="{
              'is-local': member.is_local,
              'is-offline': member.status !== 'Online',
            }"
          >
            <div class="card-accent"></div>

            <div class="card-head">
              <div class="card-identity">
                <div class="card-symbol">
                  <el-icon><Connection /></el-icon>
                </div>
                <div class="card-copy">
                  <div class="card-title-row">
                    <strong class="card-title">{{ member.id }}</strong>
                    <span v-if="member.is_local" class="card-current-mark">{{ text('本机节点', 'Local node') }}</span>
                  </div>
                  <p class="card-hostline">{{ getMemberPrimaryHost(member) }}</p>
                </div>
              </div>
            </div>

            <div class="card-body">
              <div class="compact-meta-row">
                <span class="compact-label">{{ text('状态', 'Status') }}</span>
                <div class="node-meta-pills">
                  <span class="meta-chip" :class="member.status === 'Online' ? 'online' : 'offline'">
                    <span class="status-dot" :class="member.status === 'Online' ? 'active' : 'inactive'"></span>
                    {{ member.status === 'Online' ? t.cluster.online : t.cluster.offline }}
                  </span>
                  <span class="meta-chip role" :class="roleChipClass(member.role)">{{ getRoleName(member.role) }}</span>
                </div>
              </div>

              <div class="protocol-grid">
                <div
                  v-for="endpoint in getMemberPortItems(member)"
                  :key="`${member.id}-${endpoint.key}`"
                  class="protocol-card"
                  :class="endpoint.className"
                >
                  <span class="protocol-name">{{ endpoint.label }}</span>
                  <span class="protocol-port" :title="endpoint.port">{{ endpoint.port }}</span>
                </div>
              </div>
            </div>

            <div class="card-footer">
              <span class="card-footnote">
                {{ member.is_local ? t.cluster.currentNode : (text('集群节点', 'Cluster node')) }}
              </span>
              <div class="card-actions">
                <el-button
                  v-if="canRemoveMember(member)"
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
          </article>
        </div>

        <footer class="svc-footer">
          <span class="footer-info">{{ filteredMembers.length }} {{ text('条', 'records') }}</span>
          <el-config-provider :locale="elementLocale">
            <el-pagination
              v-model:current-page="currentPage"
              v-model:page-size="pageSize"
                :page-sizes="[8, 12, 20, 50]"
              :total="filteredMembers.length"
              layout="sizes, prev, pager, next"
              background
            />
          </el-config-provider>
        </footer>
      </template>
    </section>
    </div>

    <!-- Add Node Drawer -->
    <el-drawer
      v-model="showAddDialog"
      :title="t.cluster.addNode"
      direction="rtl"
      size="560px"
      append-to-body
      @close="addForms = [{ protocol: 'http://', host: '', port: '' }]"
      class="form-drawer"
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
    </el-drawer>
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
  padding: 18px 0 0;
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
  padding: 0 0 18px;
  border-bottom: 1px solid var(--border-color);
}

.toolbar-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px 0;
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
  background: var(--control-muted-bg);
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
  padding: 18px 0 0;
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

<style scoped>
.card-grid {
  grid-template-columns: repeat(auto-fill, minmax(336px, 360px));
  justify-content: start;
  gap: 16px;
}

.node-card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px;
  border-radius: 14px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  box-shadow: none;
  overflow: hidden;
  transition: border-color 0.2s ease, background-color 0.2s ease, box-shadow 0.2s ease;
}

.node-card:hover {
  transform: none;
  border-color: rgba(59, 130, 246, 0.32);
  background: rgba(59, 130, 246, 0.03);
  box-shadow: inset 0 0 0 1px rgba(59, 130, 246, 0.08);
}

.node-card.is-local {
  border-color: rgba(16, 185, 129, 0.22);
}

.node-card.is-local:hover {
  border-color: rgba(16, 185, 129, 0.3);
  background: rgba(16, 185, 129, 0.035);
  box-shadow: inset 0 0 0 1px rgba(16, 185, 129, 0.08);
}

.node-card.is-offline {
  border-color: rgba(148, 163, 184, 0.2);
}

.card-accent {
  position: absolute;
  inset: 0 auto auto 0;
  width: 84px;
  height: 3px;
  background: rgba(59, 130, 246, 0.35);
  border-radius: 0 0 999px 0;
}

.node-card.is-local .card-accent {
  background: rgba(16, 185, 129, 0.4);
}

.node-card.is-offline .card-accent {
  background: rgba(148, 163, 184, 0.34);
}

.card-head {
  position: relative;
  z-index: 1;
  display: block;
  margin-bottom: 0;
}

.card-identity {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  min-width: 0;
}

.card-symbol {
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  background: rgba(59, 130, 246, 0.12);
  border: 1px solid rgba(59, 130, 246, 0.12);
  color: var(--accent-blue);
  font-size: 18px;
}

.node-card.is-local .card-symbol {
  background: rgba(16, 185, 129, 0.12);
  border-color: rgba(16, 185, 129, 0.14);
  color: var(--accent-green);
}

.node-card.is-offline .card-symbol {
  background: rgba(148, 163, 184, 0.12);
  border-color: rgba(148, 163, 184, 0.14);
  color: var(--text-muted);
}

.card-copy {
  min-width: 0;
  flex: 1;
  padding-top: 0;
}

.card-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 4px;
}

.card-title {
  display: block;
  color: var(--text-primary);
  font-size: 15px;
  font-weight: 800;
  line-height: 1.15;
  letter-spacing: 0;
  font-family: inherit;
}

.card-subtitle {
  margin: 0;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.45;
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  word-break: break-word;
}

.card-aside {
  display: none;
}

.meta-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 9px;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(148, 163, 184, 0.08);
  color: var(--text-secondary);
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
  white-space: nowrap;
}

.meta-chip.local {
  border-color: rgba(16, 185, 129, 0.18);
  background: rgba(16, 185, 129, 0.1);
  color: var(--accent-green);
}

.meta-chip.online {
  border-color: rgba(16, 185, 129, 0.18);
  background: rgba(16, 185, 129, 0.1);
  color: var(--accent-green);
}

.meta-chip.offline {
  border-color: rgba(148, 163, 184, 0.18);
  background: rgba(148, 163, 184, 0.1);
  color: var(--text-muted);
}

.meta-chip.role.leader {
  border-color: rgba(248, 113, 113, 0.18);
  background: rgba(248, 113, 113, 0.1);
  color: var(--accent-red);
}

.meta-chip.role.follower {
  border-color: rgba(59, 130, 246, 0.18);
  background: rgba(59, 130, 246, 0.1);
  color: var(--accent-blue);
}

.meta-chip.role.candidate {
  border-color: rgba(251, 146, 60, 0.18);
  background: rgba(251, 146, 60, 0.1);
  color: var(--accent-orange);
}

.meta-chip.role.local {
  border-color: rgba(16, 185, 129, 0.18);
  background: rgba(16, 185, 129, 0.1);
  color: var(--accent-green);
}

.meta-chip.role.neutral {
  border-color: rgba(148, 163, 184, 0.18);
  background: rgba(148, 163, 184, 0.08);
  color: var(--text-secondary);
}

.meta-chip .status-dot {
  width: 7px;
  height: 7px;
  box-shadow: none;
}

.card-body {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.compact-meta-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  min-width: 0;
  font-size: 12px;
  line-height: 1.45;
}

.compact-label {
  flex: 0 0 44px;
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
  padding-top: 1px;
}

.compact-value {
  min-width: 0;
  color: var(--text-primary);
  font-size: 12px;
  line-height: 1.5;
  word-break: break-word;
}

.mono-value {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
}

.node-meta-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  min-width: 0;
}

.node-ip-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  min-width: 0;
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
}

.node-ip-row > .compact-label {
  display: none;
}

.node-ip-label {
  flex: 0 0 44px;
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
  padding-top: 1px;
}

.node-ip-value {
  min-width: 0;
  color: var(--text-primary);
  font-size: 12px;
  line-height: 1.5;
  word-break: break-word;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
}

.protocol-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin-top: 2px;
}

.protocol-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 12px;
  background: rgba(148, 163, 184, 0.05);
  border: 1px solid rgba(148, 163, 184, 0.14);
  box-shadow: none;
}

.protocol-name {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.protocol-port {
  color: var(--text-primary);
  font-size: 12px;
  font-weight: 700;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
}

.card-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--border-color);
}

.card-footnote {
  display: none;
}

.card-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  margin-left: auto;
}

@media (max-width: 1180px) {
  .card-grid {
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  }
}

@media (max-width: 768px) {
  .card-grid {
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  }
}

@media (max-width: 640px) {
  .card-grid {
    grid-template-columns: 1fr;
  }

  .protocol-grid {
    grid-template-columns: 1fr;
  }
}

</style>

<style scoped>
.card-grid {
  grid-template-columns: repeat(auto-fill, minmax(336px, 360px));
  justify-content: start;
  gap: 16px;
}

.node-card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px;
  border-radius: 14px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  box-shadow: none;
  overflow: hidden;
  transition: border-color 0.2s ease, background-color 0.2s ease, box-shadow 0.2s ease;
}

.node-card:hover {
  transform: none;
  border-color: rgba(59, 130, 246, 0.32);
  background: rgba(59, 130, 246, 0.03);
  box-shadow: inset 0 0 0 1px rgba(59, 130, 246, 0.08);
}

.node-card.is-local {
  border-color: rgba(16, 185, 129, 0.22);
}

.node-card.is-local:hover {
  border-color: rgba(16, 185, 129, 0.3);
  background: rgba(16, 185, 129, 0.035);
  box-shadow: inset 0 0 0 1px rgba(16, 185, 129, 0.08);
}

.node-card.is-offline {
  border-color: rgba(148, 163, 184, 0.2);
}

.card-accent {
  position: absolute;
  inset: 0 auto auto 0;
  width: 84px;
  height: 3px;
  background: rgba(59, 130, 246, 0.35);
  border-radius: 0 0 999px 0;
}

.node-card.is-local .card-accent {
  background: rgba(16, 185, 129, 0.4);
}

.node-card.is-offline .card-accent {
  background: rgba(148, 163, 184, 0.34);
}

.card-head {
  position: relative;
  z-index: 1;
  display: block;
  margin-bottom: 0;
}

.card-identity {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  min-width: 0;
}

.card-symbol {
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  background: rgba(59, 130, 246, 0.12);
  border: 1px solid rgba(59, 130, 246, 0.12);
  color: var(--accent-blue);
  font-size: 18px;
}

.node-card.is-local .card-symbol {
  background: rgba(16, 185, 129, 0.12);
  border-color: rgba(16, 185, 129, 0.14);
  color: var(--accent-green);
}

.node-card.is-offline .card-symbol {
  background: rgba(148, 163, 184, 0.12);
  border-color: rgba(148, 163, 184, 0.14);
  color: var(--text-muted);
}

.card-copy {
  min-width: 0;
  flex: 1;
  padding-top: 0;
}

.card-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 4px;
}

.card-title {
  display: block;
  color: var(--text-primary);
  font-size: 15px;
  font-weight: 800;
  line-height: 1.15;
  letter-spacing: 0;
  font-family: inherit;
}

.card-subtitle {
  margin: 0;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.45;
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  word-break: break-word;
}

.card-aside {
  display: none;
}

.meta-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 9px;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(148, 163, 184, 0.08);
  color: var(--text-secondary);
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
  white-space: nowrap;
}

.meta-chip.local {
  border-color: rgba(16, 185, 129, 0.18);
  background: rgba(16, 185, 129, 0.1);
  color: var(--accent-green);
}

.meta-chip.online {
  border-color: rgba(16, 185, 129, 0.18);
  background: rgba(16, 185, 129, 0.1);
  color: var(--accent-green);
}

.meta-chip.offline {
  border-color: rgba(148, 163, 184, 0.18);
  background: rgba(148, 163, 184, 0.1);
  color: var(--text-muted);
}

.meta-chip.role.leader {
  border-color: rgba(248, 113, 113, 0.18);
  background: rgba(248, 113, 113, 0.1);
  color: var(--accent-red);
}

.meta-chip.role.follower {
  border-color: rgba(59, 130, 246, 0.18);
  background: rgba(59, 130, 246, 0.1);
  color: var(--accent-blue);
}

.meta-chip.role.candidate {
  border-color: rgba(251, 146, 60, 0.18);
  background: rgba(251, 146, 60, 0.1);
  color: var(--accent-orange);
}

.meta-chip.role.local {
  border-color: rgba(16, 185, 129, 0.18);
  background: rgba(16, 185, 129, 0.1);
  color: var(--accent-green);
}

.meta-chip.role.neutral {
  border-color: rgba(148, 163, 184, 0.18);
  background: rgba(148, 163, 184, 0.08);
  color: var(--text-secondary);
}

.meta-chip .status-dot {
  width: 7px;
  height: 7px;
  box-shadow: none;
}

.card-body {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.compact-meta-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  min-width: 0;
  font-size: 12px;
  line-height: 1.45;
}

.compact-label {
  flex: 0 0 44px;
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
  padding-top: 1px;
}

.compact-value {
  min-width: 0;
  color: var(--text-primary);
  font-size: 12px;
  line-height: 1.5;
  word-break: break-word;
}

.mono-value {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
}

.node-meta-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  min-width: 0;
}

.node-ip-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  min-width: 0;
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
}

.node-ip-row > .compact-label {
  display: none;
}

.node-ip-label {
  flex: 0 0 44px;
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
  padding-top: 1px;
}

.node-ip-value {
  min-width: 0;
  color: var(--text-primary);
  font-size: 12px;
  line-height: 1.5;
  word-break: break-word;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
}

.protocol-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin-top: 2px;
}

.protocol-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 12px;
  background: rgba(148, 163, 184, 0.05);
  border: 1px solid rgba(148, 163, 184, 0.14);
  box-shadow: none;
}

.protocol-name {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.protocol-port {
  color: var(--text-primary);
  font-size: 12px;
  font-weight: 700;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
}

.card-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--border-color);
}

.card-footnote {
  display: none;
}

.card-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  margin-left: auto;
}

@media (max-width: 1180px) {
  .card-grid {
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  }
}

@media (max-width: 768px) {
  .card-grid {
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  }
}

@media (max-width: 640px) {
  .card-grid {
    grid-template-columns: 1fr;
  }

  .protocol-grid {
    grid-template-columns: 1fr;
  }
}
</style>


<style scoped>
.card-grid.card-grid {
  grid-template-columns: repeat(auto-fill, minmax(336px, 360px));
  justify-content: start;
  gap: 16px;
}

.node-card.node-card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px;
  border-radius: 14px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  box-shadow: none;
  overflow: hidden;
  transition: border-color 0.2s ease, background-color 0.2s ease, box-shadow 0.2s ease;
}

.node-card.node-card:hover {
  transform: none;
  border-color: rgba(59, 130, 246, 0.32);
  background: rgba(59, 130, 246, 0.03);
  box-shadow: inset 0 0 0 1px rgba(59, 130, 246, 0.08);
}

.node-card.node-card.is-local {
  border-color: rgba(16, 185, 129, 0.22);
}

.node-card.node-card.is-local:hover {
  border-color: rgba(16, 185, 129, 0.3);
  background: rgba(16, 185, 129, 0.035);
  box-shadow: inset 0 0 0 1px rgba(16, 185, 129, 0.08);
}

.node-card.node-card.is-offline {
  border-color: rgba(148, 163, 184, 0.2);
}

.node-card.node-card .card-accent {
  position: absolute;
  inset: 0 auto auto 0;
  width: 84px;
  height: 3px;
  background: rgba(59, 130, 246, 0.35);
  border-radius: 0 0 999px 0;
}

.node-card.node-card.is-local .card-accent {
  background: rgba(16, 185, 129, 0.4);
}

.node-card.node-card.is-offline .card-accent {
  background: rgba(148, 163, 184, 0.34);
}

.node-card.node-card .card-head {
  position: relative;
  z-index: 1;
  display: block;
  margin-bottom: 0;
  padding-right: 148px;
}

.node-card.node-card .card-copy {
  min-width: 0;
  flex: 1;
  padding-top: 0;
}

.node-card.node-card .card-current-mark {
  position: relative;
  padding-left: 10px;
  color: var(--accent-green);
  font-size: 11px;
  font-weight: 700;
  line-height: 1.2;
}

.node-card.node-card .card-current-mark::before {
  content: '';
  position: absolute;
  left: 0;
  top: 50%;
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: currentColor;
  transform: translateY(-50%);
}

.node-card.node-card .card-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 4px;
}

.node-card.node-card .card-title {
  display: block;
  color: var(--text-primary);
  font-size: 15px;
  font-weight: 800;
  line-height: 1.15;
  letter-spacing: 0;
  font-family: inherit;
}

.node-card.node-card .card-subtitle {
  display: none;
}

.node-card.node-card .card-aside {
  display: none;
}

.node-card.node-card .card-hostline {
  margin: 0;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.45;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
}

.node-card.node-card .meta-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 9px;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(148, 163, 184, 0.08);
  color: var(--text-secondary);
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
  white-space: nowrap;
}

.node-card.node-card .meta-chip .status-dot {
  width: 7px;
  height: 7px;
  box-shadow: none;
}

.node-card.node-card .card-body {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.node-card.node-card .compact-meta-row:first-child {
  position: absolute;
  top: 14px;
  right: 14px;
  z-index: 2;
}

.node-card.node-card .compact-meta-row:first-child .compact-label {
  display: none;
}

.node-card.node-card .compact-meta-row:first-child .node-meta-pills {
  justify-content: flex-end;
}

.node-card.node-card .node-ip-row {
  display: none;
}

.node-card.node-card .node-ip-row > .compact-label {
  display: none;
}

.node-card.node-card .node-ip-label {
  flex: 0 0 44px;
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
  padding-top: 1px;
}

.node-card.node-card .node-ip-value {
  min-width: 0;
  color: var(--text-primary);
  font-size: 12px;
  line-height: 1.5;
  word-break: break-word;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
}

.node-card.node-card .protocol-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin-top: 2px;
}

.node-card.node-card .protocol-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 12px;
  background: rgba(148, 163, 184, 0.05);
  border: 1px solid rgba(148, 163, 184, 0.14);
  box-shadow: none;
}

.node-card.node-card .protocol-name {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.node-card.node-card .protocol-port {
  color: var(--text-primary);
  font-size: 12px;
  font-weight: 700;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
}

.node-card.node-card .card-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--border-color);
}

.node-card.node-card .card-footnote {
  display: none;
}

@media (max-width: 1180px) {
  .card-grid.card-grid {
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
    justify-content: stretch;
  }
}

@media (max-width: 768px) {
  .card-grid.card-grid {
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  }

  .node-card.node-card .card-head {
    padding-right: 0;
  }

  .node-card.node-card .compact-meta-row:first-child {
    position: static;
  }

  .node-card.node-card .compact-meta-row:first-child .compact-label {
    display: block;
  }

  .node-card.node-card .protocol-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .card-grid.card-grid {
    grid-template-columns: 1fr;
  }
}

.card-grid {
  grid-template-columns: repeat(auto-fill, minmax(286px, 320px));
  gap: 12px;
  padding: 2px 0 10px;
}

.node-card {
  gap: 10px;
  padding: 14px 15px 12px;
  border-radius: 16px;
  background: var(--bg-secondary);
  border: 1px solid rgba(148, 163, 184, 0.22);
  box-shadow: none;
}

.node-card:hover {
  transform: none;
  border-color: rgba(59, 130, 246, 0.28);
  box-shadow: inset 0 0 0 1px rgba(59, 130, 246, 0.08);
  background: rgba(59, 130, 246, 0.03);
}

.node-card.is-local {
  border-color: rgba(16, 185, 129, 0.24);
  box-shadow: none;
}

.card-accent {
  width: 88px;
  height: 4px;
  background: rgba(59, 130, 246, 0.38);
}

.node-card.is-local .card-accent {
  background: rgba(16, 185, 129, 0.42);
}

.card-head {
  margin-bottom: 0;
  align-items: flex-start;
}

.card-copy {
  padding-top: 4px;
}

.card-title-row {
  gap: 8px;
}

.card-title {
  font-size: 17px;
  font-weight: 700;
  line-height: 1.15;
  letter-spacing: -0.02em;
  font-family: 'Avenir Next', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
}

.card-aside {
  flex-direction: row;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.meta-chip {
  padding: 5px 10px;
  border: 1px solid rgba(148, 163, 184, 0.2);
  background: rgba(148, 163, 184, 0.1);
  font-size: 11px;
}

.meta-chip.local {
  border-color: rgba(16, 185, 129, 0.22);
  background: rgba(16, 185, 129, 0.12);
}

.meta-chip.online {
  border-color: rgba(16, 185, 129, 0.22);
  background: rgba(16, 185, 129, 0.1);
}

.meta-chip.offline {
  border-color: rgba(148, 163, 184, 0.22);
  background: rgba(148, 163, 184, 0.14);
}

.meta-chip.role.leader {
  border-color: rgba(248, 113, 113, 0.22);
  background: rgba(248, 113, 113, 0.12);
}

.meta-chip.role.follower {
  border-color: rgba(59, 130, 246, 0.22);
  background: rgba(59, 130, 246, 0.12);
}

.meta-chip.role.candidate {
  border-color: rgba(251, 146, 60, 0.22);
  background: rgba(251, 146, 60, 0.12);
}

.meta-chip.role.local {
  border-color: rgba(16, 185, 129, 0.22);
  background: rgba(16, 185, 129, 0.12);
}

.meta-chip.role.neutral {
  border-color: rgba(148, 163, 184, 0.22);
  background: rgba(148, 163, 184, 0.1);
}

.meta-chip .status-dot {
  width: 8px;
  height: 8px;
}

.card-body {
  flex: initial;
  gap: 8px;
  margin-bottom: 0;
}

.node-ip-row {
  gap: 10px;
  padding: 9px 11px;
  border: 1px solid rgba(148, 163, 184, 0.14);
  border-radius: 13px;
  background: rgba(148, 163, 184, 0.06);
}

.node-ip-label {
  flex: 0 0 auto;
  font-size: 11px;
  letter-spacing: 0.02em;
}

.node-ip-value {
  font-size: 13px;
  line-height: 1.3;
}

.protocol-grid {
  gap: 8px;
}

.protocol-card {
  padding: 8px 10px;
  border-radius: 12px;
  background: rgba(148, 163, 184, 0.06);
  border: 1px solid rgba(148, 163, 184, 0.16);
  box-shadow: none;
}

.protocol-name {
  font-size: 11px;
  letter-spacing: 0.04em;
}

.protocol-port {
  font-size: 12px;
}

.card-footer {
  justify-content: flex-end;
  gap: 10px;
  padding-top: 6px;
  margin-top: 0;
  border-top: 0;
  min-height: 0;
}

.card-footnote {
  display: none;
}

@media (max-width: 1180px) {
  .card-grid {
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    justify-content: stretch;
  }
}

@media (max-width: 768px) {
  .card-grid {
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  }

  .card-head {
    gap: 8px;
  }

  .card-aside {
    justify-content: flex-start;
  }
}

@media (max-width: 640px) {
  .card-grid {
    grid-template-columns: 1fr;
  }

  .protocol-grid {
    grid-template-columns: 1fr;
  }
}
</style>

<style scoped>
.card-grid {
  grid-template-columns: repeat(auto-fill, minmax(286px, 320px));
  gap: 14px;
  padding: 2px 0 10px;
}

.node-card {
  gap: 10px;
  padding: 14px 15px 12px;
  border-radius: 18px;
  background:
    radial-gradient(circle at top right, rgba(59, 130, 246, 0.14), transparent 36%),
    radial-gradient(circle at bottom left, rgba(16, 185, 129, 0.08), transparent 28%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.045), rgba(255, 255, 255, 0.015)),
    var(--bg-secondary);
  border: 1px solid rgba(148, 163, 184, 0.18);
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.08);
}

.node-card:hover {
  transform: translateY(-2px);
  border-color: rgba(59, 130, 246, 0.26);
  box-shadow: 0 16px 30px rgba(15, 23, 42, 0.1);
}

.node-card.is-local {
  border-color: rgba(16, 185, 129, 0.24);
  box-shadow: 0 14px 28px rgba(15, 23, 42, 0.09);
}

.card-accent {
  width: 88px;
  height: 4px;
  background: linear-gradient(90deg, var(--accent-blue), rgba(59, 130, 246, 0.22));
}

.node-card.is-local .card-accent {
  background: linear-gradient(90deg, var(--accent-green), rgba(16, 185, 129, 0.22));
}

.card-head {
  margin-bottom: 0;
}

.card-copy {
  padding-top: 4px;
}

.card-title-row {
  gap: 8px;
}

.card-title {
  font-size: 18px;
  font-weight: 700;
  line-height: 1.15;
  letter-spacing: -0.02em;
  font-family: 'Avenir Next', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
}

.card-aside {
  flex-direction: row;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.meta-chip {
  padding: 5px 10px;
  border: 1px solid rgba(148, 163, 184, 0.2);
  background: rgba(255, 255, 255, 0.08);
  font-size: 11px;
}

.meta-chip.local {
  border-color: rgba(16, 185, 129, 0.22);
  background: rgba(16, 185, 129, 0.12);
}

.meta-chip.online {
  border-color: rgba(16, 185, 129, 0.22);
  background: rgba(16, 185, 129, 0.1);
}

.meta-chip.offline {
  border-color: rgba(148, 163, 184, 0.22);
  background: rgba(148, 163, 184, 0.14);
}

.meta-chip.role.leader {
  border-color: rgba(248, 113, 113, 0.22);
  background: rgba(248, 113, 113, 0.12);
}

.meta-chip.role.follower {
  border-color: rgba(59, 130, 246, 0.22);
  background: rgba(59, 130, 246, 0.12);
}

.meta-chip.role.candidate {
  border-color: rgba(251, 146, 60, 0.22);
  background: rgba(251, 146, 60, 0.12);
}

.meta-chip.role.local {
  border-color: rgba(16, 185, 129, 0.22);
  background: rgba(16, 185, 129, 0.12);
}

.meta-chip.role.neutral {
  border-color: rgba(148, 163, 184, 0.22);
  background: rgba(148, 163, 184, 0.1);
}

.meta-chip .status-dot {
  width: 8px;
  height: 8px;
}

.card-body {
  flex: initial;
  gap: 8px;
  margin-bottom: 0;
}

.node-ip-row {
  gap: 10px;
  padding: 9px 11px;
  border: 1px dashed rgba(148, 163, 184, 0.24);
  border-radius: 13px;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.06), rgba(255, 255, 255, 0.02));
}

.node-ip-label {
  flex: 0 0 auto;
  font-size: 11px;
  letter-spacing: 0.02em;
}

.node-ip-value {
  font-size: 13px;
  line-height: 1.3;
}

.protocol-grid {
  gap: 8px;
}

.protocol-card {
  padding: 8px 10px;
  border-radius: 12px;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.08), rgba(255, 255, 255, 0.03));
  border: 1px solid rgba(148, 163, 184, 0.16);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.18);
}

.protocol-name {
  font-size: 11px;
  letter-spacing: 0.04em;
}

.protocol-port {
  font-size: 12px;
}

.card-footer {
  justify-content: flex-end;
  gap: 10px;
  padding-top: 6px;
  margin-top: 0;
  border-top: 0;
  min-height: 0;
}

.card-footnote {
  display: none;
}

@media (max-width: 1180px) {
  .card-grid {
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    justify-content: stretch;
  }
}

@media (max-width: 768px) {
  .card-grid {
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  }

  .card-head {
    gap: 8px;
  }

  .card-aside {
    justify-content: flex-start;
  }
}

@media (max-width: 640px) {
  .card-grid {
    grid-template-columns: 1fr;
  }

  .protocol-grid {
    grid-template-columns: 1fr;
  }
}
</style>

<style scoped>
.node-card {
  border-radius: 16px;
  background: var(--bg-secondary);
  border: 1px solid rgba(148, 163, 184, 0.14);
  box-shadow: 0 4px 14px rgba(15, 23, 42, 0.05);
}

.node-card::after {
  display: none;
}

.node-card:hover {
  transform: none;
  border-color: rgba(59, 130, 246, 0.18);
  box-shadow: 0 8px 18px rgba(15, 23, 42, 0.07);
  background: var(--bg-secondary);
}

.node-card.is-local {
  border-color: rgba(16, 185, 129, 0.18);
  box-shadow: 0 8px 18px rgba(15, 23, 42, 0.06);
}

.card-glow {
  height: 5px;
  opacity: 1;
}

.status-line {
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px solid var(--border-color);
}

.node-card.is-online .status-line {
  background: rgba(16, 185, 129, 0.06);
}

.node-card.is-offline .status-line {
  background: rgba(148, 163, 184, 0.08);
}

.addr-group {
  gap: 12px;
}

.addr-box {
  justify-content: space-between;
  padding: 10px 12px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(148, 163, 184, 0.14);
}

.addr-box .p-tag {
  min-width: 48px;
  padding: 4px 8px;
  border-radius: 999px;
  background: transparent;
  opacity: 1;
}

.addr-box.http .p-tag {
  color: var(--text-secondary);
}

.addr-box.grpc .p-tag {
  color: var(--accent-green);
}

.addr-box.raft .p-tag {
  color: var(--accent-orange);
}

.addr-box.quic .p-tag {
  color: var(--accent-blue);
}

.addr-box code {
  background: transparent;
  padding: 0;
}

.card-footer {
  border-top: none;
}
</style>

<style scoped>
.svc-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  background: transparent;
  backdrop-filter: none;
  border: 0;
  border-radius: 0;
}

.svc-toolbar {
  padding: 0 0 18px;
}

.field-item {
  gap: 8px;
}

.field-label {
  font-weight: 700;
}

.search-input {
  width: 180px;
  flex-shrink: 0;
}

.toolbar-sep {
  height: 20px;
  margin: 0 2px;
  opacity: 1;
}

:deep(.search-input .el-input__inner) {
  color: var(--text-primary) !important;
  font-size: 14px;
}

:deep(.search-input .el-input__prefix .el-icon) {
  color: var(--text-muted);
}

.svc-content {
  padding: 18px 0 0;
}

.empty-panel {
  flex: 1;
  min-height: 200px;
  padding: 0;
  background: transparent;
  border: 0;
  border-radius: 0;
}

.table-wrap {
  margin-bottom: 8px;
  background: transparent;
  border: 0;
  border-radius: 0;
  padding: 0;
}

.card-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 272px));
  justify-content: start;
  gap: 10px;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  align-content: start;
  padding: 0 0 8px;
}

.node-card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-height: 0;
  padding: 12px 14px;
  border-radius: 12px;
  background:
    radial-gradient(circle at top right, rgba(59, 130, 246, 0.08), transparent 34%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.035), rgba(255, 255, 255, 0.015)),
    var(--bg-secondary);
  border: 1px solid var(--border-color);
  box-shadow: 0 4px 14px rgba(15, 23, 42, 0.05);
  overflow: hidden;
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}

.node-card:hover {
  transform: translateY(-1px);
  border-color: rgba(59, 130, 246, 0.22);
  box-shadow: 0 8px 18px rgba(15, 23, 42, 0.08);
}

.card-accent {
  position: absolute;
  inset: 0 auto auto 0;
  width: 72px;
  height: 3px;
  background: linear-gradient(90deg, var(--accent-blue), rgba(59, 130, 246, 0.18));
  border-radius: 0 0 999px 0;
}

.node-card.is-local .card-accent {
  background: linear-gradient(90deg, var(--accent-green), rgba(16, 185, 129, 0.18));
}

.card-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

.card-copy {
  min-width: 0;
  flex: 1;
  padding-top: 2px;
}

.card-title-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  margin: 0;
}

.card-title {
  display: block;
  color: var(--text-primary);
  font-size: 14px;
  font-weight: 800;
  line-height: 1.2;
}

.card-subtitle {
  display: none;
}

.card-aside {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
  flex-shrink: 0;
}

.meta-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 8px;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(255, 255, 255, 0.04);
  color: var(--text-secondary);
  font-size: 10px;
  font-weight: 700;
  line-height: 1;
  white-space: nowrap;
}

.meta-chip.local {
  border-color: rgba(16, 185, 129, 0.18);
  background: rgba(16, 185, 129, 0.1);
  color: var(--accent-green);
}

.meta-chip.online {
  border-color: rgba(16, 185, 129, 0.18);
  background: rgba(16, 185, 129, 0.08);
  color: var(--accent-green);
}

.meta-chip.offline {
  border-color: rgba(148, 163, 184, 0.18);
  background: rgba(148, 163, 184, 0.12);
  color: var(--text-muted);
}

.meta-chip.role.leader {
  border-color: rgba(248, 113, 113, 0.2);
  background: rgba(248, 113, 113, 0.1);
  color: #ef4444;
}

.meta-chip.role.follower {
  border-color: rgba(59, 130, 246, 0.18);
  background: rgba(59, 130, 246, 0.1);
  color: var(--accent-blue);
}

.meta-chip.role.candidate {
  border-color: rgba(251, 146, 60, 0.18);
  background: rgba(251, 146, 60, 0.1);
  color: var(--accent-orange);
}

.meta-chip.role.local {
  border-color: rgba(16, 185, 129, 0.18);
  background: rgba(16, 185, 129, 0.1);
  color: var(--accent-green);
}

.meta-chip.role.neutral {
  border-color: rgba(148, 163, 184, 0.18);
  background: rgba(148, 163, 184, 0.08);
  color: var(--text-secondary);
}

.meta-chip .status-dot {
  width: 7px;
  height: 7px;
  box-shadow: none;
}

.card-body {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.node-ip-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  padding: 0;
  border: 0;
  background: transparent;
}

.node-ip-row > .compact-label {
  display: none;
}

.node-ip-label {
  flex: 0 0 40px;
  color: var(--text-muted);
  font-size: 10px;
  font-weight: 700;
  white-space: nowrap;
}

.node-ip-value {
  min-width: 0;
  color: var(--text-primary);
  font-size: 12px;
  line-height: 1.35;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.protocol-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px;
}

.protocol-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 8px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(148, 163, 184, 0.14);
}

.protocol-name {
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.03em;
}

.protocol-port {
  color: var(--text-primary);
  font-size: 11px;
  font-weight: 700;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
}

.protocol-card.http .protocol-name {
  color: var(--text-secondary);
}

.protocol-card.grpc .protocol-name {
  color: var(--accent-green);
}

.protocol-card.raft .protocol-name {
  color: var(--accent-orange);
}

.protocol-card.quic .protocol-name {
  color: var(--accent-blue);
}

.protocol-card.mono .protocol-name {
  color: var(--text-secondary);
}

.card-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  padding-top: 8px;
  border-top: 1px solid rgba(148, 163, 184, 0.14);
}

.card-footnote {
  display: none;
}

.card-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.mono-addr {
  display: inline-flex;
  align-items: center;
  padding: 4px 0;
  color: var(--text-secondary);
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 12px;
}

.colon-separator {
  color: var(--text-muted);
  font-weight: 600;
}

.row-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}

:deep(.protocol-select .el-select__wrapper),
:deep(.host-input .el-input__wrapper),
:deep(.port-input .el-input__wrapper) {
  background: rgba(255, 255, 255, 0.04) !important;
  border: 1px solid var(--border-color) !important;
  box-shadow: none !important;
}

@media (max-width: 1400px) {
  .card-grid {
    grid-template-columns: repeat(3, minmax(0, 272px));
  }
}

@media (max-width: 1180px) {
  .toolbar-row {
    gap: 8px;
  }

  .toolbar-group {
    padding: 0;
  }

  .toolbar-group.right-align {
    width: 100%;
    margin-left: 0;
    justify-content: space-between;
  }

  .card-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .svc-toolbar {
    padding: 0 0 14px;
  }

  .svc-content {
    padding: 14px 0 0;
  }

  .field-item {
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

  .table-wrap {
    padding: 0 12px;
  }

  .card-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .card-head {
    align-items: flex-start;
  }

  .card-aside {
    gap: 4px;
  }

  .protocol-grid {
    grid-template-columns: 1fr;
  }

  .svc-footer {
    flex-direction: column;
    align-items: stretch;
  }

  .node-input-row {
    flex-wrap: wrap;
  }

  .protocol-select,
  .host-input,
  .port-input {
    width: 100%;
  }

  .row-actions {
    width: 100%;
    margin-left: 0;
    justify-content: flex-end;
  }
}

@media (max-width: 640px) {
  .card-grid {
    grid-template-columns: 1fr;
  }
}
</style>

<style scoped>
.card-grid {
  grid-template-columns: repeat(auto-fill, minmax(286px, 320px));
  gap: 14px;
  padding: 2px 0 10px;
}

.node-card {
  gap: 10px;
  padding: 14px 15px 12px;
  border-radius: 16px;
  background: var(--bg-secondary);
  border: 1px solid rgba(148, 163, 184, 0.18);
  box-shadow: none;
}

.node-card:hover {
  transform: none;
  border-color: rgba(59, 130, 246, 0.28);
  box-shadow: inset 0 0 0 1px rgba(59, 130, 246, 0.08);
  background: rgba(59, 130, 246, 0.03);
}

.node-card.is-local {
  border-color: rgba(16, 185, 129, 0.24);
  box-shadow: none;
}

.card-accent {
  width: 88px;
  height: 4px;
  background: rgba(59, 130, 246, 0.38);
}

.node-card.is-local .card-accent {
  background: rgba(16, 185, 129, 0.42);
}

.card-head {
  margin-bottom: 0;
  align-items: flex-start;
}

.card-copy {
  padding-top: 4px;
}

.card-title-row {
  gap: 8px;
}

.card-title {
  font-size: 18px;
  font-weight: 700;
  line-height: 1.15;
  letter-spacing: -0.02em;
  font-family: 'Avenir Next', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
}

.card-aside {
  flex-direction: row;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.meta-chip {
  padding: 5px 10px;
  border: 1px solid rgba(148, 163, 184, 0.2);
  background: rgba(148, 163, 184, 0.1);
  font-size: 11px;
}

.meta-chip.local {
  border-color: rgba(16, 185, 129, 0.22);
  background: rgba(16, 185, 129, 0.12);
}

.meta-chip.online {
  border-color: rgba(16, 185, 129, 0.22);
  background: rgba(16, 185, 129, 0.1);
}

.meta-chip.offline {
  border-color: rgba(148, 163, 184, 0.22);
  background: rgba(148, 163, 184, 0.14);
}

.meta-chip.role.leader {
  border-color: rgba(248, 113, 113, 0.22);
  background: rgba(248, 113, 113, 0.12);
}

.meta-chip.role.follower {
  border-color: rgba(59, 130, 246, 0.22);
  background: rgba(59, 130, 246, 0.12);
}

.meta-chip.role.candidate {
  border-color: rgba(251, 146, 60, 0.22);
  background: rgba(251, 146, 60, 0.12);
}

.meta-chip.role.local {
  border-color: rgba(16, 185, 129, 0.22);
  background: rgba(16, 185, 129, 0.12);
}

.meta-chip.role.neutral {
  border-color: rgba(148, 163, 184, 0.22);
  background: rgba(148, 163, 184, 0.1);
}

.meta-chip .status-dot {
  width: 8px;
  height: 8px;
}

.card-body {
  flex: initial;
  gap: 8px;
  margin-bottom: 0;
}

.node-ip-row {
  gap: 10px;
  padding: 9px 11px;
  border: 1px solid rgba(148, 163, 184, 0.14);
  border-radius: 13px;
  background: rgba(148, 163, 184, 0.06);
}

.node-ip-label {
  flex: 0 0 auto;
  font-size: 11px;
  letter-spacing: 0.02em;
}

.node-ip-value {
  font-size: 13px;
  line-height: 1.3;
}

.protocol-grid {
  gap: 8px;
}

.protocol-card {
  padding: 8px 10px;
  border-radius: 12px;
  background: rgba(148, 163, 184, 0.06);
  border: 1px solid rgba(148, 163, 184, 0.16);
  box-shadow: none;
}

.protocol-name {
  font-size: 11px;
  letter-spacing: 0.04em;
}

.protocol-port {
  font-size: 12px;
}

.card-footer {
  justify-content: flex-end;
  gap: 10px;
  padding-top: 6px;
  margin-top: 0;
  border-top: 0;
  min-height: 0;
}

.card-footnote {
  display: none;
}

@media (max-width: 1180px) {
  .card-grid {
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    justify-content: stretch;
  }
}

@media (max-width: 768px) {
  .card-grid {
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  }

  .card-head {
    gap: 8px;
  }

  .card-aside {
    justify-content: flex-start;
  }
}

@media (max-width: 640px) {
  .card-grid {
    grid-template-columns: 1fr;
  }

  .protocol-grid {
    grid-template-columns: 1fr;
  }
}
</style>

<style scoped>
/* Node card redesign override: final layer. */
.card-grid.card-grid {
  grid-template-columns: repeat(auto-fill, minmax(440px, 520px));
  justify-content: start;
  align-content: start;
  gap: 14px;
  padding: 2px 0 12px;
}

.node-card.node-card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 132px;
  padding: 18px 18px 14px 20px;
  border-radius: 8px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: var(--bg-secondary);
  box-shadow: none;
  overflow: hidden;
  transition: border-color 0.18s ease, box-shadow 0.18s ease;
}

.node-card.node-card:hover {
  transform: none;
  border-color: rgba(59, 130, 246, 0.32);
  background: var(--bg-secondary);
  box-shadow: inset 0 0 0 1px rgba(59, 130, 246, 0.08);
}

.node-card.node-card.is-local {
  border-color: rgba(16, 185, 129, 0.28);
}

.node-card.node-card.is-local:hover {
  border-color: rgba(16, 185, 129, 0.36);
  box-shadow: inset 0 0 0 1px rgba(16, 185, 129, 0.1);
}

.node-card.node-card.is-offline {
  border-color: rgba(148, 163, 184, 0.2);
  opacity: 0.78;
}

.node-card.node-card .card-accent {
  position: absolute;
  inset: 16px auto 16px 0;
  width: 3px;
  height: auto;
  border-radius: 0 999px 999px 0;
  background: var(--accent-blue);
}

.node-card.node-card.is-local .card-accent {
  background: var(--accent-green);
}

.node-card.node-card.is-offline .card-accent {
  background: var(--text-muted);
}

.node-card.node-card .card-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  min-width: 0;
  margin: 0;
  padding: 0 150px 0 0;
}

.node-card.node-card .card-identity {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  min-width: 0;
  flex: 1;
}

.node-card.node-card .card-symbol {
  width: 44px;
  height: 44px;
  flex: 0 0 44px;
  display: grid;
  place-items: center;
  border-radius: 8px;
  border: 1px solid rgba(59, 130, 246, 0.16);
  background: rgba(59, 130, 246, 0.08);
  color: var(--accent-blue);
  font-size: 20px;
}

.node-card.node-card.is-local .card-symbol {
  border-color: rgba(16, 185, 129, 0.18);
  background: rgba(16, 185, 129, 0.1);
  color: var(--accent-green);
}

.node-card.node-card.is-offline .card-symbol {
  border-color: rgba(148, 163, 184, 0.16);
  background: rgba(148, 163, 184, 0.09);
  color: var(--text-muted);
}

.node-card.node-card .card-copy {
  min-width: 0;
  flex: 1;
  padding: 0;
}

.node-card.node-card .card-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  margin: 0 0 4px;
  flex-wrap: wrap;
}

.node-card.node-card .card-title {
  min-width: 0;
  color: var(--text-primary);
  font-family: inherit;
  font-size: 17px;
  font-weight: 800;
  line-height: 1.2;
  letter-spacing: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.node-card.node-card .card-current-mark {
  display: inline-flex;
  align-items: center;
  height: 22px;
  padding: 0 8px;
  border-radius: 999px;
  background: rgba(16, 185, 129, 0.1);
  color: var(--accent-green);
  font-size: 11px;
  font-weight: 800;
  line-height: 1;
}

.node-card.node-card .card-current-mark::before {
  display: none;
}

.node-card.node-card .card-hostline {
  margin: 0;
  color: var(--text-secondary);
  font-family: 'JetBrains Mono', 'Fira Code', Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.45;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.node-card.node-card .card-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
  margin: 0 0 0 56px;
}

.node-card.node-card .compact-meta-row:first-child {
  position: absolute;
  top: 17px;
  right: 18px;
  z-index: 2;
  display: block;
}

.node-card.node-card .compact-meta-row:first-child .compact-label {
  display: none;
}

.node-card.node-card .compact-meta-row:first-child .node-meta-pills {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.node-card.node-card .meta-chip {
  height: 26px;
  padding: 0 9px;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(148, 163, 184, 0.08);
  color: var(--text-secondary);
  font-size: 11px;
  font-weight: 800;
  line-height: 1;
}

.node-card.node-card .meta-chip.online {
  border-color: rgba(16, 185, 129, 0.2);
  background: rgba(16, 185, 129, 0.1);
  color: var(--accent-green);
}

.node-card.node-card .meta-chip.offline {
  border-color: rgba(148, 163, 184, 0.2);
  background: rgba(148, 163, 184, 0.12);
  color: var(--text-muted);
}

.node-card.node-card .meta-chip.role.leader {
  border-color: rgba(248, 113, 113, 0.2);
  background: rgba(248, 113, 113, 0.1);
  color: var(--accent-red);
}

.node-card.node-card .meta-chip.role.follower {
  border-color: rgba(59, 130, 246, 0.2);
  background: rgba(59, 130, 246, 0.1);
  color: var(--accent-blue);
}

.node-card.node-card .meta-chip.role.candidate {
  border-color: rgba(245, 158, 11, 0.22);
  background: rgba(245, 158, 11, 0.11);
  color: var(--accent-orange);
}

.node-card.node-card .meta-chip .status-dot {
  width: 7px;
  height: 7px;
  box-shadow: none;
}

.node-card.node-card .protocol-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin: 0;
}

.node-card.node-card .protocol-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-width: 0;
  padding: 8px 10px;
  border-radius: 8px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  background: rgba(148, 163, 184, 0.055);
  box-shadow: none;
}

.node-card.node-card .protocol-card.http {
  border-color: rgba(59, 130, 246, 0.16);
}

.node-card.node-card .protocol-card.grpc {
  border-color: rgba(16, 185, 129, 0.16);
}

.node-card.node-card .protocol-name {
  color: var(--text-secondary);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.03em;
}

.node-card.node-card .protocol-port {
  min-width: 0;
  color: var(--text-primary);
  font-family: 'JetBrains Mono', 'Fira Code', Menlo, Consolas, monospace;
  font-size: 13px;
  font-weight: 800;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.node-card.node-card .card-footer {
  display: none;
  justify-content: flex-end;
  padding: 2px 0 0;
  margin: 0 0 0 56px;
  border: 0;
}

.node-card.node-card:has(.card-actions .el-button) .card-footer {
  display: flex;
}

.node-card.node-card .card-footnote {
  display: none;
}

@media (max-width: 1180px) {
  .card-grid.card-grid {
    grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
    justify-content: stretch;
  }
}

@media (max-width: 768px) {
  .card-grid.card-grid {
    grid-template-columns: 1fr;
  }

  .node-card.node-card {
    gap: 12px;
    padding: 16px;
  }

  .node-card.node-card .card-head {
    padding-right: 0;
  }

  .node-card.node-card .compact-meta-row:first-child {
    position: static;
    margin-left: 52px;
  }

  .node-card.node-card .compact-meta-row:first-child .node-meta-pills {
    justify-content: flex-start;
  }

  .node-card.node-card .card-body,
  .node-card.node-card .card-footer {
    margin-left: 52px;
  }
}

@media (max-width: 520px) {
  .node-card.node-card .protocol-grid {
    grid-template-columns: 1fr;
  }
}
</style>

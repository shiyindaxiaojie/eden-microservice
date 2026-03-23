<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Plus, Refresh, Search } from '@element-plus/icons-vue'
import {
  addClusterMember,
  getClusterMembers,
  getClusterStats,
  removeClusterMember,
  type ClusterMember,
  type ClusterStats,
} from '../api/registry'
import { useI18n } from '../utils/i18n'

const { t, locale } = useI18n()
const members = ref<ClusterMember[]>([])
const stats = ref<ClusterStats | null>(null)
const loading = ref(true)
const showAddDialog = ref(false)
const search = ref('')
const addForms = ref([{ protocol: 'http://', host: '', port: '' }])

const filteredMembers = computed(() => {
  const query = search.value.trim().toLowerCase()
  return members.value.filter((member) => {
    if (!query) return true
    return [member.id, member.address, member.http_addr, member.grpc_addr, member.raft_addr, member.quic_addr]
      .filter(Boolean)
      .some((value) => String(value).toLowerCase().includes(query))
  })
})

const clusterHint = computed(() =>
  locale.value === 'zh'
    ? '统一查看节点状态、协议地址和当前集群角色。'
    : 'Inspect node status, protocol endpoints, and the current cluster role in one place.',
)
const nodeSearchPlaceholder = computed(() =>
  locale.value === 'zh' ? '搜索节点 ID 或协议地址' : 'Search node id or endpoint address',
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
          <el-input
            v-model="search"
            :prefix-icon="Search"
            :placeholder="nodeSearchPlaceholder"
            clearable
            class="search-input"
            style="width: 320px;"
          />
        </div>

        <div class="toolbar-group right-align">
          <el-button :icon="Refresh" class="pill-btn" @click="fetchCluster">{{ t.common.refresh }}</el-button>
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
        <div class="table-wrap">
          <el-table :data="filteredMembers" height="100%" style="width: 100%; font-size: 14px;">
            <el-table-column :label="t.cluster.nodeId" min-width="220">
              <template #default="{ row }">
                <div class="node-id-cell">
                  <div class="node-badge">{{ row.id?.slice(0, 2)?.toUpperCase() || 'N' }}</div>
                  <div class="node-main">
                    <div class="node-main-row">
                      <strong class="node-name">{{ row.id }}</strong>
                      <el-tag v-if="row.is_local" size="small" effect="plain" type="success">{{ t.cluster.currentNode }}</el-tag>
                    </div>
                    <span class="node-sub">{{ getRoleName(row.role) }}</span>
                  </div>
                </div>
              </template>
            </el-table-column>

            <el-table-column :label="t.cluster.address" min-width="360">
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

            <el-table-column :label="t.cluster.status" width="120">
              <template #default="{ row }">
                <div class="status-cell">
                  <span class="status-dot" :class="row.status === 'Online' ? 'active' : 'inactive'"></span>
                  <span class="status-text">{{ row.status === 'Online' ? t.cluster.online : t.cluster.offline }}</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column :label="t.cluster.role" width="130">
              <template #default="{ row }">
                <el-tag :type="roleTagType(row.role)" size="small" effect="dark" class="role-tag">{{ getRoleName(row.role) }}</el-tag>
              </template>
            </el-table-column>

            <el-table-column :label="t.common.actions" width="100" fixed="right" align="center">
              <template #default="{ row }">
                <el-button
                  v-if="!row.is_local && row.role !== 'Leader' && (stats?.mode !== 'cp' || stats?.role === 'Leader')"
                  type="danger"
                  link
                  :icon="Delete"
                  @click="handleRemove(row)"
                />
              </template>
            </el-table-column>
          </el-table>
        </div>
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

.toolbar-sep {
  width: 1px;
  height: 20px;
  background: var(--border-color);
  flex-shrink: 0;
  margin: 0 4px;
}

/* ── Content ── */
.svc-content {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
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


</style>




<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Connection, Delete, Plus, Refresh, Search } from '@element-plus/icons-vue'
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
const leaderValue = computed(() => stats.value?.leader_addr || '-')
const modeValue = computed(() => {
  if (!stats.value) return '-'
  if (stats.value.environment === 'standalone') return t.value.settings.singleTitle
  return stats.value.mode === 'cp' ? t.value.settings.cpTitle : t.value.settings.apTitle
})

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
  <div class="page-stack cluster-page">
    <section class="compact-hero">
      <div class="hero-copy-compact">
        <p class="hero-kicker">{{ t.nav.cluster }}</p>
        <h2 class="hero-heading">{{ t.cluster.nodeList }}</h2>
        <p class="hero-desc">{{ clusterHint }}</p>
      </div>
      <div class="cluster-actions">
        <div class="mini-stat-grid cluster-mini-stats">
          <div class="mini-stat">
            <span class="mini-stat-label">{{ t.cluster.nodeCount }}</span>
            <strong class="mini-stat-value">{{ stats?.node_count ?? 0 }}</strong>
          </div>
          <div class="mini-stat">
            <span class="mini-stat-label">{{ t.cluster.role }}</span>
            <strong class="mini-stat-value cluster-stat-text">{{ getRoleName(stats?.role || '-') }}</strong>
          </div>
          <div class="mini-stat">
            <span class="mini-stat-label">{{ t.settings.consistency }}</span>
            <strong class="mini-stat-value cluster-stat-text">{{ modeValue }}</strong>
          </div>
          <div class="mini-stat">
            <span class="mini-stat-label">{{ t.cluster.leaderAddr }}</span>
            <strong class="mini-stat-value cluster-stat-text">{{ leaderValue }}</strong>
          </div>
        </div>
        <div class="cluster-action-row">
          <el-button size="large" :icon="Refresh" @click="fetchCluster">{{ t.common.refresh }}</el-button>
          <el-button v-if="stats?.environment === 'cluster' || stats?.mode === 'cp'" type="primary" size="large" :icon="Plus" @click="showAddDialog = true">
            {{ t.cluster.addNode }}
          </el-button>
        </div>
      </div>
    </section>

    <section class="panel-toolbar cluster-toolbar">
      <el-input v-model="search" :prefix-icon="Search" :placeholder="nodeSearchPlaceholder" clearable size="large" class="cluster-search" />
    </section>

    <section v-loading="loading">
      <div v-if="!loading && filteredMembers.length === 0" class="empty-panel">
        <div class="empty-state">
          <el-icon><Connection /></el-icon>
          <p>{{ emptyNodesLabel }}</p>
        </div>
      </div>

      <div v-else class="glass-table-wrapper cluster-table-wrap">
        <el-table :data="filteredMembers" style="width: 100%">
          <el-table-column :label="t.cluster.nodeId" min-width="220">
            <template #default="{ row }">
              <div class="node-id-cell">
                <div class="node-badge">{{ row.id?.slice(0, 2)?.toUpperCase() || 'N' }}</div>
                <div class="node-main">
                  <div class="node-main-row">
                    <strong>{{ row.id }}</strong>
                    <el-tag v-if="row.is_local" size="small" effect="plain" type="success">{{ t.cluster.currentNode }}</el-tag>
                  </div>
                  <span>{{ getRoleName(row.role) }}</span>
                </div>
              </div>
            </template>
          </el-table-column>

          <el-table-column :label="t.cluster.address" min-width="360">
            <template #default="{ row }">
              <div class="address-tags">
                <el-tag v-if="row.http_addr" size="small" effect="plain" class="addr-tag"><span class="addr-proto">HTTP</span>{{ row.http_addr }}</el-tag>
                <el-tag v-if="row.grpc_addr" size="small" type="success" effect="plain" class="addr-tag"><span class="addr-proto">GRPC</span>{{ row.grpc_addr }}</el-tag>
                <el-tag v-if="row.raft_addr" size="small" type="warning" effect="plain" class="addr-tag"><span class="addr-proto">RAFT</span>{{ row.raft_addr }}</el-tag>
                <el-tag v-if="row.quic_addr" size="small" type="info" effect="plain" class="addr-tag"><span class="addr-proto">QUIC</span>{{ row.quic_addr }}</el-tag>
                <span v-if="!row.http_addr" class="mono-addr">{{ row.address }}</span>
              </div>
            </template>
          </el-table-column>

          <el-table-column :label="t.cluster.status" min-width="120">
            <template #default="{ row }">
              <div class="status-cell">
                <span class="status-dot" :class="row.status === 'Online' ? 'active' : 'inactive'"></span>
                <span>{{ row.status === 'Online' ? t.cluster.online : t.cluster.offline }}</span>
              </div>
            </template>
          </el-table-column>

          <el-table-column :label="t.cluster.role" min-width="130">
            <template #default="{ row }">
              <el-tag :type="roleTagType(row.role)" size="small" effect="dark">{{ getRoleName(row.role) }}</el-tag>
            </template>
          </el-table-column>

          <el-table-column :label="t.common.actions" width="110" fixed="right">
            <template #default="{ row }">
              <el-button
                v-if="!row.is_local && row.role !== 'Leader' && (stats?.mode !== 'cp' || stats?.role === 'Leader')"
                link
                type="danger"
                :icon="Delete"
                @click="handleRemove(row)"
              />
            </template>
          </el-table-column>
        </el-table>
      </div>
    </section>

    <el-dialog v-model="showAddDialog" :title="t.cluster.addNode" width="560px" append-to-body @close="addForms = [{ protocol: 'http://', host: '', port: '' }]">
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
.cluster-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.cluster-mini-stats {
  min-width: 0;
  width: auto;
}

.cluster-stat-text {
  font-size: 18px;
  line-height: 1.25;
}

.cluster-action-row {
  display: flex;
  gap: 8px;
}

.cluster-search {
  flex: 1;
}

.cluster-table-wrap {
  border-radius: 20px;
}

.node-id-cell {
  display: flex;
  align-items: center;
  gap: 14px;
}

.node-badge {
  width: 42px;
  height: 42px;
  display: grid;
  place-items: center;
  border-radius: 14px;
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.16), rgba(6, 182, 212, 0.12));
  color: var(--accent-blue);
  font-weight: 700;
}

.node-main {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.node-main-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.node-main span,
.mono-addr {
  color: var(--text-secondary);
}

.address-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.addr-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border-radius: 999px;
}

.addr-proto {
  font-weight: 700;
  font-size: 10px;
  letter-spacing: 0.08em;
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
  background-color: var(--accent-green);
  box-shadow: 0 0 10px rgba(16, 185, 129, 0.4);
}

.status-dot.inactive {
  background-color: var(--text-muted);
}

.add-node-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 8px 0;
}

.node-input-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px;
  border-radius: 16px;
  border: 1px solid var(--border-color);
  background: var(--bg-card);
}

.protocol-select {
  width: 100px;
}

.host-input {
  flex: 1;
}

.port-input {
  width: 90px;
}

.colon-separator {
  color: var(--text-muted);
  font-weight: 700;
}

.row-actions {
  display: flex;
  gap: 4px;
}

@media (max-width: 980px) {
  .cluster-mini-stats,
  .cluster-search {
    width: 100%;
    min-width: 0;
  }

  .cluster-actions {
    align-items: stretch;
  }
}
</style>




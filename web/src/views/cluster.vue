<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getClusterMembers, getClusterStats, addClusterMember, removeClusterMember, type ClusterMember, type ClusterStats } from '../api/registry'
import { useI18n } from '../utils/i18n'
import { Guide, Plus, Delete } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

const { t } = useI18n()
const members = ref<ClusterMember[]>([])
const stats = ref<ClusterStats | null>(null)
const loading = ref(true)
const showAddDialog = ref(false)
const addForms = ref([{ protocol: 'http://', host: '', port: '' }])

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
  } catch (e) {
    console.error('fetch cluster failed', e)
  } finally {
    loading.value = false
  }
}

async function handleAdd() {
  const addresses = addForms.value
    .filter(f => f.host && f.port)
    .map(f => `${f.protocol}${f.host}:${f.port}`)

  if (addresses.length === 0) {
    ElMessage.warning('Please enter valid address information')
    return
  }
  try {
    await addClusterMember({ addresses })
    ElMessage.success(t.value.common.success)
    showAddDialog.value = false
    addForms.value = [{ protocol: 'http://', host: '', port: '' }]
    fetchCluster()
  } catch (e: any) {
    const data = e.response?.data
    if (data?.error === 'not leader' && data?.leader) {
      const tip = t.value.settings.notLeaderTip.replace('{addr}', data.leader)
      ElMessage.warning(tip)
    } else {
      ElMessage.error(data?.error || 'Add failed')
    }
  }
}

async function handleRemove(row: ClusterMember) {
  try {
    await ElMessageBox.confirm(
      `确定要移除节点 ${row.id} (${row.address}) 吗？`,
      t.value.common.warning,
      { type: 'warning' }
    )
    await removeClusterMember(row.address, row.id)
    ElMessage.success(t.value.common.success)
    fetchCluster()
  } catch {}
}

function roleTagType(role: string) {
  switch (role) {
    case 'Leader':    return 'success'
    case 'Follower':  return 'info'
    case 'Candidate': return 'warning'
    case 'Local':     return 'primary'
    case 'Standalone': return 'primary'
    case 'Peer':      return 'info'
    default:          return 'info'
  }
}

function getRoleName(role: string) {
  const r = role.toLowerCase()
  if (r.includes('leader'))    return t.value.cluster.roles.leader
  if (r.includes('follower'))  return t.value.cluster.roles.follower
  if (r.includes('candidate')) return t.value.cluster.roles.candidate
  if (r.includes('local'))     return t.value.cluster.roles.local
  if (r.includes('standalone')) return t.value.cluster.roles.standalone
  if (r.includes('peer'))      return t.value.cluster.roles.peer
  return role
}

onMounted(fetchCluster)
</script>

<template>
  <div class="cluster-page">
    <!-- Cluster Summary Header -->
    <div class="cluster-hero glass-card">
      <div class="hero-main">
        <div class="hero-icon">
          <el-icon><Guide /></el-icon>
        </div>
        <div class="hero-content">
          <div class="hero-title">
            {{ t.cluster.nodeList }}
            <el-tag 
              v-if="stats" 
              :class="stats.environment === 'standalone' ? '' : (stats.mode === 'cp' ? 'tag-cp' : 'tag-ap')" 
              :type="stats.environment === 'standalone' ? 'info' : ''"
              effect="dark" 
              size="small"
              class="mode-tag"
            >
              {{ stats.environment === 'standalone' ? t.settings.singleTitle : (stats.mode === 'cp' ? t.settings.cpTitle : t.settings.apTitle) }}
            </el-tag>
          </div>
          <div class="hero-subtitle">
            {{ stats?.node_count || 0 }} {{ t.cluster.nodeCount }} — 
            <template v-if="stats?.mode === 'cp'">
              {{ stats?.leader_addr ? t.cluster.leaderAddr + ': ' + stats.leader_addr : t.dashboard.noEvents }}
            </template>
            <template v-else>
              {{ t.cluster.currentNode }}: {{ stats?.leader_addr || '-' }}
            </template>
          </div>
        </div>
      </div>
      
      <div class="hero-stats">
        <div class="hero-stat-item">
          <div class="stat-label">{{ t.cluster.role }}</div>
          <div class="stat-value">
            <el-tag :type="roleTagType(stats?.role || '')" size="small" effect="plain">
              {{ getRoleName(stats?.role || '') }}
            </el-tag>
          </div>
        </div>
        <div class="hero-stat-item">
          <div class="stat-label">{{ t.dashboard.healthRate }}</div>
          <div class="stat-value health">
            {{ stats?.health_rate ? stats.health_rate.toFixed(0) + '%' : '-' }}
          </div>
        </div>
        
        <!-- Add Node Action moved to far right -->
        <div class="hero-action-item" v-if="stats?.environment === 'cluster' || stats?.mode === 'cp'">
          <el-button type="primary" :icon="Plus" @click="showAddDialog = true" class="add-node-btn">
            {{ t.cluster.addNode }}
          </el-button>
        </div>
      </div>
    </div>

    <!-- Table Section -->
    <div class="glass-table-wrapper">
      <el-table :data="members" v-loading="loading" style="width: 100%">
        <el-table-column :label="t.cluster.nodeId" min-width="200">
          <template #default="{ row }">
            <div class="node-id-cell">
              <span class="node-id-text">{{ row.id }}</span>
              <el-tag 
                v-if="row.is_local" 
                size="small" 
                type="primary" 
                effect="plain" 
                color="rgba(64, 158, 255, 0.1)"
                class="current-tag"
              >
                {{ t.cluster.currentNode }}
              </el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="t.cluster.address" min-width="280">
          <template #default="{ row }">
            <div class="address-tags">
              <el-tag v-if="row.http_addr" size="small" type="primary" class="addr-tag" effect="light">
                <span class="addr-proto">HTTP</span> <span class="mono-addr">{{ row.http_addr }}</span>
              </el-tag>
              <el-tag v-if="row.grpc_addr" size="small" type="success" class="addr-tag" effect="light">
                <span class="addr-proto">GRPC</span> <span class="mono-addr">{{ row.grpc_addr }}</span>
              </el-tag>
              <el-tag v-if="row.raft_addr" size="small" type="warning" class="addr-tag" effect="light">
                <span class="addr-proto">RAFT</span> <span class="mono-addr">{{ row.raft_addr }}</span>
              </el-tag>
              <el-tag v-if="row.quic_addr" size="small" type="info" class="addr-tag" effect="light">
                <span class="addr-proto">QUIC</span> <span class="mono-addr">{{ row.quic_addr }}</span>
              </el-tag>
              <!-- Fallback to ordinary address for older APIs or standalone -->
              <span v-if="!row.http_addr" class="mono-addr">{{ row.address }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="t.cluster.role" min-width="120">
          <template #default="{ row }">
            <el-tag :type="roleTagType(row.role)" size="small" effect="dark">{{ getRoleName(row.role) }}</el-tag>
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
        <el-table-column :label="t.common.actions" width="100" fixed="right">
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

    <!-- Add Member Dialog -->
    <el-dialog v-model="showAddDialog" :title="t.cluster.addNode" width="550px" class="premium-dialog" @close="addForms = [{ protocol: 'http://', host: '', port: '' }]">
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
            <el-button 
              type="primary" 
              circle 
              :icon="Plus" 
              size="small" 
              plain
              v-if="index === addForms.length - 1"
              @click="addNodeRow"
            />
            <el-button 
              type="danger" 
              circle 
              :icon="Delete" 
              size="small" 
              plain
              v-if="addForms.length > 1"
              @click="removeNodeRow(index)"
            />
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
.cluster-hero {
  margin-bottom: 32px;
  padding: 32px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-radius: 20px;
  background: linear-gradient(135deg, var(--bg-card), rgba(59, 130, 246, 0.05));
}

.hero-main {
  display: flex;
  align-items: center;
  gap: 24px;
}

.hero-icon {
  width: 64px;
  height: 64px;
  background: var(--bg-glass);
  border: 1px solid var(--border-color);
  border-radius: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 32px;
  color: var(--accent-blue);
  box-shadow: 0 8px 16px rgba(0, 0, 0, 0.1);
}

.hero-title {
  font-size: 24px;
  font-weight: 700;
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.mode-tag {
  font-size: 11px;
  height: 20px;
  border-radius: 6px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.hero-subtitle {
  font-size: 14px;
  color: var(--text-secondary);
  font-weight: 500;
}


.hero-stats {
  display: flex;
  align-items: center;
  gap: 48px;
  padding-right: 20px;
}

.hero-action-item {
  margin-left: 24px;
}

.add-node-btn {
  height: 48px;
  padding: 0 24px;
  font-weight: 600;
  border-radius: 12px;
  box-shadow: 0 8px 16px rgba(var(--accent-blue-rgb), 0.2);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.add-node-btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 12px 24px rgba(var(--accent-blue-rgb), 0.3);
}

.hero-stat-item .stat-label {
  font-size: 12px;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 1px;
  margin-bottom: 8px;
  font-weight: 600;
}

.hero-stat-item .stat-value {
  font-size: 20px;
  font-weight: 700;
}

.hero-stat-item .stat-value.health {
  color: var(--accent-green);
}


.glass-card {
  backdrop-filter: blur(20px);
  border: 1px solid var(--border-color);
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.1);
}

.status-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  position: relative;
}

.status-dot.active {
  background-color: var(--accent-green);
  box-shadow: 0 0 8px var(--accent-green);
}

.status-dot.inactive {
  background-color: var(--text-muted);
}

.status-dot.active::after {
  content: '';
  position: absolute;
  top: -2px;
  left: -2px;
  right: -2px;
  bottom: -2px;
  border-radius: 50%;
  border: 1px solid var(--accent-green);
  opacity: 0.5;
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0% { transform: scale(1); opacity: 0.5; }
  100% { transform: scale(1.8); opacity: 0; }
}

.mono-addr {
  font-family: 'Cascadia Code', 'Consolas', monospace;
  font-size: 13px;
  color: var(--text-secondary);
}

.node-id-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.node-id-text {
  font-weight: 500;
}

.current-tag {
  border-radius: 4px;
  border-style: solid;
  font-size: 11px;
  height: 20px;
  line-height: 18px;
  padding: 0 6px;
}

.tag-cp {
  background-color: #f97316 !important;
  border-color: #f97316 !important;
  color: white !important;
}

.tag-ap {
  background-color: #10b981 !important;
  border-color: #10b981 !important;
  color: white !important;
}

/* Address Tags */
.address-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.addr-tag {
  display: inline-flex;
  align-items: center;
  justify-content: flex-start;
  width: fit-content;
  padding: 0 8px;
  border-radius: 6px;
}
.addr-proto {
  font-weight: 700;
  font-size: 10px;
  margin-right: 6px;
  opacity: 0.8;
}

/* Add Node Form */
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
  background: var(--bg-card);
  padding: 8px;
  border-radius: 12px;
  border: 1px solid var(--border-color);
  transition: all 0.2s ease;
}
.node-input-row:hover {
  border-color: var(--accent-blue);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
}
.protocol-select {
  width: 100px;
}
.host-input {
  flex: 1;
}
.colon-separator {
  font-weight: bold;
  color: var(--text-muted);
}
.port-input {
  width: 80px;
}
.row-actions {
  display: flex;
  gap: 4px;
  min-width: 64px;
  justify-content: center;
}
</style>

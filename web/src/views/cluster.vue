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
const addForm = ref({ node_id: '', address: '' })

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
  if (!addForm.value.address) {
    ElMessage.warning('Address is required')
    return
  }
  try {
    await addClusterMember({
      node_id: addForm.value.node_id,
      address: addForm.value.address
    })
    ElMessage.success(t.value.common.success)
    showAddDialog.value = false
    addForm.value = { node_id: '', address: '' }
    fetchCluster()
  } catch (e: any) {
    ElMessage.error(e.response?.data?.error || 'Add failed')
  }
}

async function handleRemove(row: ClusterMember) {
  try {
    await ElMessageBox.confirm(
      `Confirm removal of node ${row.id} (${row.address})?`,
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
            {{ stats?.node_count || 0 }} {{ t.cluster.nodeCount.toLowerCase() }} — 
            {{ stats?.leader_addr ? t.cluster.leaderAddr + ': ' + stats.leader_addr : t.dashboard.noEvents }}
          </div>
        </div>
      </div>
      
      <div class="hero-stats">
        <div class="hero-stat-item">
          <div class="stat-label">{{ t.cluster.role }}</div>
          <div class="stat-value">
            <el-tag :type="stats?.is_leader ? 'success' : 'info'" size="small" effect="plain">
              {{ stats?.is_leader ? t.cluster.roles.leader : t.cluster.roles.follower }}
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
        <el-table-column :label="t.cluster.nodeId" prop="id" min-width="200" />
        <el-table-column :label="t.cluster.raftAddr" prop="address" min-width="200">
          <template #default="{ row }">
            <span class="mono-addr">{{ row.address }}</span>
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
        <el-table-column :label="t.common.actions" width="100" align="right">
          <template #default="{ row }">
            <el-button 
              v-if="row.role !== 'Local' && row.role !== 'Standalone' && row.role !== 'Leader'" 
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
    <el-dialog v-model="showAddDialog" :title="t.cluster.nodeList" width="450px" class="premium-dialog">
      <el-form label-position="top">
        <el-form-item v-if="stats?.mode === 'cp'" :label="t.cluster.nodeId">
          <el-input v-model="addForm.node_id" placeholder="node-2" />
        </el-form-item>
        <el-form-item :label="t.common.address">
          <el-input v-model="addForm.address" placeholder="http://127.0.0.1:8501" />
        </el-form-item>
      </el-form>
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
</style>

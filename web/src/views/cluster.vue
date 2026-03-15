<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getClusterMembers, getClusterStats, type ClusterMember, type ClusterStats } from '../api/registry'
import { useI18n } from '../utils/i18n'
import { Guide } from '@element-plus/icons-vue'

const { t } = useI18n()
const members = ref<ClusterMember[]>([])
const stats = ref<ClusterStats | null>(null)
const loading = ref(true)

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
              :type="stats.mode === 'cp' ? 'primary' : 'success'" 
              effect="dark" 
              size="small"
              class="mode-tag"
            >
              {{ stats.mode === 'cp' ? t.settings.cpTitle : t.settings.apTitle }}
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
      </div>
    </div>

    <!-- Table Section -->
    <div class="glass-table-wrapper">
      <el-table :data="members" v-loading="loading" style="width: 100%">
        <el-table-column :label="t.cluster.nodeId" prop="id" min-width="200" />
        <el-table-column :label="t.cluster.raftAddr" prop="address" min-width="200">
          <template #default="{ row }">
            <span class="ip-address">{{ row.address }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t.cluster.role" min-width="120">
          <template #default="{ row }">
            <el-tag :type="roleTagType(row.role)" size="small" effect="dark">{{ getRoleName(row.role) }}</el-tag>
          </template>
        </el-table-column>
      </el-table>
    </div>
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
  gap: 48px;
  padding-right: 20px;
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
</style>

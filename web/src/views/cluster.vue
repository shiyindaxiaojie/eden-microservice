<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getClusterMembers, getClusterStats, type ClusterMember, type ClusterStats } from '../api/registry'

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
    default:          return 'info'
  }
}

onMounted(fetchCluster)
</script>

<template>
  <div>
    <!-- Cluster stats cards -->
    <div class="stat-cards" style="margin-bottom: 28px;">
      <div class="stat-card">
        <div class="stat-icon blue"><el-icon><Connection /></el-icon></div>
        <div class="stat-value">{{ members.length }}</div>
        <div class="stat-label">节点数</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon green"><el-icon><SuccessFilled /></el-icon></div>
        <div class="stat-value">{{ stats?.is_leader ? '是' : '否' }}</div>
        <div class="stat-label">当前为 Leader</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon purple"><el-icon><Position /></el-icon></div>
        <div class="stat-value" style="font-size: 16px;">{{ stats?.leader_addr || '-' }}</div>
        <div class="stat-label">Leader 地址</div>
      </div>
    </div>

    <h3 class="section-title">节点列表</h3>
    <div class="glass-table-wrapper">
      <el-table :data="members" v-loading="loading" style="width: 100%">
        <el-table-column label="节点 ID" prop="id" min-width="200" />
        <el-table-column label="Raft 地址" prop="address" min-width="200" />
        <el-table-column label="角色" min-width="120">
          <template #default="{ row }">
            <el-tag :type="roleTagType(row.role)" size="default" effect="dark">{{ row.role }}</el-tag>
          </template>
        </el-table-column>
      </el-table>
    </div>
  </div>
</template>

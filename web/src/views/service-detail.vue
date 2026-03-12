<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessageBox, ElMessage } from 'element-plus'
import { getServiceInstances, deregisterInstance, type Instance } from '../api/registry'

const route = useRoute()
const router = useRouter()
const serviceName = ref(route.params.name as string)
const instances = ref<Instance[]>([])
const loading = ref(true)

async function fetchInstances() {
  loading.value = true
  try {
    const res = await getServiceInstances(serviceName.value)
    instances.value = res.data || []
  } catch (e) {
    console.error('fetch instances failed', e)
  } finally {
    loading.value = false
  }
}

function formatTime(ts: string) {
  if (!ts) return '-'
  return new Date(ts).toLocaleString('zh-CN')
}

function formatMeta(meta: Record<string, string>) {
  if (!meta || Object.keys(meta).length === 0) return '-'
  return JSON.stringify(meta)
}

async function handleDeregister(inst: Instance) {
  try {
    await ElMessageBox.confirm(
      `确定要注销实例 ${inst.host}:${inst.port}？此操作不可撤销。`,
      '注销确认',
      { confirmButtonText: '确定', cancelButtonText: '取消', type: 'warning' }
    )
    await deregisterInstance(inst.service_name, inst.id)
    ElMessage.success('实例已注销')
    fetchInstances()
  } catch {
    // cancelled
  }
}

onMounted(fetchInstances)
</script>

<template>
  <div>
    <!-- Back -->
    <div style="margin-bottom: 20px;">
      <el-button text @click="router.push('/services')" style="color: var(--text-secondary)">
        <el-icon><ArrowLeft /></el-icon> 返回服务列表
      </el-button>
      <el-button text @click="fetchInstances" style="color: var(--text-secondary); margin-left: 8px;">
        <el-icon><Refresh /></el-icon> 刷新
      </el-button>
    </div>

    <h3 class="section-title" style="margin-bottom: 16px;">
      服务: <span style="color: var(--accent-blue);">{{ serviceName }}</span>
      <el-tag size="small" effect="dark" style="margin-left: 12px;">{{ instances.length }} 个实例</el-tag>
    </h3>

    <div class="glass-table-wrapper">
      <el-table :data="instances" v-loading="loading" height="100%" style="width: 100%">
        <el-table-column label="实例 ID" prop="id" min-width="160" />
        <el-table-column label="地址" min-width="160">
          <template #default="{ row }">{{ row.host }}:{{ row.port }}</template>
        </el-table-column>
        <el-table-column label="状态" min-width="100">
          <template #default="{ row }">
            <el-tag v-if="row.status === 'passing'" type="success" size="small" effect="dark">健康</el-tag>
            <el-tag v-else type="danger" size="small" effect="dark">不健康</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="权重" prop="weight" min-width="80" />
        <el-table-column label="元数据" min-width="200">
          <template #default="{ row }">
            <span style="font-size: 12px; color: var(--text-muted);">{{ formatMeta(row.metadata) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="最后心跳" min-width="180">
          <template #default="{ row }">{{ formatTime(row.last_heartbeat) }}</template>
        </el-table-column>
        <el-table-column label="注册时间" min-width="180">
          <template #default="{ row }">{{ formatTime(row.registered_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <el-button type="danger" size="small" text @click="handleDeregister(row)">注销</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
  </div>
</template>

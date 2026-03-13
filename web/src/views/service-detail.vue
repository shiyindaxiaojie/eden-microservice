<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessageBox, ElMessage } from 'element-plus'
import { getServiceInstances, deregisterInstance, type Instance } from '../api/registry'
import { useI18n } from '../utils/i18n'

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const serviceName = ref(route.params.name as string)
const instances = ref<Instance[]>([])
const loading = ref(true)
const userRole = ref(localStorage.getItem('user_role') || 'admin')
const canManage = computed(() => userRole.value === 'admin' || userRole.value === 'developer')

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
  return new Date(ts).toLocaleString(locale.value === 'zh' ? 'zh-CN' : 'en-US')
}

function formatMeta(meta: Record<string, string>) {
  if (!meta || Object.keys(meta).length === 0) return '-'
  return JSON.stringify(meta)
}

async function handleDeregister(inst: Instance) {
  try {
    await ElMessageBox.confirm(
      t.value.detail.deregisterConfirm.replace('{addr}', `${inst.host}:${inst.port}`),
      t.value.detail.deregisterTitle,
      { 
        confirmButtonText: t.value.common.confirm, 
        cancelButtonText: t.value.common.cancel, 
        type: 'warning' 
      }
    )
    await deregisterInstance(inst.service_name, inst.id)
    ElMessage.success(t.value.detail.deregisterSuccess)
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
        <el-icon><ArrowLeft /></el-icon> {{ t.detail.backToServices }}
      </el-button>
      <el-button text @click="fetchInstances" style="color: var(--text-secondary); margin-left: 8px;">
        <el-icon><Refresh /></el-icon> {{ t.common.refresh }}
      </el-button>
    </div>

    <h3 class="section-title" style="margin-bottom: 16px;">
      {{ t.services.service }}: <span style="color: var(--accent-blue);">{{ serviceName }}</span>
      <el-tag size="small" effect="dark" style="margin-left: 12px;">{{ instances.length }} {{ t.detail.instanceCount }}</el-tag>
    </h3>

    <div class="glass-table-wrapper">
      <el-table :data="instances" v-loading="loading" height="100%" style="width: 100%">
        <el-table-column :label="t.detail.instanceId" prop="id" min-width="160" />
        <el-table-column :label="t.common.address" min-width="160">
          <template #default="{ row }">{{ row.host }}:{{ row.port }}</template>
        </el-table-column>
        <el-table-column :label="t.detail.healthStatus" min-width="100">
          <template #default="{ row }">
            <el-tag v-if="row.status === 'passing'" type="success" size="small" effect="dark">{{ t.services.healthy }}</el-tag>
            <el-tag v-else type="danger" size="small" effect="dark">{{ t.common.warning }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t.common.weight" prop="weight" min-width="80" />
        <el-table-column :label="t.common.metadata" min-width="200">
          <template #default="{ row }">
            <span style="font-size: 12px; color: var(--text-muted);">{{ formatMeta(row.metadata) }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t.detail.lastHeartbeat" min-width="180">
          <template #default="{ row }">{{ formatTime(row.last_heartbeat) }}</template>
        </el-table-column>
        <el-table-column :label="t.detail.registeredAt" min-width="180">
          <template #default="{ row }">{{ formatTime(row.registered_at) }}</template>
        </el-table-column>
        <el-table-column v-if="canManage" :label="t.common.actions" width="100" fixed="right">
          <template #default="{ row }">
            <el-button type="danger" size="small" text @click="handleDeregister(row)">{{ t.detail.deregister }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
  </div>
</template>

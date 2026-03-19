<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getNamespaces, createNamespace, updateNamespace, deleteNamespace } from '../api/registry'
import type { Namespace } from '../api/registry'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Edit, Delete } from '@element-plus/icons-vue'
import { useI18n } from '../utils/i18n'
import dayjs from 'dayjs'

const { t } = useI18n()
const loading = ref(false)
const namespaces = ref<Namespace[]>([])

const dialogVisible = ref(false)
const isEdit = ref(false)
const form = ref<Partial<Namespace>>({
  name: '',
  description: ''
})

const fetchNamespaces = async () => {
  loading.value = true
  try {
    const res = await getNamespaces()
    namespaces.value = res.data || []
  } catch (err: any) {
    ElMessage.error(err.response?.data?.error || err.message)
  } finally {
    loading.value = false
  }
}

const handleAdd = () => {
  isEdit.value = false
  form.value = { name: '', description: '' }
  dialogVisible.value = true
}

const handleEdit = (row: Namespace) => {
  if (row.name === 'default') {
    ElMessage.warning('Cannot edit default namespace')
    return
  }
  isEdit.value = true
  form.value = { ...row }
  dialogVisible.value = true
}

const handleSubmit = async () => {
  if (!form.value.name) return
  
  try {
    if (isEdit.value) {
      await updateNamespace(form.value)
      ElMessage.success(t.value.common.success)
    } else {
      await createNamespace(form.value)
      ElMessage.success(t.value.common.success)
    }
    dialogVisible.value = false
    fetchNamespaces()
  } catch (err: any) {
    ElMessage.error(err.response?.data?.error || err.message)
  }
}

const handleDelete = async (row: Namespace) => {
  if (row.name === 'default') {
    ElMessage.warning('Cannot delete default namespace')
    return
  }
  
  try {
    await ElMessageBox.confirm(`Are you sure to delete namespace "${row.name}"?`, t.value.common.warning, {
      confirmButtonText: t.value.common.confirm,
      cancelButtonText: t.value.common.cancel,
      type: 'warning',
    })
    
    await deleteNamespace(row.name)
    ElMessage.success(t.value.common.success)
    fetchNamespaces()
  } catch (err: any) {
    if (err !== 'cancel') {
      ElMessage.error(err.response?.data?.error || err.message)
    }
  }
}

const formatDate = (dateStr: string) => {
  if (!dateStr) return '-'
  return dayjs(dateStr).format('YYYY-MM-DD HH:mm:ss')
}

onMounted(() => {
  fetchNamespaces()
})
</script>

<template>
  <div class="namespaces-container namespace-page">
    <div class="panel-header">
      <div class="header-content">
        <h2>{{ t.nav.namespaces }}</h2>
      </div>
      <div class="header-actions">
        <el-button @click="fetchNamespaces" :icon="Plus" type="primary" plain style="display: none;"></el-button>
        <el-button @click="handleAdd" type="primary" :icon="Plus">{{ t.common.actions }}</el-button>
      </div>
    </div>
    
    <div class="panel-content">
      <el-table :data="namespaces" v-loading="loading" class="custom-table">
        <el-table-column prop="name" label="Name" min-width="150" />
        <el-table-column prop="description" label="Description" min-width="250" />
        <el-table-column prop="created_at" label="Created At" min-width="180">
          <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="Actions" width="150" align="right">
          <template #default="{ row }">
            <el-button-group>
              <el-button 
                size="small" 
                :icon="Edit" 
                @click="handleEdit(row)" 
                :disabled="row.name === 'default'"
              />
              <el-button 
                size="small" 
                type="danger" 
                :icon="Delete" 
                @click="handleDelete(row)"
                :disabled="row.name === 'default'"
              />
            </el-button-group>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- Namespace Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? 'Edit Namespace' : 'Create Namespace'"
      width="500px"
      append-to-body
      class="glass-dialog"
    >
      <el-form label-position="top">
        <el-form-item label="Name" required>
          <el-input 
            v-model="form.name" 
            placeholder="e.g. dev, prod" 
            :disabled="isEdit"
          />
        </el-form-item>
        <el-form-item label="Description">
          <el-input 
            v-model="form.description" 
            type="textarea" 
            :rows="3" 
            placeholder="Namespace description"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="dialogVisible = false">{{ t.common.cancel }}</el-button>
          <el-button type="primary" @click="handleSubmit" :disabled="!form.name">
            {{ t.common.confirm }}
          </el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.namespace-page {
  animation: fadeIn 0.3s ease-in-out;
}
</style>

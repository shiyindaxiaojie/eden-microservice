<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessageBox, ElMessage } from 'element-plus'
import api from '../api/index'
import { useI18n } from '../utils/i18n'
import { sha256 } from '../utils/crypto'

const { t } = useI18n()
const users = ref<any[]>([])
const loading = ref(false)
const showDialog = ref(false)
const isEdit = ref(false)
const form = ref({ username: '', password: '', role: 'viewer', nickname: '', remark: '' })

function handleShowAdd() {
  isEdit.value = false
  form.value = { username: '', password: '', role: 'viewer', nickname: '', remark: '' }
  showDialog.value = true
}

function handleShowEdit(row: any) {
  isEdit.value = true
  form.value = { ...row, password: '' } // Don't show existing password hash
  showDialog.value = true
}

async function fetchUsers() {
  loading.value = true
  try {
    const res = await api.get('/v1/rbac/users')
    users.value = res.data || []
  } catch (e) {
    console.error('fetch users failed', e)
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  if (!form.value.username) {
    ElMessage.warning('Username is required')
    return
  }
  
  try {
    const payload = { ...form.value }
    // Only hash password if it's provided (new user or changing password)
    if (payload.password) {
      payload.password = await sha256(payload.password)
    } else if (!isEdit.value) {
      ElMessage.warning('Password is required for new user')
      return
    }
    
    await api.post('/v1/rbac/user', payload)
    ElMessage.success(t.value.common.success)
    showDialog.value = false
    fetchUsers()
  } catch (e: any) {
    ElMessage.error(e.response?.data?.error || 'Failed')
  }
}

async function deleteUser(username: string) {
  try {
    await ElMessageBox.confirm(
      t.value.rbac.deleteConfirm.replace('{name}', username),
      t.value.common.warning,
      { type: 'warning' }
    )
    await api.post(`/v1/rbac/user/delete?username=${username}`)
    ElMessage.success(t.value.common.success)
    fetchUsers()
  } catch {}
}

onMounted(fetchUsers)
</script>

<template>
  <div>
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px;">
      <h3 class="section-title" style="margin: 0">{{ t.rbac.title }}</h3>
      <el-button type="primary" @click="handleShowAdd">{{ t.rbac.addUser }}</el-button>
    </div>

    <div class="glass-table-wrapper">
      <el-table :data="users" v-loading="loading" style="width: 100%">
        <el-table-column :label="t.rbac.username" prop="username" min-width="120" />
        <el-table-column :label="t.rbac.nickname" prop="nickname" min-width="120" />
        <el-table-column :label="t.rbac.role" min-width="100">
          <template #default="{ row }">
            <el-tag :type="row.role === 'admin' ? 'danger' : (row.role === 'developer' ? 'primary' : 'info')" size="small" effect="dark">
              {{ row.role === 'admin' ? t.rbac.admin : (row.role === 'developer' ? t.rbac.developer : t.rbac.viewer) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t.rbac.remark" prop="remark" min-width="150" />
        <el-table-column :label="t.rbac.actions" width="150" align="right">
          <template #default="{ row }">
            <el-button type="primary" link @click="handleShowEdit(row)">{{ t.common.edit }}</el-button>
            <el-button v-if="!row.is_builtin" type="danger" link @click="deleteUser(row.username)">{{ t.common.delete }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="showDialog" :title="isEdit ? t.common.edit : t.rbac.addUser" width="400px">
      <el-form label-position="top">
        <el-form-item :label="t.rbac.username">
          <el-input v-model="form.username" :disabled="isEdit" />
        </el-form-item>
        <el-form-item :label="t.rbac.password">
          <el-input v-model="form.password" type="password" show-password />
        </el-form-item>
        <el-form-item :label="t.rbac.nickname">
          <el-input v-model="form.nickname" />
        </el-form-item>
        <el-form-item :label="t.rbac.remark">
          <el-input v-model="form.remark" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item :label="t.rbac.role">
          <el-select v-model="form.role" style="width: 100%">
            <el-option value="admin" :label="t.rbac.admin" />
            <el-option value="developer" :label="t.rbac.developer" />
            <el-option value="viewer" :label="t.rbac.viewer" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showDialog = false">{{ t.common.cancel }}</el-button>
        <el-button type="primary" @click="handleSave">{{ t.common.confirm }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

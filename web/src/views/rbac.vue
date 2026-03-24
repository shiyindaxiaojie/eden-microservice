<template>
  <div class="svc-shell">
    <div class="page-title">{{ text('访问控制', 'Access Control') }}</div>
    
    <div class="svc-content">
      <div class="query-area">
        <el-form :inline="true" :model="queryForm" class="query-form">
          <el-form-item :label="text('用户名', 'Username')" class="query-item">
            <el-input v-model="queryForm.username" :placeholder="text('请输入用户名', 'Input username')" clearable @keyup.enter="handleQuery" />
          </el-form-item>
          <el-form-item :label="text('角色', 'Role')" class="query-item">
            <el-select v-model="queryForm.role" :placeholder="text('请选择角色', 'Select role')" clearable style="width: 140px;">
              <el-option label="管理员" value="admin" />
              <el-option label="开发者" value="developer" />
              <el-option label="访客" value="guest" />
            </el-select>
          </el-form-item>
          <el-form-item class="query-actions">
            <el-button type="primary" class="query-btn" @click="handleQuery">{{ text('查询', 'Search') }}</el-button>
            <el-button class="reset-btn" @click="resetQuery">{{ text('重置', 'Reset') }}</el-button>
            <el-button type="success" class="add-btn" @click="handleAdd">{{ text('添加用户', 'Add User') }}</el-button>
          </el-form-item>
        </el-form>
      </div>

      <div class="table-area">
        <el-table :data="tableData" style="width: 100%" height="100%" v-loading="loading" class="custom-table">
          <el-table-column prop="username" :label="text('用户名', 'Username')" min-width="150" />
          <el-table-column prop="nickname" :label="text('昵称', 'Nickname')" min-width="150" />
          <el-table-column :label="text('角色', 'Role')" width="120">
            <template #default="scope">
              <el-tag :type="getRoleTag(scope.row.role)" size="small" effect="plain">{{ scope.row.role }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="createdAt" :label="text('注册时间', 'Registered At')" width="180" />
          <el-table-column :label="text('操作', 'Actions')" width="180" fixed="right">
            <template #default="scope">
              <el-button link type="primary" @click="handleEdit(scope.row)">{{ text('权限', 'Perms') }}</el-button>
              <el-button link type="danger" @click="handleDelete(scope.row)">{{ text('删除', 'Delete') }}</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>

    <!-- Edit Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="text('用户权限设置', 'Permissions')"
      width="500px"
      class="glass-dialog"
    >
      <el-form :model="form" ref="formRef" label-position="top">
        <el-form-item :label="text('用户名', 'Username')">
          <el-input v-model="form.username" disabled />
        </el-form-item>
        <el-form-item :label="text('角色', 'Role')" prop="role">
          <el-select v-model="form.role" style="width: 100%">
            <el-option label="管理员" value="admin" />
            <el-option label="开发者" value="developer" />
            <el-option label="访客" value="guest" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="dialogVisible = false">{{ text('取消', 'Cancel') }}</el-button>
          <el-button type="primary" @click="submitForm" :loading="submitting">{{ text('确定', 'Confirm') }}</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from '../utils/i18n'

const { locale } = useI18n()
const text = (zh, en) => (locale.value === 'zh' ? zh : en)

const loading = ref(false)
const tableData = ref([])
const queryForm = ref({
  username: '',
  role: ''
})

const dialogVisible = ref(false)
const submitting = ref(false)
const formRef = ref(null)
const form = ref({
  username: '',
  role: ''
})

const getRoleTag = (role) => {
  switch (role) {
    case 'admin': return 'danger'
    case 'developer': return 'primary'
    default: return 'info'
  }
}

const fetchData = async () => {
  loading.value = true
  try {
    tableData.value = [
      { username: 'admin', nickname: '管理员', role: 'admin', createdAt: '2024-03-24 09:00:00' },
      { username: 'sion1', nickname: '开发者', role: 'developer', createdAt: '2024-03-24 10:30:00' }
    ]
  } finally {
    loading.value = false
  }
}

const handleQuery = () => fetchData()
const resetQuery = () => {
  queryForm.value = { username: '', role: '' }
  handleQuery()
}

const handleAdd = () => {
  ElMessage.info('研发环境直接在 settings 注册即可')
}

const handleEdit = (row) => {
  form.value = { ...row }
  dialogVisible.value = true
}

const handleDelete = (row) => {
  ElMessageBox.confirm(`确定要删除用户 "${row.username}" 吗?`, '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    ElMessage.success('删除成功')
  })
}

const submitForm = async () => {
  submitting.value = true
  setTimeout(() => {
    submitting.value = false
    dialogVisible.value = false
    ElMessage.success('权限更新成功')
  }, 500)
}

onMounted(() => fetchData())
</script>

<style scoped>
.svc-shell {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--bg-primary);
}

.page-title {
  padding: 16px 24px;
  font-size: 20px;
  font-weight: 700;
  color: var(--text-primary);
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border-color);
}

.svc-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 24px;
  overflow: hidden;
}

.query-area {
  padding: 24px 32px;
  background: #f5f7fa; 
  border-radius: 8px;
  margin-bottom: 24px;
  border: 1px solid #e4e7ed;
}

.query-form :deep(.el-form-item__label) {
  font-weight: 700 !important;
  color: #303133;
  font-size: 14px;
}

.table-area {
  flex: 1;
  min-height: 0;
  background: #ffffff;
  border-radius: 4px;
  overflow: hidden;
}

:deep(.el-table) {
  --el-table-header-bg-color: #fafafa;
  border: none;
}

.query-btn {
  background: #409eff;
  border: none;
  font-weight: 600;
}

.reset-btn {
  font-weight: 600;
}

.add-btn {
  background: #67c23a;
  border: none;
  font-weight: 600;
}
</style>

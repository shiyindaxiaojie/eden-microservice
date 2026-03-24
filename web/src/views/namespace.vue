<template>
  <div class="svc-shell">
    <div class="page-title">{{ text('命名空间', 'Namespace') }}</div>
    
    <div class="svc-content">
      <div class="query-area">
        <el-form :inline="true" :model="queryForm" class="query-form">
          <el-form-item :label="text('名称', 'Name')" class="query-item">
            <el-input v-model="queryForm.name" :placeholder="text('请输入名称', 'Input name')" clearable @keyup.enter="handleQuery" />
          </el-form-item>
          <el-form-item class="query-actions">
            <el-button type="primary" class="query-btn" @click="handleQuery">{{ text('查询', 'Search') }}</el-button>
            <el-button class="reset-btn" @click="resetQuery">{{ text('重置', 'Reset') }}</el-button>
            <el-button type="success" class="add-btn" @click="handleAdd">{{ text('新建命名空间', 'New') }}</el-button>
          </el-form-item>
        </el-form>
      </div>

      <div class="table-area">
        <el-table :data="tableData" style="width: 100%" height="100%" v-loading="loading" class="custom-table">
          <el-table-column prop="name" :label="text('名称', 'Name')" min-width="150" />
          <el-table-column prop="description" :label="text('描述', 'Description')" min-width="250" show-overflow-tooltip />
          <el-table-column prop="createdAt" :label="text('创建时间', 'Created At')" width="180" />
          <el-table-column :label="text('操作', 'Actions')" width="150" fixed="right">
            <template #default="scope">
              <el-button link type="primary" @click="handleEdit(scope.row)">{{ text('编辑', 'Edit') }}</el-button>
              <el-button link type="danger" @click="handleDelete(scope.row)">{{ text('删除', 'Delete') }}</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>

    <!-- Edit Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="dialogType === 'add' ? text('新建命名空间', 'New') : text('编辑', 'Edit')"
      width="500px"
      class="glass-dialog"
    >
      <el-form :model="form" :rules="rules" ref="formRef" label-position="top">
        <el-form-item :label="text('名称', 'Name')" prop="name">
          <el-input v-model="form.name" :disabled="dialogType === 'edit'" :placeholder="text('请输入名称', 'Name')" />
        </el-form-item>
        <el-form-item :label="text('描述', 'Description')" prop="description">
          <el-input v-model="form.description" type="textarea" :rows="3" :placeholder="text('请输入描述', 'Desc')" />
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
  name: ''
})

const dialogVisible = ref(false)
const dialogType = ref('add')
const submitting = ref(false)
const formRef = ref(null)
const form = ref({
  name: '',
  description: ''
})

const rules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }]
}

const fetchData = async () => {
  loading.value = true
  try {
    tableData.value = [
      { name: 'default', description: '默认命名空间', createdAt: '2024-03-24 10:00:00' },
      { name: 'production', description: '生产环境', createdAt: '2024-03-24 11:30:00' }
    ]
  } finally {
    loading.value = false
  }
}

const handleQuery = () => fetchData()
const resetQuery = () => {
  queryForm.value.name = ''
  handleQuery()
}

const handleAdd = () => {
  dialogType.value = 'add'
  form.value = { name: '', description: '' }
  dialogVisible.value = true
}

const handleEdit = (row) => {
  dialogType.value = 'edit'
  form.value = { ...row }
  dialogVisible.value = true
}

const handleDelete = (row) => {
  ElMessageBox.confirm(`确定要删除命名空间 "${row.name}" 吗?`, '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    ElMessage.success('删除成功')
  })
}

const submitForm = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (valid) {
      submitting.value = true
      try {
        ElMessage.success(dialogType.value === 'add' ? '创建成功' : '编辑成功')
        dialogVisible.value = false
        fetchData()
      } finally {
        submitting.value = false
      }
    }
  })
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
  background: #f5f7fa; /* Matches screenshot light gray background */
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

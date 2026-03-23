<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Edit, Lock, Plus, Refresh, Search } from '@element-plus/icons-vue'
import api from '../api/index'
import { useI18n } from '../utils/i18n'
import { sha256 } from '../utils/crypto'

const { t, locale } = useI18n()
const users = ref<any[]>([])
const loading = ref(false)
const showDialog = ref(false)
const isEdit = ref(false)
const search = ref('')
const roleFilter = ref('')
const form = ref({ username: '', password: '', role: 'viewer', nickname: '', remark: '' })

const filteredUsers = computed(() => {
  const query = search.value.trim().toLowerCase()
  return users.value.filter((user) => {
    const matchesSearch =
      !query ||
      user.username?.toLowerCase().includes(query) ||
      user.nickname?.toLowerCase().includes(query) ||
      user.remark?.toLowerCase().includes(query)
    const matchesRole = !roleFilter.value || user.role === roleFilter.value
    return matchesSearch && matchesRole
  })
})


const builtInLabel = computed(() => (locale.value === 'zh' ? '内置' : 'Built-in'))
const userHint = computed(() => (locale.value === 'zh' ? '统一管理控制台账号、角色和备注信息。' : 'Manage console users, roles, and remarks in one place.'))
const searchPlaceholder = computed(() => (locale.value === 'zh' ? '搜索用户名、昵称或备注' : 'Search username, nickname, or remark'))
const usernameRequired = computed(() => (locale.value === 'zh' ? '用户名不能为空' : 'Username is required'))
const passwordRequired = computed(() => (locale.value === 'zh' ? '新建用户时必须填写密码' : 'Password is required for new users'))
const saveFailed = computed(() => (locale.value === 'zh' ? '保存失败' : 'Failed to save user'))
const noUsersLabel = computed(() => (locale.value === 'zh' ? '暂无用户' : 'No users found'))

function handleShowAdd() {
  isEdit.value = false
  form.value = { username: '', password: '', role: 'viewer', nickname: '', remark: '' }
  showDialog.value = true
}

function handleShowEdit(row: any) {
  isEdit.value = true
  form.value = { ...row, password: '' }
  showDialog.value = true
}

async function fetchUsers() {
  loading.value = true
  try {
    const response = await api.get('/v1/rbac/users')
    users.value = response.data || []
  } catch (error) {
    console.error('fetch users failed', error)
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  if (!form.value.username) {
    ElMessage.warning(usernameRequired.value)
    return
  }

  try {
    const payload = { ...form.value }
    if (payload.password) {
      payload.password = await sha256(payload.password)
    } else if (!isEdit.value) {
      ElMessage.warning(passwordRequired.value)
      return
    }

    await api.post('/v1/rbac/user', payload)
    ElMessage.success(t.value.common.success)
    showDialog.value = false
    await fetchUsers()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || saveFailed.value)
  }
}

async function deleteUser(username: string) {
  try {
    await ElMessageBox.confirm(
      t.value.rbac.deleteConfirm.replace('{name}', username),
      t.value.common.warning,
      { type: 'warning' },
    )
    await api.post(`/v1/rbac/user/delete?username=${username}`)
    ElMessage.success(t.value.common.success)
    await fetchUsers()
  } catch {
    // ignore cancel
  }
}

function roleTagType(role: string) {
  if (role === 'admin') return 'danger'
  if (role === 'developer') return 'primary'
  return 'info'
}

function roleLabel(role: string) {
  if (role === 'admin') return t.value.rbac.admin
  if (role === 'developer') return t.value.rbac.developer
  return t.value.rbac.viewer
}

onMounted(fetchUsers)
</script>

<template>
  <div class="svc-shell">
    <!-- Toolbar -->
    <div class="svc-toolbar">
      <div class="toolbar-row">
        <div class="toolbar-group">
          <el-input
            v-model="search"
            :prefix-icon="Search"
            :placeholder="searchPlaceholder"
            clearable
            class="search-input"
            style="width: 280px;"
          />
          <el-select v-model="roleFilter" clearable class="pill-select" :placeholder="t.rbac.role" style="width: 140px;">
            <el-option value="admin" :label="t.rbac.admin" />
            <el-option value="developer" :label="t.rbac.developer" />
            <el-option value="viewer" :label="t.rbac.viewer" />
          </el-select>
        </div>
        
        <div class="toolbar-group right-align">
          <el-button :icon="Refresh" class="pill-btn" @click="fetchUsers">{{ t.common.refresh }}</el-button>
          <div class="toolbar-sep"></div>
          <el-button type="primary" class="add-btn" :icon="Plus" @click="handleShowAdd">
            {{ t.rbac.addUser }}
          </el-button>
        </div>
      </div>
    </div>

    <!-- Content -->
    <section class="svc-content" v-loading="loading">
      <div v-if="!loading && filteredUsers.length === 0" class="empty-panel">
        <div class="empty-inner">
          <strong>{{ noUsersLabel }}</strong>
          <p>{{ userHint }}</p>
        </div>
      </div>

      <template v-else>
        <div class="table-wrap">
          <el-table :data="filteredUsers" height="100%" style="width: 100%; font-size: 14px;">
            <el-table-column :label="t.rbac.username" min-width="200">
              <template #default="{ row }">
                <div class="user-cell">
                  <div class="user-avatar">{{ row.username?.slice(0, 1)?.toUpperCase() || 'U' }}</div>
                  <div class="user-main">
                    <div class="user-main-row">
                      <strong class="user-name">{{ row.username }}</strong>
                      <el-tag v-if="row.is_builtin" size="small" effect="plain" type="warning" class="mini-tag">{{ builtInLabel }}</el-tag>
                    </div>
                    <span class="user-sub">{{ row.nickname || t.common.none }}</span>
                  </div>
                </div>
              </template>
            </el-table-column>

            <el-table-column :label="t.rbac.role" width="130">
              <template #default="{ row }">
                <el-tag :type="roleTagType(row.role)" size="small" effect="dark" class="role-tag">{{ roleLabel(row.role) }}</el-tag>
              </template>
            </el-table-column>

            <el-table-column :label="t.rbac.remark" min-width="200">
              <template #default="{ row }">
                <span class="remark-text">{{ row.remark || '-' }}</span>
              </template>
            </el-table-column>

            <el-table-column :label="t.common.actions" width="140" fixed="right" align="center">
              <template #default="{ row }">
                <div class="row-actions">
                  <el-button type="primary" link :icon="Edit" @click="handleShowEdit(row)">{{ t.common.edit }}</el-button>
                  <el-button v-if="!row.is_builtin" type="danger" link :icon="Lock" @click="deleteUser(row.username)">{{ t.common.delete }}</el-button>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </template>
    </section>

    <!-- Dialog -->
    <el-dialog v-model="showDialog" :title="isEdit ? t.common.edit : t.rbac.addUser" width="460px" append-to-body class="glass-dialog">
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
          <el-input v-model="form.remark" type="textarea" :rows="3" />
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

<style scoped>
/* ===== Shell ===== */
.svc-shell {
  display: flex;
  flex-direction: column;
  height: calc(100vh - var(--header-height) - 48px);
  min-height: 0;
  overflow: hidden;
  gap: 0;
  padding: 0;
}

/* ===== Toolbar ===== */
.svc-toolbar {
  flex-shrink: 0;
  padding: 12px 24px;
  border-bottom: 1px solid var(--border-color);
}

.toolbar-row {
  display: flex;
  align-items: center;
  gap: 0;
}

.toolbar-group {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 16px;
}

.toolbar-group:first-child {
  padding-left: 0;
}

.toolbar-group:last-child {
  padding-right: 0;
}

.toolbar-group.right-align {
  margin-left: auto;
}

.toolbar-sep {
  width: 1px;
  height: 20px;
  background: var(--border-color);
  flex-shrink: 0;
  margin: 0 4px;
}

/* ── Content ── */
.svc-content {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 16px 24px 12px;
}

.empty-panel {
  display: grid;
  place-items: center;
  height: 100%;
}

.empty-inner {
  text-align: center;
  color: var(--text-muted);
}
.empty-inner strong {
  display: block;
  font-size: 16px;
  color: var(--text-primary);
  margin-bottom: 8px;
}

:deep(.search-input .el-input__wrapper),
:deep(.pill-select .el-input__wrapper) {
  background: rgba(255, 255, 255, 0.04) !important;
  border: 1px solid var(--border-color) !important;
  box-shadow: none !important;
  border-radius: 6px;
  color: var(--text-primary);
  height: 32px;
}
:deep(.search-input .el-input__wrapper:hover),
:deep(.search-input .el-input__wrapper:focus-within),
:deep(.pill-select .el-input__wrapper:hover),
:deep(.pill-select .el-input__wrapper:focus-within) {
  border-color: rgba(59, 130, 246, 0.4) !important;
  background: rgba(255, 255, 255, 0.06) !important;
}

.pill-btn {
  border-radius: 8px;
  height: 32px;
}
.add-btn {
  height: 32px;
  border-radius: 8px;
  padding: 0 16px;
}

.table-wrap {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  background: transparent;
}

:deep(.table-wrap .el-table) {
  height: 100%;
  --el-table-bg-color: transparent;
  --el-table-tr-bg-color: transparent;
  --el-table-row-hover-bg-color: rgba(255, 255, 255, 0.03);
  --el-table-border-color: rgba(255, 255, 255, 0.04);
  --el-table-header-bg-color: transparent;
  --el-table-header-text-color: var(--text-muted);
  --el-table-text-color: var(--text-primary);
}

:deep(.table-wrap .el-table__inner-wrapper::before) {
  display: none;
}

:deep(.table-wrap .el-table th.el-table__cell) {
  background: transparent;
  color: var(--text-muted);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

:deep(.table-wrap .el-table td.el-table__cell) {
  padding-top: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.03);
}

:deep(.table-wrap .el-table tr:hover > td.el-table__cell) {
  background: rgba(255, 255, 255, 0.02) !important;
}

:deep(.table-wrap .el-table__fixed-right) {
  box-shadow: none;
}

.user-cell {
  display: flex;
  align-items: center;
  gap: 14px;
}

.user-avatar {
  width: 40px;
  height: 40px;
  display: grid;
  place-items: center;
  border-radius: 10px;
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.1), rgba(6, 182, 212, 0.1));
  color: var(--accent-blue);
  font-weight: 700;
  font-size: 14px;
}

.user-main {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.user-main-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.user-name {
  font-size: 14px;
}

.mini-tag {
  height: 18px;
  padding: 0 4px;
  font-size: 10px;
}

.user-sub {
  font-size: 12px;
  color: var(--text-muted);
}

.remark-text {
  font-size: 13px;
  color: var(--text-secondary);
}

.row-actions {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
}


</style>



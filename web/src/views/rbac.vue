<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Edit, Grid, List as ListIcon, Lock, Plus, Refresh, Search } from '@element-plus/icons-vue'
import api from '../api/index'
import { useI18n } from '../utils/i18n'
import { sha256 } from '../utils/crypto'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import en from 'element-plus/es/locale/lang/en'

const { t, locale } = useI18n()
const users = ref<any[]>([])
const loading = ref(false)
const showDialog = ref(false)
const isEdit = ref(false)
const searchUser = ref('')
const searchRole = ref('')
const statusFilter = ref('all')
const currentPage = ref(1)
const pageSize = ref(12)
const viewMode = ref<'list' | 'grid'>('list')

const userStats = computed(() => ({
  total: users.value.length,
  admin: users.value.filter(u => u.role === 'admin').length,
  normal: users.value.filter(u => u.role !== 'admin').length
}))

const form = ref({ username: '', password: '', role: 'viewer', nickname: '', remark: '' })

const filteredUsers = computed(() => {
  const qUser = searchUser.value.trim().toLowerCase()
  const qRole = searchRole.value.trim().toLowerCase()
  let list = users.value

  if (qUser) {
    list = list.filter(u => 
      u.username?.toLowerCase().includes(qUser) || 
      u.nickname?.toLowerCase().includes(qUser)
    )
  }
  if (qRole) {
    list = list.filter(u => u.role === qRole)
  }
  if (statusFilter.value !== 'all') {
    list = list.filter(u => u.status === statusFilter.value)
  }

  return list
})

const pagedUsers = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredUsers.value.slice(start, start + pageSize.value)
})

function resetFilters() {
  searchUser.value = ''
  searchRole.value = ''
  statusFilter.value = 'all'
  fetchUsers()
}


const builtInLabel = computed(() => (locale.value === 'zh' ? '内置' : 'Built-in'))
const userHint = computed(() => (locale.value === 'zh' ? '统一管理控制台账号、角色和备注信息。' : 'Manage console users, roles, and remarks in one place.'))
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
          <div class="field-item">
            <span class="field-label">{{ locale === 'zh' ? '用户名称' : 'Username' }}</span>
            <el-input
              v-model="searchUser"
              :prefix-icon="Search"
              :placeholder="locale === 'zh' ? '输入用户名' : 'Username'"
              clearable
              class="search-input"
              style="width: 160px;"
            />
          </div>
        </div>

        <div class="toolbar-group">
          <div class="field-item">
            <span class="field-label">{{ locale === 'zh' ? '角色' : 'Role' }}</span>
            <el-select v-model="searchRole" clearable class="pill-select" :placeholder="t.rbac.role" style="width: 120px;">
              <el-option value="admin" :label="t.rbac.admin" />
              <el-option value="developer" :label="t.rbac.developer" />
              <el-option value="viewer" :label="t.rbac.viewer" />
            </el-select>
          </div>
        </div>

        <div class="toolbar-group">
          <div class="field-item">
            <span class="field-label">{{ locale === 'zh' ? '状态' : 'Status' }}</span>
            <el-select v-model="statusFilter" class="pill-select" style="width: 100px;">
              <el-option value="all" :label="t.common.all || '全部'" />
              <el-option value="active" :label="locale === 'zh' ? '正常' : 'Active'" />
              <el-option value="frozen" :label="locale === 'zh' ? '冻结' : 'Frozen'" />
            </el-select>
          </div>
        </div>

        <div class="toolbar-group">
          <button
            type="button"
            class="refresh-btn"
            :title="t.common.refresh"
            @click="resetFilters()"
          >
            <el-icon><Refresh /></el-icon>
          </button>
        </div>
        
        <div class="toolbar-group right-align">
          <div class="pill-group">
            <button type="button" :class="{ active: searchRole === '' }" @click="searchRole = ''">
              {{ t.common.all || '全部' }}
              <span class="pill-count">{{ userStats.total }}</span>
            </button>
            <button type="button" :class="{ active: searchRole === 'admin' }" @click="searchRole = 'admin'">
              {{ t.rbac.admin }}
              <span class="pill-count">{{ userStats.admin }}</span>
            </button>
          </div>

          <div class="toolbar-sep"></div>

          <div class="pill-group icon-pills">
            <button type="button" :class="{ active: viewMode === 'list' }" @click="viewMode = 'list'">
              <el-icon><ListIcon /></el-icon>
            </button>
            <button type="button" :class="{ active: viewMode === 'grid' }" @click="viewMode = 'grid'">
              <el-icon><Grid /></el-icon>
            </button>
          </div>

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
        <div v-if="viewMode === 'list'" class="table-wrap">
          <el-table :data="pagedUsers" height="100%" style="width: 100%; font-size: 14px;">
            <el-table-column type="index" label="#" width="60" align="center" />
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

        <div v-else class="user-grid">
          <article v-for="user in pagedUsers" :key="user.username" class="user-card">
            <div class="card-glow" :style="{ background: user.role === 'admin' ? 'var(--accent-red)' : 'var(--accent-blue)' }" />
            <div class="user-card-head">
              <div class="user-avatar text">{{ user.username?.slice(0, 1)?.toUpperCase() }}</div>
              <div class="user-card-info">
                <h3 class="user-card-name">{{ user.username }}</h3>
                <span class="user-card-sub">{{ user.nickname || t.common.none }}</span>
              </div>
              <el-tag :type="roleTagType(user.role)" size="small" effect="dark">{{ roleLabel(user.role) }}</el-tag>
            </div>
            <div class="user-card-body">
               <p class="remark-line">{{ user.remark || '-' }}</p>
            </div>
            <div class="user-card-foot">
               <div class="footer-spacer"></div>
               <div class="card-actions">
                  <el-button type="primary" link :icon="Edit" @click="handleShowEdit(user)" />
                  <el-button v-if="!user.is_builtin" type="danger" link :icon="Delete" @click="deleteUser(user.username)" />
               </div>
            </div>
          </article>
        </div>

        <footer class="svc-footer">
          <span class="footer-info">{{ filteredUsers.length }} {{ locale === 'zh' ? '条' : 'records' }}</span>
          <el-config-provider :locale="locale === 'zh' ? zhCn : en">
            <el-pagination
              v-model:current-page="currentPage"
              v-model:page-size="pageSize"
              :page-sizes="[12, 20, 50]"
              :total="filteredUsers.length"
              layout="sizes, prev, pager, next"
              background
            />
          </el-config-provider>
        </footer>
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

.field-item {
  display: flex;
  align-items: center;
  gap: 12px;
}

.field-label {
  font-size: 14px;
  color: var(--text-secondary);
  white-space: nowrap;
}

.toolbar-sep {
  width: 1px;
  height: 24px;
  background: var(--border-color);
  flex-shrink: 0;
  margin: 0 4px;
  opacity: 0.5;
}

.refresh-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.15s ease;
}

.refresh-btn:hover {
  color: var(--accent-blue);
  background: rgba(59, 130, 246, 0.08);
}

/* -- Pill groups -- */
.pill-group {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 2px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.03);
}

.pill-group button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--text-muted);
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;
  white-space: nowrap;
  font-family: inherit;
  font-size: 14px;
}

.pill-group button:hover {
  color: var(--text-secondary);
  background: rgba(255, 255, 255, 0.04);
}

.pill-count {
  font-weight: 600;
  opacity: 0.5;
  font-variant-numeric: tabular-nums;
  font-size: 14px;
}

.pill-group button.active {
  background: rgba(59, 130, 246, 0.12);
  color: var(--accent-blue);
  font-weight: 600;
}

.pill-group button.active .pill-count {
  opacity: 0.8;
}

.icon-pills button {
  padding: 6px 10px;
}

/* ── Content ── */
.svc-content {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
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

/* ===== Footer ===== */
.svc-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 0 0;
  flex-shrink: 0;
}

.footer-info {
  font-size: 14px;
  color: var(--text-muted);
  opacity: 0.8;
}

:deep(.svc-footer .el-pagination) {
  flex-wrap: wrap;
  justify-content: flex-end;
}

:deep(.svc-footer .el-pagination *) {
  font-size: 14px !important;
}

/* ===== Grid ===== */
.user-grid {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 24px;
  padding: 2px 2px 24px;
}

/* Hide scrollbar for Chrome/Safari */
.user-grid::-webkit-scrollbar {
  width: 6px;
}
.user-grid::-webkit-scrollbar-thumb {
  background: rgba(255,255,255,0.05);
  border-radius: 10px;
}

.user-card {
  padding: 24px;
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  position: relative;
  overflow: hidden;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.user-card:hover {
  transform: translateY(-4px);
  border-color: var(--accent-blue);
  box-shadow: 0 12px 24px -8px rgba(0, 0, 0, 0.3), 0 0 0 1px var(--accent-blue);
}

.card-glow {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 4px;
  opacity: 0.6;
}

.user-card-head {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 20px;
}

.user-avatar.text {
  font-size: 18px;
  background: rgba(255,255,255,0.05);
}

.user-card-info {
  flex: 1;
}

.user-card-name {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.user-card-sub {
  font-size: 13px;
  color: var(--text-muted);
}

.user-card-body {
  margin-bottom: 16px;
}

.remark-line {
  font-size: 14px;
  color: var(--text-secondary);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.user-card-foot {
  display: flex;
  padding-top: 16px;
  border-top: 1px solid rgba(255,255,255,0.03);
}

.footer-spacer { flex: 1; }


</style>



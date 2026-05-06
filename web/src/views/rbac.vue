<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Grid, List as ListIcon, Plus, RefreshLeft, Search } from '@element-plus/icons-vue'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import en from 'element-plus/es/locale/lang/en'
import {
  deleteRbacUser,
  getRbacUsers,
  saveRbacUser,
  type RbacUser,
} from '../api/registry'
import { useI18n } from '../utils/i18n'

type ViewMode = 'list' | 'grid'
type FilterMode = 'all' | 'admin' | 'developer' | 'guest'
type DialogType = 'add' | 'edit'

interface UserRow {
  username: string
  nickname: string
  role: string
  email: string
  phone: string
  remark: string
  isBuiltIn: boolean
}

interface QueryForm {
  username: string
  role: string
}

interface UserForm {
  username: string
  nickname: string
  role: string
  password: string
  email: string
  phone: string
  remark: string
  isBuiltIn: boolean
}

const { locale } = useI18n()
const text = (zh: string, enText: string) => (locale.value === 'zh' ? zh : enText)
const elLocale = computed(() => (locale.value === 'zh' ? zhCn : en))

const loading = ref(false)
const rows = ref<UserRow[]>([])
const queryForm = ref<QueryForm>({
  username: '',
  role: '',
})
const dialogVisible = ref(false)
const dialogType = ref<DialogType>('add')
const submitting = ref(false)
const formRef = ref<FormInstance>()
const form = ref<UserForm>({
  username: '',
  nickname: '',
  role: 'guest',
  password: '',
  email: '',
  phone: '',
  remark: '',
  isBuiltIn: false,
})
const viewMode = ref<ViewMode>('list')
const filterMode = ref<FilterMode>('all')
const currentPage = ref(1)
const pageSize = ref(12)

const totalUsers = computed(() => rows.value.length)
const adminCount = computed(() => rows.value.filter((row) => row.role === 'admin').length)
const developerCount = computed(() => rows.value.filter((row) => row.role === 'developer').length)
const guestCount = computed(() => rows.value.filter((row) => row.role === 'guest').length)

const filteredRows = computed(() => {
  const username = queryForm.value.username.trim().toLowerCase()
  const role = queryForm.value.role
  let list = rows.value

  if (filterMode.value !== 'all') {
    list = list.filter((row) => row.role === filterMode.value)
  }

  if (username) {
    list = list.filter((row) =>
      [row.username, row.nickname, row.email, row.phone, row.remark]
        .filter(Boolean)
        .some((field) => field.toLowerCase().includes(username)),
    )
  }

  if (role) {
    list = list.filter((row) => row.role === role)
  }

  return list
})

const pagedRows = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredRows.value.slice(start, start + pageSize.value)
})

const totalPages = computed(() => Math.max(1, Math.ceil(filteredRows.value.length / pageSize.value)))

const validatePassword = (_rule: any, value: string, callback: (error?: Error) => void) => {
  if (dialogType.value === 'add' && !value.trim()) {
    callback(new Error(text('请输入密码', 'Please input password')))
    return
  }
  callback()
}

const rules: FormRules<UserForm> = {
  username: [{ required: true, message: text('请输入用户名', 'Please input username'), trigger: 'blur' }],
  role: [{ required: true, message: text('请选择角色', 'Please select role'), trigger: 'change' }],
  password: [{ validator: validatePassword, trigger: 'blur' }],
}

const emptyTitle = computed(() => text('暂无匹配用户', 'No users matched'))
const emptyDesc = computed(() =>
  text('试试调整筛选条件或清空搜索关键词。', 'Try adjusting filters or clearing the search.'),
)

const getRoleTag = (role: string) => {
  switch (role) {
    case 'admin':
      return 'danger'
    case 'developer':
      return 'primary'
    default:
      return 'info'
  }
}

const roleLabel = (role: string) => {
  switch (role) {
    case 'admin':
      return text('管理员', 'Admin')
    case 'developer':
      return text('开发者', 'Developer')
    default:
      return text('访客', 'Guest')
  }
}

const getErrorMessage = (error: any, fallbackZh: string, fallbackEn: string) =>
  error?.response?.data?.error || text(fallbackZh, fallbackEn)

const mapUser = (user: RbacUser): UserRow => ({
  username: user.username,
  nickname: user.nickname || '',
  role: user.role,
  email: user.email || '',
  phone: user.phone || '',
  remark: user.remark || '',
  isBuiltIn: !!user.is_builtin,
})

const displayNickname = (row: UserRow) => row.nickname || '-'

const userInitial = (row: UserRow) => (row.nickname || row.username || '?').trim().charAt(0).toUpperCase()

const fetchData = async () => {
  loading.value = true
  try {
    const response = await getRbacUsers()
    rows.value = (response.data || []).map(mapUser)
  } catch (error: any) {
    rows.value = []
    ElMessage.error(getErrorMessage(error, '加载用户失败', 'Failed to load users'))
  } finally {
    loading.value = false
  }
}

const resetFilters = async () => {
  queryForm.value = { username: '', role: '' }
  filterMode.value = 'all'
  currentPage.value = 1
  await fetchData()
}

const openAddDialog = () => {
  dialogType.value = 'add'
  form.value = {
    username: '',
    nickname: '',
    role: 'guest',
    password: '',
    email: '',
    phone: '',
    remark: '',
    isBuiltIn: false,
  }
  dialogVisible.value = true
}

const handleEdit = (row: UserRow) => {
  dialogType.value = 'edit'
  form.value = {
    username: row.username,
    nickname: row.nickname,
    role: row.role,
    password: '',
    email: row.email,
    phone: row.phone,
    remark: row.remark,
    isBuiltIn: row.isBuiltIn,
  }
  dialogVisible.value = true
}

const handleDelete = (row: UserRow) => {
  ElMessageBox.confirm(
    text(`确定要删除用户 "${row.username}" 吗？`, `Are you sure you want to delete user "${row.username}"?`),
    text('提示', 'Notice'),
    {
      confirmButtonText: text('确定', 'Confirm'),
      cancelButtonText: text('取消', 'Cancel'),
      type: 'warning',
    },
  ).then(async () => {
    try {
      await deleteRbacUser(row.username)
      ElMessage.success(text('删除成功', 'Deleted successfully'))
      await fetchData()
    } catch (error: any) {
      ElMessage.error(getErrorMessage(error, '删除用户失败', 'Failed to delete user'))
    }
  }).catch(() => undefined)
}

const submitForm = async () => {
  if (!formRef.value) return

  try {
    await formRef.value.validate()
    submitting.value = true

    const payload: Partial<RbacUser> = {
      username: form.value.username,
      nickname: form.value.nickname,
      role: form.value.role,
      email: form.value.email,
      phone: form.value.phone,
      remark: form.value.remark,
    }

    if (form.value.password.trim()) {
      payload.password = form.value.password
    }

    await saveRbacUser(payload)
    ElMessage.success(
      text(
        dialogType.value === 'add' ? '保存用户成功' : '更新用户成功',
        dialogType.value === 'add' ? 'User created successfully' : 'User updated successfully',
      ),
    )
    dialogVisible.value = false
    await fetchData()
  } catch (error: any) {
    if (error?.response) {
      ElMessage.error(
        getErrorMessage(
          error,
          dialogType.value === 'add' ? '创建用户失败' : '更新用户失败',
          dialogType.value === 'add' ? 'Failed to create user' : 'Failed to update user',
        ),
      )
    }
  } finally {
    submitting.value = false
  }
}

watch([() => queryForm.value.username, () => queryForm.value.role, filterMode], () => {
  currentPage.value = 1
})

watch(pageSize, () => {
  currentPage.value = 1
})

watch(filteredRows, () => {
  if (currentPage.value > totalPages.value) currentPage.value = totalPages.value
})

onMounted(() => {
  fetchData()
})
</script>

<template>
  <div class="svc-shell">
    <div class="svc-main glass-card">
      <div class="svc-toolbar">
        <div class="toolbar-row">
          <div class="toolbar-group">
            <div class="field-item">
              <span class="field-label">{{ text('用户名', 'Username') }}</span>
              <el-input
                v-model="queryForm.username"
                :prefix-icon="Search"
                :placeholder="text('输入用户名', 'Input username')"
                clearable
                class="search-input"
              />
            </div>
          </div>

          <div class="toolbar-group">
            <div class="field-item">
              <span class="field-label">{{ text('角色', 'Role') }}</span>
              <el-select
                v-model="queryForm.role"
                :placeholder="text('选择角色', 'Select role')"
                clearable
                class="role-select"
              >
                <el-option label="管理员" value="admin" />
                <el-option label="开发者" value="developer" />
                <el-option label="访客" value="guest" />
              </el-select>
            </div>
          </div>

          <div class="toolbar-group">
            <button
              type="button"
              class="refresh-btn"
              :title="text('重置筛选', 'Reset Filters')"
              @click="resetFilters()"
            >
              <el-icon><RefreshLeft /></el-icon>
            </button>
          </div>

          <div class="toolbar-group right-align">
            <div class="pill-group">
              <button type="button" :class="{ active: filterMode === 'all' }" @click="filterMode = 'all'">
                {{ text('全部', 'All') }}
                <span class="pill-count">{{ totalUsers }}</span>
              </button>
              <button type="button" :class="{ active: filterMode === 'admin' }" @click="filterMode = 'admin'">
                {{ text('管理员', 'Admin') }}
                <span class="pill-count">{{ adminCount }}</span>
              </button>
              <button type="button" :class="{ active: filterMode === 'developer' }" @click="filterMode = 'developer'">
                {{ text('开发者', 'Dev') }}
                <span class="pill-count">{{ developerCount }}</span>
              </button>
              <button type="button" :class="{ active: filterMode === 'guest' }" @click="filterMode = 'guest'">
                {{ text('访客', 'Guest') }}
                <span class="pill-count">{{ guestCount }}</span>
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

            <el-button type="primary" class="add-btn" :icon="Plus" @click="openAddDialog">
              {{ text('添加用户', 'Add User') }}
            </el-button>
          </div>
        </div>
      </div>

      <section class="svc-content" v-loading="loading">
        <div v-if="!loading && filteredRows.length === 0" class="empty-panel">
          <div class="empty-inner">
            <strong>{{ emptyTitle }}</strong>
            <p>{{ emptyDesc }}</p>
          </div>
        </div>

        <template v-else>
          <div v-if="viewMode === 'list'" class="table-wrap">
            <el-table :data="pagedRows" height="100%" style="width: 100%; font-size: 14px;">
              <el-table-column type="index" :label="text('序号', 'No.')" width="60" align="center" />
              <el-table-column :label="text('用户名', 'Username')" min-width="180">
                <template #default="{ row }">
                  <div class="user-cell">
                    <strong>{{ row.username }}</strong>
                    <el-tag v-if="row.isBuiltIn" size="small" effect="plain" type="warning" class="account-tag">
                      {{ text('内置', 'Built-in') }}
                    </el-tag>
                  </div>
                </template>
              </el-table-column>
              <el-table-column :label="text('昵称', 'Nickname')" min-width="160">
                <template #default="{ row }">
                  {{ displayNickname(row) }}
                </template>
              </el-table-column>
              <el-table-column :label="text('角色', 'Role')" width="140">
                <template #default="{ row }">
                  <el-tag :type="getRoleTag(row.role)" size="small" effect="plain">{{ roleLabel(row.role) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column :label="text('邮箱', 'Email')" min-width="220" show-overflow-tooltip>
                <template #default="{ row }">
                  {{ row.email || '-' }}
                </template>
              </el-table-column>
              <el-table-column :label="text('电话', 'Phone')" min-width="180" show-overflow-tooltip>
                <template #default="{ row }">
                  {{ row.phone || '-' }}
                </template>
              </el-table-column>
              <el-table-column :label="text('操作', 'Actions')" width="180" fixed="right">
                <template #default="{ row }">
                  <el-button link type="primary" @click="handleEdit(row)">{{ text('编辑', 'Edit') }}</el-button>
                  <el-button
                    v-if="!row.isBuiltIn"
                    link
                    type="danger"
                    @click="handleDelete(row)"
                  >
                    {{ text('删除', 'Delete') }}
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
          </div>

          <div v-else class="card-grid">
            <article
              v-for="row in pagedRows"
              :key="row.username"
              class="info-card"
              :class="{ 'builtin-card': row.isBuiltIn }"
            >
              <div class="card-accent"></div>

              <div class="card-head">
                <div class="identity-block">
                  <div class="identity-avatar">{{ userInitial(row) }}</div>
                  <div class="identity-copy">
                    <div class="title-row">
                      <strong class="card-title">{{ row.username }}</strong>
                      <el-tag v-if="row.isBuiltIn" size="small" effect="plain" type="warning" class="account-tag">
                        {{ text('内置', 'Built-in') }}
                      </el-tag>
                    </div>
                    <p class="card-subtitle">{{ displayNickname(row) }}</p>
                  </div>
                </div>
                <div class="role-pill">
                  <el-tag :type="getRoleTag(row.role)" size="small" effect="light">
                    {{ roleLabel(row.role) }}
                  </el-tag>
                </div>
              </div>

              <div class="card-body">
                <div class="compact-meta-row">
                  <span class="compact-label">{{ text('邮箱', 'Email') }}</span>
                  <span class="compact-value" :title="row.email || '-'">{{ row.email || '-' }}</span>
                </div>
                <div class="compact-meta-row">
                  <span class="compact-label">{{ text('电话', 'Phone') }}</span>
                  <span class="compact-value" :title="row.phone || '-'">{{ row.phone || '-' }}</span>
                </div>
              </div>

              <div class="card-footer">
                <div class="card-actions">
                  <el-button link type="primary" @click="handleEdit(row)">
                    {{ text('编辑', 'Edit') }}
                  </el-button>
                  <el-button
                    v-if="!row.isBuiltIn"
                    link
                    type="danger"
                    @click="handleDelete(row)"
                  >
                    {{ text('删除', 'Delete') }}
                  </el-button>
                </div>
              </div>
            </article>
          </div>

          <footer class="svc-footer">
            <span class="footer-info">{{ filteredRows.length }} {{ text('条', 'records') }}</span>
            <el-config-provider :locale="elLocale">
              <el-pagination
                v-model:current-page="currentPage"
                v-model:page-size="pageSize"
                :page-sizes="[12, 20, 50]"
                :total="filteredRows.length"
                layout="sizes, prev, pager, next"
                background
              />
            </el-config-provider>
          </footer>
        </template>
      </section>
    </div>

    <el-dialog
      v-model="dialogVisible"
      :title="dialogType === 'add' ? text('添加用户', 'Add User') : text('编辑用户', 'Edit User')"
      width="560px"
      top="4vh"
      class="glass-dialog user-dialog"
    >
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" class="user-dialog-form">
        <div class="form-grid">
        <el-form-item :label="text('用户名', 'Username')" prop="username">
          <el-input
            v-model="form.username"
            :disabled="dialogType === 'edit'"
            :placeholder="text('请输入用户名', 'Please input username')"
          />
        </el-form-item>
        <el-form-item :label="text('昵称', 'Nickname')" prop="nickname">
          <el-input
            v-model="form.nickname"
            :placeholder="text('请输入昵称', 'Please input nickname')"
          />
        </el-form-item>
        <el-form-item :label="text('角色', 'Role')" prop="role">
          <el-select v-model="form.role" style="width: 100%">
            <el-option label="管理员" value="admin" />
            <el-option label="开发者" value="developer" />
            <el-option label="访客" value="guest" />
          </el-select>
        </el-form-item>
        <el-form-item :label="text('密码', 'Password')" prop="password">
          <el-input
            v-model="form.password"
            type="password"
            show-password
            :placeholder="dialogType === 'add' ? text('请输入密码', 'Please input password') : text('留空表示不修改密码', 'Leave blank to keep current password')"
          />
        </el-form-item>
        <el-form-item :label="text('邮箱', 'Email')" prop="email">
          <el-input v-model="form.email" :placeholder="text('请输入邮箱', 'Please input email')" />
        </el-form-item>
        <el-form-item :label="text('电话', 'Phone')" prop="phone">
          <el-input v-model="form.phone" :placeholder="text('请输入电话', 'Please input phone')" />
        </el-form-item>
        <el-form-item :label="text('备注', 'Remark')" prop="remark" class="form-span-2">
          <el-input
            v-model="form.remark"
            type="textarea"
            :rows="2"
            :placeholder="text('请输入备注', 'Please input remark')"
          />
        </el-form-item>
        </div>
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

<style scoped>
.svc-shell {
  display: flex;
  flex-direction: column;
  height: calc(100vh - var(--header-height) - 48px);
  min-height: 0;
  overflow: hidden;
}

.svc-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  background: var(--bg-card);
  backdrop-filter: blur(20px);
  border: 1px solid var(--border-color);
  border-radius: 0;
}

.svc-toolbar {
  flex-shrink: 0;
  padding: 16px 24px;
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
  gap: 8px;
}

.field-label {
  font-size: 14px;
  color: var(--text-secondary);
  font-weight: 700;
  white-space: nowrap;
}

.search-input {
  width: 220px;
  flex-shrink: 0;
}

.role-select {
  width: 180px;
  flex-shrink: 0;
}

.toolbar-sep {
  width: 1px;
  height: 20px;
  background: var(--border-color);
  flex-shrink: 0;
  margin: 0 2px;
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
  margin-left: -20px;
}

.refresh-btn:hover {
  color: var(--accent-blue);
  background: rgba(59, 130, 246, 0.08);
}

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

.pill-group button.active {
  background: rgba(59, 130, 246, 0.12);
  color: var(--accent-blue);
  font-weight: 600;
}

.pill-count {
  font-weight: 600;
  opacity: 0.5;
  font-variant-numeric: tabular-nums;
  font-size: 14px;
}

.pill-group button.active .pill-count {
  opacity: 0.8;
}

.icon-pills button {
  padding: 6px 10px;
}

.add-btn {
  height: 32px;
  border-radius: 8px;
  padding: 0 16px;
}

.svc-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  padding: 16px 24px 12px;
}

.empty-panel {
  flex: 1;
  display: grid;
  place-items: center;
  min-height: 200px;
  padding: 24px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid var(--border-color);
  border-radius: 12px;
}

.empty-inner {
  text-align: center;
}

.empty-inner strong {
  display: block;
  color: var(--text-secondary);
  font-weight: 600;
  margin-bottom: 6px;
}

.empty-inner p {
  color: var(--text-muted);
  opacity: 0.6;
}

.table-wrap {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

:deep(.search-input .el-input__wrapper),
:deep(.role-select .el-select__wrapper) {
  background: rgba(255, 255, 255, 0.04) !important;
  border: 1px solid var(--border-color) !important;
  box-shadow: none !important;
  border-radius: 6px;
  color: var(--text-primary);
  height: 32px;
}

:deep(.search-input .el-input__wrapper:hover),
:deep(.search-input .el-input__wrapper:focus-within),
:deep(.role-select .el-select__wrapper:hover),
:deep(.role-select .el-select__wrapper.is-focused) {
  border-color: rgba(59, 130, 246, 0.4) !important;
  background: rgba(255, 255, 255, 0.06) !important;
}

:deep(.search-input .el-input__inner),
:deep(.role-select .el-select__selected-item),
:deep(.role-select .el-select__placeholder) {
  color: var(--text-primary) !important;
  font-size: 14px;
}

:deep(.search-input .el-input__prefix .el-icon) {
  color: var(--text-muted);
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
  gap: 10px;
}

.card-grid {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 272px));
  justify-content: start;
  gap: 10px;
  align-content: start;
  padding: 0 0 8px;
}

.info-card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  border-radius: 12px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  box-shadow: none;
  overflow: hidden;
  transition: border-color 0.2s ease, background-color 0.2s ease, box-shadow 0.2s ease;
}

.info-card:hover {
  border-color: rgba(59, 130, 246, 0.28);
  background: rgba(59, 130, 246, 0.03);
  box-shadow: inset 0 0 0 1px rgba(59, 130, 246, 0.08);
}

.card-accent {
  position: absolute;
  inset: 0 auto auto 0;
  width: 84px;
  height: 3px;
  background: rgba(59, 130, 246, 0.35);
  border-radius: 0 0 999px 0;
}

.builtin-card .card-accent {
  background: rgba(245, 158, 11, 0.4);
}

.card-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.identity-block {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  min-width: 0;
}

.identity-avatar {
  width: 32px;
  height: 32px;
  border-radius: 10px;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--accent-blue);
  font-size: 13px;
  font-weight: 700;
  background: rgba(59, 130, 246, 0.1);
  border: 1px solid rgba(59, 130, 246, 0.14);
}

.builtin-card .identity-avatar {
  color: var(--accent-orange);
  background: rgba(245, 158, 11, 0.12);
  border-color: rgba(245, 158, 11, 0.18);
}

.identity-copy {
  min-width: 0;
}

.account-tag {
  flex-shrink: 0;
}

.role-pill {
  flex-shrink: 0;
}

.user-dialog-form {
  max-height: calc(92vh - 146px);
  overflow-y: auto;
  overflow-x: hidden;
  padding-right: 4px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 2px 14px;
  align-items: start;
}

.form-span-2 {
  grid-column: 1 / -1;
}

.dialog-footer {
  width: 100%;
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.card-title {
  display: block;
  color: var(--text-primary);
  font-size: 15px;
  font-weight: 700;
  line-height: 1.2;
}

.card-subtitle {
  margin: 2px 0 0;
  color: var(--text-secondary);
  font-size: 11px;
  line-height: 1.4;
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 1;
}

.card-body {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.compact-meta-row {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  font-size: 12px;
  line-height: 1.45;
}

.compact-label {
  flex: 0 0 32px;
  color: rgba(100, 116, 139, 0.88);
  font-size: 10px;
  font-weight: 700;
  line-height: 1.5;
  letter-spacing: 0.02em;
}

.compact-value {
  flex: 1;
  min-width: 0;
  color: var(--text-primary);
  font-size: 12px;
  line-height: 1.45;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  padding-top: 8px;
  border-top: 1px solid rgba(148, 163, 184, 0.14);
}

.card-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

:deep(.account-tag.el-tag) {
  color: #c27a0a;
  border-color: rgba(245, 158, 11, 0.18);
  background: rgba(245, 158, 11, 0.08);
}

:deep(.user-dialog.el-dialog) {
  width: min(560px, calc(100vw - 24px)) !important;
  max-height: 92vh;
  margin-bottom: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-radius: 16px;
}

:deep(.user-dialog .el-dialog__header) {
  margin-right: 0;
  padding: 18px 20px 10px;
  flex-shrink: 0;
}

:deep(.user-dialog .el-dialog__headerbtn) {
  top: 18px;
  right: 18px;
}

:deep(.user-dialog .el-dialog__body) {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  padding: 0 20px 6px;
}

:deep(.user-dialog .el-dialog__footer) {
  flex-shrink: 0;
  padding: 12px 20px 18px;
  border-top: 1px solid rgba(148, 163, 184, 0.14);
}

:deep(.user-dialog .el-form-item) {
  margin-bottom: 12px;
}

:deep(.user-dialog .el-form-item__label) {
  padding-bottom: 6px;
  line-height: 1.35;
}

:deep(.user-dialog .el-input__wrapper),
:deep(.user-dialog .el-select__wrapper) {
  min-height: 38px;
  border-radius: 10px;
}

:deep(.user-dialog .el-textarea__inner) {
  min-height: 72px !important;
  border-radius: 10px;
}

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

@media (max-width: 1400px) {
  .card-grid {
    grid-template-columns: repeat(3, minmax(0, 272px));
  }
}

@media (max-width: 1180px) {
  .toolbar-row {
    flex-wrap: wrap;
    gap: 8px;
  }

  .toolbar-group {
    padding: 0;
  }

  .toolbar-group.right-align {
    width: 100%;
    margin-left: 0;
    justify-content: space-between;
  }
}

@media (max-width: 768px) {
  .svc-toolbar {
    padding: 12px 16px;
  }

  .svc-content {
    padding: 12px 16px 8px;
  }

  .field-item {
    flex-wrap: wrap;
    width: 100%;
  }

  .search-input,
  .role-select {
    width: 100% !important;
  }

  .pill-group {
    width: 100%;
  }

  .pill-group button {
    flex: 1;
    justify-content: center;
  }

  .table-wrap {
    min-height: 0;
  }

  .card-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .svc-footer {
    flex-direction: column;
    align-items: stretch;
  }

  .user-dialog-form {
    max-height: calc(92vh - 132px);
  }

  .form-grid {
    grid-template-columns: 1fr;
    gap: 0;
  }

  .form-span-2 {
    grid-column: auto;
  }
}

@media (max-width: 640px) {
  .card-grid {
    grid-template-columns: 1fr;
  }

  :deep(.user-dialog.el-dialog) {
    width: calc(100vw - 16px) !important;
    max-height: 94vh;
  }

  :deep(.user-dialog .el-dialog__header) {
    padding: 16px 16px 8px;
  }

  :deep(.user-dialog .el-dialog__body) {
    padding: 0 16px 4px;
  }

  :deep(.user-dialog .el-dialog__footer) {
    padding: 10px 16px 16px;
  }
}
</style>

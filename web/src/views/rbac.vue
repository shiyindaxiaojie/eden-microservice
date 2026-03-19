<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Edit, Lock, Plus, Search, User } from '@element-plus/icons-vue'
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

const stats = computed(() => ({
  total: users.value.length,
  admin: users.value.filter((user) => user.role === 'admin').length,
  developer: users.value.filter((user) => user.role === 'developer').length,
  viewer: users.value.filter((user) => user.role === 'viewer').length,
}))

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
  <div class="page-stack rbac-page">
    <section class="compact-hero">
      <div class="hero-copy-compact">
        <p class="hero-kicker">{{ t.nav.accessControl }}</p>
        <h2 class="hero-heading">{{ t.rbac.title }}</h2>
        <p class="hero-desc">{{ userHint }}</p>
      </div>
      <div class="mini-stat-grid rbac-mini-stats">
        <div class="mini-stat">
          <span class="mini-stat-label">{{ locale === 'zh' ? '账号总数' : 'Total Users' }}</span>
          <strong class="mini-stat-value">{{ stats.total }}</strong>
        </div>
        <div class="mini-stat">
          <span class="mini-stat-label">{{ t.rbac.admin }}</span>
          <strong class="mini-stat-value">{{ stats.admin }}</strong>
        </div>
        <div class="mini-stat">
          <span class="mini-stat-label">{{ t.rbac.developer }}</span>
          <strong class="mini-stat-value">{{ stats.developer }}</strong>
        </div>
        <div class="mini-stat">
          <span class="mini-stat-label">{{ t.rbac.viewer }}</span>
          <strong class="mini-stat-value">{{ stats.viewer }}</strong>
        </div>
      </div>
    </section>

    <section class="panel-toolbar rbac-toolbar">
      <el-input v-model="search" :prefix-icon="Search" :placeholder="searchPlaceholder" clearable size="large" class="rbac-search" />
      <el-select v-model="roleFilter" clearable size="large" class="rbac-role-filter" :placeholder="t.rbac.role">
        <el-option value="admin" :label="t.rbac.admin" />
        <el-option value="developer" :label="t.rbac.developer" />
        <el-option value="viewer" :label="t.rbac.viewer" />
      </el-select>
      <el-button type="primary" size="large" :icon="Plus" @click="handleShowAdd">{{ t.rbac.addUser }}</el-button>
    </section>

    <section v-loading="loading">
      <div v-if="!loading && filteredUsers.length === 0" class="empty-panel">
        <div class="empty-state">
          <el-icon><User /></el-icon>
          <p>{{ noUsersLabel }}</p>
        </div>
      </div>

      <div v-else class="glass-table-wrapper rbac-table-wrap">
        <el-table :data="filteredUsers" style="width: 100%">
          <el-table-column :label="t.rbac.username" min-width="220">
            <template #default="{ row }">
              <div class="user-cell">
                <div class="user-avatar">{{ row.username?.slice(0, 1)?.toUpperCase() || 'U' }}</div>
                <div class="user-meta">
                  <div class="user-name-row">
                    <strong>{{ row.username }}</strong>
                    <el-tag v-if="row.is_builtin" size="small" effect="plain" type="warning">{{ builtInLabel }}</el-tag>
                  </div>
                  <span>{{ row.nickname || t.common.none }}</span>
                </div>
              </div>
            </template>
          </el-table-column>
          <el-table-column :label="t.rbac.role" min-width="120">
            <template #default="{ row }">
              <el-tag :type="roleTagType(row.role)" size="small" effect="dark">{{ roleLabel(row.role) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t.rbac.nickname" prop="nickname" min-width="140">
            <template #default="{ row }">{{ row.nickname || t.common.none }}</template>
          </el-table-column>
          <el-table-column :label="t.rbac.remark" min-width="220">
            <template #default="{ row }">
              <span class="remark-text">{{ row.remark || t.common.none }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="t.common.actions" width="140" fixed="right">
            <template #default="{ row }">
              <div class="rbac-actions">
                <el-button type="primary" link :icon="Edit" @click="handleShowEdit(row)">{{ t.common.edit }}</el-button>
                <el-button v-if="!row.is_builtin" type="danger" link :icon="Lock" @click="deleteUser(row.username)">{{ t.common.delete }}</el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </section>

    <el-dialog v-model="showDialog" :title="isEdit ? t.common.edit : t.rbac.addUser" width="460px" append-to-body>
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
.rbac-mini-stats {
  min-width: 0;
  width: auto;
}

.rbac-toolbar {
  justify-content: space-between;
}

.rbac-search {
  flex: 1;
}

.rbac-role-filter {
  width: 180px;
}

.rbac-table-wrap {
  border-radius: 20px;
}

.user-cell {
  display: flex;
  align-items: center;
  gap: 14px;
}

.user-avatar {
  width: 42px;
  height: 42px;
  display: grid;
  place-items: center;
  border-radius: 14px;
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.16), rgba(6, 182, 212, 0.12));
  color: var(--accent-blue);
  font-weight: 700;
}

.user-meta {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.user-name-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.user-meta span,
.remark-text {
  color: var(--text-secondary);
}

.rbac-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

@media (max-width: 980px) {
  .rbac-mini-stats,
  .rbac-search,
  .rbac-role-filter {
    width: 100%;
    min-width: 0;
  }
}
</style>



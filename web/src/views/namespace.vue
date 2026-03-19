<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import dayjs from 'dayjs'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Edit, Plus, Refresh, Search } from '@element-plus/icons-vue'
import { createNamespace, deleteNamespace, getNamespaces, updateNamespace } from '../api/registry'
import type { Namespace } from '../api/registry'
import { useI18n } from '../utils/i18n'

const { t } = useI18n()
const loading = ref(false)
const namespaces = ref<Namespace[]>([])
const dialogVisible = ref(false)
const isEdit = ref(false)
const search = ref('')
const form = ref<Partial<Namespace>>({
  name: '',
  description: '',
})

const filteredNamespaces = computed(() => {
  const query = search.value.trim().toLowerCase()
  const list = namespaces.value.filter((namespace) => {
    if (!query) return true
    return namespace.name.toLowerCase().includes(query) || (namespace.description || '').toLowerCase().includes(query)
  })

  return [...list].sort((left, right) => {
    if (left.name === 'default') return -1
    if (right.name === 'default') return 1
    return left.name.localeCompare(right.name)
  })
})

const namespaceStats = computed(() => ({
  total: namespaces.value.length,
  custom: namespaces.value.filter((namespace) => namespace.name !== 'default').length,
  system: namespaces.value.some((namespace) => namespace.name === 'default') ? 1 : 0,
}))

async function fetchNamespaces() {
  loading.value = true
  try {
    const response = await getNamespaces()
    namespaces.value = response.data || []
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || error.message)
  } finally {
    loading.value = false
  }
}

function handleAdd() {
  isEdit.value = false
  form.value = { name: '', description: '' }
  dialogVisible.value = true
}

function handleEdit(namespace: Namespace) {
  if (namespace.name === 'default') {
    ElMessage.warning(t.value.namespace.cannotEditDefault)
    return
  }
  isEdit.value = true
  form.value = { ...namespace }
  dialogVisible.value = true
}

async function handleSubmit() {
  if (!form.value.name) return

  try {
    if (isEdit.value) {
      await updateNamespace(form.value)
    } else {
      await createNamespace(form.value)
    }
    ElMessage.success(t.value.common.success)
    dialogVisible.value = false
    await fetchNamespaces()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || error.message)
  }
}

async function handleDelete(namespace: Namespace) {
  if (namespace.name === 'default') {
    ElMessage.warning(t.value.namespace.cannotDeleteDefault)
    return
  }

  try {
    await ElMessageBox.confirm(
      t.value.namespace.deleteConfirm.replace('{name}', namespace.name),
      t.value.common.warning,
      {
        confirmButtonText: t.value.common.confirm,
        cancelButtonText: t.value.common.cancel,
        type: 'warning',
      },
    )
    await deleteNamespace(namespace.name)
    ElMessage.success(t.value.common.success)
    await fetchNamespaces()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.error || error.message)
    }
  }
}

function formatDate(dateStr: string) {
  if (!dateStr) return '-'
  return dayjs(dateStr).format('YYYY-MM-DD HH:mm:ss')
}

function getDescription(namespace: Namespace) {
  if (namespace.name === 'default' && (!namespace.description || namespace.description === 'Default namespace')) {
    return t.value.namespace.defaultDescription
  }
  return namespace.description || t.value.common.none
}

onMounted(fetchNamespaces)
</script>

<template>
  <div class="page-stack namespace-page">
    <section class="compact-hero namespace-hero-compact">
      <div class="hero-copy-compact">
        <p class="hero-kicker">{{ t.nav.namespaces }}</p>
        <h2 class="hero-heading">{{ t.namespace.title }}</h2>
        <p class="hero-desc">{{ t.namespace.subtitle }}</p>
      </div>

      <div class="namespace-action-area">
        <div class="mini-stat-grid namespace-mini-stats">
          <div class="mini-stat">
            <span class="mini-stat-label">{{ t.namespace.total }}</span>
            <strong class="mini-stat-value">{{ namespaceStats.total }}</strong>
          </div>
          <div class="mini-stat">
            <span class="mini-stat-label">{{ t.namespace.custom }}</span>
            <strong class="mini-stat-value">{{ namespaceStats.custom }}</strong>
          </div>
          <div class="mini-stat">
            <span class="mini-stat-label">{{ t.namespace.system }}</span>
            <strong class="mini-stat-value">{{ namespaceStats.system }}</strong>
          </div>
        </div>
        <el-button type="primary" size="large" :icon="Plus" class="namespace-create-btn" @click="handleAdd">
          {{ t.namespace.create }}
        </el-button>
      </div>
    </section>

    <section class="panel-toolbar namespace-toolbar-compact">
      <el-input
        v-model="search"
        :prefix-icon="Search"
        :placeholder="t.namespace.searchPlaceholder"
        clearable
        size="large"
        class="namespace-search"
      />
      <el-button size="large" :icon="Refresh" @click="fetchNamespaces">{{ t.common.refresh }}</el-button>
    </section>

    <section v-loading="loading">
      <div v-if="!loading && filteredNamespaces.length === 0" class="empty-panel">
        <div class="empty-state">
          <el-icon><Search /></el-icon>
          <p>{{ t.namespace.emptyDescription }}</p>
        </div>
      </div>

      <div v-else class="namespace-grid">
        <article v-for="namespace in filteredNamespaces" :key="namespace.name" class="namespace-card" :class="{ default: namespace.name === 'default' }">
          <div class="namespace-card-head">
            <div>
              <div class="namespace-title-row">
                <h3>{{ namespace.name }}</h3>
                <el-tag size="small" effect="plain" :type="namespace.name === 'default' ? 'warning' : 'success'">
                  {{ namespace.name === 'default' ? t.namespace.defaultBadge : t.namespace.customBadge }}
                </el-tag>
              </div>
              <p class="namespace-card-desc">{{ getDescription(namespace) }}</p>
            </div>
            <div class="namespace-card-actions">
              <el-button circle plain :icon="Edit" @click="handleEdit(namespace)" :disabled="namespace.name === 'default'" />
              <el-button circle plain type="danger" :icon="Delete" @click="handleDelete(namespace)" :disabled="namespace.name === 'default'" />
            </div>
          </div>

          <div class="namespace-card-foot">
            <div>
              <div class="meta-label">{{ t.namespace.createdAt }}</div>
              <strong class="meta-value">{{ formatDate(namespace.created_at) }}</strong>
            </div>
            <span v-if="namespace.name === 'default'" class="namespace-protection">{{ t.namespace.lockedHint }}</span>
          </div>
        </article>
      </div>
    </section>

    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? t.namespace.editDialog : t.namespace.createDialog"
      width="560px"
      append-to-body
    >
      <el-form label-position="top">
        <el-form-item :label="t.namespace.name" required>
          <el-input v-model="form.name" :placeholder="t.namespace.namePlaceholder" :disabled="isEdit" />
        </el-form-item>
        <el-form-item :label="t.namespace.description">
          <el-input v-model="form.description" type="textarea" :rows="4" :placeholder="t.namespace.descriptionPlaceholder" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t.common.cancel }}</el-button>
        <el-button type="primary" @click="handleSubmit" :disabled="!form.name">{{ t.common.confirm }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.namespace-hero-compact {
  gap: 18px;
}

.namespace-action-area {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.namespace-mini-stats {
  min-width: 0;
  width: auto;
}

.namespace-create-btn {
  min-width: 160px;
  border-radius: 14px;
  box-shadow: 0 12px 22px rgba(59, 130, 246, 0.2);
}

.namespace-toolbar-compact {
  justify-content: space-between;
}

.namespace-search {
  flex: 1;
}

.namespace-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}

.namespace-card {
  display: flex;
  flex-direction: column;
  gap: 18px;
  min-height: 190px;
  padding: 20px;
  border-radius: 22px;
  border: 1px solid var(--border-color);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.03), transparent),
    var(--bg-card);
  box-shadow: 0 14px 30px rgba(15, 23, 42, 0.08);
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}

.namespace-card:hover {
  transform: translateY(-3px);
  border-color: rgba(59, 130, 246, 0.28);
  box-shadow: 0 20px 36px rgba(15, 23, 42, 0.14);
}

.namespace-card.default {
  background:
    radial-gradient(circle at top right, rgba(245, 158, 11, 0.12), transparent 34%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.03), transparent),
    var(--bg-card);
}

.namespace-card-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}

.namespace-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.namespace-title-row h3 {
  font-size: 24px;
  line-height: 1.1;
}

.namespace-card-desc {
  color: var(--text-secondary);
  line-height: 1.7;
}

.namespace-card-actions {
  display: flex;
  gap: 8px;
}

.namespace-card-foot {
  margin-top: auto;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding-top: 14px;
  border-top: 1px solid var(--border-color);
}

.meta-label {
  margin-bottom: 6px;
  color: var(--text-muted);
  font-size: 11px;
  letter-spacing: 0.16em;
  text-transform: uppercase;
}

.meta-value {
  font-size: 18px;
  line-height: 1.2;
}

.namespace-protection {
  display: inline-flex;
  width: fit-content;
  padding: 8px 12px;
  border-radius: 999px;
  background: rgba(245, 158, 11, 0.12);
  color: #d97706;
  font-size: 12px;
}

@media (max-width: 980px) {
  .namespace-action-area,
  .namespace-create-btn,
  .namespace-mini-stats,
  .namespace-search {
    width: 100%;
    min-width: 0;
  }

  .namespace-action-area {
    align-items: stretch;
  }
}
</style>




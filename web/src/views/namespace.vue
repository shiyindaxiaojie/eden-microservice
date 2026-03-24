<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import dayjs from 'dayjs'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Calendar, Delete, Edit, Grid, List as ListIcon, Lock, Plus, RefreshLeft, Search } from '@element-plus/icons-vue'
import { createNamespace, deleteNamespace, getNamespaces, updateNamespace } from '../api/registry'
import type { Namespace } from '../api/registry'
import { useI18n } from '../utils/i18n'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import en from 'element-plus/es/locale/lang/en'

const { t, locale } = useI18n()
const loading = ref(false)
const namespaces = ref<Namespace[]>([])
const dialogVisible = ref(false)
const isEdit = ref(false)
const search = ref('')
const form = ref<Partial<Namespace>>({
  name: '',
  description: '',
})
const viewMode = ref<'grid' | 'list'>('grid')
const filterMode = ref<'all' | 'system' | 'custom'>('all')
const currentPage = ref(1)
const pageSize = ref(12)

const stats = computed(() => ({
  total: namespaces.value.length,
  system: namespaces.value.filter(ns => ns.name === 'default').length,
  custom: namespaces.value.filter(ns => ns.name !== 'default').length
}))

const filteredNamespaces = computed(() => {
  let list = namespaces.value

  // Filter by stat mode
  if (filterMode.value === 'system') {
    list = list.filter(ns => ns.name === 'default')
  } else if (filterMode.value === 'custom') {
    list = list.filter(ns => ns.name !== 'default')
  }

  const query = search.value.trim().toLowerCase()
  list = list.filter((namespace) => {
    if (!query) return true
    return namespace.name.toLowerCase().includes(query) || (namespace.description || '').toLowerCase().includes(query)
  })

  return [...list].sort((left, right) => {
    if (left.name === 'default') return -1
    if (right.name === 'default') return 1
    return left.name.localeCompare(right.name)
  })
})

const pagedNamespaces = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredNamespaces.value.slice(start, start + pageSize.value)
})


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

function resetFilters() {
  search.value = ''
  filterMode.value = 'all'
  fetchNamespaces()
}

onMounted(fetchNamespaces)
</script>

<template>
  <div class="svc-shell">
    <!-- Toolbar -->
    <div class="svc-toolbar">
      <div class="toolbar-row">
        <div class="toolbar-group">
          <div class="field-item">
            <span class="field-label">{{ locale === 'zh' ? '命名空间名称' : 'Namespace Name' }}</span>
            <el-input
              v-model="search"
              :prefix-icon="Search"
              :placeholder="t.namespace.searchPlaceholder"
              clearable
              class="search-input"
              style="width: 240px"
            />
          </div>
        </div>

        <div class="toolbar-group">
          <button
            type="button"
            class="refresh-btn"
            :title="t.common.refresh"
            @click="resetFilters()"
          >
            <el-icon><RefreshLeft /></el-icon>
          </button>
        </div>
        
        <div class="toolbar-group right-align">
          <div class="pill-group">
            <button type="button" :class="{ active: filterMode === 'all' }" @click="filterMode = 'all'">
              {{ t.common.all || '全部' }}
              <span class="pill-count">{{ stats.total }}</span>
            </button>
            <button type="button" :class="{ active: filterMode === 'system' }" @click="filterMode = 'system'">
              {{ t.namespace.system }}
              <span class="pill-count">{{ stats.system }}</span>
            </button>
            <button type="button" :class="{ active: filterMode === 'custom' }" @click="filterMode = 'custom'">
              {{ t.namespace.custom }}
              <span class="pill-count">{{ stats.custom }}</span>
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

          <el-button type="primary" class="add-btn" :icon="Plus" @click="handleAdd">
            {{ t.namespace.create }}
          </el-button>
        </div>
      </div>
    </div>

    <!-- Content -->
    <section class="svc-content" v-loading="loading">
      <div v-if="!loading && filteredNamespaces.length === 0" class="empty-panel">
        <div class="empty-inner">
          <strong>{{ t.namespace.emptyDescription }}</strong>
          <p>{{ t.namespace.subtitle }}</p>
        </div>
      </div>

      <template v-else>
        <div v-if="viewMode === 'grid'" class="namespace-grid">
          <article v-for="ns in pagedNamespaces" :key="ns.name" class="ns-card" :class="{ is_default: ns.name === 'default' }">
            <div class="ns-card-head">
              <div class="ns-info">
                <h3 class="ns-name">{{ ns.name }}</h3>
                <span v-if="ns.name === 'default'" class="ns-badge default">{{ t.namespace.defaultBadge }}</span>
                <span v-else class="ns-badge">{{ t.namespace.customBadge }}</span>
              </div>
              <div class="ns-actions">
                <el-button link :icon="Edit" @click="handleEdit(ns)" :disabled="ns.name === 'default'">{{ t.common.edit }}</el-button>
                <el-button link type="danger" :icon="Delete" @click="handleDelete(ns)" :disabled="ns.name === 'default'">{{ t.common.delete }}</el-button>
              </div>
            </div>

            <div class="ns-card-body">
              <p class="ns-desc">{{ getDescription(ns) }}</p>
            </div>

            <div class="ns-card-foot">
              <div class="foot-item">
                <el-icon><Calendar /></el-icon>
                <span>{{ formatDate(ns.created_at) }}</span>
              </div>
              <div v-if="ns.name === 'default'" class="ns-lock">
                <el-icon><Lock /></el-icon> {{ t.namespace.lockedHint }}
              </div>
            </div>
          </article>
        </div>

        <div v-else class="table-wrap">
            <el-table :data="pagedNamespaces" height="100%" style="width: 100%; font-size: 14px;">
              <el-table-column type="index" :label="locale === 'zh' ? '序号' : 'No.'" width="60" align="center" />
            <el-table-column :label="t.namespace.name" min-width="180">
              <template #default="{ row }">
                <div class="ns-list-info">
                  <span class="ns-list-name">{{ row.name }}</span>
                  <el-tag v-if="row.name === 'default'" size="small" type="warning" effect="plain">{{ t.namespace.defaultBadge }}</el-tag>
                  <el-tag v-else size="small" type="success" effect="plain">{{ t.namespace.customBadge }}</el-tag>
                </div>
              </template>
            </el-table-column>
            
            <el-table-column :label="t.namespace.description" min-width="300">
              <template #default="{ row }">
                <span class="ns-list-desc">{{ getDescription(row) }}</span>
              </template>
            </el-table-column>

            <el-table-column :label="t.common.createdAt" width="180">
              <template #default="{ row }">
                <div class="foot-item">
                  <el-icon><Calendar /></el-icon>
                  <span>{{ formatDate(row.created_at) }}</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column :label="t.common.actions" width="160" align="center" fixed="right">
              <template #default="{ row }">
                <div class="ns-list-actions">
                  <el-button link :icon="Edit" @click="handleEdit(row)" :disabled="row.name === 'default'">{{ t.common.edit }}</el-button>
                  <el-button link type="danger" :icon="Delete" @click="handleDelete(row)" :disabled="row.name === 'default'">{{ t.common.delete }}</el-button>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <footer class="svc-footer">
          <span class="footer-info">{{ filteredNamespaces.length }} {{ locale === 'zh' ? '条' : 'records' }}</span>
          <el-config-provider :locale="locale === 'zh' ? zhCn : en">
            <el-pagination
              v-model:current-page="currentPage"
              v-model:page-size="pageSize"
              :page-sizes="[12, 20, 50]"
              :total="filteredNamespaces.length"
              layout="sizes, prev, pager, next"
              background
            />
          </el-config-provider>
        </footer>
      </template>
    </section>

    <!-- Dialogs -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? t.namespace.editDialog : t.namespace.createDialog"
      width="460px"
      append-to-body
      class="glass-dialog"
    >
      <el-form label-position="top">
        <el-form-item :label="t.namespace.name" required>
          <el-input v-model="form.name" :placeholder="t.namespace.namePlaceholder" :disabled="isEdit" />
        </el-form-item>
        <el-form-item :label="t.namespace.description">
          <el-input v-model="form.description" type="textarea" :rows="3" :placeholder="t.namespace.descriptionPlaceholder" />
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
  height: 20px;
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
  margin-left: -20px;
}

.refresh-btn:hover {
  color: var(--accent-blue);
  background: rgba(59, 130, 246, 0.08);
}

/* ── Content ── */
.svc-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  padding: 16px 24px 12px;
}

.namespace-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  align-content: start;
  padding: 2px;
}

.table-wrap {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  margin-bottom: 8px;
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

:deep(.search-input .el-input__wrapper) {
  background: rgba(255, 255, 255, 0.04) !important;
  border: 1px solid var(--border-color) !important;
  box-shadow: none !important;
  border-radius: 6px;
  color: var(--text-primary);
  height: 32px;
}
:deep(.search-input .el-input__wrapper:hover),
:deep(.search-input .el-input__wrapper:focus-within) {
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

/* ===== Pill Groups ===== */
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

.pill-sep {
  width: 1px;
  height: 14px;
  background: rgba(255, 255, 255, 0.08);
  margin: 0 4px;
  align-self: center;
}

.icon-pills button {
  padding: 6px 10px;
}

.icon-pills .el-icon {
  font-size: 16px;
}

.refresh-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  display: grid;
  place-items: center;
  border-radius: 8px;
  border: 1px solid var(--border-color);
  background: rgba(255, 255, 255, 0.03);
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.2s;
}

.refresh-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-primary);
  border-color: var(--accent-blue);
}

/* ===== Table Styling ===== */
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

.ns-list-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.ns-list-name {
  font-weight: 600;
  font-size: 15px;
}

.ns-list-desc {
  color: var(--text-secondary);
  font-size: 14px;
}

.namespace-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 24px;
  padding-bottom: 24px;
}

.ns-card {
  display: flex;
  flex-direction: column;
  padding: 24px;
  border-radius: 12px;
  background: var(--bg-primary);
  border: 1px solid rgba(255, 255, 255, 0.05);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  overflow: hidden;
}

.ns-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  width: 4px;
  height: 100%;
  background: var(--accent-blue);
  opacity: 0.6;
}

.ns-card.is_default::before {
  background: var(--accent-orange);
}

.ns-card:hover {
  transform: translateY(-4px);
  border-color: var(--accent-blue);
  box-shadow: 0 12px 24px -8px rgba(0, 0, 0, 0.2), 0 0 0 1px var(--accent-blue);
  background: var(--bg-glass);
}

.ns-card-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 16px;
}

.ns-info {
  display: flex;
  align-items: center;
  gap: 10px;
}

.ns-name {
  font-size: 18px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
}

.ns-badge {
  font-size: 11px;
  padding: 1px 6px;
  background: rgba(16, 185, 129, 0.1);
  color: #10b981;
  border-radius: 4px;
  font-weight: 600;
}

.ns-badge.default {
  background: rgba(245, 158, 11, 0.1);
  color: #f59e0b;
}

.ns-actions {
  display: flex;
  gap: 4px;
  opacity: 0;
  transition: opacity 0.2s;
}

.ns-card:hover .ns-actions {
  opacity: 1;
}

.action-btn {
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 6px;
  background: var(--border-color);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s;
}

.action-btn:hover:not(:disabled) {
  background: var(--accent-blue);
  color: #fff;
}

.action-btn.delete:hover:not(:disabled) {
  background: var(--accent-red);
}

.ns-card-body {
  flex: 1;
  margin-bottom: 20px;
}

.ns-desc {
  font-size: 14px;
  color: var(--text-secondary);
  line-height: 1.6;
  margin: 0;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.ns-card-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-top: 16px;
  border-top: 1px solid var(--border-color);
}

.foot-item {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-muted);
  font-size: 13px;
}

.ns-lock .el-icon { font-size: 12px; }

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

.ns-lock {
  font-size: 12px;
  color: var(--accent-orange);
  display: flex;
  align-items: center;
  gap: 4px;
  font-weight: 500;
}


</style>





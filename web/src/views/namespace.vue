<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { FolderOpened, Grid, List as ListIcon, Plus, RefreshLeft, Search } from '@element-plus/icons-vue'
import {
  createNamespace,
  deleteNamespace,
  getNamespaces,
  updateNamespace,
  type Namespace,
} from '../api/registry'
import { useI18n } from '../utils/i18n'

type ViewMode = 'list' | 'grid'
type FilterMode = 'all' | 'system' | 'custom'
type DialogType = 'add' | 'edit'

interface NamespaceRow {
  name: string
  description: string
  createdAt: string
  updatedAt: string
}

interface QueryForm {
  name: string
}

interface NamespaceForm {
  name: string
  description: string
}

const { text, elementLocale } = useI18n()

const loading = ref(false)
const rows = ref<NamespaceRow[]>([])
const queryForm = ref<QueryForm>({ name: '' })
const dialogVisible = ref(false)
const dialogType = ref<DialogType>('add')
const submitting = ref(false)
const formRef = ref<FormInstance>()
const form = ref<NamespaceForm>({
  name: '',
  description: '',
})
const viewMode = ref<ViewMode>('grid')
const filterMode = ref<FilterMode>('all')
const currentPage = ref(1)
const pageSize = ref(8)
const selectedNamespace = ref('')

const namespaceType = (row: NamespaceRow): Exclude<FilterMode, 'all'> =>
  row.name === 'default' ? 'system' : 'custom'

const namespaceTypeLabel = (row: NamespaceRow) =>
  namespaceType(row) === 'system' ? text('系统', 'System') : text('自定义', 'Custom')

const totalCount = computed(() => rows.value.length)
const systemCount = computed(() => rows.value.filter((row) => namespaceType(row) === 'system').length)
const customCount = computed(() => rows.value.filter((row) => namespaceType(row) === 'custom').length)

const filteredRows = computed(() => {
  const query = queryForm.value.name.trim().toLowerCase()
  let list = rows.value

  if (filterMode.value !== 'all') {
    list = list.filter((row) => namespaceType(row) === filterMode.value)
  }

  if (query) {
    list = list.filter(
      (row) =>
        row.name.toLowerCase().includes(query) ||
        row.description.toLowerCase().includes(query),
    )
  }

  return list
})

const pagedRows = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredRows.value.slice(start, start + pageSize.value)
})

const totalPages = computed(() => Math.max(1, Math.ceil(filteredRows.value.length / pageSize.value)))

const rules: FormRules<NamespaceForm> = {
  name: [{ required: true, message: text('请输入名称', 'Please input name'), trigger: 'blur' }],
}

const emptyTitle = computed(() => text('暂无匹配命名空间', 'No namespaces matched'))
const emptyDesc = computed(() =>
  text('试试调整筛选条件或清空搜索关键词。', 'Try adjusting filters or clearing the search.'),
)

const displayDescription = (row: NamespaceRow) =>
  row.description || text('未填写描述', 'No description')

const displayUpdatedAt = (row: NamespaceRow) =>
  row.updatedAt && row.updatedAt !== '-' ? row.updatedAt : text('未更新', 'Not updated')

const formatDateTime = (value: string) => {
  if (!value || value === '-') return '-'

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value

  const year = date.getFullYear()
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  const hours = `${date.getHours()}`.padStart(2, '0')
  const minutes = `${date.getMinutes()}`.padStart(2, '0')
  const seconds = `${date.getSeconds()}`.padStart(2, '0')

  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
}

const mapNamespace = (item: Namespace): NamespaceRow => ({
  name: item.name,
  description: item.description || '',
  createdAt: formatDateTime(item.created_at || '-'),
  updatedAt: formatDateTime(item.updated_at || '-'),
})

const getErrorMessage = (error: any, fallbackZh: string, fallbackEn: string) =>
  error?.response?.data?.error || text(fallbackZh, fallbackEn)

const fetchData = async () => {
  loading.value = true
  try {
    const response = await getNamespaces()
    rows.value = (response.data || []).map(mapNamespace)
  } catch (error: any) {
    rows.value = []
    ElMessage.error(getErrorMessage(error, '加载命名空间失败', 'Failed to load namespaces'))
  } finally {
    loading.value = false
  }
}

const resetFilters = async () => {
  queryForm.value.name = ''
  filterMode.value = 'all'
  currentPage.value = 1
  await fetchData()
}

const handleAdd = () => {
  dialogType.value = 'add'
  form.value = { name: '', description: '' }
  dialogVisible.value = true
}

const handleEdit = (row: NamespaceRow) => {
  dialogType.value = 'edit'
  form.value = {
    name: row.name,
    description: row.description,
  }
  dialogVisible.value = true
}

const handleDelete = (row: NamespaceRow) => {
  ElMessageBox.confirm(
    text(`确定要删除命名空间 "${row.name}" 吗？`, `Are you sure you want to delete namespace "${row.name}"?`),
    text('提示', 'Notice'),
    {
      confirmButtonText: text('确定', 'Confirm'),
      cancelButtonText: text('取消', 'Cancel'),
      type: 'warning',
    },
  ).then(async () => {
    try {
      await deleteNamespace(row.name)
      ElMessage.success(text('删除成功', 'Deleted successfully'))
      await fetchData()
    } catch (error: any) {
      ElMessage.error(getErrorMessage(error, '删除命名空间失败', 'Failed to delete namespace'))
    }
  }).catch(() => undefined)
}

const submitForm = async () => {
  if (!formRef.value) return

  try {
    await formRef.value.validate()
    submitting.value = true

    const payload = {
      name: form.value.name,
      description: form.value.description,
    }

    if (dialogType.value === 'add') {
      await createNamespace(payload)
    } else {
      await updateNamespace(payload)
    }

    ElMessage.success(
      text(
        dialogType.value === 'add' ? '创建成功' : '编辑成功',
        dialogType.value === 'add' ? 'Created successfully' : 'Updated successfully',
      ),
    )

    dialogVisible.value = false
    await fetchData()
  } catch (error: any) {
    if (error?.response) {
      ElMessage.error(
        getErrorMessage(
          error,
          dialogType.value === 'add' ? '创建命名空间失败' : '更新命名空间失败',
          dialogType.value === 'add' ? 'Failed to create namespace' : 'Failed to update namespace',
        ),
      )
    }
  } finally {
    submitting.value = false
  }
}

watch([() => queryForm.value.name, filterMode], () => {
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
              <span class="field-label">{{ text('名称', 'Name') }}</span>
              <el-input
                v-model="queryForm.name"
                :prefix-icon="Search"
                :placeholder="text('输入名称', 'Input name')"
                clearable
                class="search-input"
              />
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
                <span class="pill-count">{{ totalCount }}</span>
              </button>
              <button type="button" :class="{ active: filterMode === 'system' }" @click="filterMode = 'system'">
                {{ text('系统', 'System') }}
                <span class="pill-count">{{ systemCount }}</span>
              </button>
              <button type="button" :class="{ active: filterMode === 'custom' }" @click="filterMode = 'custom'">
                {{ text('自定义', 'Custom') }}
                <span class="pill-count">{{ customCount }}</span>
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
              {{ text('新建命名空间', 'New') }}
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
              <el-table-column prop="name" :label="text('名称', 'Name')" min-width="160" />
              <el-table-column :label="text('描述', 'Description')" min-width="260" show-overflow-tooltip>
                <template #default="{ row }">
                  {{ displayDescription(row) }}
                </template>
              </el-table-column>
              <el-table-column :label="text('类型', 'Type')" width="120">
                <template #default="{ row }">
                  <el-tag :type="namespaceType(row) === 'system' ? 'warning' : 'success'" size="small" effect="plain">
                    {{ namespaceTypeLabel(row) }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="createdAt" :label="text('创建时间', 'Created At')" width="190" />
              <el-table-column :label="text('操作', 'Actions')" width="150" fixed="right">
                <template #default="{ row }">
                  <el-button link type="primary" @click="handleEdit(row)">{{ text('编辑', 'Edit') }}</el-button>
                  <el-button
                    v-if="row.name !== 'default'"
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
              :key="row.name"
              class="info-card"
              tabindex="0"
              @click="selectedNamespace = row.name"
              @keydown.enter.prevent="selectedNamespace = row.name"
              @keydown.space.prevent="selectedNamespace = row.name"
              :class="{ 'system-card': namespaceType(row) === 'system', 'is-selected': selectedNamespace === row.name }"
            >
              <div class="card-accent"></div>

              <div class="card-head">
                <div class="card-identity">
                  <div class="card-symbol">
                    <el-icon><FolderOpened /></el-icon>
                  </div>
                  <div class="card-copy">
                    <div class="card-title-row">
                      <strong class="card-title">{{ row.name }}</strong>
                      <el-tag :type="namespaceType(row) === 'system' ? 'warning' : 'success'" size="small" effect="plain">
                        {{ namespaceTypeLabel(row) }}
                      </el-tag>
                    </div>
                    <p class="card-subtitle">{{ displayDescription(row) }}</p>
                  </div>
                </div>
              </div>

              <div class="card-body">
                <div class="compact-meta-row">
                  <span class="compact-label">{{ text('创建', 'Created') }}</span>
                  <span class="compact-value">{{ row.createdAt }}</span>
                </div>
                <div class="compact-meta-row">
                  <span class="compact-label">{{ text('更新', 'Updated') }}</span>
                  <span class="compact-value">{{ displayUpdatedAt(row) }}</span>
                </div>
              </div>

              <div class="card-footer">
                <div class="card-actions">
                  <el-button link type="primary" @click="handleEdit(row)">{{ text('编辑', 'Edit') }}</el-button>
                  <el-button
                    v-if="row.name !== 'default'"
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
            <el-config-provider :locale="elementLocale">
              <el-pagination
                v-model:current-page="currentPage"
                v-model:page-size="pageSize"
                :page-sizes="[8, 16, 32, 64]"
                :total="filteredRows.length"
                layout="sizes, prev, pager, next"
                background
              />
            </el-config-provider>
          </footer>
        </template>
      </section>
    </div>

    <el-drawer
      v-model="dialogVisible"
      :title="dialogType === 'add' ? text('新建命名空间', 'New') : text('编辑', 'Edit')"
      direction="rtl"
      size="460px"
      class="form-drawer"
    >
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
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
    </el-drawer>
  </div>
</template>

<style scoped lang="scss">
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
  background: transparent;
  backdrop-filter: none;
  border: 0;
  border-radius: 0;
}

.svc-toolbar {
  flex-shrink: 0;
  padding: 0 0 18px;
  border-bottom: 1px solid var(--border-color);
}

.toolbar-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px 0;
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
  background: var(--control-muted-bg);
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
  padding: 18px 0 0;
}

.empty-panel {
  flex: 1;
  display: grid;
  place-items: center;
  min-height: 200px;
  padding: 0;
  background: transparent;
  border: 0;
  border-radius: 0;
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

:deep(.search-input .el-input__inner) {
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

.card-grid {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  justify-content: stretch;
  gap: 12px;
  align-content: start;
  padding: 2px 2px 8px;
}

.info-card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px;
  min-height: 160px;
  border-radius: 10px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  box-shadow: none;
  overflow: hidden;
  transition: border-color 0.2s ease, background-color 0.2s ease, box-shadow 0.2s ease;
}

.info-card:hover {
  border-color: rgba(59, 130, 246, 0.32);
  background: rgba(59, 130, 246, 0.03);
  box-shadow: inset 0 0 0 1px rgba(59, 130, 246, 0.08);
}

.info-card:focus-within,
.info-card:active {
  border-color: rgba(59, 130, 246, 0.52);
  background: rgba(59, 130, 246, 0.045);
  box-shadow: inset 0 0 0 1px rgba(59, 130, 246, 0.16);
}

.info-card.is-selected {
  border-color: rgba(59, 130, 246, 0.52);
  background: rgba(59, 130, 246, 0.045);
  box-shadow: inset 0 0 0 1px rgba(59, 130, 246, 0.16);
}

.card-accent {
  position: absolute;
  inset: 0 auto auto 0;
  width: 84px;
  height: 3px;
  background: rgba(59, 130, 246, 0.35);
  border-radius: 0 0 999px 0;
}

.system-card {
  border-color: rgba(245, 158, 11, 0.22);
}

.system-card .card-accent {
  background: rgba(245, 158, 11, 0.4);
}

.card-head {
  position: relative;
  z-index: 1;
}

.card-identity {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  min-width: 0;
}

.card-symbol {
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  background: rgba(59, 130, 246, 0.12);
  border: 1px solid rgba(59, 130, 246, 0.12);
  color: var(--accent-blue);
  font-size: 18px;
}

.system-card .card-symbol {
  background: rgba(245, 158, 11, 0.12);
  border-color: rgba(245, 158, 11, 0.14);
  color: var(--accent-orange);
}

.card-copy {
  min-width: 0;
  flex: 1;
}

.card-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 4px;
}

.card-title {
  display: block;
  color: var(--text-primary);
  font-size: 15px;
  font-weight: 800;
  line-height: 1.15;
}

.card-subtitle {
  margin: 0;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.45;
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.card-body {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.compact-meta-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  min-width: 0;
  font-size: 12px;
  line-height: 1.45;
}

.compact-label {
  flex: 0 0 40px;
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
}

.compact-value {
  min-width: 0;
  color: var(--text-primary);
  font-size: 12px;
  line-height: 1.5;
  word-break: break-word;
}

.card-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--border-color);
}

.card-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}

:deep(.card-actions .el-button) {
  min-height: 24px;
  padding: 2px 4px;
  font-size: 12px !important;
  font-weight: 600;
  line-height: 1.35;
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

@media (max-width: 1180px) {
  .card-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

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
    padding: 0 0 14px;
  }

  .svc-content {
    padding: 14px 0 0;
  }

  .field-item {
    flex-wrap: wrap;
    width: 100%;
  }

  .search-input {
    width: 100% !important;
  }

  .pill-group {
    width: 100%;
  }

  .pill-group button {
    flex: 1;
    justify-content: center;
  }

  .card-grid,
  .single-card-grid {
    grid-template-columns: 1fr;
  }

  .compact-meta-row {
    flex-direction: column;
    gap: 4px;
  }

  .compact-label {
    flex-basis: auto;
  }

  .svc-footer {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>

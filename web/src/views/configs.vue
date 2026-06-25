<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Clock,
  Delete,
  EditPen,
  Plus,
  RefreshLeft,
  Search,
  Upload,
} from '@element-plus/icons-vue'
import {
  deleteMockConfig,
  listMockConfigHistory,
  listMockConfigs,
  saveMockConfig,
  type ConfigContentType,
  type ConfigHistoryEntry,
  type ConfigResource,
} from '../api/mock/control-plane'
import { useI18n } from '../utils/i18n'

type ConfigFilter = 'all' | ConfigContentType

interface ConfigForm {
  namespace: string
  group: string
  data_id: string
  type: ConfigContentType
  description: string
  tagsText: string
  content: string
}

const { text, elementLocale } = useI18n()

const loading = ref(false)
const configs = ref<ConfigResource[]>([])
const history = ref<ConfigHistoryEntry[]>([])
const query = ref('')
const namespaceFilter = ref('all')
const typeFilter = ref<ConfigFilter>('all')
const selectedKey = ref('')
const editorVisible = ref(false)
const historyVisible = ref(false)
const editorMode = ref<'create' | 'edit'>('create')
const submitting = ref(false)
const currentPage = ref(1)
const pageSize = ref(12)

const form = ref<ConfigForm>({
  namespace: 'default',
  group: 'DEFAULT_GROUP',
  data_id: '',
  type: 'yaml',
  description: '',
  tagsText: '',
  content: '',
})

const typeOptions: ConfigContentType[] = ['yaml', 'properties', 'json', 'text']

const configKey = (item: Pick<ConfigResource, 'namespace' | 'group' | 'data_id'>) =>
  `${item.namespace}@@${item.group}@@${item.data_id}`

const namespaceOptions = computed(() => {
  const values = new Set(['default', 'prod', 'dev'])
  configs.value.forEach((item) => values.add(item.namespace))
  return [...values]
})

const filteredConfigs = computed(() => {
  const term = query.value.trim().toLowerCase()

  return configs.value.filter((item) => {
    const matchesNamespace = namespaceFilter.value === 'all' || item.namespace === namespaceFilter.value
    const matchesType = typeFilter.value === 'all' || item.type === typeFilter.value
    const matchesTerm =
      !term ||
      item.data_id.toLowerCase().includes(term) ||
      item.group.toLowerCase().includes(term) ||
      item.description.toLowerCase().includes(term) ||
      item.tags.some((tag) => tag.toLowerCase().includes(term))

    return matchesNamespace && matchesType && matchesTerm
  })
})

const pagedConfigs = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredConfigs.value.slice(start, start + pageSize.value)
})

const selectedConfig = computed(() =>
  configs.value.find((item) => configKey(item) === selectedKey.value) || filteredConfigs.value[0] || null,
)

const selectedHistory = computed(() => {
  const selected = selectedConfig.value
  if (!selected) return []
  const key = configKey(selected)
  return history.value.filter((item) => configKey(item) === key)
})

const totalCount = computed(() => configs.value.length)
const typeCount = (type: ConfigContentType) => configs.value.filter((item) => item.type === type).length
const totalPages = computed(() => Math.max(1, Math.ceil(filteredConfigs.value.length / pageSize.value)))

function formatDateTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value || '-'
  const pad = (input: number) => `${input}`.padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function md5Short(md5: string) {
  return md5 ? `${md5.slice(0, 8)}...${md5.slice(-6)}` : '-'
}

function typeTag(type: ConfigContentType) {
  if (type === 'yaml') return 'success'
  if (type === 'json') return 'warning'
  if (type === 'properties') return 'primary'
  return 'info'
}

function actionLabel(action: ConfigHistoryEntry['action']) {
  if (action === 'delete') return text('删除', 'Delete')
  if (action === 'rollback') return text('回滚', 'Rollback')
  return text('发布', 'Publish')
}

async function fetchConfigs() {
  loading.value = true
  try {
    const [configRows, historyRows] = await Promise.all([
      listMockConfigs(),
      listMockConfigHistory(),
    ])
    configs.value = configRows.sort((left, right) => right.updated_at.localeCompare(left.updated_at))
    history.value = historyRows.sort((left, right) => right.created_at.localeCompare(left.created_at))

    if (!selectedKey.value || !configs.value.find((item) => configKey(item) === selectedKey.value)) {
      selectedKey.value = configs.value[0] ? configKey(configs.value[0]) : ''
    }
  } finally {
    loading.value = false
  }
}

function resetFilters() {
  query.value = ''
  namespaceFilter.value = 'all'
  typeFilter.value = 'all'
  currentPage.value = 1
}

function openCreate() {
  editorMode.value = 'create'
  form.value = {
    namespace: namespaceFilter.value === 'all' ? 'default' : namespaceFilter.value,
    group: 'DEFAULT_GROUP',
    data_id: '',
    type: 'yaml',
    description: '',
    tagsText: '',
    content: 'server:\n  port: 8080\nfeature:\n  enabled: true\n',
  }
  editorVisible.value = true
}

function openEdit(item: ConfigResource) {
  editorMode.value = 'edit'
  form.value = {
    namespace: item.namespace,
    group: item.group,
    data_id: item.data_id,
    type: item.type,
    description: item.description,
    tagsText: item.tags.join(', '),
    content: item.content,
  }
  editorVisible.value = true
}

async function submitConfig() {
  const payload = form.value
  if (!payload.data_id.trim()) {
    ElMessage.warning(text('请填写 Data ID', 'Data ID is required'))
    return
  }
  if (!payload.group.trim()) {
    ElMessage.warning(text('请填写 Group', 'Group is required'))
    return
  }

  try {
    await ElMessageBox.confirm(
      text('确认发布当前配置内容吗？发布后会生成新的 revision 和 md5。', 'Publish this config content? A new revision and md5 will be generated.'),
      text('发布配置', 'Publish config'),
      {
        confirmButtonText: text('发布', 'Publish'),
        cancelButtonText: text('取消', 'Cancel'),
        type: 'warning',
      },
    )

    submitting.value = true
    const saved = await saveMockConfig({
      namespace: payload.namespace,
      group: payload.group,
      data_id: payload.data_id,
      type: payload.type,
      description: payload.description,
      tags: payload.tagsText.split(','),
      content: payload.content,
    })
    selectedKey.value = configKey(saved)
    editorVisible.value = false
    ElMessage.success(text('配置已发布', 'Config published'))
    await fetchConfigs()
  } catch (error) {
    if (error !== 'cancel') {
      console.error(error)
    }
  } finally {
    submitting.value = false
  }
}

async function handleDelete(item: ConfigResource) {
  try {
    await ElMessageBox.confirm(
      text(`确定删除配置 "${item.data_id}" 吗？历史版本会保留。`, `Delete config "${item.data_id}"? History will be kept.`),
      text('删除配置', 'Delete config'),
      {
        confirmButtonText: text('删除', 'Delete'),
        cancelButtonText: text('取消', 'Cancel'),
        type: 'warning',
      },
    )
    await deleteMockConfig(item)
    ElMessage.success(text('配置已删除', 'Config deleted'))
    await fetchConfigs()
  } catch (error) {
    if (error !== 'cancel') {
      console.error(error)
    }
  }
}

function showHistory(item: ConfigResource) {
  selectedKey.value = configKey(item)
  historyVisible.value = true
}

watch([query, namespaceFilter, typeFilter], () => {
  currentPage.value = 1
})

watch(pageSize, () => {
  currentPage.value = 1
})

watch(filteredConfigs, () => {
  if (currentPage.value > totalPages.value) currentPage.value = totalPages.value
})

onMounted(fetchConfigs)
</script>

<template>
  <div class="control-shell">
    <section class="control-panel">
      <div class="control-toolbar">
        <div class="field-item">
          <span class="field-label">{{ text('命名空间', 'Namespace') }}</span>
          <el-select v-model="namespaceFilter" class="small-select">
            <el-option :label="text('全部', 'All')" value="all" />
            <el-option v-for="namespace in namespaceOptions" :key="namespace" :label="namespace" :value="namespace" />
          </el-select>
        </div>

        <div class="field-item">
          <span class="field-label">{{ text('搜索', 'Search') }}</span>
          <el-input
            v-model="query"
            :prefix-icon="Search"
            clearable
            class="search-input"
            :placeholder="text('Data ID / Group / 标签', 'Data ID / Group / tag')"
          />
        </div>

        <button class="icon-btn" type="button" :title="text('重置筛选', 'Reset filters')" @click="resetFilters">
          <el-icon><RefreshLeft /></el-icon>
        </button>

        <div class="toolbar-spacer"></div>

        <div class="pill-group">
          <button type="button" :class="{ active: typeFilter === 'all' }" @click="typeFilter = 'all'">
            {{ text('全部', 'All') }}
            <span>{{ totalCount }}</span>
          </button>
          <button
            v-for="type in typeOptions"
            :key="type"
            type="button"
            :class="{ active: typeFilter === type }"
            @click="typeFilter = type"
          >
            {{ type }}
            <span>{{ typeCount(type) }}</span>
          </button>
        </div>

        <el-button type="primary" :icon="Plus" @click="openCreate">
          {{ text('新建配置', 'New config') }}
        </el-button>
      </div>

      <div class="config-workbench" v-loading="loading">
        <div class="table-wrap">
          <el-table
            :data="pagedConfigs"
            height="100%"
            highlight-current-row
          >
            <el-table-column :label="text('Data ID', 'Data ID')" min-width="210">
              <template #default="{ row }">
                <div class="primary-cell">
                  <strong>{{ row.data_id }}</strong>
                  <span>{{ row.namespace }} / {{ row.group }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column :label="text('类型', 'Type')" width="120">
              <template #default="{ row }">
                <el-tag :type="typeTag(row.type)" effect="plain">{{ row.type }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="text('Revision', 'Revision')" width="110">
              <template #default="{ row }">
                <span class="revision-badge">r{{ row.revision }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="text('MD5', 'MD5')" min-width="180">
              <template #default="{ row }">
                <code class="mono-chip">{{ md5Short(row.md5) }}</code>
              </template>
            </el-table-column>
            <el-table-column :label="text('更新时间', 'Updated')" width="170">
              <template #default="{ row }">{{ formatDateTime(row.updated_at) }}</template>
            </el-table-column>
            <el-table-column :label="text('操作', 'Actions')" width="190" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" :icon="EditPen" @click.stop="openEdit(row)">
                  {{ text('编辑', 'Edit') }}
                </el-button>
                <el-button link :icon="Clock" @click.stop="showHistory(row)">
                  {{ text('历史', 'History') }}
                </el-button>
                <el-button link type="danger" :icon="Delete" @click.stop="handleDelete(row)">
                  {{ text('删除', 'Delete') }}
                </el-button>
              </template>
            </el-table-column>
          </el-table>

          <footer class="control-footer">
            <span>{{ filteredConfigs.length }} {{ text('条', 'records') }}</span>
            <el-config-provider :locale="elementLocale">
              <el-pagination
                v-model:current-page="currentPage"
                v-model:page-size="pageSize"
                :page-sizes="[12, 20, 50]"
                :total="filteredConfigs.length"
                layout="sizes, prev, pager, next"
                background
              />
            </el-config-provider>
          </footer>
        </div>

      </div>
    </section>

    <el-drawer v-model="editorVisible" :title="editorMode === 'create' ? text('新建配置', 'New config') : text('发布配置', 'Publish config')" size="620px">
      <div class="drawer-form">
        <el-form label-position="top">
          <div class="form-grid">
            <el-form-item :label="text('命名空间', 'Namespace')">
              <el-select v-model="form.namespace" filterable allow-create>
                <el-option v-for="namespace in namespaceOptions" :key="namespace" :label="namespace" :value="namespace" />
              </el-select>
            </el-form-item>
            <el-form-item label="Group">
              <el-input v-model="form.group" />
            </el-form-item>
          </div>
          <el-form-item label="Data ID">
            <el-input v-model="form.data_id" :disabled="editorMode === 'edit'" />
          </el-form-item>
          <div class="form-grid">
            <el-form-item :label="text('类型', 'Type')">
              <el-select v-model="form.type">
                <el-option v-for="type in typeOptions" :key="type" :label="type" :value="type" />
              </el-select>
            </el-form-item>
            <el-form-item :label="text('标签', 'Tags')">
              <el-input v-model="form.tagsText" :placeholder="text('逗号分隔', 'Comma separated')" />
            </el-form-item>
          </div>
          <el-form-item :label="text('描述', 'Description')">
            <el-input v-model="form.description" />
          </el-form-item>
          <el-form-item :label="text('配置内容', 'Content')">
            <el-input v-model="form.content" type="textarea" :rows="18" resize="none" class="code-textarea" />
          </el-form-item>
        </el-form>
      </div>
      <template #footer>
        <div class="drawer-footer">
          <el-button @click="editorVisible = false">{{ text('取消', 'Cancel') }}</el-button>
          <el-button type="primary" :icon="Upload" :loading="submitting" @click="submitConfig">
            {{ text('发布', 'Publish') }}
          </el-button>
        </div>
      </template>
    </el-drawer>

    <el-drawer v-model="historyVisible" :title="text('配置历史', 'Config history')" size="520px">
      <div class="history-list">
        <article v-for="item in selectedHistory" :key="`${item.revision}-${item.created_at}`" class="history-item">
          <div class="history-dot"></div>
          <div class="history-body">
            <div class="history-title">
              <strong>r{{ item.revision }} · {{ actionLabel(item.action) }}</strong>
              <span>{{ formatDateTime(item.created_at) }}</span>
            </div>
            <p>{{ item.summary }} · {{ item.operator }}</p>
            <code>{{ md5Short(item.md5) }}</code>
          </div>
        </article>
      </div>
    </el-drawer>
  </div>
</template>

<style scoped>
.control-shell {
  display: flex;
  flex-direction: column;
  gap: 0;
  height: calc(100vh - var(--header-height) - 48px);
  min-height: 0;
}

.control-hero {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  padding: 16px 18px;
  border: 1px solid var(--border-color);
  background: var(--bg-card);
  border-radius: 0;
}

.hero-copy {
  min-width: 0;
}

.hero-kicker,
.detail-kicker {
  display: block;
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.hero-copy h2 {
  margin: 4px 0 4px;
  color: var(--text-primary);
  font-size: 22px;
  line-height: 1.15;
}

.hero-copy p {
  margin: 0;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.55;
}

.hero-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(92px, 1fr));
  gap: 10px;
  min-width: 330px;
}

.mini-stat {
  padding: 12px;
  border: 1px solid var(--border-color);
  background: rgba(255, 255, 255, 0.03);
  border-radius: 8px;
}

.mini-stat span {
  display: block;
  color: var(--text-muted);
  font-size: 12px;
  margin-bottom: 6px;
}

.mini-stat strong {
  color: var(--text-primary);
  font-size: 22px;
  font-variant-numeric: tabular-nums;
}

.control-panel {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  border: 0;
  background: transparent;
  border-radius: 0;
  overflow: hidden;
}

.control-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  padding: 0 0 18px;
  border-bottom: 1px solid var(--border-color);
}

.toolbar-spacer {
  flex: 1;
}

.field-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.field-label {
  color: var(--text-secondary);
  font-size: 13px;
  font-weight: 700;
  white-space: nowrap;
}

.small-select {
  width: 150px;
}

.search-input {
  width: 260px;
}

.icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  padding: 0;
  border: 0;
  border-radius: 6px;
  color: var(--text-muted);
  background: transparent;
  cursor: pointer;
}

.icon-btn:hover {
  color: var(--accent-blue);
  background: rgba(59, 130, 246, 0.08);
}

.pill-group {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 2px;
  border-radius: 6px;
  background: rgba(148, 163, 184, 0.1);
}

.pill-group button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 30px;
  padding: 0 10px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--text-muted);
  font-family: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.pill-group button.active {
  color: var(--accent-blue);
  background: rgba(59, 130, 246, 0.12);
}

.pill-group span {
  opacity: 0.65;
  font-variant-numeric: tabular-nums;
}

.config-workbench {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.table-wrap {
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding: 18px 0 0;
  overflow: hidden;
}

:deep(.el-table) {
  --el-table-bg-color: transparent;
  --el-table-tr-bg-color: transparent;
  --el-table-row-hover-bg-color: rgba(59, 130, 246, 0.04);
  --el-table-border-color: var(--border-color);
  --el-table-header-bg-color: transparent;
  --el-table-header-text-color: var(--text-muted);
  --el-table-text-color: var(--text-primary);
}

:deep(.el-table__inner-wrapper::before) {
  display: none;
}

:deep(.selected-row td.el-table__cell) {
  background: rgba(59, 130, 246, 0.06) !important;
}

.primary-cell {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.primary-cell strong {
  color: var(--text-primary);
  font-weight: 800;
}

.primary-cell span {
  color: var(--text-muted);
  font-size: 12px;
}

.revision-badge,
.mono-chip {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  padding: 0 8px;
  border-radius: 6px;
  background: rgba(59, 130, 246, 0.08);
  color: var(--accent-blue);
  font-size: 12px;
  font-weight: 700;
}

.mono-chip,
.fingerprint code,
.history-item code,
.content-preview {
  font-family: 'JetBrains Mono', 'Fira Code', Menlo, Consolas, monospace;
}

.control-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-top: 10px;
  color: var(--text-muted);
  font-size: 13px;
}

.detail-panel {
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px;
  border-left: 1px solid var(--border-color);
  background: rgba(255, 255, 255, 0.02);
  overflow: hidden;
}

.detail-head {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.detail-icon {
  width: 42px;
  height: 42px;
  flex-shrink: 0;
  display: grid;
  place-items: center;
  border-radius: 8px;
  color: var(--accent-blue);
  background: rgba(59, 130, 246, 0.12);
}

.detail-head h3 {
  margin: 2px 0 0;
  color: var(--text-primary);
  font-size: 18px;
  line-height: 1.25;
  overflow-wrap: anywhere;
}

.detail-meta {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.detail-meta div {
  min-width: 0;
  padding: 10px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--bg-secondary);
}

.detail-meta span,
.fingerprint span {
  display: block;
  color: var(--text-muted);
  font-size: 11px;
  margin-bottom: 5px;
}

.detail-meta strong {
  color: var(--text-primary);
  font-size: 13px;
  overflow-wrap: anywhere;
}

.fingerprint {
  padding: 10px 12px;
  border: 1px solid rgba(59, 130, 246, 0.16);
  border-radius: 8px;
  background: rgba(59, 130, 246, 0.06);
}

.fingerprint code {
  color: var(--accent-blue);
  font-size: 12px;
  overflow-wrap: anywhere;
}

.tag-row {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.detail-desc {
  margin: 0;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.content-preview {
  flex: 1;
  min-height: 0;
  margin: 0;
  padding: 14px;
  overflow: auto;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--bg-primary);
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.65;
  white-space: pre-wrap;
}

.detail-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.drawer-form {
  padding-right: 6px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.code-textarea :deep(.el-textarea__inner) {
  font-family: 'JetBrains Mono', 'Fira Code', Menlo, Consolas, monospace;
  line-height: 1.6;
}

.drawer-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.history-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.history-item {
  display: grid;
  grid-template-columns: 12px minmax(0, 1fr);
  gap: 12px;
  padding: 12px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--bg-secondary);
}

.history-dot {
  width: 10px;
  height: 10px;
  margin-top: 5px;
  border-radius: 50%;
  background: var(--accent-blue);
}

.history-title {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  color: var(--text-primary);
  font-size: 13px;
}

.history-title span,
.history-body p {
  color: var(--text-muted);
  font-size: 12px;
}

.history-body p {
  margin: 6px 0;
}

.history-body code {
  color: var(--accent-blue);
  font-size: 12px;
}

@media (max-width: 1200px) {
  .control-hero,
  .control-toolbar {
    flex-wrap: wrap;
  }

  .hero-stats,
  .toolbar-spacer {
    min-width: 0;
    width: 100%;
  }

  .config-workbench {
    grid-template-columns: 1fr;
  }

  .detail-panel {
    border-left: 0;
    border-top: 1px solid var(--border-color);
    max-height: 420px;
  }
}

@media (max-width: 760px) {
  .control-shell {
    height: auto;
    min-height: calc(100vh - var(--header-height) - 48px);
  }

  .hero-stats,
  .detail-meta,
  .form-grid {
    grid-template-columns: 1fr;
  }

  .field-item,
  .search-input,
  .small-select,
  .pill-group {
    width: 100%;
  }

  .control-toolbar {
    align-items: stretch;
  }

  .control-footer,
  .detail-actions {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>

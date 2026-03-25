<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Bell,
  Connection,
  CopyDocument,
  Delete,
  Document,
  Grid,
  InfoFilled,
  Key,
  Message,
  Monitor,
  Operation,
  Plus,
  RefreshRight,
  Setting,
  Share,
  Timer,
} from '@element-plus/icons-vue'
import api from '../api'
import { getSystemSettings, updateSystemSettings, type SystemSettings } from '../api/registry'

type CredentialView = 'grid' | 'list'

interface ApiKeyItem {
  key: string
  description?: string
  created_at: number
  expires_at: number
  status?: string
}

const activeTab = ref('basic')
const credentialView = ref<CredentialView>('grid')

const systemLoading = ref(false)
const saveLoading = ref(false)
const keysLoading = ref(false)
const showDialog = ref(false)

const keys = ref<ApiKeyItem[]>([])
const keyForm = ref({
  key: '',
  description: '',
  expDays: 0,
})

const webhookForm = ref({
  url: '',
  events: ['service_register', 'service_offline'],
})

const alarmForm = ref({
  enabled: false,
  threshold: 80,
  contact: 'email',
})

const smtpForm = ref({
  host: 'smtp.example.com',
  port: 465,
  user: '',
  pass: '',
  from: '',
})

const EVENT_OPTIONS = [
  { value: 'service_register', label: '服务注册' },
  { value: 'service_online', label: '服务上线' },
  { value: 'service_offline', label: '服务下线' },
  { value: 'registry_node_sync', label: '节点同步' },
  { value: 'service_heartbeat', label: '服务心跳' },
  { value: 'service_remove', label: '服务移除' },
]

const LOG_LEVEL_OPTIONS = ['DEBUG', 'INFO', 'WARN', 'ERROR'] as const

function createDefaultSettings(): SystemSettings {
  return {
    mode: 'ap',
    environment: 'standalone',
    log_level: 'INFO',
    event_retention_days: 30,
    log_retention_days: 30,
    event_types: EVENT_OPTIONS.map((item) => item.value),
    heartbeat_max_failures: 3,
    instance_removal_delay_seconds: 600,
  }
}

function cloneSettings(settings: SystemSettings): SystemSettings {
  return {
    ...settings,
    event_types: [...settings.event_types],
  }
}

function normalizeEventTypes(values?: string[]): string[] {
  if (!values) return []
  const valid = new Set(EVENT_OPTIONS.map((item) => item.value))
  const seen = new Set<string>()
  return values.filter((value) => {
    if (!valid.has(value) || seen.has(value)) {
      return false
    }
    seen.add(value)
    return true
  })
}

function normalizeLogLevel(level?: string | null): string {
  if (level === 'TRACE') return 'DEBUG'
  if (level === 'FATAL') return 'ERROR'
  return LOG_LEVEL_OPTIONS.includes((level || '') as (typeof LOG_LEVEL_OPTIONS)[number]) ? level! : 'INFO'
}

function normalizeSettings(input?: Partial<SystemSettings> | null): SystemSettings {
  const defaults = createDefaultSettings()
  return {
    mode: input?.mode === 'cp' ? 'cp' : defaults.mode,
    environment: input?.environment === 'cluster' ? 'cluster' : defaults.environment,
    log_level: normalizeLogLevel(input?.log_level ?? defaults.log_level),
    event_retention_days: Math.max(1, input?.event_retention_days ?? defaults.event_retention_days),
    log_retention_days: Math.max(1, input?.log_retention_days ?? defaults.log_retention_days),
    event_types: normalizeEventTypes(input?.event_types ?? defaults.event_types),
    heartbeat_max_failures: Math.max(1, input?.heartbeat_max_failures ?? defaults.heartbeat_max_failures),
    instance_removal_delay_seconds: Math.max(60, input?.instance_removal_delay_seconds ?? defaults.instance_removal_delay_seconds),
  }
}

const appliedSettings = ref<SystemSettings>(createDefaultSettings())
const draftSettings = ref<SystemSettings>(createDefaultSettings())

const previewEnvironment = computed(() => draftSettings.value.environment)
const previewMode = computed(() => draftSettings.value.mode)
const appliedEnvironment = computed(() => appliedSettings.value.environment)
const appliedMode = computed(() => appliedSettings.value.mode)

const removalDelayMinutes = computed({
  get: () => Math.max(1, Math.round(draftSettings.value.instance_removal_delay_seconds / 60)),
  set: (value: number) => {
    draftSettings.value.instance_removal_delay_seconds = Math.max(60, value * 60)
  },
})

const isDirty = computed(() => {
  const normalizeForCompare = (settings: SystemSettings) => ({
    ...settings,
    event_types: [...settings.event_types].sort(),
  })
  return JSON.stringify(normalizeForCompare(draftSettings.value)) !== JSON.stringify(normalizeForCompare(appliedSettings.value))
})

async function fetchSystemConfig() {
  systemLoading.value = true
  try {
    const { data } = await getSystemSettings()
    const normalized = normalizeSettings(data)
    appliedSettings.value = cloneSettings(normalized)
    draftSettings.value = cloneSettings(normalized)
  } catch (error) {
    console.error('fetch system settings failed', error)
    ElMessage.error('加载系统设置失败')
  } finally {
    systemLoading.value = false
  }
}

async function saveSystemConfig() {
  saveLoading.value = true
  try {
    const payload = normalizeSettings(draftSettings.value)
    await updateSystemSettings(payload)
    appliedSettings.value = cloneSettings(payload)
    draftSettings.value = cloneSettings(payload)
    ElMessage.success('系统设置已保存')
  } catch (error: any) {
    const data = error.response?.data
    if (data?.error === 'not leader' && data?.leader) {
      ElMessage.warning(`当前节点不是 Leader，请到 ${data.leader} 节点执行保存`)
    } else {
      ElMessage.error(data?.error || '保存系统设置失败')
    }
  } finally {
    saveLoading.value = false
  }
}

async function fetchKeys() {
  keysLoading.value = true
  try {
    const res = await api.get('/v1/settings/apikeys')
    keys.value = res.data || []
  } catch (error) {
    console.error('fetch api keys failed', error)
  } finally {
    keysLoading.value = false
  }
}

function generateRandKey() {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
  let result = 'eden_'
  for (let i = 0; i < 24; i += 1) {
    result += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  keyForm.value.key = result
}

async function handleGenerate() {
  try {
    await api.post('/v1/settings/apikey', {
      key: keyForm.value.key,
      description: keyForm.value.description,
      expires_at: keyForm.value.expDays > 0 ? Math.floor(Date.now() / 1000) + keyForm.value.expDays * 86400 : 0,
    })
    ElMessage.success('API Key 已创建')
    showDialog.value = false
    keyForm.value.description = ''
    keyForm.value.expDays = 0
    await fetchKeys()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '创建 API Key 失败')
  }
}

async function deleteKey(key: string) {
  try {
    await ElMessageBox.confirm(`确定删除 API Key：${key.slice(0, 12)}... 吗？`, '删除确认', {
      type: 'warning',
    })
    await api.post(`/v1/settings/apikey/delete?key=${encodeURIComponent(key)}`)
    ElMessage.success('API Key 已删除')
    await fetchKeys()
  } catch (error) {
    if (error) {
      console.debug('delete key cancelled or failed', error)
    }
  }
}

async function copyKey(key: string) {
  try {
    await navigator.clipboard.writeText(key)
    ElMessage.success('已复制到剪贴板')
  } catch {
    ElMessage.error('复制失败')
  }
}

function formatDate(ts: number) {
  if (!ts) return '永不过期'
  return new Date(ts * 1000).toLocaleString('zh-CN')
}

function setDraftEnvironment(env: 'standalone' | 'cluster') {
  draftSettings.value.environment = env
}

function setDraftMode(mode: 'ap' | 'cp') {
  draftSettings.value.mode = mode
}

function resetDraftSettings() {
  draftSettings.value = cloneSettings(appliedSettings.value)
}

function handleNonBlockingSave() {
  ElMessage.info('这一页签暂未接入后端，这次已优先完善基础设置和注册中心配置')
}

onMounted(() => {
  fetchSystemConfig()
  fetchKeys()
})
</script>

<template>
  <div class="settings-container">
    <el-tabs v-model="activeTab" class="settings-tabs">
      <el-tab-pane name="basic">
        <template #label>
          <span class="tab-label"><el-icon><Setting /></el-icon>基本设置</span>
        </template>

        <div class="tab-content basic-settings">
          <div class="basic-grid">
            <div class="settings-column">
              <section class="settings-section glass-card mode-section">
                <div class="section-header">
                  <el-icon><Operation /></el-icon>
                  <h4>运行模式</h4>
                  <div class="status-tags">
                    <div class="status-indicator">
                      <span class="indicator-dot" :class="appliedEnvironment === 'cluster' ? 'active' : 'idle'"></span>
                      <span class="dot-label">当前运行:</span>
                      <el-tag :type="appliedEnvironment === 'cluster' ? 'primary' : 'info'" size="small" effect="dark">
                        {{ appliedEnvironment === 'cluster' ? '集群模式' : '单机模式' }}
                      </el-tag>
                    </div>
                    <div class="status-indicator" v-if="appliedEnvironment === 'cluster'">
                      <span class="dot-label">一致性:</span>
                      <el-tag :class="appliedMode === 'cp' ? 'tag-cp' : 'tag-ap'" size="small" effect="dark">
                        {{ appliedMode === 'cp' ? '强一致性' : '最终一致性' }}
                      </el-tag>
                    </div>
                  </div>
                </div>

                <div class="consistency-wrapper-v7" v-loading="systemLoading">
                  <div class="main-mode-grid">
                    <div
                      class="mode-card-v7"
                      :class="{ active: previewEnvironment === 'standalone' }"
                      @click="setDraftEnvironment('standalone')"
                    >
                      <div class="card-glow"></div>
                      <div class="card-inner">
                        <div class="mode-icon-v7"><el-icon><Monitor /></el-icon></div>
                        <div class="mode-content-v7">
                          <div class="mode-title-v7">单机模式</div>
                          <div class="mode-desc-v7">本地闭环，轻量化部署，不涉及分布式一致性协议。</div>
                        </div>
                      </div>
                    </div>

                    <div
                      class="mode-card-v7 cluster"
                      :class="{ active: previewEnvironment === 'cluster' }"
                      @click="setDraftEnvironment('cluster')"
                    >
                      <div class="card-glow"></div>
                      <div class="card-inner">
                        <div class="mode-icon-v7 cluster"><el-icon><Share /></el-icon></div>
                        <div class="mode-content-v7">
                          <div class="mode-title-v7">分布式集群模式</div>

                          <div class="integrated-toggle-v7" :class="[previewMode]" v-if="previewEnvironment === 'cluster'" @click.stop>
                            <div class="toggle-option" :class="{ selected: previewMode === 'ap' }" @click="setDraftMode('ap')">
                              <div class="toggle-bg"></div>
                              <div class="toggle-text">
                                <span class="primary">AP 模式</span>
                                <span class="secondary">可用性优先</span>
                              </div>
                            </div>
                            <div class="toggle-option" :class="{ selected: previewMode === 'cp' }" @click="setDraftMode('cp')">
                              <div class="toggle-bg"></div>
                              <div class="toggle-text">
                                <span class="primary">CP 模式</span>
                                <span class="secondary">强一致性</span>
                              </div>
                            </div>
                          </div>
                          <div class="mode-desc-v7" v-else>启用分布式协调，支持 AP / CP 协议切换。</div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <div class="technical-footer-v7">
                  <div class="info-bubble-v7" :class="[previewEnvironment === 'standalone' ? 'standalone' : previewMode]">
                    <el-icon><InfoFilled /></el-icon>
                    <div class="bubble-content">
                      <template v-if="previewEnvironment === 'standalone'">
                        <strong>单机模式:</strong> 本地闭环，适用于开发测试或边缘单点场景。
                      </template>
                      <template v-else-if="previewMode === 'ap'">
                        <strong>AP 模式 (Gossip):</strong> 分区容错性高，优先保证系统可用性，节点间数据最终对齐。
                      </template>
                      <template v-else>
                        <strong>CP 模式 (Raft):</strong> 通过多数派与 Leader 协调保证强一致，适合关键元数据管理。
                      </template>
                    </div>
                  </div>
                </div>
              </section>

              <section class="settings-section glass-card registry-section">
                <div class="section-header">
                  <el-icon><Connection /></el-icon>
                  <div class="section-title-with-tip">
                    <h4>注册中心配置</h4>
                    <el-tooltip
                      content="注册中心不会在首次心跳异常时立即摘除实例，而是先进入保护窗口，给实例恢复与补发心跳的机会。"
                      placement="top"
                    >
                      <el-icon class="help-icon"><InfoFilled /></el-icon>
                    </el-tooltip>
                  </div>
                </div>

                <el-form label-position="left" label-width="140px" class="compact-form basic-inline-form registry-form">
                  <div class="form-grid">
                    <el-form-item>
                      <template #label>
                        <span class="label-with-tip">
                          心跳超时次数
                          <el-tooltip
                            content="默认 3 次。连续丢失心跳达到阈值后，实例会先标记为下线，而不是立即删除。"
                            placement="top"
                          >
                            <el-icon class="help-icon"><InfoFilled /></el-icon>
                          </el-tooltip>
                        </span>
                      </template>
                      <el-input-number
                        v-model="draftSettings.heartbeat_max_failures"
                        :min="1"
                        :max="10"
                        controls-position="right"
                        style="width: 100%"
                      />
                    </el-form-item>

                    <el-form-item>
                      <template #label>
                        <span class="label-with-tip">
                          实例保留（分钟）
                          <el-tooltip
                            content="默认 10 分钟。适合处理服务重启、网络抖动或 CPU 过高导致的心跳延迟。"
                            placement="top"
                          >
                            <el-icon class="help-icon"><InfoFilled /></el-icon>
                          </el-tooltip>
                        </span>
                      </template>
                      <el-input-number
                        v-model="removalDelayMinutes"
                        :min="1"
                        :max="1440"
                        controls-position="right"
                        style="width: 100%"
                      />
                    </el-form-item>
                  </div>
                </el-form>
              </section>
            </div>

            <div class="settings-column">
              <section class="settings-section glass-card side-section">
                <div class="section-header">
                  <el-icon><Bell /></el-icon>
                  <h4>事件设置</h4>
                </div>

                <el-form label-position="left" label-width="108px" class="compact-form basic-inline-form side-inline-form">
                  <el-form-item label="事件保留（天）">
                    <div class="slider-row">
                      <el-slider v-model="draftSettings.event_retention_days" :min="1" :max="365" style="flex: 1" />
                      <span class="val-text">{{ draftSettings.event_retention_days }}</span>
                    </div>
                  </el-form-item>

                  <el-form-item label="触发事件">
                    <el-checkbox-group v-model="draftSettings.event_types" class="event-checkbox-grid">
                      <el-checkbox v-for="item in EVENT_OPTIONS" :key="item.value" :label="item.value">
                        {{ item.label }}
                      </el-checkbox>
                    </el-checkbox-group>
                  </el-form-item>
                </el-form>
              </section>

              <section class="settings-section glass-card side-section">
                <div class="section-header">
                  <el-icon><Document /></el-icon>
                  <div class="section-title-with-tip">
                    <h4>日志配置</h4>
                    <el-tooltip
                      content="日志级别会在保存后立即更新当前进程；日志保留时长会同步写入持久化配置，用于后续日志滚动与清理策略。"
                      placement="top"
                    >
                      <el-icon class="help-icon"><InfoFilled /></el-icon>
                    </el-tooltip>
                  </div>
                </div>

                <el-form label-position="left" label-width="108px" class="compact-form basic-inline-form side-inline-form">
                  <el-form-item label="日志级别">
                    <div class="log-level-options" role="radiogroup" aria-label="日志级别">
                      <button
                        v-for="level in LOG_LEVEL_OPTIONS"
                        :key="level"
                        type="button"
                        :class="['log-level-option', `log-level-${level.toLowerCase()}`, { active: draftSettings.log_level === level }]"
                        :aria-checked="draftSettings.log_level === level"
                        @click="draftSettings.log_level = level"
                      >
                        {{ level }}
                      </button>
                    </div>
                  </el-form-item>

                  <el-form-item label="日志保留（天）">
                    <div class="slider-row">
                      <el-slider v-model="draftSettings.log_retention_days" :min="1" :max="365" style="flex: 1" />
                      <span class="val-text">{{ draftSettings.log_retention_days }}</span>
                    </div>
                  </el-form-item>
                </el-form>
              </section>
            </div>
          </div>

          <div class="save-toolbar">
            <div class="save-actions">
              <el-button @click="resetDraftSettings" :disabled="!isDirty || saveLoading">重置</el-button>
              <el-button type="primary" @click="saveSystemConfig" :loading="saveLoading" :disabled="!isDirty">
                保存设置
              </el-button>
            </div>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane name="credentials">
        <template #label>
          <span class="tab-label"><el-icon><Key /></el-icon>凭据管理</span>
        </template>

        <div class="tab-content">
          <div class="content-header credentials-header">
            <div class="header-main">
              <div class="title-row">
                <h3 class="tab-content-title">API Key 管理</h3>
                <el-radio-group v-model="credentialView" size="small" class="view-switch">
                  <el-radio-button label="list"><el-icon><Operation /></el-icon></el-radio-button>
                  <el-radio-button label="grid"><el-icon><Grid /></el-icon></el-radio-button>
                </el-radio-group>
              </div>
              <p class="content-desc">管理服务注册与控制台访问使用的 API Key，支持快速生成、复制与回收。</p>
              <div class="action-row">
                <el-button type="primary" @click="showDialog = true; generateRandKey()">
                  <el-icon style="margin-right: 4px"><Plus /></el-icon>
                  新建 API Key
                </el-button>
              </div>
            </div>
          </div>

          <div v-if="credentialView === 'grid'" class="key-grid" v-loading="keysLoading">
            <div v-for="item in keys" :key="item.key" class="key-card glass-card" :class="{ 'is-expired': item.status === 'expired' }">
              <div class="key-header">
                <code class="key-code" :title="item.key">{{ item.key.substring(0, 24) }}...</code>
                <div class="key-actions">
                  <el-tag v-if="item.status === 'expired'" type="danger" size="small" effect="dark" class="status-tag">
                    已过期
                  </el-tag>
                  <el-button link type="primary" :icon="CopyDocument" @click="copyKey(item.key)" />
                  <el-button link type="danger" :icon="Delete" @click="deleteKey(item.key)" />
                </div>
              </div>
              <div class="key-body">
                <div class="key-desc">{{ item.description || '无描述' }}</div>
                <div class="key-info">
                  <div class="key-date">
                    <el-icon><Timer /></el-icon> 过期时间: {{ formatDate(item.expires_at) }}
                  </div>
                  <div class="key-date">
                    <el-icon><Timer /></el-icon> 创建时间: {{ formatDate(item.created_at) }}
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div v-else class="key-list glass-card">
            <el-table :data="keys" style="width: 100%" class="custom-table" v-loading="keysLoading">
              <el-table-column prop="key" label="Key" min-width="320">
                <template #default="scope">
                  <div class="key-with-copy">
                    <code class="key-code-list" :title="scope.row.key">{{ scope.row.key.substring(0, 32) }}...</code>
                    <el-button link type="primary" :icon="CopyDocument" @click="copyKey(scope.row.key)" />
                  </div>
                </template>
              </el-table-column>
              <el-table-column prop="description" label="描述" min-width="180" />
              <el-table-column prop="status" label="状态" width="100">
                <template #default="scope">
                  <el-tag :type="scope.row.status === 'expired' ? 'danger' : 'success'" size="small">
                    {{ scope.row.status === 'expired' ? '已过期' : '生效中' }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="过期时间" width="180">
                <template #default="scope">
                  {{ formatDate(scope.row.expires_at) }}
                </template>
              </el-table-column>
              <el-table-column label="操作" width="80" align="right">
                <template #default="scope">
                  <el-button link type="danger" :icon="Delete" @click="deleteKey(scope.row.key)" />
                </template>
              </el-table-column>
            </el-table>
          </div>

          <el-empty v-if="!keysLoading && keys.length === 0" description="暂无 API Key" />
        </div>
      </el-tab-pane>

      <el-tab-pane name="channels">
        <template #label>
          <span class="tab-label"><el-icon><Share /></el-icon>通知渠道</span>
        </template>

        <div class="tab-content channels-grid">
          <section class="settings-section glass-card">
            <div class="section-header">
              <el-icon><Share /></el-icon>
              <h4>Webhook 配置</h4>
            </div>
            <el-form label-position="top" class="compact-form">
              <el-form-item label="Webhook 地址">
                <el-input v-model="webhookForm.url" placeholder="https://api.example.com/webhook" />
              </el-form-item>
              <el-form-item label="触发事件">
                <el-checkbox-group v-model="webhookForm.events" class="event-checkbox-grid">
                  <el-checkbox label="service_register">服务注册</el-checkbox>
                  <el-checkbox label="service_offline">服务下线</el-checkbox>
                  <el-checkbox label="service_remove">服务移除</el-checkbox>
                </el-checkbox-group>
              </el-form-item>
              <el-button type="primary" @click="handleNonBlockingSave" style="width: 100%">保存设置</el-button>
            </el-form>
          </section>

          <section class="settings-section glass-card">
            <div class="section-header">
              <el-icon><Message /></el-icon>
              <h4>SMTP 配置</h4>
            </div>
            <el-form label-position="top" class="compact-form">
              <el-form-item label="SMTP Host">
                <div class="form-inline">
                  <el-input v-model="smtpForm.host" placeholder="smtp.example.com" />
                  <el-input-number v-model="smtpForm.port" :controls="false" :min="1" :max="65535" />
                </div>
              </el-form-item>
              <el-form-item label="用户名">
                <el-input v-model="smtpForm.user" placeholder="user@example.com" />
              </el-form-item>
              <el-form-item label="密码">
                <el-input v-model="smtpForm.pass" type="password" show-password />
              </el-form-item>
              <el-form-item label="发件人">
                <el-input v-model="smtpForm.from" placeholder="Eden Registry <noreply@example.com>" />
              </el-form-item>
              <el-button type="primary" @click="handleNonBlockingSave" style="width: 100%">保存设置</el-button>
            </el-form>
          </section>
        </div>
      </el-tab-pane>

      <el-tab-pane name="monitoring">
        <template #label>
          <span class="tab-label"><el-icon><Monitor /></el-icon>告警监控</span>
        </template>

        <div class="tab-content">
          <section class="settings-section glass-card monitoring-card">
            <div class="section-header">
              <el-icon><Bell /></el-icon>
              <h4>告警配置</h4>
              <el-switch v-model="alarmForm.enabled" />
            </div>
            <el-form label-position="top" class="compact-form" :disabled="!alarmForm.enabled">
              <el-form-item label="健康阈值">
                <div class="slider-row">
                  <el-slider v-model="alarmForm.threshold" :min="10" :max="99" style="flex: 1" />
                  <span class="val-text">{{ alarmForm.threshold }}%</span>
                </div>
              </el-form-item>
              <el-form-item label="通知渠道">
                <el-radio-group v-model="alarmForm.contact" class="channel-radio">
                  <el-radio label="email" border>邮件</el-radio>
                  <el-radio label="webhook" border>Webhook</el-radio>
                </el-radio-group>
              </el-form-item>
              <el-button type="primary" @click="handleNonBlockingSave" style="width: 100%">保存设置</el-button>
            </el-form>
          </section>
        </div>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="showDialog" title="新建 API Key" width="520px">
      <el-form label-position="top" class="compact-form">
        <el-form-item label="API Key">
          <div class="key-input-container">
            <el-input v-model="keyForm.key" readonly />
            <el-button @click="generateRandKey" :icon="RefreshRight" />
          </div>
        </el-form-item>

        <el-form-item label="描述">
          <el-input v-model="keyForm.description" type="textarea" :rows="3" placeholder="用于说明此 API Key 的用途" />
        </el-form-item>

        <el-form-item label="有效期">
          <el-select v-model="keyForm.expDays" style="width: 100%">
            <el-option label="永不过期" :value="0" />
            <el-option label="7 天" :value="7" />
            <el-option label="30 天" :value="30" />
            <el-option label="90 天" :value="90" />
            <el-option label="365 天" :value="365" />
          </el-select>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="showDialog = false">取消</el-button>
        <el-button type="primary" @click="handleGenerate">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.settings-container {
  width: 100%;
  height: 100%;
}

.settings-tabs {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.settings-tabs :deep(.el-tabs__header) {
  margin-bottom: 16px;
}

.settings-tabs :deep(.el-tabs__nav-wrap::after) {
  display: none;
}

.settings-tabs :deep(.el-tabs__content) {
  flex: 1;
  min-height: 0;
}

.settings-tabs :deep(.el-tab-pane) {
  height: 100%;
}

.tab-label {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  font-weight: 600;
}

.tab-content {
  animation: fadeIn 0.25s ease;
}

.basic-settings {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.basic-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.55fr) minmax(340px, 0.95fr);
  gap: 16px;
  align-items: start;
}

.settings-column {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.channels-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
}

.monitoring-card {
  max-width: 520px;
}

.glass-card {
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: 18px;
  backdrop-filter: blur(18px);
}

.settings-section {
  padding: 20px 22px;
}

.mode-section {
  padding: 28px;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 18px;
}

.section-header h4 {
  margin: 0;
  flex: 1;
  font-size: 16px;
  font-weight: 700;
}

.section-title-with-tip {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}

.section-header .el-icon {
  font-size: 20px;
  color: var(--accent-blue);
}

.help-icon {
  color: var(--text-muted);
  cursor: help;
  transition: color 0.2s ease;
}

.help-icon:hover {
  color: var(--accent-blue);
}

.status-tags {
  display: flex;
  gap: 8px;
  align-items: center;
}

.status-indicator {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.04);
}

.indicator-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #94a3b8;
}

.indicator-dot.active {
  background: #10b981;
  box-shadow: 0 0 10px rgba(16, 185, 129, 0.55);
}

.dot-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
}

.consistency-wrapper-v7 {
  margin-top: 8px;
}

.main-mode-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.mode-card-v7 {
  position: relative;
  min-height: 160px;
  overflow: hidden;
  display: flex;
  cursor: pointer;
  border: 1px solid var(--border-color);
  border-radius: 12px;
  background: var(--bg-card);
  transition: border-color 0.2s ease, background-color 0.2s ease, box-shadow 0.2s ease;
}

.mode-card-v7:hover {
  border-color: var(--accent-blue);
  background: rgba(59, 130, 246, 0.03);
  box-shadow: inset 0 0 0 1px rgba(59, 130, 246, 0.08);
}

.mode-card-v7.active {
  border-color: var(--accent-blue);
  background: rgba(59, 130, 246, 0.02);
  box-shadow: inset 0 0 0 1px rgba(59, 130, 246, 0.08);
}

.card-glow {
  position: absolute;
  inset: 0;
  background: none;
  opacity: 0;
  transition: opacity 0.3s ease;
}

.mode-card-v7.active .card-glow {
  opacity: 1;
}

.card-inner {
  position: relative;
  z-index: 1;
  display: flex;
  gap: 20px;
  width: 100%;
  padding: 24px;
}

.mode-icon-v7 {
  width: 56px;
  height: 56px;
  flex-shrink: 0;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 26px;
  color: var(--text-muted);
  background: rgba(0, 0, 0, 0.03);
  transition: all 0.3s ease;
}

.mode-card-v7.active .mode-icon-v7 {
  color: #fff;
  background: var(--accent-blue);
}

.mode-content-v7 {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.mode-title-v7 {
  margin-bottom: 8px;
  font-size: 17px;
  font-weight: 800;
  color: var(--text-primary);
}

.mode-desc-v7 {
  color: var(--text-muted);
  font-size: 13px;
  line-height: 1.55;
}

.integrated-toggle-v7 {
  display: flex;
  gap: 4px;
  width: fit-content;
  margin-top: 12px;
  padding: 4px;
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.04);
}

.toggle-option {
  position: relative;
  min-width: 100px;
  padding: 8px 16px;
  overflow: hidden;
  cursor: pointer;
  text-align: center;
  border-radius: 8px;
}

.toggle-bg {
  position: absolute;
  inset: 0;
  opacity: 0;
  transform: scale(0.9);
  border-radius: 8px;
  transition: all 0.25s ease;
}

.integrated-toggle-v7.ap .toggle-bg {
  background: #10b981;
}

.integrated-toggle-v7.cp .toggle-bg {
  background: #f97316;
}

.toggle-option.selected .toggle-bg {
  opacity: 1;
  transform: scale(1);
}

.toggle-text {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  white-space: nowrap;
}

.toggle-text .primary {
  font-size: 13px;
  font-weight: 800;
  color: var(--text-muted);
}

.toggle-text .secondary {
  font-size: 10px;
  opacity: 0.75;
  color: var(--text-muted);
}

.toggle-option.selected .primary,
.toggle-option.selected .secondary {
  color: #fff;
}

.technical-footer-v7 {
  margin-top: 20px;
}

.info-bubble-v7 {
  display: flex;
  gap: 16px;
  align-items: flex-start;
  padding: 16px 20px;
  border: 1px solid var(--border-color);
  border-radius: 12px;
  background: rgba(0, 0, 0, 0.02);
}

.info-bubble-v7 .el-icon {
  margin-top: 2px;
  font-size: 18px;
}

.bubble-content {
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.info-bubble-v7.ap {
  border-left: 4px solid #10b981;
  background: rgba(16, 185, 129, 0.05);
}

.info-bubble-v7.ap .el-icon {
  color: #10b981;
}

.info-bubble-v7.cp {
  border-left: 4px solid #f97316;
  background: rgba(249, 115, 22, 0.06);
}

.info-bubble-v7.cp .el-icon {
  color: #f97316;
}

.info-bubble-v7.standalone {
  border-left: 4px solid #94a3b8;
}

.tag-ap {
  color: #fff !important;
  border-color: #10b981 !important;
  background: #10b981 !important;
}

.tag-cp {
  color: #fff !important;
  border-color: #f97316 !important;
  background: #f97316 !important;
}

.compact-form :deep(.el-form-item) {
  margin-bottom: 16px;
}

.basic-inline-form :deep(.el-form-item) {
  align-items: flex-start;
  margin-bottom: 12px;
}

.basic-inline-form :deep(.el-form-item__label) {
  min-height: 32px;
  padding-top: 6px;
  line-height: 20px;
  color: var(--text-secondary);
  white-space: nowrap;
  word-break: keep-all;
}

.basic-inline-form :deep(.el-form-item__content) {
  min-width: 0;
}

.label-with-tip {
  display: inline-flex;
  align-items: center;
  flex-wrap: nowrap;
  gap: 6px;
  line-height: 1.4;
  white-space: nowrap;
}

.label-with-tip .help-icon {
  flex-shrink: 0;
}

.side-inline-form :deep(.el-form-item__label) {
  padding-top: 4px;
}

.registry-form :deep(.el-form-item__label) {
  padding-top: 5px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px 16px;
}

.slider-row {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 32px;
  width: 100%;
  min-width: 0;
}

.slider-row :deep(.el-slider) {
  flex: 1 1 auto;
  min-width: 120px;
}

.log-level-options {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  width: auto;
  max-width: 100%;
  padding: 3px;
  border: 1px solid rgba(148, 163, 184, 0.22);
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.08);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.4);
}

.log-level-option {
  min-width: 0;
  min-height: 28px;
  padding: 0 12px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.01em;
  white-space: nowrap;
  cursor: pointer;
  transition: all 0.2s ease;
}

.log-level-option:hover {
  background: rgba(255, 255, 255, 0.4);
  color: var(--text-primary);
}

.log-level-option.active {
  background: var(--bg-secondary);
  box-shadow: 0 6px 14px rgba(15, 23, 42, 0.08);
}

.log-level-debug.active {
  color: #64748b;
  box-shadow: inset 0 0 0 1px rgba(100, 116, 139, 0.22), 0 6px 14px rgba(15, 23, 42, 0.08);
}

.log-level-info.active {
  color: var(--accent-blue);
  box-shadow: inset 0 0 0 1px rgba(59, 130, 246, 0.24), 0 6px 14px rgba(59, 130, 246, 0.12);
}

.log-level-warn.active {
  color: #d97706;
  box-shadow: inset 0 0 0 1px rgba(217, 119, 6, 0.24), 0 6px 14px rgba(217, 119, 6, 0.12);
}

.log-level-error.active {
  color: #dc2626;
  box-shadow: inset 0 0 0 1px rgba(220, 38, 38, 0.22), 0 6px 14px rgba(220, 38, 38, 0.1);
}

.val-text {
  min-width: 40px;
  text-align: left;
  color: var(--accent-blue);
  font-size: 14px;
  font-weight: 700;
}

.event-checkbox-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px 8px;
}

.save-toolbar {
  position: sticky;
  bottom: 6px;
  z-index: 12;
  display: flex;
  justify-content: flex-end;
  margin-top: auto;
  padding-top: 10px;
  background: transparent;
}

.save-actions {
  display: flex;
  gap: 12px;
  flex-shrink: 0;
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  backdrop-filter: none;
  box-shadow: none;
}

.credentials-header {
  margin-bottom: 24px;
}

.title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 12px;
}

.tab-content-title {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
}

.content-desc {
  margin-bottom: 20px;
  color: var(--text-secondary);
  font-size: 14px;
  line-height: 1.6;
}

.action-row {
  display: flex;
  align-items: center;
  padding: 20px;
  border: 1px dashed var(--border-color);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.02);
}

.key-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 18px;
}

.key-card {
  padding: 24px;
  transition: border-color 0.2s ease, background-color 0.2s ease, box-shadow 0.2s ease;
}

.key-card:hover {
  border-color: var(--accent-blue);
  background: rgba(59, 130, 246, 0.03);
  box-shadow: inset 0 0 0 1px rgba(59, 130, 246, 0.08);
}

.key-card.is-expired {
  border-color: var(--el-color-danger-light-5);
  background: rgba(245, 108, 108, 0.04);
}

.key-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}

.key-actions {
  display: flex;
  align-items: center;
}

.key-code,
.key-code-list {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  color: var(--accent-blue);
}

.key-code {
  padding: 6px 12px;
  border-radius: 10px;
  background: rgba(59, 130, 246, 0.1);
  font-size: 13px;
}

.key-code-list {
  font-size: 12px;
}

.key-body .key-desc {
  margin-bottom: 12px;
  font-size: 15px;
  font-weight: 600;
}

.key-info {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.key-date {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--text-secondary);
  font-size: 12px;
}

.key-list {
  overflow: hidden;
}

.custom-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.key-with-copy {
  display: flex;
  align-items: center;
  gap: 8px;
}

.key-input-container,
.form-inline {
  display: flex;
  gap: 12px;
}

.form-inline :deep(.el-input),
.form-inline :deep(.el-input-number) {
  flex: 1;
}

.channel-radio {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(8px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (max-width: 1320px) {
  .basic-grid,
  .channels-grid,
  .form-grid {
    grid-template-columns: 1fr;
  }

  .registry-form :deep(.el-form-item__label) {
    width: 140px !important;
  }

  .side-inline-form :deep(.el-form-item__label) {
    width: 108px !important;
  }
}

@media (max-width: 900px) {
  .main-mode-grid,
  .event-checkbox-grid,
  .channel-radio {
    grid-template-columns: 1fr;
  }

  .log-level-options {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    width: 100%;
  }

  .log-level-option {
    min-width: 0;
  }

  .basic-inline-form :deep(.el-form-item) {
    display: block;
  }

  .basic-inline-form :deep(.el-form-item__label) {
    display: block;
    width: 100% !important;
    min-height: auto;
    padding-top: 0;
    margin-bottom: 6px;
  }

  .basic-inline-form :deep(.el-form-item__content) {
    width: 100%;
    margin-left: 0 !important;
  }

  .title-row {
    flex-direction: column;
    align-items: stretch;
  }

  .save-actions {
    width: 100%;
  }

  .save-actions :deep(.el-button) {
    flex: 1;
  }
}
</style>

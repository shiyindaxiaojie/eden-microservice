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
  List,
  Monitor,
  Operation,
  Plus,
  RefreshRight,
  Setting,
  Share,
  Timer,
} from '@element-plus/icons-vue'
import api from '../api'
import {
  getAlertConfig,
  getClusterMembers,
  getNotificationConfig,
  getSystemSettings,
  updateAlertConfig,
  updateNotificationConfig,
  updateSystemSettings,
  type AlertConfig,
  type AlertRule,
  type AlertTemplate,
  type ClusterMember,
  type NotificationChannel,
  type NotificationConfig,
  type SystemSettings,
} from '../api/registry'

type CredentialView = 'grid' | 'list'
type EditMode = 'create' | 'edit'

type ChannelEditor = NotificationChannel & { configText: string }

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
const members = ref<ClusterMember[]>([])
const keyForm = ref({
  key: '',
  description: '',
  expDays: 0,
})

const messageLoading = ref(false)
const messageSaving = ref(false)
const alertLoading = ref(false)
const alertSaving = ref(false)

const notifyConfig = ref<NotificationConfig>({ channels: [] })
const alertConfig = ref<AlertConfig>({ templates: [], rules: [] })
const notifySnapshot = ref('')
const alertSnapshot = ref('')

const channelDialogVisible = ref(false)
const templateDialogVisible = ref(false)
const ruleDialogVisible = ref(false)
const channelMode = ref<EditMode>('create')
const templateMode = ref<EditMode>('create')
const ruleMode = ref<EditMode>('create')
const channelForm = ref<ChannelEditor>(emptyChannel())
const templateForm = ref<AlertTemplate>(emptyTemplate())
const ruleForm = ref<AlertRule>(emptyRule())

const EVENT_OPTIONS = [
  { value: 'service_register', label: '服务注册' },
  { value: 'service_online', label: '服务上线' },
  { value: 'service_offline', label: '服务下线' },
  { value: 'registry_node_sync', label: '节点同步' },
  { value: 'service_heartbeat', label: '服务心跳' },
  { value: 'service_remove', label: '服务移除' },
]

const LOG_LEVEL_OPTIONS = ['DEBUG', 'INFO', 'WARN', 'ERROR'] as const

const MESSAGE_PROVIDER_OPTIONS = [
  { label: '通用 Webhook', value: 'generic' },
  { label: '钉钉', value: 'dingtalk' },
  { label: '飞书', value: 'feishu' },
  { label: '企业微信', value: 'wecom' },
]

function serializeNotificationConfig(value: NotificationConfig) {
  return JSON.stringify({ channels: value.channels })
}

function serializeAlertConfig(value: AlertConfig) {
  return JSON.stringify({ templates: value.templates, rules: value.rules })
}

function createID(prefix: string) {
  return `${prefix}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
}

function emptyChannel(item?: NotificationChannel): ChannelEditor {
  return {
    id: item?.id || '',
    name: item?.name || '',
    type: item?.type || 'webhook',
    provider: item?.provider || 'generic',
    description: item?.description || '',
    enabled: item?.enabled ?? true,
    config: item?.config || {},
    configText: JSON.stringify(item?.config || {}, null, 2),
  }
}

function emptyTemplate(item?: AlertTemplate): AlertTemplate {
  return {
    id: item?.id || '',
    name: item?.name || '',
    channel_id: item?.channel_id || '',
    title_template: item?.title_template || '',
    body_template: item?.body_template || '',
    enabled: item?.enabled ?? true,
  }
}

function emptyRule(item?: AlertRule): AlertRule {
  return {
    id: item?.id || '',
    name: item?.name || '',
    event_code: item?.event_code || 'service_offline',
    threshold: item?.threshold || 1,
    window_sec: item?.window_sec || 300,
    template_ids: [...(item?.template_ids || [])],
    enabled: item?.enabled ?? true,
  }
}

function parseChannelConfig(textValue: string) {
  const raw = textValue.trim()
  if (!raw) return {}
  const value = JSON.parse(raw)
  if (!value || Array.isArray(value) || typeof value !== 'object') {
    throw new Error('渠道配置必须是 JSON 对象')
  }
  return value as Record<string, any>
}

function formatConfigTime(value?: string) {
  if (!value) return '未保存'
  return new Date(value).toLocaleString('zh-CN')
}

function channelTarget(channel: NotificationChannel) {
  const cfg = channel.config || {}
  const recipients = Array.isArray(cfg.recipients) ? cfg.recipients.join(', ') : ''
  return cfg.url || recipients || cfg.from || '-'
}

function channelName(channelID: string) {
  return notifyConfig.value.channels.find((item) => item.id === channelID)?.name || '-'
}

function templateName(templateID: string) {
  return alertConfig.value.templates.find((item) => item.id === templateID)?.name || '-'
}

function createDefaultSettings(): SystemSettings {
  return {
    mode: 'standalone',
    consistency: 'ap',
    log_level: 'INFO',
    event_retention_days: 30,
    log_retention_days: 30,
    event_types: EVENT_OPTIONS.map((item) => item.value),
    heartbeat_max_failures: 3,
    instance_removal_delay_seconds: 600,
    api_key_auth_enabled: false,
    notify_alert_node_id: '',
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
  const mode = input?.mode === 'cluster' ? 'cluster' : defaults.mode
  return {
    mode,
    consistency: mode === 'cluster' && input?.consistency === 'cp' ? 'cp' : 'ap',
    log_level: normalizeLogLevel(input?.log_level ?? defaults.log_level),
    event_retention_days: Math.max(1, input?.event_retention_days ?? defaults.event_retention_days),
    log_retention_days: Math.max(1, input?.log_retention_days ?? defaults.log_retention_days),
    event_types: normalizeEventTypes(input?.event_types ?? defaults.event_types),
    heartbeat_max_failures: Math.max(1, input?.heartbeat_max_failures ?? defaults.heartbeat_max_failures),
    instance_removal_delay_seconds: Math.max(60, input?.instance_removal_delay_seconds ?? defaults.instance_removal_delay_seconds),
    api_key_auth_enabled: Boolean(input?.api_key_auth_enabled ?? defaults.api_key_auth_enabled),
    notify_alert_node_id: typeof input?.notify_alert_node_id === 'string' ? input.notify_alert_node_id : defaults.notify_alert_node_id,
  }
}

const appliedSettings = ref<SystemSettings>(createDefaultSettings())
const draftSettings = ref<SystemSettings>(createDefaultSettings())

const previewEnvironment = computed(() => draftSettings.value.mode)
const previewMode = computed(() => draftSettings.value.consistency)
const appliedEnvironment = computed(() => appliedSettings.value.mode)
const appliedMode = computed(() => appliedSettings.value.consistency)

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

const messageProviderOptions = computed(() =>
  channelForm.value.type === 'email'
    ? [{ label: 'SMTP', value: 'smtp' }]
    : MESSAGE_PROVIDER_OPTIONS,
)

const enabledChannels = computed(() => notifyConfig.value.channels.filter((item) => item.enabled))
const enabledTemplates = computed(() => alertConfig.value.templates.filter((item) => item.enabled))
const messageDirty = computed(() => notifySnapshot.value !== serializeNotificationConfig(notifyConfig.value))
const alertRuleDirty = computed(() => alertSnapshot.value !== serializeAlertConfig(alertConfig.value))

async function fetchSystemConfig() {
  systemLoading.value = true
  try {
    const [{ data }, membersRes] = await Promise.all([getSystemSettings(), getClusterMembers()])
    members.value = membersRes.data || []
    const normalized = normalizeSettings(data)
    if (!normalized.notify_alert_node_id) {
      normalized.notify_alert_node_id = members.value.find((item) => item.is_local)?.id || ''
    }
    appliedSettings.value = cloneSettings(normalized)
    draftSettings.value = cloneSettings(normalized)
  } catch (error) {
    console.error('fetch system settings failed', error)
    ElMessage.error('Failed to load system settings')
  } finally {
    systemLoading.value = false
  }
}

async function saveSystemConfig() {
  saveLoading.value = true
  try {
    const payload = normalizeSettings(draftSettings.value)
    const { data } = await updateSystemSettings(payload)
    appliedSettings.value = cloneSettings(payload)
    draftSettings.value = cloneSettings(payload)
    if (data.restart_required) {
      ElMessage.warning(data.message || '当前变更需要重启后生效')
    } else {
      ElMessage.success('系统设置已保存')
    }
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

async function fetchNotifyConfig() {
  messageLoading.value = true
  try {
    const response = await getNotificationConfig('default')
    notifyConfig.value = response.data || { channels: [] }
    notifySnapshot.value = serializeNotificationConfig(notifyConfig.value)
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '加载消息渠道配置失败')
  } finally {
    messageLoading.value = false
  }
}

async function saveNotifyConfig() {
  messageSaving.value = true
  try {
    const response = await updateNotificationConfig('default', notifyConfig.value)
    notifyConfig.value = response.data || notifyConfig.value
    notifySnapshot.value = serializeNotificationConfig(notifyConfig.value)
    ElMessage.success('消息渠道配置已保存')
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '保存消息渠道配置失败')
  } finally {
    messageSaving.value = false
  }
}

async function reloadNotifyConfig() {
  if (messageDirty.value) {
    try {
      await ElMessageBox.confirm('当前消息渠道有未保存变更，确认重新加载吗？', '重新加载确认', { type: 'warning' })
    } catch {
      return
    }
  }
  await fetchNotifyConfig()
}

function openChannelDialog(mode: EditMode, item?: NotificationChannel) {
  channelMode.value = mode
  channelForm.value = emptyChannel(item)
  if (channelForm.value.type === 'email') {
    channelForm.value.provider = 'smtp'
  }
  channelDialogVisible.value = true
}

function saveChannelDraft() {
  try {
    const payload: NotificationChannel = {
      id: channelForm.value.id || createID('channel'),
      name: channelForm.value.name.trim(),
      type: channelForm.value.type,
      provider: channelForm.value.type === 'email' ? 'smtp' : channelForm.value.provider,
      description: channelForm.value.description?.trim() || '',
      enabled: channelForm.value.enabled,
      config: parseChannelConfig(channelForm.value.configText),
    }
    if (!payload.name) {
      ElMessage.warning('请填写渠道名称')
      return
    }

    const channels = [...notifyConfig.value.channels]
    const index = channels.findIndex((item) => item.id === payload.id)
    if (index >= 0) channels[index] = payload
    else channels.unshift(payload)
    notifyConfig.value = { ...notifyConfig.value, channels }
    channelDialogVisible.value = false
  } catch (error: any) {
    ElMessage.error(error.message || '渠道配置格式错误')
  }
}

function removeChannel(channel: NotificationChannel) {
  ElMessageBox.confirm(`确定删除渠道 ${channel.name} 吗？`, '删除确认', { type: 'warning' })
    .then(() => {
      const removedID = channel.id
      notifyConfig.value = {
        ...notifyConfig.value,
        channels: notifyConfig.value.channels.filter((item) => item.id !== removedID),
      }
      alertConfig.value = {
        ...alertConfig.value,
        templates: alertConfig.value.templates.filter((item) => item.channel_id !== removedID),
      }
      ElMessage.success('渠道已删除')
    })
    .catch(() => {})
}

async function fetchAlertRuleConfig() {
  alertLoading.value = true
  try {
    const response = await getAlertConfig('default')
    alertConfig.value = response.data || { templates: [], rules: [] }
    alertSnapshot.value = serializeAlertConfig(alertConfig.value)
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '加载告警配置失败')
  } finally {
    alertLoading.value = false
  }
}

async function saveAlertRuleConfig() {
  alertSaving.value = true
  try {
    const response = await updateAlertConfig('default', alertConfig.value)
    alertConfig.value = response.data || alertConfig.value
    alertSnapshot.value = serializeAlertConfig(alertConfig.value)
    ElMessage.success('告警配置已保存')
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '保存告警配置失败')
  } finally {
    alertSaving.value = false
  }
}

async function reloadAlertRuleConfig() {
  if (alertRuleDirty.value) {
    try {
      await ElMessageBox.confirm('当前告警配置有未保存变更，确认重新加载吗？', '重新加载确认', { type: 'warning' })
    } catch {
      return
    }
  }
  await fetchAlertRuleConfig()
}

function openTemplateDialog(mode: EditMode, item?: AlertTemplate) {
  templateMode.value = mode
  templateForm.value = emptyTemplate(item)
  templateDialogVisible.value = true
}

function saveTemplateDraft() {
  const payload: AlertTemplate = {
    id: templateForm.value.id || createID('tpl'),
    name: templateForm.value.name.trim(),
    channel_id: templateForm.value.channel_id,
    title_template: templateForm.value.title_template?.trim() || '',
    body_template: templateForm.value.body_template.trim(),
    enabled: templateForm.value.enabled,
  }
  if (!payload.name || !payload.channel_id || !payload.body_template) {
    ElMessage.warning('请完整填写告警模板')
    return
  }

  const templates = [...alertConfig.value.templates]
  const index = templates.findIndex((item) => item.id === payload.id)
  if (index >= 0) templates[index] = payload
  else templates.unshift(payload)
  alertConfig.value = { ...alertConfig.value, templates }
  templateDialogVisible.value = false
}

function removeTemplate(template: AlertTemplate) {
  ElMessageBox.confirm(`确定删除模板 ${template.name} 吗？`, '删除确认', { type: 'warning' })
    .then(() => {
      alertConfig.value = {
        ...alertConfig.value,
        templates: alertConfig.value.templates.filter((item) => item.id !== template.id),
        rules: alertConfig.value.rules.map((item) => ({
          ...item,
          template_ids: item.template_ids.filter((id) => id !== template.id),
        })),
      }
      ElMessage.success('告警模板已删除')
    })
    .catch(() => {})
}

function openRuleDialog(mode: EditMode, item?: AlertRule) {
  ruleMode.value = mode
  ruleForm.value = emptyRule(item)
  ruleDialogVisible.value = true
}

function saveRuleDraft() {
  const payload: AlertRule = {
    id: ruleForm.value.id || createID('rule'),
    name: ruleForm.value.name.trim(),
    event_code: ruleForm.value.event_code,
    threshold: Math.max(1, ruleForm.value.threshold || 1),
    window_sec: Math.max(60, ruleForm.value.window_sec || 300),
    template_ids: [...ruleForm.value.template_ids],
    enabled: ruleForm.value.enabled,
  }
  if (!payload.name || !payload.event_code || payload.template_ids.length === 0) {
    ElMessage.warning('请完整填写告警规则')
    return
  }

  const rules = [...alertConfig.value.rules]
  const index = rules.findIndex((item) => item.id === payload.id)
  if (index >= 0) rules[index] = payload
  else rules.unshift(payload)
  alertConfig.value = { ...alertConfig.value, rules }
  ruleDialogVisible.value = false
}

function removeRule(rule: AlertRule) {
  ElMessageBox.confirm(`确定删除规则 ${rule.name} 吗？`, '删除确认', { type: 'warning' })
    .then(() => {
      alertConfig.value = {
        ...alertConfig.value,
        rules: alertConfig.value.rules.filter((item) => item.id !== rule.id),
      }
      ElMessage.success('告警规则已删除')
    })
    .catch(() => {})
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
  draftSettings.value.mode = env
  if (env === 'standalone') {
    draftSettings.value.consistency = 'ap'
  }
}

function setDraftMode(mode: 'ap' | 'cp') {
  draftSettings.value.consistency = mode
}

function resetDraftSettings() {
  draftSettings.value = cloneSettings(appliedSettings.value)
}

onMounted(() => {
  fetchSystemConfig()
  fetchKeys()
  fetchNotifyConfig()
  fetchAlertRuleConfig()
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

                <el-form label-position="left" label-width="120px" class="compact-form basic-inline-form registry-form">
                  <div class="form-grid registry-form-grid">
                    <el-form-item class="switch-form-item">
                      <template #label>
                        <span class="label-with-tip">
                          客户端凭据认证
                          <el-tooltip content="主要用于控制客户端集成注册中心时是否需要携带凭据，默认不开启。" placement="top">
                            <el-icon class="help-icon"><InfoFilled /></el-icon>
                          </el-tooltip>
                        </span>
                      </template>
                      <div class="inline-switch-control">
                        <el-switch v-model="draftSettings.api_key_auth_enabled" />
                      </div>
                    </el-form-item>

                    <el-form-item>
                      <template #label>
                        <span class="label-with-tip">
                          Notify/Alert Node
                          <el-tooltip content="指定哪个节点持有通知渠道与告警配置，其他节点会转发到这里。" placement="top">
                            <el-icon class="help-icon"><InfoFilled /></el-icon>
                          </el-tooltip>
                        </span>
                      </template>
                      <el-select v-model="draftSettings.notify_alert_node_id" style="width: 100%">
                        <el-option v-for="item in members" :key="item.id" :label="`${item.id}${item.is_local ? ' (Local)' : ''}`" :value="item.id" />
                      </el-select>
                    </el-form-item>

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
            <p class="content-desc credentials-desc">管理服务注册与控制台访问使用的 API Key，支持快速生成、复制与回收。</p>
            <div class="credentials-toolbar">
              <div class="pill-group icon-pills credential-view-pills" aria-label="API Key 视图切换">
                <button
                  type="button"
                  :class="{ active: credentialView === 'list' }"
                  title="列表视图"
                  @click="credentialView = 'list'"
                >
                  <el-icon><List /></el-icon>
                </button>
                <button
                  type="button"
                  :class="{ active: credentialView === 'grid' }"
                  title="卡片视图"
                  @click="credentialView = 'grid'"
                >
                  <el-icon><Grid /></el-icon>
                </button>
              </div>
              <el-button type="primary" class="new-key-btn" @click="showDialog = true; generateRandKey()">
                <el-icon style="margin-right: 4px"><Plus /></el-icon>
                新建 API Key
              </el-button>
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
          <span class="tab-label"><el-icon><Share /></el-icon>消息渠道</span>
        </template>

        <div class="tab-content ops-settings-tab" v-loading="messageLoading">
          <section class="settings-section glass-card ops-settings-section">
            <div class="section-header">
              <el-icon><Share /></el-icon>
              <div class="section-title-with-tip">
                <h4>消息渠道配置</h4>
                <el-tooltip content="只保存基础渠道配置，不记录发送历史。" placement="top">
                  <el-icon class="help-icon"><InfoFilled /></el-icon>
                </el-tooltip>
              </div>
              <div class="section-actions">
                <el-button :icon="RefreshRight" @click="reloadNotifyConfig">重新加载</el-button>
                <el-button type="primary" :icon="Plus" @click="openChannelDialog('create')">新增渠道</el-button>
              </div>
            </div>

            <el-table :data="notifyConfig.channels" class="custom-table" empty-text="暂无消息渠道">
              <el-table-column prop="name" label="名称" min-width="140" />
              <el-table-column prop="type" label="类型" width="110" />
              <el-table-column prop="provider" label="提供方" width="120" />
              <el-table-column label="目标配置" min-width="240">
                <template #default="{ row }">
                  <span class="muted-text">{{ channelTarget(row) }}</span>
                </template>
              </el-table-column>
              <el-table-column label="状态" width="100">
                <template #default="{ row }">
                  <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '启用' : '停用' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="140" align="right">
                <template #default="{ row }">
                  <el-button link type="primary" @click="openChannelDialog('edit', row)">编辑</el-button>
                  <el-button link type="danger" @click="removeChannel(row)">删除</el-button>
                </template>
              </el-table-column>
            </el-table>

            <div class="ops-footer">
              <span class="muted-text">最近保存：{{ formatConfigTime(notifyConfig.updated_at) }}</span>
              <el-button type="primary" :loading="messageSaving" :disabled="!messageDirty" @click="saveNotifyConfig">
                保存消息渠道
              </el-button>
            </div>
          </section>
        </div>
      </el-tab-pane>

      <el-tab-pane name="monitoring">
        <template #label>
          <span class="tab-label"><el-icon><Monitor /></el-icon>告警配置</span>
        </template>

        <div class="tab-content ops-settings-tab" v-loading="alertLoading">
          <section class="settings-section glass-card ops-settings-section">
            <div class="section-header">
              <el-icon><Bell /></el-icon>
              <div class="section-title-with-tip">
                <h4>告警模板</h4>
                <el-tooltip content="模板绑定消息渠道，用于拼装触发后的通知内容。" placement="top">
                  <el-icon class="help-icon"><InfoFilled /></el-icon>
                </el-tooltip>
              </div>
              <div class="section-actions">
                <el-button type="primary" link @click="openTemplateDialog('create')">新增模板</el-button>
              </div>
            </div>

            <el-table :data="alertConfig.templates" class="custom-table" empty-text="暂无告警模板">
              <el-table-column prop="name" label="名称" min-width="160" />
              <el-table-column label="消息渠道" min-width="180">
                <template #default="{ row }">{{ channelName(row.channel_id) }}</template>
              </el-table-column>
              <el-table-column label="状态" width="100">
                <template #default="{ row }">
                  <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '启用' : '停用' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="140" align="right">
                <template #default="{ row }">
                  <el-button link type="primary" @click="openTemplateDialog('edit', row)">编辑</el-button>
                  <el-button link type="danger" @click="removeTemplate(row)">删除</el-button>
                </template>
              </el-table-column>
            </el-table>
          </section>

          <section class="settings-section glass-card ops-settings-section">
            <div class="section-header">
              <el-icon><Monitor /></el-icon>
              <div class="section-title-with-tip">
                <h4>事件阈值规则</h4>
                <el-tooltip content="按事件类型、阈值次数和统计窗口触发告警，不保存历史记录。" placement="top">
                  <el-icon class="help-icon"><InfoFilled /></el-icon>
                </el-tooltip>
              </div>
              <div class="section-actions">
                <el-button :icon="RefreshRight" @click="reloadAlertRuleConfig">重新加载</el-button>
                <el-button type="primary" :icon="Plus" @click="openRuleDialog('create')">新增规则</el-button>
              </div>
            </div>

            <el-table :data="alertConfig.rules" class="custom-table" empty-text="暂无告警规则">
              <el-table-column prop="name" label="名称" min-width="160" />
              <el-table-column prop="event_code" label="事件" min-width="160" />
              <el-table-column label="触发阈值" width="180">
                <template #default="{ row }">
                  <span class="muted-text">{{ row.window_sec }} 秒内 {{ row.threshold }} 次</span>
                </template>
              </el-table-column>
              <el-table-column label="模板" min-width="220">
                <template #default="{ row }">
                  <span class="muted-text">{{ row.template_ids.map(templateName).join('、') || '-' }}</span>
                </template>
              </el-table-column>
              <el-table-column label="状态" width="100">
                <template #default="{ row }">
                  <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '启用' : '停用' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="140" align="right">
                <template #default="{ row }">
                  <el-button link type="primary" @click="openRuleDialog('edit', row)">编辑</el-button>
                  <el-button link type="danger" @click="removeRule(row)">删除</el-button>
                </template>
              </el-table-column>
            </el-table>

            <div class="ops-footer">
              <span class="muted-text">最近保存：{{ formatConfigTime(alertConfig.updated_at) }}</span>
              <el-button type="primary" :loading="alertSaving" :disabled="!alertRuleDirty" @click="saveAlertRuleConfig">
                保存告警配置
              </el-button>
            </div>
          </section>
        </div>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="showDialog" title="新建 API Key" width="560px" top="6vh" class="glass-dialog credential-dialog">
      <el-form label-position="top" class="compact-form credential-dialog-form">
        <el-form-item label="API Key">
          <div class="key-preview-shell">
            <div class="key-preview-value" :title="keyForm.key">{{ keyForm.key }}</div>
            <el-button class="regenerate-key-btn" @click="generateRandKey" :icon="RefreshRight" />
          </div>
          <div class="field-hint">创建前可重复生成，完整密钥会按原样展示。</div>
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

    <el-dialog v-model="channelDialogVisible" :title="channelMode === 'create' ? '新增消息渠道' : '编辑消息渠道'" width="680px">
      <el-form label-position="top" class="compact-form">
        <div class="ops-form-grid">
          <el-form-item label="名称">
            <el-input v-model="channelForm.name" />
          </el-form-item>
          <el-form-item label="类型">
            <el-select v-model="channelForm.type">
              <el-option label="Webhook" value="webhook" />
              <el-option label="Email" value="email" />
            </el-select>
          </el-form-item>
          <el-form-item label="提供方">
            <el-select v-model="channelForm.provider" :disabled="channelForm.type === 'email'">
              <el-option v-for="item in messageProviderOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
          <el-form-item label="启用">
            <el-switch v-model="channelForm.enabled" />
          </el-form-item>
        </div>
        <el-form-item label="描述">
          <el-input v-model="channelForm.description" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="配置(JSON)">
          <el-input v-model="channelForm.configText" type="textarea" :rows="8" placeholder='{"url":"https://example.com/webhook"}' />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="channelDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveChannelDraft">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="templateDialogVisible" :title="templateMode === 'create' ? '新增告警模板' : '编辑告警模板'" width="680px">
      <el-form label-position="top" class="compact-form">
        <div class="ops-form-grid">
          <el-form-item label="模板名称">
            <el-input v-model="templateForm.name" />
          </el-form-item>
          <el-form-item label="消息渠道">
            <el-select v-model="templateForm.channel_id">
              <el-option v-for="item in enabledChannels" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
          </el-form-item>
        </div>
        <el-form-item label="标题模板">
          <el-input v-model="templateForm.title_template" placeholder="例如：服务告警 - {{ event_code }}" />
        </el-form-item>
        <el-form-item label="内容模板">
          <el-input v-model="templateForm.body_template" type="textarea" :rows="5" placeholder="例如：{{ event_code }} 在 {{ window_sec }} 秒内触发 {{ threshold }} 次" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="templateForm.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="templateDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveTemplateDraft">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="ruleDialogVisible" :title="ruleMode === 'create' ? '新增告警规则' : '编辑告警规则'" width="680px">
      <el-form label-position="top" class="compact-form">
        <div class="ops-form-grid">
          <el-form-item label="规则名称">
            <el-input v-model="ruleForm.name" />
          </el-form-item>
          <el-form-item label="事件类型">
            <el-select v-model="ruleForm.event_code">
              <el-option v-for="item in EVENT_OPTIONS" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
          <el-form-item label="阈值次数">
            <el-input-number v-model="ruleForm.threshold" :min="1" controls-position="right" style="width: 100%" />
          </el-form-item>
          <el-form-item label="统计窗口(秒)">
            <el-input-number v-model="ruleForm.window_sec" :min="60" controls-position="right" style="width: 100%" />
          </el-form-item>
        </div>
        <el-form-item label="关联模板">
          <el-select v-model="ruleForm.template_ids" multiple collapse-tags collapse-tags-tooltip>
            <el-option v-for="item in enabledTemplates" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="ruleForm.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="ruleDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveRuleDraft">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.settings-container {
  width: 100%;
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.settings-tabs {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.settings-tabs :deep(.el-tabs__header) {
  margin-bottom: 12px;
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
  min-height: 0;
}

.basic-settings {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.basic-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.55fr) minmax(340px, 0.95fr);
  gap: 12px;
  align-items: start;
}

.settings-column {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.channels-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
}

.monitoring-card {
  max-width: 520px;
}

.ops-settings-tab {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.ops-settings-section {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.glass-card {
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: 18px;
  backdrop-filter: blur(18px);
}

.settings-section {
  padding: 18px 20px;
}

.mode-section {
  padding: 22px;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
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

.section-actions {
  display: flex;
  align-items: center;
  gap: 8px;
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
  margin-top: 4px;
}

.main-mode-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.mode-card-v7 {
  position: relative;
  min-height: 136px;
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
  gap: 16px;
  width: 100%;
  padding: 18px;
}

.mode-icon-v7 {
  width: 50px;
  height: 50px;
  flex-shrink: 0;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 22px;
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
  margin-bottom: 6px;
  font-size: 16px;
  font-weight: 800;
  color: var(--text-primary);
}

.mode-desc-v7 {
  color: var(--text-muted);
  font-size: 12px;
  line-height: 1.45;
}

.integrated-toggle-v7 {
  display: flex;
  gap: 4px;
  width: fit-content;
  margin-top: 10px;
  padding: 3px;
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.04);
}

.toggle-option {
  position: relative;
  min-width: 100px;
  padding: 7px 14px;
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
  font-size: 12px;
  font-weight: 800;
  color: var(--text-muted);
}

.toggle-text .secondary {
  font-size: 9px;
  opacity: 0.75;
  color: var(--text-muted);
}

.toggle-option.selected .primary,
.toggle-option.selected .secondary {
  color: #fff;
}

.technical-footer-v7 {
  margin-top: 14px;
}

.info-bubble-v7 {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  padding: 13px 16px;
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
  font-size: 12px;
  line-height: 1.5;
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
  margin-bottom: 12px;
}

.basic-inline-form :deep(.el-form-item) {
  align-items: flex-start;
  margin-bottom: 10px;
}

.basic-inline-form :deep(.el-form-item__label) {
  min-height: 30px;
  padding-top: 4px;
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
  gap: 7px;
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
  padding-top: 4px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px 14px;
}

.registry-form-grid {
  grid-template-columns: minmax(220px, 0.92fr) minmax(0, 1fr) minmax(0, 1fr);
}

.switch-form-item :deep(.el-form-item__content) {
  display: flex;
  align-items: center;
  min-height: 32px;
}

.inline-switch-control {
  display: inline-flex;
  align-items: center;
  min-height: 32px;
}

.slider-row {
  display: flex;
  align-items: center;
  gap: 10px;
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
  font-size: 13px;
  font-weight: 700;
}

.event-checkbox-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px 8px;
}

.save-toolbar {
  display: flex;
  justify-content: flex-end;
  margin-top: auto;
  padding-top: 0;
  background: transparent;
}

.save-actions {
  display: flex;
  gap: 10px;
  flex-shrink: 0;
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  backdrop-filter: none;
  box-shadow: none;
}

.credentials-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 24px;
}

.credentials-desc {
  margin: 0 !important;
  max-width: 720px;
}

.credentials-toolbar {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  margin-left: auto;
  flex-shrink: 0;
}

.content-desc {
  margin-bottom: 20px;
  color: var(--text-secondary);
  font-size: 14px;
  line-height: 1.6;
}

.new-key-btn {
  height: 36px;
  padding: 0 16px;
  border-radius: 10px;
}

.pill-group {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 2px;
  border-radius: 10px;
  background: rgba(148, 163, 184, 0.12);
}

.pill-group button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  min-width: 34px;
  height: 32px;
  padding: 0 9px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--text-muted);
  font-weight: 500;
  cursor: pointer;
  transition: all 0.18s ease;
  font-family: inherit;
}

.pill-group button:hover {
  color: var(--text-secondary);
  background: rgba(255, 255, 255, 0.7);
}

.pill-group button.active {
  background: var(--accent-blue);
  color: #fff;
  box-shadow: 0 8px 18px rgba(59, 130, 246, 0.22);
}

.icon-pills button {
  width: 42px;
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

.credential-dialog-form {
  display: flex;
  flex-direction: column;
}

.form-inline {
  display: flex;
  gap: 12px;
}

.key-preview-shell {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
  align-items: stretch;
}

.key-preview-value {
  min-height: 54px;
  padding: 14px 16px;
  border-radius: 14px;
  border: 1px solid rgba(148, 163, 184, 0.24);
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.08), rgba(255, 255, 255, 0.92));
  color: var(--text-primary);
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 13px;
  line-height: 1.6;
  word-break: break-all;
  white-space: normal;
  overflow-wrap: anywhere;
}

.regenerate-key-btn {
  width: 52px;
  min-height: 54px;
  border-radius: 14px;
}

.field-hint {
  margin-top: 8px;
  color: var(--text-muted);
  font-size: 12px;
  line-height: 1.5;
}

.muted-text {
  color: var(--text-secondary);
}

.ops-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.ops-form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 16px;
}

.form-inline :deep(.el-input),
.form-inline :deep(.el-input-number) {
  flex: 1;
}

:deep(.credential-dialog.el-dialog) {
  width: min(560px, calc(100vw - 24px)) !important;
  max-height: 92vh;
  margin-bottom: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-radius: 18px;
}

:deep(.credential-dialog .el-dialog__header) {
  margin-right: 0;
  padding: 22px 22px 8px;
}

:deep(.credential-dialog .el-dialog__headerbtn) {
  top: 20px;
  right: 20px;
}

:deep(.credential-dialog .el-dialog__body) {
  padding: 0 22px 8px;
}

:deep(.credential-dialog .el-dialog__footer) {
  padding: 14px 22px 20px;
  border-top: 1px solid rgba(148, 163, 184, 0.14);
}

:deep(.credential-dialog .el-form-item) {
  margin-bottom: 16px;
}

:deep(.credential-dialog .el-form-item__label) {
  padding-bottom: 8px;
  color: var(--text-primary);
  font-weight: 700;
  line-height: 1.4;
}

:deep(.credential-dialog .el-input__wrapper),
:deep(.credential-dialog .el-select__wrapper) {
  min-height: 44px;
  border-radius: 12px;
}

:deep(.credential-dialog .el-textarea__inner) {
  min-height: 96px !important;
  border-radius: 12px;
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
  .form-grid,
  .ops-form-grid {
    grid-template-columns: 1fr;
  }

  .registry-form-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .credentials-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .credentials-toolbar {
    margin-left: 0;
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

  .section-actions,
  .ops-footer {
    width: 100%;
    flex-direction: column;
    align-items: stretch;
  }

  .section-actions :deep(.el-button),
  .ops-footer :deep(.el-button) {
    width: 100%;
  }

  .registry-form-grid {
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

  .credentials-toolbar {
    width: 100%;
    justify-content: space-between;
    flex-wrap: wrap;
  }

  .save-actions {
    width: 100%;
  }

  .save-actions :deep(.el-button) {
    flex: 1;
  }

  .new-key-btn {
    width: 100%;
  }

  .key-preview-shell {
    grid-template-columns: 1fr;
  }

  .regenerate-key-btn {
    width: 100%;
  }
}

@media (max-height: 760px) {
  .basic-settings {
    overflow-y: auto;
    overflow-x: hidden;
    padding-right: 4px;
  }
}
</style>

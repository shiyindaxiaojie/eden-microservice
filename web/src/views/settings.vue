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

interface ChannelEditor extends NotificationChannel {
  webhookUrl: string
  webhookSecret: string
  emailHost: string
  emailPort: number
  emailUsername: string
  emailPassword: string
  emailFrom: string
  emailRecipientsText: string
  emailUseTLS: boolean
  extraConfigText: string
}

interface TemplatePreset {
  id: string
  label: string
  title: string
  body: string
}

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
const notifyNodeSaving = ref(false)
const alertLoading = ref(false)
const alertSaving = ref(false)

const notifyConfig = ref<NotificationConfig>({ channels: [] })
const alertConfig = ref<AlertConfig>({ templates: [], rules: [] })
const notifySnapshot = ref('')

const channelEditorVisible = ref(false)
const templateEditorVisible = ref(false)
const ruleEditorVisible = ref(false)
const alertSubTab = ref<'templates' | 'rules'>('templates')
const channelMode = ref<EditMode>('create')
const templateMode = ref<EditMode>('create')
const ruleMode = ref<EditMode>('create')

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

const WEBHOOK_CONFIG_KEYS = new Set(['url', 'secret'])
const EMAIL_CONFIG_KEYS = new Set([
  'host',
  'port',
  'username',
  'password',
  'from',
  'recipients',
  'tls',
  'use_tls',
  'secure',
  'user',
  'pass',
  'smtp_host',
  'smtp_port',
  'smtp_user',
  'smtp_pass',
  'smtp_from',
  'to',
])

const TEMPLATE_PRESETS: TemplatePreset[] = [
  {
    id: 'default',
    label: '默认告警',
    title: 'Eden 告警 - {{ event_code }}',
    body: ['事件：{{ event_code }}', '触发条件：{{ window_sec }} 秒内达到 {{ threshold }} 次', '请尽快检查对应服务实例与节点状态。'].join('\n'),
  },
  {
    id: 'compact',
    label: '简洁通知',
    title: '',
    body: ['[Eden] {{ event_code }}', '{{ window_sec }} 秒内累计 {{ threshold }} 次'].join('\n'),
  },
  {
    id: 'email',
    label: '邮件详情',
    title: 'Eden Registry 告警通知 - {{ event_code }}',
    body: ['告警事件：{{ event_code }}', '统计窗口：{{ window_sec }} 秒', '触发次数：{{ threshold }} 次', '', '建议登录控制台进一步排查。'].join('\n'),
  },
]

function cloneConfig(input?: Record<string, any>) {
  return input && typeof input === 'object' && !Array.isArray(input) ? { ...input } : {}
}

function firstNonEmptyString(...values: any[]) {
  for (const value of values) {
    if (typeof value === 'string' && value.trim()) {
      return value.trim()
    }
  }
  return ''
}

function firstPositiveNumber(fallback: number, ...values: any[]) {
  for (const value of values) {
    const parsed = Number(value)
    if (Number.isFinite(parsed) && parsed > 0) {
      return parsed
    }
  }
  return fallback
}

function firstBoolean(fallback: boolean, ...values: any[]) {
  for (const value of values) {
    if (typeof value === 'boolean') return value
    if (typeof value === 'string') {
      const normalized = value.trim().toLowerCase()
      if (['true', '1', 'yes', 'on'].includes(normalized)) return true
      if (['false', '0', 'no', 'off'].includes(normalized)) return false
    }
  }
  return fallback
}

function normalizeRecipients(value: any): string[] {
  if (Array.isArray(value)) {
    return value
      .map((item) => String(item || '').trim())
      .filter(Boolean)
  }
  if (typeof value === 'string') {
    return value
      .split(/[\n,;]+/)
      .map((item) => item.trim())
      .filter(Boolean)
  }
  return []
}

function omitConfigKeys(config: Record<string, any>, keys: Set<string>) {
  return Object.entries(config).reduce<Record<string, any>>((result, [key, value]) => {
    if (!keys.has(key)) {
      result[key] = value
    }
    return result
  }, {})
}

function stringifyExtraConfig(value: Record<string, any>) {
  return Object.keys(value).length ? JSON.stringify(value, null, 2) : ''
}

function cleanConfigValue(value: any) {
  if (value === undefined || value === null) return undefined
  if (typeof value === 'string') {
    const trimmed = value.trim()
    return trimmed ? trimmed : undefined
  }
  if (Array.isArray(value)) {
    return value.length ? value : undefined
  }
  return value
}

function parseExtraConfig(textValue: string) {
  const raw = textValue.trim()
  if (!raw) return {}
  const value = JSON.parse(raw)
  if (!value || Array.isArray(value) || typeof value !== 'object') {
    throw new Error('扩展字段必须是 JSON 对象')
  }
  return value as Record<string, any>
}

function buildChannelConfig(input: ChannelEditor) {
  const extraConfig = parseExtraConfig(input.extraConfigText)
  if (input.type === 'email') {
    const recipients = normalizeRecipients(input.emailRecipientsText)
    return Object.entries({
      ...extraConfig,
      host: input.emailHost,
      port: input.emailPort,
      username: input.emailUsername,
      password: input.emailPassword,
      from: input.emailFrom,
      recipients,
      tls: input.emailUseTLS,
    }).reduce<Record<string, any>>((result, [key, value]) => {
      const cleaned = cleanConfigValue(value)
      if (cleaned !== undefined) {
        result[key] = cleaned
      }
      return result
    }, {})
  }

  return Object.entries({
    ...extraConfig,
    url: input.webhookUrl,
    secret: input.webhookSecret,
  }).reduce<Record<string, any>>((result, [key, value]) => {
    const cleaned = cleanConfigValue(value)
    if (cleaned !== undefined) {
      result[key] = cleaned
    }
    return result
  }, {})
}

function serializeNotificationConfig(value: NotificationConfig) {
  return JSON.stringify({ channels: value.channels })
}

function createID(prefix: string) {
  return `${prefix}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
}

function emptyChannel(item?: NotificationChannel): ChannelEditor {
  const config = cloneConfig(item?.config)
  const isEmail = item?.type === 'email'
  return {
    id: item?.id || '',
    name: item?.name || '',
    type: item?.type || 'webhook',
    provider: item?.provider || 'generic',
    description: item?.description || '',
    enabled: item?.enabled ?? true,
    config,
    webhookUrl: firstNonEmptyString(config.url),
    webhookSecret: firstNonEmptyString(config.secret),
    emailHost: firstNonEmptyString(config.host, config.smtp_host),
    emailPort: firstPositiveNumber(465, config.port, config.smtp_port),
    emailUsername: firstNonEmptyString(config.username, config.user, config.smtp_user),
    emailPassword: firstNonEmptyString(config.password, config.pass, config.smtp_pass),
    emailFrom: firstNonEmptyString(config.from, config.smtp_from),
    emailRecipientsText: normalizeRecipients(config.recipients ?? config.to).join('\n'),
    emailUseTLS: firstBoolean(true, config.tls, config.use_tls, config.secure),
    extraConfigText: stringifyExtraConfig(omitConfigKeys(config, isEmail ? EMAIL_CONFIG_KEYS : WEBHOOK_CONFIG_KEYS)),
  }
}

function emptyTemplate(item?: AlertTemplate): AlertTemplate {
  const preset = TEMPLATE_PRESETS[0] || { title: '', body: '' }
  return {
    id: item?.id || '',
    name: item?.name || '',
    channel_id: item?.channel_id || '',
    title_template: item?.title_template ?? preset.title,
    body_template: item?.body_template ?? preset.body,
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

const channelForm = ref<ChannelEditor>(emptyChannel())
const templateForm = ref<AlertTemplate>(emptyTemplate())
const ruleForm = ref<AlertRule>(emptyRule())

function channelName(channelID: string) {
  return notifyConfig.value.channels.find((item) => item.id === channelID)?.name || '-'
}

function templateName(templateID: string) {
  return alertConfig.value.templates.find((item) => item.id === templateID)?.name || '-'
}

function providerLabel(provider: string) {
  return MESSAGE_PROVIDER_OPTIONS.find((item) => item.value === provider)?.label || provider || '-'
}

function channelTypeLabel(type: string) {
  return type === 'email' ? '邮件' : 'Webhook'
}

function eventLabel(code: string) {
  return EVENT_OPTIONS.find((item) => item.value === code)?.label || code
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

const notifyNodeDirty = computed(() => (draftSettings.value.notify_alert_node_id || '') !== (appliedSettings.value.notify_alert_node_id || ''))
const templateChannelOptions = computed(() => notifyConfig.value.channels)
const ruleTemplateOptions = computed(() => alertConfig.value.templates)
const messageDirty = computed(() => notifySnapshot.value !== serializeNotificationConfig(notifyConfig.value))
const channelEditorTitle = computed(() => (channelMode.value === 'create' ? '新增消息渠道' : '编辑消息渠道'))
const templateEditorTitle = computed(() => (templateMode.value === 'create' ? '新增告警模板' : '编辑告警模板'))
const ruleEditorTitle = computed(() => (ruleMode.value === 'create' ? '新增告警规则' : '编辑告警规则'))

function handleSettingsSaveError(error: any, fallback: string) {
  const data = error.response?.data
  if (data?.error === 'not leader' && data?.leader) {
    ElMessage.warning(`当前节点不是 Leader，请到 ${data.leader} 节点执行保存`)
  } else {
    ElMessage.error(data?.error || fallback)
  }
}

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
    handleSettingsSaveError(error, '保存系统设置失败')
  } finally {
    saveLoading.value = false
  }
}

async function saveNotifyAlertNode() {
  notifyNodeSaving.value = true
  try {
    const payload = normalizeSettings({
      ...appliedSettings.value,
      notify_alert_node_id: draftSettings.value.notify_alert_node_id,
    })
    const { data } = await updateSystemSettings(payload)
    appliedSettings.value = {
      ...cloneSettings(appliedSettings.value),
      notify_alert_node_id: payload.notify_alert_node_id,
    }
    draftSettings.value.notify_alert_node_id = payload.notify_alert_node_id
    if (data.restart_required) {
      ElMessage.warning(data.message || '通知配置节点修改后需要重启生效')
    } else {
      ElMessage.success('通知配置节点已保存')
    }
  } catch (error: any) {
    handleSettingsSaveError(error, '保存通知配置节点失败')
  } finally {
    notifyNodeSaving.value = false
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
    const alertResponse = await updateAlertConfig('default', alertConfig.value)
    alertConfig.value = alertResponse.data || alertConfig.value
    ElMessage.success('消息渠道配置已保存')
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '保存消息渠道配置失败')
  } finally {
    messageSaving.value = false
  }
}

function openChannelDialog(mode: EditMode, item?: NotificationChannel) {
  channelMode.value = mode
  channelForm.value = emptyChannel(item)
  if (channelForm.value.type === 'email') {
    channelForm.value.provider = 'smtp'
  }
  channelEditorVisible.value = true
}

function closeChannelEditor() {
  channelEditorVisible.value = false
  channelForm.value = emptyChannel()
}

function handleChannelTypeChange(value: string) {
  if (value === 'email') {
    channelForm.value.type = 'email'
    channelForm.value.provider = 'smtp'
    channelForm.value.emailPort = channelForm.value.emailPort || 465
    return
  }
  channelForm.value.type = 'webhook'
  if (channelForm.value.provider === 'smtp') {
    channelForm.value.provider = 'generic'
  }
}

function saveChannelDraft() {
  try {
    const config = buildChannelConfig(channelForm.value)
    const payload: NotificationChannel = {
      id: channelForm.value.id || createID('channel'),
      name: channelForm.value.name.trim(),
      type: channelForm.value.type,
      provider: channelForm.value.type === 'email' ? 'smtp' : channelForm.value.provider,
      description: channelForm.value.description?.trim() || '',
      enabled: channelForm.value.enabled,
      config,
    }
    if (!payload.name) {
      ElMessage.warning('请填写渠道名称')
      return
    }
    if (payload.type === 'email') {
      if (!firstNonEmptyString(config.host)) {
        ElMessage.warning('请填写 SMTP 主机')
        return
      }
      if (!firstPositiveNumber(0, config.port)) {
        ElMessage.warning('请填写 SMTP 端口')
        return
      }
      if (!firstNonEmptyString(config.from)) {
        ElMessage.warning('请填写发件人邮箱')
        return
      }
      if (normalizeRecipients(config.recipients).length === 0) {
        ElMessage.warning('请至少填写一个收件人邮箱')
        return
      }
    } else if (!firstNonEmptyString(config.url)) {
      ElMessage.warning('请填写 Webhook 地址')
      return
    }

    const channels = [...notifyConfig.value.channels]
    const index = channels.findIndex((item) => item.id === payload.id)
    if (index >= 0) channels[index] = payload
    else channels.unshift(payload)
    notifyConfig.value = { ...notifyConfig.value, channels }
    channelEditorVisible.value = false
    channelForm.value = emptyChannel(payload)
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
      if (channelEditorVisible.value && channelForm.value.id === removedID) {
        closeChannelEditor()
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
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '加载告警配置失败')
  } finally {
    alertLoading.value = false
  }
}

async function persistAlertConfig(nextConfig: AlertConfig, successMessage: string) {
  alertSaving.value = true
  try {
    const response = await updateAlertConfig('default', nextConfig)
    alertConfig.value = response.data || nextConfig
    ElMessage.success(successMessage)
    return true
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '保存告警配置失败')
    return false
  } finally {
    alertSaving.value = false
  }
}

function openTemplateDialog(mode: EditMode, item?: AlertTemplate) {
  if (mode === 'create' && notifyConfig.value.channels.length === 0) {
    activeTab.value = 'channels'
    ElMessage.warning('请先新增一个消息渠道，再创建告警模板')
    return
  }
  ruleEditorVisible.value = false
  templateMode.value = mode
  templateForm.value = emptyTemplate(item)
  templateEditorVisible.value = true
}

function closeTemplateEditor() {
  templateEditorVisible.value = false
  templateForm.value = emptyTemplate()
}

function applyTemplatePreset(presetID: string) {
  const preset = TEMPLATE_PRESETS.find((item) => item.id === presetID)
  if (!preset) return
  templateForm.value.title_template = preset.title
  templateForm.value.body_template = preset.body
}

async function saveTemplateDraft() {
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

  const success = await persistAlertConfig(
    { ...alertConfig.value, templates },
    templateMode.value === 'create' ? '告警模板已创建' : '告警模板已保存',
  )
  if (!success) return

  templateEditorVisible.value = false
  templateForm.value = emptyTemplate()
}

function removeTemplate(template: AlertTemplate) {
  ElMessageBox.confirm(`确定删除模板 ${template.name} 吗？`, '删除确认', { type: 'warning' })
    .then(async () => {
      const nextConfig: AlertConfig = {
        ...alertConfig.value,
        templates: alertConfig.value.templates.filter((item) => item.id !== template.id),
        rules: alertConfig.value.rules.map((item) => ({
          ...item,
          template_ids: item.template_ids.filter((id) => id !== template.id),
        })),
      }
      const success = await persistAlertConfig(nextConfig, '告警模板已删除')
      if (!success) return

      if (templateEditorVisible.value && templateForm.value.id === template.id) {
        closeTemplateEditor()
      }
    })
    .catch(() => {})
}

function openRuleDialog(mode: EditMode, item?: AlertRule) {
  if (mode === 'create' && alertConfig.value.templates.length === 0) {
    ElMessage.warning('请先创建告警模板，再新增规则')
    return
  }
  templateEditorVisible.value = false
  ruleMode.value = mode
  ruleForm.value = emptyRule(item)
  ruleEditorVisible.value = true
}

function closeRuleEditor() {
  ruleEditorVisible.value = false
  ruleForm.value = emptyRule()
}

async function saveRuleDraft() {
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

  const success = await persistAlertConfig(
    { ...alertConfig.value, rules },
    ruleMode.value === 'create' ? '告警规则已创建' : '告警规则已保存',
  )
  if (!success) return

  ruleEditorVisible.value = false
  ruleForm.value = emptyRule()
}

function removeRule(rule: AlertRule) {
  ElMessageBox.confirm(`确定删除规则 ${rule.name} 吗？`, '删除确认', { type: 'warning' })
    .then(async () => {
      const nextConfig: AlertConfig = {
        ...alertConfig.value,
        rules: alertConfig.value.rules.filter((item) => item.id !== rule.id),
      }
      const success = await persistAlertConfig(nextConfig, '告警规则已删除')
      if (!success) return

      if (ruleEditorVisible.value && ruleForm.value.id === rule.id) {
        closeRuleEditor()
      }
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
                        <strong>单机模式:</strong> 本地闭环，用于开发测试或边缘单点场景。
                      </template>
                      <template v-else-if="previewMode === 'ap'">
                        <strong>AP 模式 (Gossip):</strong> 分区容错性高，优先保证系统可用，节点间数据最终对齐。
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
                    <h4>实例存活策略</h4>
                    <el-tooltip
                      content="注册中心不会在首次心跳异常时立即摘除实例，而是先进入保护窗口，给实例恢复与补发心跳的机会。" placement="top"
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
                          心跳超时次数
                          <el-tooltip
                            content="默认 3 次连续丢失心跳达到阈值后，实例会先标记为下线，而不是立即删除。" placement="top"
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
                            content="默认 10 分钟。适合处理服务重启、网络抖动或 CPU 过高导致的心跳延迟。" placement="top"
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
                      content="日志级别会在保存后立即更新当前进程；日志保留时长会同步写入持久化配置，用于后续日志滚动与清理策略。" placement="top"
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
            <div class="ops-toolbar-v2">
              <div class="toolbar-left">
                <el-icon class="toolbar-icon"><Connection /></el-icon>
                <span class="toolbar-label">消息发送节点</span>
                <el-tooltip content="所有消息将由该节点负责推送。" placement="bottom">
                  <el-select v-model="draftSettings.notify_alert_node_id" class="minimal-select" size="default" style="width: 200px">
                    <el-option
                      v-for="item in members"
                      :key="item.id"
                      :label="`${item.id}${item.is_local ? '（当前节点）' : ''}`"
                      :value="item.id"
                    />
                  </el-select>
                </el-tooltip>
                <el-button v-if="notifyNodeDirty" type="primary" link :loading="notifyNodeSaving" @click="saveNotifyAlertNode">
                  保存节点配置
                </el-button>
              </div>
              <div class="toolbar-right">
                <el-button type="primary" :icon="Plus" @click="openChannelDialog('create')">新增渠道</el-button>
              </div>
            </div>

            <div class="ops-workbench">
              <div class="ops-list-panel">
                <div v-if="notifyConfig.channels.length > 0" class="entity-list">
                  <div
                    v-for="item in notifyConfig.channels"
                    :key="item.id"
                    class="entity-card-v2"
                    :class="{ active: channelEditorVisible && channelForm.id === item.id, 'type-email': item.type === 'email' }"
                    @click="openChannelDialog('edit', item)"
                  >
                    <div class="entity-strip"></div>
                    <div class="entity-body-v2">
                      <div class="entity-row-top">
                        <div class="entity-icon-chip" :class="item.type">
                          <el-icon v-if="item.type === 'email'"><Document /></el-icon>
                          <el-icon v-else><Connection /></el-icon>
                        </div>
                        <div class="entity-info-v2">
                          <div class="entity-title">{{ item.name }}</div>
                          <div class="entity-subline">{{ channelTypeLabel(item.type) }} · {{ providerLabel(item.provider) }}</div>
                        </div>
                      </div>
                      <div class="entity-actions-v2">
                        <el-tag :type="item.enabled ? 'success' : 'info'" size="small" effect="plain" class="status-tag">{{ item.enabled ? '启用' : '停用' }}</el-tag>
                        <div class="action-btn-group">
                          <el-button link type="primary" size="small" @click.stop="openChannelDialog('edit', item)">编辑</el-button>
                          <el-divider direction="vertical" />
                          <el-button link type="danger" size="small" @click.stop="removeChannel(item)">删除</el-button>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
                <div v-else class="ops-empty-state">
                  <div class="empty-icon-large">
                    <el-icon><Share /></el-icon>
                  </div>
                  <div class="empty-state-title">尚未配置消息渠道</div>
                  <div class="empty-state-desc">新增 Webhook 或邮件渠道后，模板和规则都会基于此进行关联推送。</div>
                  <el-button type="primary" :icon="Plus" @click="openChannelDialog('create')">新增第一个渠道</el-button>
                </div>
              </div>

              <div class="ops-editor-panel glass-subcard">
                <template v-if="channelEditorVisible">
                  <div class="editor-head-v2">
                    <div class="editor-head-accent channel"></div>
                    <div class="editor-head-content">
                      <div>
                        <div class="editor-title">{{ channelEditorTitle }}</div>
                        <div class="editor-subtitle">配置渠道参数，完成后点击确认。</div>
                      </div>
                      <div class="editor-actions">
                        <el-button @click="closeChannelEditor">取消</el-button>
                        <el-button type="primary" @click="saveChannelDraft">确认</el-button>
                      </div>
                    </div>
                  </div>

                  <el-form label-position="left" label-width="85px" class="compact-form editor-form inline-label-form">
                    <div class="ops-form-grid-3col">
                      <el-form-item label="名称">
                        <el-input v-model="channelForm.name" placeholder="例如：生产环境告警" />
                      </el-form-item>
                      <el-form-item label="类型">
                        <el-select v-model="channelForm.type" @change="handleChannelTypeChange">
                          <el-option label="Webhook" value="webhook" />
                          <el-option label="邮件" value="email" />
                        </el-select>
                      </el-form-item>
                      <el-form-item label="提供方">
                        <el-select v-model="channelForm.provider" :disabled="channelForm.type === 'email'">
                          <el-option v-for="item in messageProviderOptions" :key="item.value" :label="item.label" :value="item.value" />
                        </el-select>
                      </el-form-item>
                      <el-form-item label="启用状态">
                        <el-switch v-model="channelForm.enabled" />
                      </el-form-item>
                    </div>

                    <el-form-item label="渠道描述">
                      <el-input v-model="channelForm.description" type="textarea" :rows="2" placeholder="可选，用于说明这个渠道的用途" />
                    </el-form-item>

                    <template v-if="channelForm.type === 'webhook'">
                      <div class="ops-form-grid-3col">
                        <el-form-item label="地址 (URL)" style="grid-column: span 2;">
                          <el-input v-model="channelForm.webhookUrl" placeholder="https://your-webhook-endpoint" />
                        </el-form-item>
                        <el-form-item label="签名密钥">
                          <el-input v-model="channelForm.webhookSecret" show-password placeholder="可选" />
                        </el-form-item>
                      </div>
                    </template>
                    <template v-else>
                      <div class="ops-form-grid-3col channel-email-grid">
                        <el-form-item label="SMTP 主机">
                          <el-input v-model="channelForm.emailHost" placeholder="例如：smtp.example.com" />
                        </el-form-item>
                        <el-form-item label="SMTP 端口">
                          <el-input-number v-model="channelForm.emailPort" :min="1" :max="65535" controls-position="right" style="width: 100%" />
                        </el-form-item>
                        <el-form-item label="用户名">
                          <el-input v-model="channelForm.emailUsername" placeholder="可选" />
                        </el-form-item>
                        <el-form-item label="密码">
                          <el-input v-model="channelForm.emailPassword" show-password placeholder="可选" />
                        </el-form-item>
                        <el-form-item label="发件人邮箱">
                          <el-input v-model="channelForm.emailFrom" placeholder="例如：noreply@example.com" />
                        </el-form-item>
                        <el-form-item label="启用 TLS">
                          <el-switch v-model="channelForm.emailUseTLS" />
                        </el-form-item>
                      </div>

                      <el-form-item label="收件人列表">
                        <el-input
                          v-model="channelForm.emailRecipientsText"
                          type="textarea"
                          :rows="4"
                          placeholder="ops@example.com&#10;owner@example.com"
                        />
                      </el-form-item>
                    </template>

                    <el-form-item label="扩展字段(JSON)">
                      <el-input
                        v-model="channelForm.extraConfigText"
                        type="textarea"
                        :rows="4"
                        placeholder='可，例如：{"headers":{"X-Token":"abc"}}'
                      />
                    </el-form-item>
                  </el-form>
                </template>
                <div v-else class="editor-empty-v2">
                  <div class="editor-empty-icon channel">
                    <el-icon><Connection /></el-icon>
                  </div>
                  <div class="editor-empty-title">选择左侧渠道进行编辑</div>
                  <div class="editor-empty-desc">或者直接新建一个渠道。</div>
                  <el-button type="primary" plain @click="openChannelDialog('create')">新增渠道</el-button>
                </div>
              </div>
            </div>

            <div class="ops-footer-v2" style="justify-content: flex-end;">
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
            <div class="alert-tabs-header">
              <div class="alert-sub-tabs">
                <button type="button" class="alert-sub-tab" :class="{ active: alertSubTab === 'templates' }" @click="alertSubTab = 'templates'">
                  <el-icon><Bell /></el-icon>
                  <span>告警模板</span>
                  <span v-if="alertConfig.templates.length" class="sub-tab-badge">{{ alertConfig.templates.length }}</span>
                </button>
                <button type="button" class="alert-sub-tab" :class="{ active: alertSubTab === 'rules' }" @click="alertSubTab = 'rules'">
                  <el-icon><Timer /></el-icon>
                  <span>事件阈值规则</span>
                  <span v-if="alertConfig.rules.length" class="sub-tab-badge">{{ alertConfig.rules.length }}</span>
                </button>
              </div>
              <div class="section-actions">
                <el-button v-if="alertSubTab === 'templates'" type="primary" :icon="Plus" @click="openTemplateDialog('create')">新增模板</el-button>
                <el-button v-else type="primary" :icon="Plus" @click="openRuleDialog('create')">新增规则</el-button>
              </div>
            </div>

            <div v-show="alertSubTab === 'templates'" class="ops-workbench alert-workbench" :class="{ 'with-editor': templateEditorVisible }">
              <div class="ops-list-panel">
                <div v-if="alertConfig.templates.length > 0" class="entity-list">
                  <div v-for="item in alertConfig.templates" :key="item.id" class="entity-card-v2 type-template" :class="{ active: templateEditorVisible && templateForm.id === item.id }" @click="openTemplateDialog('edit', item)">
                    <div class="entity-strip"></div>
                    <div class="entity-body-v2">
                      <div class="entity-row-top">
                        <div class="entity-icon-chip template"><el-icon><Document /></el-icon></div>
                        <div class="entity-info-v2">
                          <div class="entity-title">{{ item.name }}</div>
                          <div class="entity-subline">{{ channelName(item.channel_id) }}</div>
                        </div>
                      </div>
                      <div class="entity-detail-v2 multiline-clamp">{{ item.body_template }}</div>
                      <div class="entity-actions-v2">
                        <el-tag :type="item.enabled ? 'success' : 'info'" size="small" effect="plain" class="status-tag">{{ item.enabled ? '启用' : '停用' }}</el-tag>
                        <div class="action-btn-group">
                          <el-button link type="primary" size="small" @click.stop="openTemplateDialog('edit', item)">编辑</el-button>
                          <el-divider direction="vertical" />
                          <el-button link type="danger" size="small" @click.stop="removeTemplate(item)">删除</el-button>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
                <div v-else class="ops-empty-state">
                  <div class="empty-icon-large template"><el-icon><Document /></el-icon></div>
                  <div class="empty-state-title">尚未配置告警模板</div>
                  <div class="empty-state-desc">模板决定消息标题和正文内容，可以直接从预设开始。</div>
                  <el-button type="primary" :icon="Plus" @click="openTemplateDialog('create')">新增第一个模板</el-button>
                </div>
              </div>
              <Transition name="slide-panel">
                <div v-if="templateEditorVisible" class="ops-editor-panel glass-subcard">
                  <div class="editor-head-v2">
                    <div class="editor-head-accent template"></div>
                    <div class="editor-head-content">
                      <div>
                        <div class="editor-title">{{ templateEditorTitle }}</div>
                        <div class="editor-subtitle">模板和渠道绑定，规则只负责触发条件。</div>
                      </div>
                      <div class="editor-actions">
                        <el-button @click="closeTemplateEditor">取消</el-button>
                        <el-button type="primary" :loading="alertSaving" @click="saveTemplateDraft">确认</el-button>
                      </div>
                    </div>
                  </div>
                  <el-form label-position="left" label-width="85px" class="compact-form editor-form inline-label-form">
                    <div class="inline-preset-row">
                      <span class="inline-label">快速套用</span>
                      <button v-for="preset in TEMPLATE_PRESETS" :key="preset.id" type="button" class="preset-chip" @click="applyTemplatePreset(preset.id)">{{ preset.label }}</button>
                    </div>
                    <div class="ops-form-grid-3col">
                      <el-form-item label="模板名称"><el-input v-model="templateForm.name" placeholder="例如：服务下线通知" /></el-form-item>
                      <el-form-item label="消息渠道">
                        <el-select v-model="templateForm.channel_id">
                          <el-option v-for="item in templateChannelOptions" :key="item.id" :label="item.enabled ? item.name : `${item.name}（已停用）`" :value="item.id" />
                        </el-select>
                      </el-form-item>
                      <el-form-item label="启用状态"><el-switch v-model="templateForm.enabled" /></el-form-item>
                    </div>
                    <el-form-item label="标题模板"><el-input v-model="templateForm.title_template" placeholder="例如：Eden 告警 - {{ event_code }}" /></el-form-item>
                    <el-form-item label="内容模板">
                      <el-input v-model="templateForm.body_template" type="textarea" :rows="4" placeholder="例如：事件：{{ event_code }}&#10;{{ window_sec }} 秒内达到 {{ threshold }} 次" />
                    </el-form-item>
                  </el-form>
                </div>
              </Transition>
            </div>

            <div v-show="alertSubTab === 'rules'" class="ops-workbench alert-workbench" :class="{ 'with-editor': ruleEditorVisible }">
              <div class="ops-list-panel">
                <div v-if="alertConfig.rules.length > 0" class="entity-list">
                  <div v-for="item in alertConfig.rules" :key="item.id" class="entity-card-v2 type-rule" :class="{ active: ruleEditorVisible && ruleForm.id === item.id }" @click="openRuleDialog('edit', item)">
                    <div class="entity-strip"></div>
                    <div class="entity-body-v2">
                      <div class="entity-row-top">
                        <div class="entity-icon-chip rule"><el-icon><Timer /></el-icon></div>
                        <div class="entity-info-v2">
                          <div class="entity-title">{{ item.name }}</div>
                          <div class="entity-subline">{{ eventLabel(item.event_code) }}</div>
                        </div>
                      </div>
                      <div class="entity-detail-v2">{{ item.window_sec }} 秒内累计 {{ item.threshold }} 次 · {{ item.template_ids.map(templateName).join('、') || '-' }}</div>
                      <div class="entity-actions-v2">
                        <el-tag :type="item.enabled ? 'success' : 'info'" size="small" effect="plain" class="status-tag">{{ item.enabled ? '启用' : '停用' }}</el-tag>
                        <div class="action-btn-group">
                          <el-button link type="primary" size="small" @click.stop="openRuleDialog('edit', item)">编辑</el-button>
                          <el-divider direction="vertical" />
                          <el-button link type="danger" size="small" @click.stop="removeRule(item)">删除</el-button>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
                <div v-else class="ops-empty-state">
                  <div class="empty-icon-large rule"><el-icon><Timer /></el-icon></div>
                  <div class="empty-state-title">尚未配置告警规则</div>
                  <div class="empty-state-desc">规则把事件类型、次数窗口和模板关联起来。</div>
                  <el-button type="primary" :icon="Plus" @click="openRuleDialog('create')">新增第一条规则</el-button>
                </div>
              </div>
              <Transition name="slide-panel">
                <div v-if="ruleEditorVisible" class="ops-editor-panel glass-subcard">
                  <div class="editor-head-v2">
                    <div class="editor-head-accent rule"></div>
                    <div class="editor-head-content">
                      <div>
                        <div class="editor-title">{{ ruleEditorTitle }}</div>
                        <div class="editor-subtitle">规则决定什么事件在什么阈值下触发哪些模板。</div>
                      </div>
                      <div class="editor-actions">
                        <el-button @click="closeRuleEditor">取消</el-button>
                        <el-button type="primary" :loading="alertSaving" @click="saveRuleDraft">确认</el-button>
                      </div>
                    </div>
                  </div>
                  <el-form label-position="left" label-width="95px" class="compact-form editor-form inline-label-form">
                    <div class="ops-form-grid-3col">
                      <el-form-item label="规则名称"><el-input v-model="ruleForm.name" placeholder="例如：服务下线连续告警" /></el-form-item>
                      <el-form-item label="事件类型">
                        <el-select v-model="ruleForm.event_code">
                          <el-option v-for="item in EVENT_OPTIONS" :key="item.value" :label="item.label" :value="item.value" />
                        </el-select>
                      </el-form-item>
                      <el-form-item label="关联模板">
                        <el-select v-model="ruleForm.template_ids" multiple collapse-tags collapse-tags-tooltip>
                          <el-option v-for="item in ruleTemplateOptions" :key="item.id" :label="item.enabled ? item.name : `${item.name}（已停用）`" :value="item.id" />
                        </el-select>
                      </el-form-item>
                    </div>
                    <div class="ops-form-grid-3col">
                      <el-form-item label="阈值次/秒"><el-input-number v-model="ruleForm.threshold" :min="1" controls-position="right" style="width: 100%" /></el-form-item>
                      <el-form-item label="统计窗口(秒)"><el-input-number v-model="ruleForm.window_sec" :min="60" controls-position="right" style="width: 100%" /></el-form-item>
                      <el-form-item label="启用状态"><el-switch v-model="ruleForm.enabled" /></el-form-item>
                    </div>
                  </el-form>
                </div>
              </Transition>
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
  height: 100%;
  gap: 12px;
  min-height: 0;
  overflow: hidden;
}

.ops-settings-section {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  gap: 14px;
}

.ops-workbench {
  display: grid;
  grid-template-columns: minmax(300px, 25%) minmax(0, 1fr);
  gap: 18px;
  align-items: stretch;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.alert-workbench {
  grid-template-columns: 1fr;
  transition: grid-template-columns 0.28s ease;
}

.alert-workbench.with-editor {
  grid-template-columns: minmax(280px, 25%) minmax(0, 1fr);
}

.ops-list-panel {
  min-width: 0;
  overflow-y: auto;
  height: 100%;
  padding-right: 4px;
}

.ops-editor-panel {
  min-width: 0;
  overflow-y: auto;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.slide-panel-enter-active,
.slide-panel-leave-active {
  transition: opacity 0.24s ease, transform 0.28s ease;
}

.slide-panel-enter-from,
.slide-panel-leave-to {
  opacity: 0;
  transform: translateX(36px);
}

.glass-subcard {
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 16px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(248, 250, 252, 0.96));
}

.entity-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.entity-card {
  padding: 16px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.72);
  cursor: pointer;
  transition: border-color 0.18s ease, transform 0.18s ease, box-shadow 0.18s ease;
}

.entity-card:hover {
  border-color: rgba(59, 130, 246, 0.28);
  box-shadow: 0 12px 24px rgba(15, 23, 42, 0.05);
  transform: translateY(-1px);
}

.entity-card.active {
  border-color: rgba(59, 130, 246, 0.36);
  box-shadow: inset 0 0 0 1px rgba(59, 130, 246, 0.08);
  background: rgba(59, 130, 246, 0.04);
}

.entity-card-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.entity-title {
  color: var(--text-primary);
  font-size: 15px;
  font-weight: 700;
  line-height: 1.4;
}

.entity-subline {
  margin-top: 4px;
  color: var(--text-muted);
  font-size: 12px;
  line-height: 1.5;
}

.entity-detail {
  margin-top: 12px;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.65;
}

.entity-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 12px;
}

.editor-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 18px 0;
}

.editor-title {
  color: var(--text-primary);
  font-size: 16px;
  font-weight: 700;
}

.editor-subtitle {
  margin-top: 4px;
  color: var(--text-muted);
  font-size: 12px;
  line-height: 1.6;
}

.editor-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.editor-form {
  padding: 14px 18px 18px;
}

.editor-empty {
  display: flex;
  min-height: 260px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 24px;
  text-align: center;
}

.editor-empty-title {
  color: var(--text-primary);
  font-size: 15px;
  font-weight: 700;
}

.editor-empty-desc {
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.multiline-clamp {
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.glass-card {
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: 18px;
  backdrop-filter: blur(18px);
}

.settings-section {
  padding: 14px 16px;
}

.mode-section {
  padding: 16px;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 10px;
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

.ops-toolbar-v2 {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.toolbar-icon {
  font-size: 16px;
  color: var(--accent-blue);
}

.toolbar-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary);
}

.minimal-select {
  width: 180px;
}

.toolbar-right {
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

.ops-form-grid-3col {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px 16px;
}

.inline-label-form :deep(.el-form-item) {
  display: flex;
  align-items: center;
  margin-bottom: 12px;
}

.inline-label-form :deep(.el-form-item__label) {
  padding-bottom: 0;
  padding-right: 12px;
  line-height: 32px;
  height: 32px;
  align-items: center;
  flex-shrink: 0;
}

.inline-label-form :deep(.el-form-item__content) {
  flex: 1;
  min-width: 0;
  line-height: normal;
}

.entity-actions-v2 {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px dashed rgba(148, 163, 184, 0.2);
}

.action-btn-group {
  display: flex;
  align-items: center;
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

.empty-copy {
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.6;
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

.channel-email-grid {
  align-items: start;
}

.notify-node-inline {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  padding: 6px 10px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  border-radius: 999px;
  background: rgba(248, 250, 252, 0.9);
}

.notify-node-select {
  width: 240px;
  max-width: 100%;
}

.inline-preset-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 16px;
}

.inline-label {
  margin-right: 4px;
  color: var(--text-muted);
  font-size: 12px;
  font-weight: 700;
}

.preset-chip {
  min-height: 34px;
  padding: 0 14px;
  border: 1px solid rgba(59, 130, 246, 0.14);
  border-radius: 999px;
  background: rgba(59, 130, 246, 0.06);
  color: var(--accent-blue);
  font-family: inherit;
  font-size: 13px;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.2s ease;
}

.preset-chip:hover {
  border-color: rgba(59, 130, 246, 0.3);
  background: rgba(59, 130, 246, 0.12);
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

  .ops-workbench {
    grid-template-columns: 1fr;
  }

  .ops-editor-panel {
    position: static;
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
  .ops-footer,
  .editor-head {
    width: 100%;
    flex-direction: column;
    align-items: stretch;
  }

  .section-actions :deep(.el-button),
  .ops-footer :deep(.el-button) {
    width: 100%;
  }

  .notify-node-select {
    width: 100%;
  }

  .notify-node-inline {
    width: 100%;
    border-radius: 14px;
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

/* ===== Pipeline Flow ===== */
.pipeline-flow {
  display: flex;
  align-items: center;
  padding: 14px 22px;
  gap: 0;
}

.pipeline-step {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 14px;
  border-radius: 10px;
  flex-shrink: 0;
  transition: all 0.3s ease;
}

.pipeline-num {
  width: 30px;
  height: 30px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 800;
  background: rgba(148, 163, 184, 0.1);
  color: var(--text-muted);
  border: 2px solid rgba(148, 163, 184, 0.18);
  transition: all 0.3s ease;
  flex-shrink: 0;
}

.pipeline-step.done .pipeline-num {
  background: var(--accent-blue);
  color: #fff;
  border-color: var(--accent-blue);
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
}

.pipeline-text {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.pipeline-label {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-muted);
  transition: color 0.3s ease;
}

.pipeline-step.done .pipeline-label {
  color: var(--text-primary);
}

.pipeline-sub {
  font-size: 11px;
  color: var(--text-muted);
}

.pipeline-line {
  flex: 1;
  height: 2px;
  min-width: 20px;
  background: rgba(148, 163, 184, 0.14);
  border-radius: 1px;
  transition: background 0.3s ease;
}

.pipeline-line.done {
  background: linear-gradient(90deg, var(--accent-blue), rgba(59, 130, 246, 0.25));
}

/* ===== Entity Card V2 ===== */
.entity-card-v2 {
  display: flex;
  overflow: hidden;
  border: 1px solid rgba(148, 163, 184, 0.14);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.7);
  cursor: pointer;
  transition: all 0.22s ease;
}

.entity-card-v2:hover {
  border-color: rgba(59, 130, 246, 0.24);
  box-shadow: 0 8px 20px rgba(15, 23, 42, 0.06);
  transform: translateY(-1px);
}

.entity-card-v2.active {
  border-color: rgba(59, 130, 246, 0.36);
  background: rgba(59, 130, 246, 0.04);
  box-shadow: 0 0 0 1px rgba(59, 130, 246, 0.1) inset;
}

.entity-strip {
  width: 4px;
  flex-shrink: 0;
  background: var(--accent-blue);
  border-radius: 4px 0 0 4px;
  opacity: 0.5;
  transition: opacity 0.2s ease;
}

.entity-card-v2:hover .entity-strip,
.entity-card-v2.active .entity-strip {
  opacity: 1;
}

.entity-card-v2.type-email .entity-strip { background: #10b981; }
.entity-card-v2.type-template .entity-strip { background: #8b5cf6; }
.entity-card-v2.type-rule .entity-strip { background: #f59e0b; }

.entity-body-v2 {
  flex: 1;
  padding: 14px 16px;
  min-width: 0;
}

.entity-row-top {
  display: flex;
  align-items: center;
  gap: 12px;
}

.entity-icon-chip {
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  background: rgba(59, 130, 246, 0.1);
  color: var(--accent-blue);
  transition: all 0.2s ease;
}

.entity-icon-chip.email { background: rgba(16, 185, 129, 0.1); color: #10b981; }
.entity-icon-chip.template { background: rgba(139, 92, 246, 0.1); color: #8b5cf6; }
.entity-icon-chip.rule { background: rgba(245, 158, 11, 0.1); color: #f59e0b; }

.entity-info-v2 {
  flex: 1;
  min-width: 0;
}

.entity-detail-v2 {
  margin-top: 10px;
  padding-left: 48px;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.6;
}

/* ===== Custom Empty State ===== */
.ops-empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  min-height: 260px;
  padding: 40px 24px;
  text-align: center;
  border: 1px dashed rgba(148, 163, 184, 0.2);
  border-radius: 14px;
  background: rgba(248, 250, 252, 0.5);
}

.empty-icon-large {
  width: 60px;
  height: 60px;
  border-radius: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 26px;
  color: var(--text-muted);
  background: rgba(148, 163, 184, 0.08);
  margin-bottom: 4px;
}

.empty-icon-large.template { color: #8b5cf6; background: rgba(139, 92, 246, 0.08); }
.empty-icon-large.rule { color: #f59e0b; background: rgba(245, 158, 11, 0.08); }

.empty-state-title {
  color: var(--text-primary);
  font-size: 15px;
  font-weight: 700;
}

.empty-state-desc {
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.6;
  max-width: 320px;
}

/* ===== Editor V2 ===== */
.editor-head-v2 {
  overflow: hidden;
  border-radius: 16px 16px 0 0;
}

.editor-head-accent {
  height: 4px;
  background: linear-gradient(90deg, var(--accent-blue), #60a5fa, var(--accent-blue));
  background-size: 200% 100%;
  animation: gradientShift 4s ease infinite;
}

.editor-head-accent.channel {
  background: linear-gradient(90deg, var(--accent-blue), #38bdf8, var(--accent-blue));
  background-size: 200% 100%;
}

.editor-head-accent.template {
  background: linear-gradient(90deg, #8b5cf6, #a78bfa, #8b5cf6);
  background-size: 200% 100%;
}

.editor-head-accent.rule {
  background: linear-gradient(90deg, #f59e0b, #fbbf24, #f59e0b);
  background-size: 200% 100%;
}

@keyframes gradientShift {
  0%, 100% { background-position: 0% 50%; }
  50% { background-position: 100% 50%; }
}

.editor-head-content {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 18px 0;
}

/* ===== Editor Empty V2 ===== */
.editor-empty-v2 {
  display: flex;
  min-height: 280px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 32px;
  text-align: center;
}

.editor-empty-icon {
  width: 52px;
  height: 52px;
  border-radius: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 22px;
  color: var(--text-muted);
  background: rgba(148, 163, 184, 0.08);
  margin-bottom: 4px;
}

.editor-empty-icon.channel { color: var(--accent-blue); background: rgba(59, 130, 246, 0.08); }
.editor-empty-icon.template { color: #8b5cf6; background: rgba(139, 92, 246, 0.08); }
.editor-empty-icon.rule { color: #f59e0b; background: rgba(245, 158, 11, 0.08); }

/* ===== Footer V2 ===== */
.ops-footer-v2 {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.footer-status {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #10b981;
  transition: all 0.3s ease;
}

.status-dot.dirty {
  background: #f59e0b;
  box-shadow: 0 0 8px rgba(245, 158, 11, 0.5);
  animation: pulse-dot 1.5s ease infinite;
}

@keyframes pulse-dot {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

/* ===== Alert Sub-Tabs ===== */
.alert-tabs-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.alert-sub-tabs {
  display: flex;
  gap: 4px;
  padding: 4px;
  border-radius: 12px;
  background: rgba(148, 163, 184, 0.08);
  border: 1px solid rgba(148, 163, 184, 0.12);
}

.alert-sub-tab {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 38px;
  padding: 0 18px;
  border: 0;
  border-radius: 9px;
  background: transparent;
  color: var(--text-secondary);
  font-size: 14px;
  font-weight: 600;
  font-family: inherit;
  cursor: pointer;
  transition: all 0.22s ease;
}

.alert-sub-tab:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.6);
}

.alert-sub-tab.active {
  color: #fff;
  background: var(--accent-blue);
  box-shadow: 0 6px 16px rgba(59, 130, 246, 0.24);
}

.alert-sub-tab .el-icon {
  font-size: 16px;
}

.sub-tab-badge {
  min-width: 20px;
  height: 20px;
  padding: 0 6px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.2);
  color: inherit;
  font-size: 11px;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
}

.alert-sub-tab:not(.active) .sub-tab-badge {
  background: rgba(148, 163, 184, 0.12);
  color: var(--text-muted);
}

@media (max-width: 900px) {
  .alert-tabs-header {
    flex-direction: column;
    align-items: stretch;
  }

  .alert-sub-tabs {
    width: 100%;
  }

  .alert-sub-tab {
    flex: 1;
    justify-content: center;
  }

  .pipeline-flow {
    flex-wrap: wrap;
    gap: 8px;
  }

  .pipeline-line {
    display: none;
  }

  .ops-footer-v2 {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>



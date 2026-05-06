<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from '../utils/i18n'
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
  testNotification,
  testChannel,
  type AlertConfig,
  type AlertRule,
  type ClusterMember,
  type NotificationChannel,
  type NotificationConfig,
  type SystemSettings,
} from '../api/registry'

type CredentialView = 'grid' | 'list'
type EditMode = 'create' | 'edit'

const { t, locale } = useI18n()

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

interface TemplateVariable {
  name: string
  label: string
  desc: string
  example: string
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
const alertTesting = ref(false)

const notifyConfig = ref<NotificationConfig>({ channels: [] })
const alertConfig = ref<AlertConfig>({ rules: [] })

const channelEditorVisible = ref(false)
const ruleEditorVisible = ref(false)
const channelMode = ref<EditMode>('create')
const ruleMode = ref<EditMode>('create')
const showCustomTemplate = ref(false)

const channelCurrentPage = ref(1)
const ruleCurrentPage = ref(1)
const PAGE_SIZE = 9

const pagedChannels = computed(() => notifyConfig.value.channels.slice((channelCurrentPage.value - 1) * PAGE_SIZE, channelCurrentPage.value * PAGE_SIZE))
const pagedRules = computed(() => alertConfig.value.rules.slice((ruleCurrentPage.value - 1) * PAGE_SIZE, ruleCurrentPage.value * PAGE_SIZE))

const EVENT_OPTIONS = computed(() => [
  { value: 'service_register', label: t.value.settings.serviceRegister },
  { value: 'service_online', label: t.value.settings.serviceOnline },
  { value: 'service_offline', label: t.value.settings.serviceOffline },
  { value: 'service_heartbeat', label: t.value.settings.serviceHeartbeat },
  { value: 'service_remove', label: t.value.settings.serviceRemove },
  { value: 'registry_node_sync', label: t.value.settings.nodeSync },
])

const LOG_LEVEL_OPTIONS = ['DEBUG', 'INFO', 'WARN', 'ERROR'] as const

const MESSAGE_PROVIDER_OPTIONS = computed(() => [
  { label: t.value.settings.providers.generic, value: 'generic' },
  { label: t.value.settings.providers.dingtalk, value: 'dingtalk' },
  { label: t.value.settings.providers.feishu, value: 'feishu' },
  { label: t.value.settings.providers.wecom, value: 'wecom' },
])

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

const TEMPLATE_VARIABLES = computed<TemplateVariable[]>(() => [
  { name: 'event_code', label: locale.value === 'zh' ? '事件类型' : 'Event Code', desc: locale.value === 'zh' ? '触发事件的英文标识' : 'Internal event identifier', example: 'service_offline' },
  { name: 'event_name', label: locale.value === 'zh' ? '事件名称' : 'Event Name', desc: locale.value === 'zh' ? '事件类型的本地化名称' : 'Localized name of the event', example: locale.value === 'zh' ? '服务下线' : 'Service Offline' },
  { name: 'rule_name', label: locale.value === 'zh' ? '规则名称' : 'Rule Name', desc: locale.value === 'zh' ? '当前告警规则名称' : 'Name of the current alarm rule', example: locale.value === 'zh' ? '服务下线告警' : 'Offline Alarm' },
  { name: 'threshold', label: locale.value === 'zh' ? '阈值次数' : 'Threshold', desc: locale.value === 'zh' ? '规则配置的触发阈值' : 'Configured trigger threshold', example: '3' },
  { name: 'window_sec', label: locale.value === 'zh' ? '窗口秒数' : 'Window (Sec)', desc: locale.value === 'zh' ? '统计窗口时长（秒）' : 'Duration of the evaluation window in seconds', example: '300' },
  { name: 'window_min', label: locale.value === 'zh' ? '窗口分钟' : 'Window (Min)', desc: locale.value === 'zh' ? '统计窗口时长（分钟）' : 'Duration of the evaluation window in minutes', example: '5' },
  { name: 'count', label: locale.value === 'zh' ? '实际次数' : 'Actual Count', desc: locale.value === 'zh' ? '窗口内实际累计次数' : 'Cumulative count within the window', example: '5' },
  { name: 'service', label: locale.value === 'zh' ? '服务名' : 'Service Name', desc: locale.value === 'zh' ? '触发事件的服务名称' : 'Name of the service that triggered the event', example: 'order-service' },
  { name: 'instance', label: locale.value === 'zh' ? '实例地址' : 'Instance Addr', desc: locale.value === 'zh' ? '触发事件的实例地址' : 'Address of the instance that triggered the event', example: '10.0.1.5:8080' },
  { name: 'message', label: locale.value === 'zh' ? '事件描述' : 'Event Desc', desc: locale.value === 'zh' ? '事件的详细描述信息' : 'Detailed description information of the event', example: 'Instance deregistered' },
  { name: 'timestamp', label: locale.value === 'zh' ? '触发时间' : 'Timestamp', desc: locale.value === 'zh' ? '事件发生的时间' : 'Time when the event occurred', example: '2026-04-02 13:30:00' },
])

function getDynamicTitleTemplate() {
  return locale.value === 'zh' ? '芙卡洛斯告警 - {{ event_name }}' : 'Focalors Alert - {{ event_name }}'
}

function getDynamicBodyTemplate(eventCode: string) {
  if (locale.value === 'zh') {
    switch (eventCode) {
      case 'service_offline': return '服务下线告警：{{ service }}\n实例地址：{{ instance }}\n触发时间：{{ timestamp }}\n触发条件：{{ window_sec }} 秒内达到 {{ threshold }} 次\n请检查服务运行状态。'
      case 'service_heartbeat': return '服务心跳异常：{{ service }}\n实例地址：{{ instance }}\n触发时间：{{ timestamp }}\n说明：服务实例连续多次心跳失败。'
      case 'registry_node_sync': return '集群节点同步异常告警\n事件类型：{{ event_code }}\n触发时间：{{ timestamp }}\n影响范围：可能导致服务列表短暂不一致。'
      case 'service_online': return '服务上线通知：{{ service }}\n实例地址：{{ instance }}\n触发时间：{{ timestamp }}\n说明：新服务实例已上线并准备好接收流量。'
      case 'service_remove': return '服务移除告警：{{ service }}\n触发时间：{{ timestamp }}\n说明：服务已被管理员或系统从注册中心完全移除。'
      default: return '事件：{{ event_name }}（{{ event_code }}）\n触发条件：{{ window_sec }} 秒内达到 {{ threshold }} 次\n记录时间：{{ timestamp }}\n请及时关注系统状态。'
    }
  } else {
    switch (eventCode) {
      case 'service_offline': return 'Service Offline Alert: {{ service }}\nInstance: {{ instance }}\nTime: {{ timestamp }}\nCondition: {{ threshold }} times within {{ window_sec }}s\nPlease check the service status.'
      case 'service_heartbeat': return 'Heartbeat Failure: {{ service }}\nInstance: {{ instance }}\nTime: {{ timestamp }}\nDesc: Multiple consecutive heartbeat failures.'
      case 'registry_node_sync': return 'Node Sync Error\nEvent: {{ event_code }}\nTime: {{ timestamp }}\nImpact: Potential inconsistency in service list.'
      case 'service_online': return 'Service Online: {{ service }}\nInstance: {{ instance }}\nTime: {{ timestamp }}\nDesc: New instance is ready for traffic.'
      case 'service_remove': return 'Service Removed: {{ service }}\nTime: {{ timestamp }}\nDesc: Service was completely removed from the registry.'
      default: return 'Event: {{ event_name }} ({{ event_code }})\nCondition: {{ threshold }} times within {{ window_sec }}s\nTime: {{ timestamp }}\nPlease monitor system health.'
    }
  }
}

const dynamicTitleTemplate = computed(() => getDynamicTitleTemplate())
const dynamicBodyTemplate = computed(() => getDynamicBodyTemplate(ruleForm.value.event_code))

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

function emptyRule(item?: AlertRule): AlertRule {
  const eventCode = item?.event_code || 'service_offline'
  return {
    id: item?.id || '',
    name: item?.name || '',
    event_code: eventCode,
    threshold: item?.threshold || 1,
    window_sec: item?.window_sec || 300,
    channel_ids: [...(item?.channel_ids || [])],
    title_template: item?.title_template || getDynamicTitleTemplate(),
    body_template: item?.body_template || getDynamicBodyTemplate(eventCode),
    enabled: item?.enabled ?? true,
  }
}

const channelForm = ref<ChannelEditor>(emptyChannel())
const ruleForm = ref<AlertRule>(emptyRule())

function handleEventCodeChange(val: string) {
  // Check if current template is default (or default for previous event), if so update to new event default
  const currentBody = ruleForm.value.body_template
  const isBodyEmptyOrDefault = !currentBody || EVENT_OPTIONS.value.some(opt => getDynamicBodyTemplate(opt.value) === currentBody)
  
  if (isBodyEmptyOrDefault) {
    ruleForm.value.body_template = getDynamicBodyTemplate(val)
  }
}

function channelName(channelID: string) {
  return notifyConfig.value.channels.find((item) => item.id === channelID)?.name || '-'
}

function ruleChannelNames(channelIDs: string[]) {
  return channelIDs.map(channelName).join(locale.value === 'zh' ? '、' : ', ') || '-'
}

function insertVariable(varName: string) {
  const tag = `{{ ${varName} }}`
  if (!ruleForm.value.body_template) {
    ruleForm.value.body_template = tag
  } else {
    ruleForm.value.body_template += tag
  }
}

const defaultTitleTemplate = computed(() => {
  const rule = ruleForm.value
  const tpl = rule.title_template || dynamicTitleTemplate.value
  return tpl
    .replace(/\{\{\s*event_name\s*\}\}/g, eventLabel(rule.event_code))
    .replace(/\{\{\s*event_code\s*\}\}/g, rule.event_code)
    .replace(/\{\{\s*rule_name\s*\}\}/g, rule.name || '—')
    .replace(/\{\{\s*threshold\s*\}\}/g, String(rule.threshold || 1))
    .replace(/\{\{\s*window_sec\s*\}\}/g, String(rule.window_sec || 300))
    .replace(/\{\{\s*window_min\s*\}\}/g, String(Math.round((rule.window_sec || 300) / 60)))
})

const defaultBodyTemplate = computed(() => {
  const rule = ruleForm.value
  const tpl = rule.body_template || dynamicBodyTemplate.value
  return tpl
    .replace(/\{\{\s*event_name\s*\}\}/g, eventLabel(rule.event_code))
    .replace(/\{\{\s*event_code\s*\}\}/g, rule.event_code)
    .replace(/\{\{\s*rule_name\s*\}\}/g, rule.name || '—')
    .replace(/\{\{\s*threshold\s*\}\}/g, String(rule.threshold || 1))
    .replace(/\{\{\s*window_sec\s*\}\}/g, String(rule.window_sec || 300))
    .replace(/\{\{\s*window_min\s*\}\}/g, String(Math.round((rule.window_sec || 300) / 60)))
    .replace(/\{\{\s*service\s*\}\}/g, 'example-service')
    .replace(/\{\{\s*instance\s*\}\}/g, '10.0.1.5:8080')
    .replace(/\{\{\s*message\s*\}\}/g, 'Instance deregistered')
    .replace(/\{\{\s*timestamp\s*\}\}/g, new Date().toLocaleString(locale.value === 'zh' ? 'zh-CN' : 'en-US'))
    .replace(/\{\{\s*count\s*\}\}/g, String(rule.threshold || 1))
})

function providerLabel(provider: string) {
  return MESSAGE_PROVIDER_OPTIONS.value.find((item) => item.value === provider)?.label || provider || '-'
}

function channelTypeLabel(type: string) {
  return type === 'email' ? t.value.settings.email : t.value.settings.webhook
}

function eventLabel(code: string) {
  return EVENT_OPTIONS.value.find((item) => item.value === code)?.label || code
}

function createDefaultSettings(): SystemSettings {
  return {
    mode: 'standalone',
    consistency: 'ap',
    log_level: 'INFO',
    registry_flush_mode: 'async',
    registry_flush_interval_ms: 1000,
    event_storage_mode: 'memory',
    event_retention_days: 30,
    metrics_storage_mode: 'memory',
    log_retention_days: 30,
    event_types: EVENT_OPTIONS.value.map((item) => item.value),
    heartbeat_max_failures: 3,
    instance_removal_delay_seconds: 600,
    api_key_auth_enabled: false,
    notify_alert_node_id: '',
    metrics_retention_days: 30,
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
  const valid = new Set(EVENT_OPTIONS.value.map((item) => item.value))
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
    registry_flush_mode: input?.registry_flush_mode === 'sync' ? 'sync' : defaults.registry_flush_mode,
    registry_flush_interval_ms: Math.max(50, input?.registry_flush_interval_ms ?? defaults.registry_flush_interval_ms),
    event_storage_mode: input?.event_storage_mode === 'persistent' ? 'persistent' : defaults.event_storage_mode,
    event_retention_days: Math.max(1, input?.event_retention_days ?? defaults.event_retention_days),
    metrics_storage_mode: input?.metrics_storage_mode === 'persistent' ? 'persistent' : defaults.metrics_storage_mode,
    metrics_retention_days: Math.max(1, input?.metrics_retention_days ?? (defaults.metrics_retention_days || 30)),
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

const notifyNodeDirty = computed(() => (draftSettings.value.notify_alert_node_id || '') !== (appliedSettings.value.notify_alert_node_id || ''))
const ruleChannelOptions = computed(() => notifyConfig.value.channels)
const channelEditorTitle = computed(() => (channelMode.value === 'create' ? t.value.settings.addChannel : t.value.settings.editChannel))
const ruleEditorTitle = computed(() => (ruleMode.value === 'create' ? t.value.settings.addAlarmRule : t.value.settings.editAlarmRule))

function handleSettingsSaveError(error: any, fallback: string) {
  const data = error.response?.data
  if (data?.error === 'not leader' && data?.leader) {
    ElMessage.warning(`${locale.value === 'zh' ? '当前节点不是 Leader，请到' : 'Current node is not Leader, please go to'} ${data.leader} ${locale.value === 'zh' ? '节点执行保存' : 'to save'}`)
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
      ElMessage.warning(data.message || t.value.settings.restartWarning)
    } else {
      ElMessage.success(t.value.settings.saveSuccess)
    }
  } catch (error: any) {
    handleSettingsSaveError(error, locale.value === 'zh' ? '保存系统设置失败' : 'Failed to save system settings')
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
      ElMessage.warning(data.message || (locale.value === 'zh' ? '通知配置节点修改后需要重启生效' : 'Notification node change requires restart'))
    } else {
      ElMessage.success(t.value.common.success)
    }
  } catch (error: any) {
    handleSettingsSaveError(error, locale.value === 'zh' ? '保存通知配置节点失败' : 'Failed to save notification node config')
  } finally {
    notifyNodeSaving.value = false
  }
}

async function fetchNotifyConfig() {
  messageLoading.value = true
  try {
    const response = await getNotificationConfig('default')
    notifyConfig.value = response.data || { channels: [] }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || (locale.value === 'zh' ? '加载消息渠道配置失败' : 'Failed to load notification channels'))
  } finally {
    messageLoading.value = false
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

async function testChannelNotification() {
  if (channelForm.value.type === 'webhook' && !channelForm.value.webhookUrl) {
    ElMessage.warning('请先填写 Webhook 地址')
    return
  }
  if (channelForm.value.type === 'email' && !channelForm.value.emailFrom) {
    ElMessage.warning('请先填写发件人邮箱')
    return
  }
  
  messageSaving.value = true
  try {
    const config = buildChannelConfig(channelForm.value)
    const channel: NotificationChannel = {
      id: channelForm.value.id || 'test-channel',
      name: channelForm.value.name.trim() || '测试渠道',
      type: channelForm.value.type,
      provider: channelForm.value.type === 'email' ? 'smtp' : channelForm.value.provider,
      description: '',
      enabled: true,
      config,
    }
    await testChannel({ channel })
    ElMessage.success('测试通知已成功发送，请检查接收端')
  } catch (err: any) {
    console.error('[TestChannel] Failed:', err)
    ElMessage.error(err.response?.data?.error || '发送测试通知失败')
  } finally {
    messageSaving.value = false
  }
}

async function saveChannelDraft() {

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
    
    // Save to backend immediately
    messageSaving.value = true
    try {
      const nextNotifyConfig = { ...notifyConfig.value, channels }
      const response = await updateNotificationConfig('default', nextNotifyConfig)
      notifyConfig.value = response.data || nextNotifyConfig
      
      ElMessage.success(channelMode.value === 'create' ? '消息渠道已创建' : '消息渠道已保存')
      channelEditorVisible.value = false
      channelForm.value = emptyChannel()
    } catch (apiError: any) {
      ElMessage.error(apiError.response?.data?.error || '保存消息渠道失败')
    } finally {
      messageSaving.value = false
    }
  } catch (error: any) {
    ElMessage.error(error.message || '渠道配置格式错误')
  }
}

function removeChannel(channel: NotificationChannel) {
  ElMessageBox.confirm(`确定删除渠道 ${channel.name} 吗？`, '删除确认', { type: 'warning' })
    .then(async () => {
      const removedID = channel.id
      const prevNotify = notifyConfig.value
      const prevAlert = alertConfig.value

      const nextNotifyConfig = {
        ...notifyConfig.value,
        channels: notifyConfig.value.channels.filter((item) => item.id !== removedID),
      }
      const nextAlertConfig = {
        ...alertConfig.value,
        rules: alertConfig.value.rules.map((item) => ({
          ...item,
          channel_ids: item.channel_ids.filter((id) => id !== removedID),
        })),
      }

      try {
        messageSaving.value = true
        const resNotify = await updateNotificationConfig('default', nextNotifyConfig)
        notifyConfig.value = resNotify.data || nextNotifyConfig
        
        const resAlert = await updateAlertConfig('default', nextAlertConfig)
        alertConfig.value = resAlert.data || nextAlertConfig

        if (channelEditorVisible.value && channelForm.value.id === removedID) {
          closeChannelEditor()
        }
        ElMessage.success('渠道已删除')
      } catch (error: any) {
        notifyConfig.value = prevNotify
        alertConfig.value = prevAlert
        ElMessage.error(error.response?.data?.error || '删除渠道失败')
      } finally {
        messageSaving.value = false
      }
    })
    .catch(() => {})
}

async function fetchAlertRuleConfig() {
  alertLoading.value = true
  try {
    const response = await getAlertConfig('default')
    alertConfig.value = response.data || { rules: [] }
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

async function testAlertNotification() {
  if (ruleForm.value.channel_ids.length === 0) {
    ElMessage.warning('请至少选择一个通知渠道来测试')
    return
  }
  alertTesting.value = true
  try {
    await testNotification({
      rule: {
        ...ruleForm.value,
        title_template: defaultTitleTemplate.value,
        body_template: defaultBodyTemplate.value,
      }
    })
    ElMessage.success('测试通知已发送')
  } catch (err: any) {
    ElMessage.error(err.response?.data?.error || '发送测试通知失败')
  } finally {
    alertTesting.value = false
  }
}

function openRuleDialog(mode: EditMode, item?: AlertRule) {
  if (mode === 'create' && notifyConfig.value.channels.length === 0) {
    activeTab.value = 'channels'
    ElMessage.warning('请先新增一个消息渠道，再创建告警规则')
    return
  }
  ruleMode.value = mode
  ruleForm.value = emptyRule(item)
  showCustomTemplate.value = !!(item?.title_template || item?.body_template)
  ruleEditorVisible.value = true
}

function closeRuleEditor() {
  ruleEditorVisible.value = false
  ruleForm.value = emptyRule()
  showCustomTemplate.value = false
}

async function saveRuleDraft() {
  const payload: AlertRule = {
    id: ruleForm.value.id || createID('rule'),
    name: ruleForm.value.name.trim(),
    event_code: ruleForm.value.event_code,
    threshold: Math.max(1, ruleForm.value.threshold || 1),
    window_sec: Math.max(60, ruleForm.value.window_sec || 300),
    channel_ids: [...ruleForm.value.channel_ids],
    title_template: ruleForm.value.title_template?.trim() || '',
    body_template: ruleForm.value.body_template?.trim() || '',
    enabled: ruleForm.value.enabled,
  }
  if (!payload.name || !payload.event_code || payload.channel_ids.length === 0) {
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
  showCustomTemplate.value = false
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
    ElMessage.success(t.value.common.success)
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
          <span class="tab-label"><el-icon><Setting /></el-icon>{{ t.settings.basic }}</span>
        </template>

        <div class="tab-content basic-settings">
          <div class="basic-grid">
            <div class="settings-column">
              <section class="settings-section glass-card mode-section">
                <div class="section-header">
                  <el-icon><Operation /></el-icon>
                  <h4>{{ t.settings.runningMode }}</h4>
                  <div class="status-tags">
                    <div class="status-indicator">
                      <span class="indicator-dot" :class="appliedEnvironment === 'cluster' ? 'active' : 'idle'"></span>
                      <span class="dot-label">{{ t.settings.currentRunning }}</span>
                      <el-tag :type="appliedEnvironment === 'cluster' ? 'primary' : 'info'" size="small" effect="dark">
                        {{ appliedEnvironment === 'cluster' ? t.settings.clusterTitle : t.settings.singleTitle }}
                      </el-tag>
                    </div>
                    <div class="status-indicator consistency-indicator" v-if="appliedEnvironment === 'cluster'">
                      <span class="dot-label">{{ t.settings.consistency }}</span>
                      <span class="consistency-mode-tag" :class="appliedMode === 'cp' ? 'tag-cp-soft' : 'tag-ap-soft'">
                        {{ appliedMode === 'cp' ? t.settings.cpTitle : t.settings.apTitle }}
                      </span>
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
                          <div class="mode-title-v7">{{ t.settings.singleTitle }}</div>
                          <div class="mode-desc-v7">{{ t.settings.modeDescStandalone }}</div>
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
                          <div class="mode-title-v7">{{ t.settings.clusterTitle }}</div>

                          <div class="integrated-toggle-v7" :class="[previewMode]" v-if="previewEnvironment === 'cluster'" @click.stop>
                            <div class="toggle-option" :class="{ selected: previewMode === 'ap' }" @click="setDraftMode('ap')">
                              <div class="toggle-bg"></div>
                              <div class="toggle-text">
                                <span class="primary">{{ t.settings.apModeShort }}</span>
                                <span class="secondary">{{ t.settings.availabilityFirst }}</span>
                              </div>
                            </div>
                            <div class="toggle-option" :class="{ selected: previewMode === 'cp' }" @click="setDraftMode('cp')">
                              <div class="toggle-bg"></div>
                              <div class="toggle-text">
                                <span class="primary">{{ t.settings.cpModeShort }}</span>
                                <span class="secondary">{{ t.settings.consistencyFirst }}</span>
                              </div>
                            </div>
                          </div>
                          <div class="mode-desc-v7" style="margin-top: 8px;" v-if="previewEnvironment === 'cluster'">{{ previewMode === 'ap' ? t.settings.apDesc : t.settings.cpDesc }}</div>
                          <div class="mode-desc-v7" v-else>{{ t.settings.modeDescCluster }}</div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </section>

              <section class="settings-section glass-card registry-section">
                <div class="section-header">
                  <el-icon><Connection /></el-icon>
                  <div class="section-title-with-tip">
                    <h4>{{ t.settings.survivalStrategy }}</h4>
                    <el-tooltip
                      :content="t.settings.survivalStrategyTip" placement="top"
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
                          {{ t.settings.clientAuth }}
                          <el-tooltip :content="t.settings.clientAuthTip" placement="top">
                            <el-icon class="help-icon"><InfoFilled /></el-icon>
                          </el-tooltip>
                        </span>
                      </template>
                      <div class="inline-switch-control">
                        <el-switch v-model="draftSettings.api_key_auth_enabled" />
                      </div>
                    </el-form-item>

                    <el-form-item class="compact-number-item">
                      <template #label>
                        <span class="label-with-tip">
                          {{ t.settings.heartbeatTimeout }}
                          <el-tooltip
                            :content="t.settings.heartbeatTimeoutTip" placement="top"
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

                    <el-form-item class="compact-number-item">
                      <template #label>
                        <span class="label-with-tip">
                          {{ t.settings.instanceRetention }}
                          <el-tooltip
                            :content="t.settings.instanceRetentionTip" placement="top"
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

                    <el-form-item class="compact-segment-item">
                      <template #label>
                        <span class="label-with-tip">
                          {{ t.settings.registrySync }}
                          <el-tooltip :content="t.settings.registrySyncTip" placement="top">
                            <el-icon class="help-icon"><InfoFilled /></el-icon>
                          </el-tooltip>
                        </span>
                      </template>
                      <el-segmented
                        v-model="draftSettings.registry_flush_mode"
                        :options="[
                          { label: t.settings.asyncFlush, value: 'async' },
                          { label: t.settings.syncFlush, value: 'sync' }
                        ]"
                      />
                    </el-form-item>

                    <el-form-item
                      v-if="draftSettings.registry_flush_mode === 'async'"
                      class="flush-interval-item compact-number-item"
                    >
                      <template #label>
                        <span class="label-with-tip">
                          {{ t.settings.flushIntervalMs }}
                          <el-tooltip :content="t.settings.flushIntervalMsTip" placement="top">
                            <el-icon class="help-icon"><InfoFilled /></el-icon>
                          </el-tooltip>
                        </span>
                      </template>
                      <el-input-number
                        v-model="draftSettings.registry_flush_interval_ms"
                        :min="50"
                        :max="10000"
                        :step="50"
                        controls-position="right"
                        style="width: 100%"
                      />
                    </el-form-item>
                  </div>
                </el-form>
              </section>

            </div>

            <div class="settings-column">
              <section class="settings-section glass-card event-section">
                <div class="section-header">
                  <el-icon><Bell /></el-icon>
                  <h4>{{ t.settings.eventSettings }}</h4>
                </div>
                <el-form label-position="left" label-width="144px" class="compact-form basic-inline-form side-inline-form">
                  <el-form-item :label="locale === 'zh' ? '事件存储方式' : 'Storage Mode'" class="compact-segment-item">
                    <el-segmented
                      v-model="draftSettings.event_storage_mode"
                      :options="[
                        { label: locale === 'zh' ? '内存' : 'Memory', value: 'memory' },
                        { label: locale === 'zh' ? '持久化' : 'Persistent', value: 'persistent' }
                      ]"
                    />
                  </el-form-item>

                  <el-form-item :label="t.settings.retention" v-if="draftSettings.event_storage_mode === 'persistent'">
                    <div class="slider-row">
                      <el-slider v-model="draftSettings.event_retention_days" :min="1" :max="365" style="flex: 1" />
                      <span class="val-text">{{ draftSettings.event_retention_days }}</span>
                    </div>
                  </el-form-item>

                  <el-form-item :label="t.settings.triggerEvents">
                    <el-checkbox-group v-model="draftSettings.event_types" class="event-checkbox-grid">
                      <el-checkbox v-for="item in EVENT_OPTIONS" :key="item.value" :label="item.value">
                        {{ item.label }}
                      </el-checkbox>
                    </el-checkbox-group>
                  </el-form-item>
                </el-form>
              </section>

              <section class="settings-section glass-card log-section">
                <div class="section-header">
                  <el-icon><Document /></el-icon>
                  <div class="section-title-with-tip">
                    <h4>{{ t.settings.logConfig }}</h4>
                    <el-tooltip
                      :content="t.settings.logConfigTip" placement="top"
                    >
                      <el-icon class="help-icon"><InfoFilled /></el-icon>
                    </el-tooltip>
                  </div>
                </div>

                <el-form label-position="left" label-width="144px" class="compact-form basic-inline-form side-inline-form">
                  <el-form-item :label="t.settings.logLevel" class="log-level-item">
                    <el-segmented
                      v-model="draftSettings.log_level"
                      :options="LOG_LEVEL_OPTIONS.map(l => ({ label: l, value: l }))"
                    />
                  </el-form-item>

                  <el-form-item :label="t.settings.logRetention">
                    <div class="slider-row">
                      <el-slider v-model="draftSettings.log_retention_days" :min="1" :max="365" style="flex: 1" />
                      <span class="val-text">{{ draftSettings.log_retention_days }}</span>
                    </div>
                  </el-form-item>
                </el-form>
              </section>

              <section class="settings-section glass-card metrics-section">
                <div class="section-header">
                  <el-icon><RefreshRight /></el-icon>
                  <h4>{{ locale === 'zh' ? '指标配置' : 'Metrics Config' }}</h4>
                </div>
                <el-form label-position="left" label-width="144px" class="compact-form basic-inline-form side-inline-form">
                  <el-form-item :label="locale === 'zh' ? '存储方式' : 'Storage Mode'" class="compact-segment-item">
                    <el-segmented
                      v-model="draftSettings.metrics_storage_mode"
                      :options="[
                        { label: locale === 'zh' ? '内存' : 'Memory', value: 'memory' },
                        { label: locale === 'zh' ? '持久化' : 'Persistent', value: 'persistent' }
                      ]"
                    />
                  </el-form-item>
                  <el-form-item :label="t.settings.metricsRetention" v-if="draftSettings.metrics_storage_mode === 'persistent'">
                    <div class="slider-row">
                      <el-slider v-model="draftSettings.metrics_retention_days" :min="1" :max="365" style="flex: 1" />
                      <span class="val-text">{{ draftSettings.metrics_retention_days }}</span>
                    </div>
                  </el-form-item>
                </el-form>
              </section>
            </div>
          </div>

          <div class="save-toolbar">
            <div class="save-actions">
              <el-button @click="resetDraftSettings" :disabled="!isDirty || saveLoading">{{ t.settings.reset }}</el-button>
              <el-button type="primary" @click="saveSystemConfig" :loading="saveLoading" :disabled="!isDirty">
                {{ t.settings.saveSettings }}
              </el-button>
            </div>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane name="credentials">
        <template #label>
          <span class="tab-label"><el-icon><Key /></el-icon>{{ t.settings.credentials }}</span>
        </template>

        <div class="tab-content">
          <div class="content-header credentials-header">
            <p class="content-desc credentials-desc">{{ t.settings.credentialsDesc }}</p>
            <div class="credentials-toolbar">
              <div class="pill-group icon-pills credential-view-pills" aria-label="API Key 视图切换">
                <button
                  type="button"
                  :class="{ active: credentialView === 'list' }"
                  :title="t.settings.listView"
                  @click="credentialView = 'list'"
                >
                  <el-icon><List /></el-icon>
                </button>
                <button
                  type="button"
                  :class="{ active: credentialView === 'grid' }"
                  :title="t.settings.gridView"
                  @click="credentialView = 'grid'"
                >
                  <el-icon><Grid /></el-icon>
                </button>
              </div>
              <el-button type="primary" class="new-key-btn" @click="showDialog = true; generateRandKey()">
                <el-icon style="margin-right: 4px"><Plus /></el-icon>
                {{ t.settings.newApiKey }}
              </el-button>
            </div>
          </div>

          <div v-if="credentialView === 'grid'" class="key-grid" v-loading="keysLoading">
            <div v-for="item in keys" :key="item.key" class="key-card glass-card" :class="{ 'is-expired': item.status === 'expired' }">
              <div class="key-header">
                <code class="key-code" :title="item.key">{{ item.key.substring(0, 24) }}...</code>
                <div class="key-actions">
                  <el-tag v-if="item.status === 'expired'" type="danger" size="small" effect="dark" class="status-tag">
                    {{ t.settings.expired }}
                  </el-tag>
                  <el-button link type="primary" :icon="CopyDocument" @click="copyKey(item.key)" />
                  <el-button link type="danger" :icon="Delete" @click="deleteKey(item.key)" />
                </div>
              </div>
              <div class="key-body">
                <div class="key-desc">{{ item.description || t.settings.noDescription }}</div>
                <div class="key-info">
                  <div class="key-date">
                    <el-icon><Timer /></el-icon> {{ t.settings.expiration }}: {{ formatDate(item.expires_at) }}
                  </div>
                  <div class="key-date">
                    <el-icon><Timer /></el-icon> {{ t.settings.created }}: {{ formatDate(item.created_at) }}
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
              <el-table-column prop="description" :label="t.settings.description" min-width="180" />
              <el-table-column prop="status" :label="t.common.status" width="100">
                <template #default="scope">
                  <el-tag :type="scope.row.status === 'expired' ? 'danger' : 'success'" size="small">
                    {{ scope.row.status === 'expired' ? t.settings.expired : t.settings.activeStatus }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column :label="t.settings.expiration" width="180">
                <template #default="scope">
                  {{ formatDate(scope.row.expires_at) }}
                </template>
              </el-table-column>
              <el-table-column :label="t.settings.actions" width="80" align="right">
                <template #default="scope">
                  <el-button link type="danger" :icon="Delete" @click="deleteKey(scope.row.key)" />
                </template>
              </el-table-column>
            </el-table>
          </div>

          <el-empty v-if="!keysLoading && keys.length === 0" :description="t.settings.noCredentials" />
        </div>
      </el-tab-pane>

      <el-tab-pane name="channels">
        <template #label>
          <span class="tab-label"><el-icon><Share /></el-icon>{{ t.settings.channels }}</span>
        </template>

        <div class="tab-content ops-settings-tab" v-loading="messageLoading">
          <section class="settings-section glass-card ops-settings-section">
            <div class="ops-toolbar-v2">
              <div class="toolbar-left">
                <el-icon class="toolbar-icon"><Connection /></el-icon>
                <span class="toolbar-label">{{ t.settings.messageSendNode }}</span>
                <el-tooltip :content="t.settings.messageSendNodeTip" placement="bottom">
                  <el-select v-model="draftSettings.notify_alert_node_id" class="minimal-select" size="default" style="width: 200px">
                    <el-option
                      v-for="item in members"
                      :key="item.id"
                      :label="`${item.id}${item.is_local ? `（${t.cluster.currentNode}）` : ''}`"
                      :value="item.id"
                    />
                  </el-select>
                </el-tooltip>
                <el-button v-if="notifyNodeDirty" type="primary" link :loading="notifyNodeSaving" @click="saveNotifyAlertNode">
                  {{ t.settings.saveNodeConfig }}
                </el-button>
              </div>
              <div class="toolbar-right">
                <el-button type="primary" :icon="Plus" @click="openChannelDialog('create')">{{ t.settings.addChannel }}</el-button>
              </div>
            </div>

            <div class="ops-workbench alert-workbench" :class="{ 'with-editor': channelEditorVisible }">
              <div class="ops-list-panel">
                <div v-if="notifyConfig.channels.length > 0" class="entity-list-container">
                  <div class="entity-list">
                  <div
                    v-for="item in pagedChannels"
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
                        <el-tag :type="item.enabled ? 'success' : 'info'" size="small" effect="plain" class="status-tag">{{ item.enabled ? t.settings.channelEnabled : t.settings.channelDisabled }}</el-tag>
                        <div class="action-btn-group">
                          <el-button link type="primary" size="small" @click.stop="openChannelDialog('edit', item)">{{ t.common.edit }}</el-button>
                          <el-divider direction="vertical" />
                          <el-button link type="danger" size="small" @click.stop="removeChannel(item)">{{ t.common.delete }}</el-button>
                        </div>
                      </div>
                    </div>
                  </div>
                  </div>
                  <div class="list-pagination" v-if="notifyConfig.channels.length > 9">
                    <el-pagination
                      v-model:current-page="channelCurrentPage"
                      :page-size="9"
                      :total="notifyConfig.channels.length"
                      layout="prev, pager, next"
                      background
                    />
                  </div>
                </div>
                <div v-else class="ops-empty-state">
                  <div class="empty-icon-large">
                    <el-icon><Share /></el-icon>
                  </div>
                  <div class="empty-state-title">{{ t.settings.noChannel }}</div>
                  <div class="empty-state-desc">{{ t.settings.noChannelDesc }}</div>
                  <el-button type="primary" :icon="Plus" @click="openChannelDialog('create')">{{ t.settings.addFirstChannel }}</el-button>
                </div>
              </div>

              <Transition name="slide-panel">
                <div v-if="channelEditorVisible" class="ops-editor-panel glass-subcard">
                  <div class="editor-head-v2">
                    <div class="editor-head-accent channel"></div>
                    <div class="editor-head-content">
                      <div>
                        <div class="editor-title">{{ channelEditorTitle }}</div>
                      </div>
                      <div class="editor-actions">
                        <el-button @click="closeChannelEditor">{{ t.common.cancel }}</el-button>
                        <el-button :icon="Bell" @click="testChannelNotification">{{ t.settings.testNotice }}</el-button>
                        <el-button type="primary" :loading="messageSaving" @click="saveChannelDraft">{{ t.common.confirm }}</el-button>
                      </div>
                    </div>
                  </div>

                  <el-form label-position="left" label-width="100px" class="compact-form editor-form inline-label-form">
                    <div class="ops-form-grid-3col">
                      <el-form-item :label="t.common.name">
                        <el-input v-model="channelForm.name" />
                      </el-form-item>
                      <el-form-item :label="t.common.type">
                        <el-select v-model="channelForm.type" @change="handleChannelTypeChange">
                          <el-option :label="t.settings.webhook" value="webhook" />
                          <el-option :label="t.settings.email" value="email" />
                        </el-select>
                      </el-form-item>
                      <el-form-item :label="t.settings.provider">
                        <el-select v-model="channelForm.provider" :disabled="channelForm.type === 'email'">
                          <el-option v-for="item in MESSAGE_PROVIDER_OPTIONS" :key="item.value" :label="item.label" :value="item.value" />
                        </el-select>
                      </el-form-item>
                      <el-form-item :label="t.settings.channelStatus">
                        <el-switch v-model="channelForm.enabled" />
                      </el-form-item>
                    </div>

                    <el-form-item :label="t.settings.channelDesc">
                      <el-input v-model="channelForm.description" type="textarea" :rows="2" :placeholder="t.settings.channelDescPlaceholder" />
                    </el-form-item>

                    <template v-if="channelForm.type === 'webhook'">
                      <div class="ops-form-grid-3col">
                        <el-form-item :label="t.settings.webhookUrl" style="grid-column: span 2;">
                          <el-input v-model="channelForm.webhookUrl" />
                        </el-form-item>
                        <el-form-item :label="t.settings.webhookSecret">
                          <el-input v-model="channelForm.webhookSecret" show-password :placeholder="t.common.none" />
                        </el-form-item>
                      </div>
                    </template>
                    <template v-else>
                      <div class="ops-form-grid-2col channel-email-grid">
                        <el-form-item :label="t.settings.smtpHost">
                          <el-input v-model="channelForm.emailHost" />
                        </el-form-item>
                        <el-form-item :label="t.settings.smtpPort">
                          <el-input-number v-model="channelForm.emailPort" :min="1" :max="65535" controls-position="right" style="width: 100%" />
                        </el-form-item>
                        <el-form-item :label="t.settings.smtpUser">
                          <el-input v-model="channelForm.emailUsername" :placeholder="t.common.none" />
                        </el-form-item>
                        <el-form-item :label="t.settings.smtpPass">
                          <el-input v-model="channelForm.emailPassword" show-password :placeholder="t.common.none" />
                        </el-form-item>
                        <el-form-item :label="t.settings.smtpFrom">
                          <el-input v-model="channelForm.emailFrom" />
                        </el-form-item>
                        <el-form-item :label="t.settings.enableTls">
                          <el-switch v-model="channelForm.emailUseTLS" />
                        </el-form-item>
                      </div>

                      <el-form-item :label="t.settings.emailRecipients">
                        <el-input
                          v-model="channelForm.emailRecipientsText"
                          type="textarea"
                          :rows="4"
                          :placeholder="locale === 'zh' ? 'ops@example.com\\nowner@example.com' : 'ops@example.com\\nowner@example.com'"
                        />
                      </el-form-item>
                    </template>

                    <el-form-item :label="t.settings.extraConfig">
                      <el-input
                        v-model="channelForm.extraConfigText"
                        type="textarea"
                        :rows="4"
                        :placeholder="t.settings.extraConfigPlaceholder"
                      />
                    </el-form-item>
                  </el-form>
                </div>
              </Transition>
            </div>

            <!-- Removed redundant footer save button -->
          </section>
        </div>
      </el-tab-pane>

      <el-tab-pane name="monitoring">
        <template #label>
          <span class="tab-label"><el-icon><Monitor /></el-icon>{{ t.settings.alarmConfig }}</span>
        </template>

        <div class="tab-content ops-settings-tab" v-loading="alertLoading">
          <section class="settings-section glass-card ops-settings-section">
            <div class="ops-toolbar-v2" style="margin-bottom: 0;">
              <div class="toolbar-left">
                <el-icon class="toolbar-icon"><Bell /></el-icon>
                <span class="toolbar-label">{{ t.settings.alarmRules }}</span>
                <span style="font-size: 12px; color: var(--text-muted); margin-left: 8px;">{{ t.settings.alarmRulesDesc }}</span>
              </div>
              <div class="toolbar-right">
                <el-button type="primary" :icon="Plus" @click="openRuleDialog('create')">{{ t.settings.addAlarmRule }}</el-button>
              </div>
            </div>

            <div class="ops-workbench alert-workbench" :class="{ 'with-editor': ruleEditorVisible }">
              <div class="ops-list-panel">
                <div v-if="alertConfig.rules.length > 0" class="entity-list-container">
                  <div class="entity-list">
                  <div v-for="item in pagedRules" :key="item.id" class="entity-card-v2 type-rule" :class="{ active: ruleEditorVisible && ruleForm.id === item.id }" @click="openRuleDialog('edit', item)">
                    <div class="entity-strip"></div>
                    <div class="entity-body-v2">
                      <div class="entity-row-top">
                        <div class="entity-icon-chip rule"><el-icon><Timer /></el-icon></div>
                        <div class="entity-info-v2">
                          <div class="entity-title">{{ item.name }}</div>
                          <div class="entity-subline">{{ eventLabel(item.event_code) }}</div>
                        </div>
                      </div>
                      <div class="entity-detail-v2">{{ t.settings.ruleSummary.replace('{sec}', String(item.window_sec)).replace('{count}', String(item.threshold)) }} · {{ ruleChannelNames(item.channel_ids) }}</div>
                      <div class="entity-actions-v2">
                        <el-tag :type="item.enabled ? 'success' : 'info'" size="small" effect="plain" class="status-tag">{{ item.enabled ? t.settings.channelEnabled : t.settings.channelDisabled }}</el-tag>
                        <div class="action-btn-group">
                          <el-button link type="primary" size="small" @click.stop="openRuleDialog('edit', item)">{{ t.common.edit }}</el-button>
                          <el-divider direction="vertical" />
                          <el-button link type="danger" size="small" @click.stop="removeRule(item)">{{ t.common.delete }}</el-button>
                        </div>
                      </div>
                    </div>
                  </div>
                  </div>
                  <div class="list-pagination" v-if="alertConfig.rules.length > 9">
                    <el-pagination
                      v-model:current-page="ruleCurrentPage"
                      :page-size="9"
                      :total="alertConfig.rules.length"
                      layout="prev, pager, next"
                      background
                    />
                  </div>
                </div>
                <div v-else class="ops-empty-state">
                  <div class="empty-icon-large rule"><el-icon><Timer /></el-icon></div>
                  <div class="empty-state-title">{{ t.settings.noAlarmRule }}</div>
                  <div class="empty-state-desc">{{ t.settings.noAlarmRuleDesc }}</div>
                  <el-button type="primary" :icon="Plus" @click="openRuleDialog('create')">{{ t.settings.addFirstAlarmRule }}</el-button>
                </div>
              </div>
              <Transition name="slide-panel">
                <div v-if="ruleEditorVisible" class="ops-editor-panel glass-subcard">
                  <div class="editor-head-v2">
                    <div class="editor-head-accent rule"></div>
                    <div class="editor-head-content">
                      <div>
                        <div class="editor-title">{{ ruleEditorTitle }}</div>
                        <div class="editor-subtitle">{{ t.settings.alarmRulesDesc }}</div>
                      </div>
                      <div class="editor-actions">
                        <el-button @click="testAlertNotification" :loading="alertTesting">{{ t.settings.testNotice }}</el-button>
                        <el-button @click="closeRuleEditor">{{ t.common.cancel }}</el-button>
                        <el-button type="primary" :loading="alertSaving" @click="saveRuleDraft">{{ t.common.confirm }}</el-button>
                      </div>
                    </div>
                  </div>
                  <el-form label-position="left" label-width="80px" class="compact-form editor-form inline-label-form" style="padding-right: 16px;">
                    <div class="ops-form-grid-3col">
                      <el-form-item :label="t.common.name"><el-input v-model="ruleForm.name" /></el-form-item>
                      <el-form-item :label="t.common.type">
                        <el-select v-model="ruleForm.event_code" @change="handleEventCodeChange">
                          <el-option v-for="item in EVENT_OPTIONS" :key="item.value" :label="item.label" :value="item.value" />
                        </el-select>
                      </el-form-item>
                      <el-form-item :label="t.settings.channelStatus"><el-switch v-model="ruleForm.enabled" /></el-form-item>
                    </div>
                    <div class="ops-form-grid-3col">
                      <el-form-item :label="t.settings.notificationChannel">
                        <el-select v-model="ruleForm.channel_ids" multiple collapse-tags collapse-tags-tooltip>
                          <el-option v-for="item in ruleChannelOptions" :key="item.id" :label="item.enabled ? item.name : `${item.name}（${t.settings.channelDisabled}）`" :value="item.id" />
                        </el-select>
                      </el-form-item>
                      <el-form-item :label="t.settings.ruleCondition" style="grid-column: span 2;"><div style="display:flex; gap:8px; align-items:center;"><el-input-number v-model="ruleForm.window_sec" :min="60" controls-position="right" style="width: 120px;" /><span style="font-size:13px; color:var(--text-secondary); white-space:nowrap">{{ t.settings.ruleConditionPrefix }}</span><el-input-number v-model="ruleForm.threshold" :min="1" controls-position="right" style="width: 100px;" /><span style="font-size:13px; color:var(--text-secondary)">{{ t.settings.ruleConditionSuffix }}</span></div></el-form-item>
                    </div>

                    <div class="template-section" style="margin-top: 16px; border: 1px solid var(--border-color); overflow: hidden;">
                      <div class="template-section-header" style="background: rgba(0,0,0,0.02); padding: 12px 16px; display: flex; align-items: center; gap: 8px; border-bottom: 1px solid var(--border-color);">
                        <el-icon><Operation /></el-icon>
                        <span style="font-weight: 600; font-size: 14px;">{{ t.settings.messageTemplate }}</span>
                      </div>

                      <div style="padding: 16px; display: flex; gap: 16px;">
                        <div style="flex: 1; display: flex; flex-direction: column;">
                          <el-form-item :label="t.settings.titleTemplate" label-width="70px" style="margin-bottom: 16px;">
                            <el-input v-model="ruleForm.title_template" :placeholder="dynamicTitleTemplate" />
                          </el-form-item>
                          <el-form-item :label="t.settings.bodyTemplate" label-width="70px" style="margin-bottom: 8px;">
                            <el-input v-model="ruleForm.body_template" type="textarea" :rows="5" :placeholder="dynamicBodyTemplate" />
                          </el-form-item>
                          
                          <div style="margin-left: 70px; margin-top: 4px; display: flex; flex-wrap: wrap; gap: 6px;">
                            <el-tooltip v-for="variable in TEMPLATE_VARIABLES" :key="variable.name" placement="top">
                              <template #content>
                                <div>
                                  <div>{{ variable.desc }}</div>
                                  <div style="color: #94a3b8; margin-top: 4px;">示例：{{ variable.example }}</div>
                                </div>
                              </template>
                              <div @click="insertVariable(variable.name)" style="font-size: 12px; background: #fff; border: 1px solid #e2e8f0; padding: 2px 6px; border-radius: 4px; cursor: pointer; transition: all 0.2s;">
                                <span style="color: #3b82f6; font-family: monospace;" v-text="'{{ ' + variable.name + ' }}'"></span>
                                <span style="color: #64748b; margin-left: 4px;">{{ variable.label }}</span>
                              </div>
                            </el-tooltip>
                          </div>
                        </div>
                        
                        <div style="width: 250px; flex-shrink: 0; background: #f8fafc; border-left: 1px dashed #e2e8f0; padding: 0 12px;">
                          <div style="font-size: 12px; font-weight: 600; color: #64748b; margin-bottom: 12px; display: flex; align-items: center; gap: 4px;"><el-icon><Monitor /></el-icon> {{ locale === 'zh' ? '实时预览' : 'Live Preview' }}</div>
                          <div style="background: #fff; padding: 12px; border-radius: 6px; border: 1px solid rgba(148, 163, 184, 0.14); box-shadow: 0 2px 6px rgba(0,0,0,0.02);">
                            <div style="font-weight: 600; margin-bottom: 8px; font-size: 13px; color: var(--text-primary);">{{ defaultTitleTemplate }}</div>
                            <div style="white-space: pre-wrap; font-size: 12px; color: var(--text-secondary); line-height: 1.6; word-break: break-all;">{{ defaultBodyTemplate }}</div>
                          </div>
                        </div>
                      </div>
                    </div>
                  </el-form>
                </div>
              </Transition>
            </div>
          </section>
        </div>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="showDialog" :title="t.settings.newApiKey" width="560px" top="6vh" class="glass-dialog credential-dialog">
      <el-form label-position="top" class="compact-form credential-dialog-form">
        <el-form-item label="API Key">
          <div class="key-preview-shell">
            <div class="key-preview-value" :title="keyForm.key">{{ keyForm.key }}</div>
            <el-button class="regenerate-key-btn" @click="generateRandKey" :icon="RefreshRight" />
          </div>
          <div class="field-hint">{{ locale === 'zh' ? '创建前可重复生成，完整密钥会按原样展示。' : 'Can be regenerated before creation, the full key is displayed as-is.' }}</div>
        </el-form-item>

        <el-form-item :label="t.settings.description">
          <el-input
            v-model="keyForm.description"
            type="textarea"
            :rows="3"
            :placeholder="locale === 'zh' ? '请输入描述' : 'Enter a description'"
          />
        </el-form-item>

        <el-form-item :label="locale === 'zh' ? '有效期' : 'Expiration'">
          <el-select v-model="keyForm.expDays" style="width: 100%">
            <el-option :label="t.settings.neverExpire" :value="0" />
            <el-option label="7 {{ t.settings.expireIn.replace('{n}', '7') }}" :value="7" />
            <el-option label="30 {{ t.settings.expireIn.replace('{n}', '30') }}" :value="30" />
            <el-option label="90 {{ t.settings.expireIn.replace('{n}', '90') }}" :value="90" />
            <el-option label="365 {{ t.settings.expireIn.replace('{n}', '365') }}" :value="365" />
          </el-select>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="showDialog = false">{{ t.common.cancel }}</el-button>
        <el-button type="primary" @click="handleGenerate">{{ locale === 'zh' ? '创建' : 'Create' }}</el-button>
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

:global(html[data-theme="dark"]) .settings-container .status-indicator,
:global(html.dark) .settings-container .status-indicator {
  border-color: rgba(71, 85, 105, 0.9) !important;
  background: rgba(15, 23, 42, 0.94) !important;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04) !important;
}

:global(html[data-theme="dark"]) .settings-container .dot-label,
:global(html.dark) .settings-container .dot-label {
  color: #cbd5e1 !important;
}

:global(html[data-theme="dark"]) .settings-container .consistency-mode-tag.tag-ap-soft,
:global(html.dark) .settings-container .consistency-mode-tag.tag-ap-soft {
  color: #34d399 !important;
  border-color: rgba(16, 185, 129, 0.26) !important;
  background: rgba(16, 185, 129, 0.12) !important;
}

:global(html[data-theme="dark"]) .settings-container .consistency-mode-tag.tag-cp-soft,
:global(html.dark) .settings-container .consistency-mode-tag.tag-cp-soft {
  color: #fb923c !important;
  border-color: rgba(249, 115, 22, 0.28) !important;
  background: rgba(249, 115, 22, 0.12) !important;
}

:global(html[data-theme="dark"]) .settings-container .integrated-toggle-v7,
:global(html.dark) .settings-container .integrated-toggle-v7 {
  background: rgba(255, 255, 255, 0.06) !important;
}

:global(html[data-theme="dark"]) .settings-container .save-toolbar,
:global(html.dark) .settings-container .save-toolbar {
  background: linear-gradient(180deg, rgba(10, 14, 26, 0), rgba(10, 14, 26, 0.9) 36%, rgba(10, 14, 26, 0.98)) !important;
}

:global(html[data-theme="dark"]) .settings-container .ops-empty-state,
:global(html.dark) .settings-container .ops-empty-state {
  border-color: rgba(71, 85, 105, 0.32) !important;
  background: rgba(15, 23, 42, 0.88) !important;
}

:global(html[data-theme="dark"]) .settings-container .empty-icon-large,
:global(html.dark) .settings-container .empty-icon-large {
  background: rgba(255, 255, 255, 0.04) !important;
}

:global(html[data-theme="dark"]) .settings-container .empty-state-title,
:global(html.dark) .settings-container .empty-state-title {
  color: #e2e8f0 !important;
}

:global(html[data-theme="dark"]) .settings-container .empty-state-desc,
:global(html.dark) .settings-container .empty-state-desc {
  color: #94a3b8 !important;
}

:global(html[data-theme="dark"]) .settings-container .template-section-header,
:global(html.dark) .settings-container .template-section-header {
  background: rgba(255, 255, 255, 0.04) !important;
}

:global(html[data-theme="dark"]) .settings-container .template-section div[style*="background: #f8fafc"],
:global(html.dark) .settings-container .template-section div[style*="background: #f8fafc"] {
  background: rgba(15, 23, 42, 0.86) !important;
  border-left-color: rgba(71, 85, 105, 0.7) !important;
}

:global(html[data-theme="dark"]) .settings-container .template-section div[style*="background: #fff; padding: 12px; border-radius: 6px;"],
:global(html.dark) .settings-container .template-section div[style*="background: #fff; padding: 12px; border-radius: 6px;"] {
  background: rgba(15, 23, 42, 0.96) !important;
  box-shadow: inset 0 0 0 1px rgba(71, 85, 105, 0.35) !important;
}

:global(html[data-theme="dark"]) .settings-container .template-section div[style*="background: #fff; border: 1px solid #e2e8f0"],
:global(html.dark) .settings-container .template-section div[style*="background: #fff; border: 1px solid #e2e8f0"] {
  background: rgba(15, 23, 42, 0.92) !important;
  border-color: rgba(71, 85, 105, 0.9) !important;
}

:global(html[data-theme="dark"]) .settings-container .template-section span[style*="color: #64748b"],
:global(html.dark) .settings-container .template-section span[style*="color: #64748b"] {
  color: #cbd5e1 !important;
}

.settings-tabs {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.settings-tabs :deep(.el-tabs__header) {
  margin-bottom: 10px;
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
  height: 100%;
  min-height: 0;
}

.basic-settings {
  display: flex;
  flex-direction: column;
  height: 100%;
  gap: 10px;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding-right: 4px;
}

.basic-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.48fr) minmax(360px, 0.92fr);
  gap: 10px;
  align-items: start;
}

.settings-column {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
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
  overflow: hidden;
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
  border-radius: 0;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(248, 250, 252, 0.96));
}

.glass-subcard :deep(.el-input__wrapper),
.glass-subcard :deep(.el-select__wrapper),
.glass-subcard :deep(.el-textarea__inner),
.glass-subcard :deep(.el-button) {
  border-radius: 0 !important;
}

.entity-list-container {
  display: flex;
  flex-direction: column;
  gap: 16px;
  height: 100%;
}

.entity-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 320px), 1fr));
  gap: 16px;
  align-content: start;
}

.list-pagination {
  display: flex;
  justify-content: flex-end;
  padding-top: 12px;
  margin-top: auto;
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
  padding: 6px 24px 18px 18px;
  flex: 1;
  overflow-y: auto;
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
  padding: 12px 14px;
}

.mode-section {
  padding: 14px;
}

.registry-section {
  padding: 10px 14px 8px;
}

.registry-form {
  margin-top: 10px;
}

.event-section,
.log-section,
.metrics-section {
  padding-top: 10px;
  padding-bottom: 10px;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 6px;
  flex-wrap: wrap;
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
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
  margin-left: auto;
}

.status-indicator {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 34px;
  padding: 5px 12px;
  border-radius: 999px;
  border: none;
  background: transparent;
  box-shadow: none;
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
  font-weight: 700;
  color: var(--text-muted);
}

.status-indicator :deep(.el-tag) {
  border-radius: 999px;
  font-size: 12px;
  font-weight: 700;
  height: 28px;
  padding: 0 12px;
}

.consistency-mode-tag {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 28px;
  padding: 0 12px;
  border: none;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.01em;
  white-space: nowrap;
}

.consistency-indicator {
  gap: 10px;
}

.tag-ap-soft {
  color: #059669;
  border-color: rgba(16, 185, 129, 0.24);
  background: rgba(16, 185, 129, 0.1);
}

.tag-cp-soft {
  color: #c2410c;
  border-color: rgba(249, 115, 22, 0.24);
  background: rgba(249, 115, 22, 0.1);
}

:global(html[data-theme="dark"]) .settings-container .status-indicator,
:global(html.dark) .settings-container .status-indicator {
  border: none;
  background: transparent;
  box-shadow: none;
}

:global(html[data-theme="dark"]) .settings-container .dot-label,
:global(html.dark) .settings-container .dot-label {
  color: #cbd5e1;
}

:global(html[data-theme="dark"]) .settings-container .consistency-mode-tag.tag-ap-soft,
:global(html.dark) .settings-container .consistency-mode-tag.tag-ap-soft {
  color: #34d399;
  border: none;
  background: rgba(16, 185, 129, 0.12);
}

:global(html[data-theme="dark"]) .settings-container .consistency-mode-tag.tag-cp-soft,
:global(html.dark) .settings-container .consistency-mode-tag.tag-cp-soft {
  color: #fb923c;
  border: none;
  background: rgba(249, 115, 22, 0.12);
}

:global(html[data-theme="dark"]) .settings-container .integrated-toggle-v7,
:global(html.dark) .settings-container .integrated-toggle-v7 {
  background: rgba(255, 255, 255, 0.06);
}

.consistency-wrapper-v7 {
  margin-top: 2px;
}

.main-mode-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  align-items: stretch;
}

.mode-card-v7 {
  position: relative;
  min-height: 152px;
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
  gap: 12px;
  width: 100%;
  padding: 18px 18px 16px;
}

.mode-icon-v7 {
  width: 42px;
  height: 42px;
  flex-shrink: 0;
  border-radius: 10px;
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
  margin-top: 6px;
  padding: 2px;
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.04);
}

.toggle-option {
  position: relative;
  min-width: 84px;
  padding: 4px 10px;
  overflow: hidden;
  cursor: pointer;
  text-align: center;
  border-radius: 6px;
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
  align-items: center;
  padding: 13px 16px;
  border: 1px solid var(--border-color);
  border-radius: 12px;
  background: rgba(0, 0, 0, 0.02);
}

.info-bubble-v7 .el-icon {
  font-size: 18px;
  flex-shrink: 0;
}

.bubble-content {
  display: flex;
  align-items: center;
  flex: 1;
  min-height: 24px;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.bubble-copy {
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
}

.bubble-copy strong {
  color: var(--text-primary);
  font-weight: 700;
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
  margin-bottom: 10px;
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
  padding-right: 14px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary);
  white-space: nowrap;
}

.registry-form :deep(.el-form-item__label) {
  padding-top: 4px;
}

.side-inline-form :deep(.el-form-item) {
  margin-bottom: 8px;
}

.registry-form :deep(.el-form-item) {
  margin-bottom: 6px;
}

.ops-form-grid-3col {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px 16px;
}

.ops-form-grid-2col {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px 16px;
}

.inline-label-form :deep(.el-form-item) {
  display: flex;
  align-items: flex-start;
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
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px 12px;
}

.switch-form-item {
  grid-column: 1 / -1;
}

.flush-interval-item {
  grid-column: auto;
}

.switch-form-item :deep(.el-form-item__content) {
  display: flex;
  align-items: center;
  min-height: 28px;
}

.inline-switch-control {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
}

.compact-number-item :deep(.el-input-number) {
  width: min(200px, 100%);
}

.compact-number-item.flush-interval-item :deep(.el-input-number) {
  width: min(220px, 100%);
}

.compact-segment-item :deep(.el-segmented) {
  width: auto;
  max-width: 176px;
  border: none;
  background: transparent;
  box-shadow: none;
  padding: 2px;
}

.compact-segment-item :deep(.el-segmented__item) {
  min-width: 0;
  min-height: 32px;
  padding: 0 14px;
  font-size: 12px;
}

.side-inline-form .compact-segment-item :deep(.el-segmented) {
  max-width: none;
}

.side-inline-form .compact-segment-item :deep(.el-segmented__item) {
  min-width: 72px;
  padding: 0 18px;
}

.log-level-item :deep(.el-segmented) {
  width: auto;
  max-width: 320px;
  border: none;
  background: transparent;
  box-shadow: none;
  padding: 3px;
}

.log-level-item :deep(.el-segmented__item) {
  min-width: 66px;
  min-height: 34px;
  padding: 0 12px;
  font-size: 12px;
  font-weight: 700;
}

.slider-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 28px;
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
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 2px 14px;
}

.event-checkbox-grid :deep(.el-checkbox) {
  display: inline-flex;
  align-items: center;
  margin-right: 0;
  min-height: 22px;
  line-height: 1.2;
}

.event-checkbox-grid :deep(.el-checkbox__label) {
  padding-left: 6px;
  line-height: 1.2;
}

.save-toolbar {
  display: flex;
  justify-content: flex-end;
  position: sticky;
  bottom: 0;
  z-index: 5;
  margin-top: 2px;
  padding: 10px 0 4px;
  background: transparent;
  min-height: 32px;
}

:global(html[data-theme="dark"]) .settings-container .save-toolbar,
:global(html.dark) .settings-container .save-toolbar {
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
  height: 32px;
}

.save-actions .el-button {
  height: 32px;
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
  grid-template-columns: minmax(0, 1fr) 44px;
  gap: 10px;
  align-items: stretch;
  width: 100%;
}

.key-preview-value {
  min-height: 46px;
  padding: 11px 14px;
  border-radius: 12px;
  border: 1px solid rgba(148, 163, 184, 0.24);
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.08), rgba(255, 255, 255, 0.92));
  color: var(--text-primary);
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 12px;
  line-height: 1.45;
  word-break: break-all;
  white-space: normal;
  overflow-wrap: anywhere;
}

.regenerate-key-btn {
  width: 44px;
  min-height: 46px;
  padding: 0;
  border-radius: 12px;
}

.field-hint {
  margin-top: 6px;
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
  padding: 20px 22px 6px;
}

:deep(.credential-dialog .el-dialog__title) {
  font-size: 22px;
  font-weight: 800;
  line-height: 1.2;
  color: var(--text-primary);
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
  margin-bottom: 14px;
}

:deep(.credential-dialog .el-form-item__label) {
  padding-bottom: 6px;
  color: var(--text-primary);
  font-weight: 700;
  line-height: 1.4;
}

:deep(.credential-dialog .el-input__wrapper),
:deep(.credential-dialog .el-select__wrapper) {
  min-height: 42px;
  border-radius: 12px;
}

:deep(.credential-dialog .el-textarea__inner) {
  min-height: 84px !important;
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
    width: 144px !important;
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
  max-width: 480px;
  margin: 40px auto;
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
  border-radius: 0;
  flex-shrink: 0;
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
  padding: 18px 24px 12px 18px;
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




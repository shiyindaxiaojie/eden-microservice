import api from '../index'

export interface Namespace {
  name: string
  description?: string
  created_at: string
  updated_at?: string
}
export interface ServiceSummary {
  name: string
  instance_count: number
  healthy_count: number
  subscriber_count?: number
}

export interface TopologyInstance {
  id: string
  host: string
  port: number
  status: 'passing' | 'critical'
  datacenter?: string
}

export interface TopologyNode {
  id: string
  name: string
  namespace: string
  instance_count: number
  healthy_count: number
  instances: TopologyInstance[]
}

export interface TopologyEdge {
  source: string
  target: string
  checksum?: string
  updated_at?: string
}

export interface TopologyGraph {
  namespace: string
  nodes: TopologyNode[]
  edges: TopologyEdge[]
}

export interface Instance {
  id: string
  service_name: string
  namespace?: string
  host: string
  port: number
  weight: number
  datacenter: string
  metadata: Record<string, string>
  status: 'passing' | 'critical'
  manual_offline?: boolean
  last_heartbeat: string
  registered_at: string
}

export interface ClusterMember {
  id: string
  address: string
  role: string
  status: 'Online' | 'Offline'
  is_local?: boolean
  http_addr?: string
  grpc_addr?: string
  raft_addr?: string
  quic_addr?: string
}

export interface ClusterStats {
  node_count: number
  service_count: number
  instance_count: number
  healthy_count: number
  health_rate: number
  is_leader: boolean
  leader_addr: string
  memory_usage: number
  mode: 'ap' | 'cp'
  environment: 'standalone' | 'cluster'
  role: string
}

export interface RegistryEvent {
  id: number
  type: string
  service: string
  instance: string
  message: string
  timestamp: string
}

export interface SystemSettings {
  mode: 'standalone' | 'cluster'
  consistency: 'ap' | 'cp'
  log_level: string
  event_retention_days: number
  log_retention_days: number
  event_types: string[]
  heartbeat_max_failures: number
  instance_removal_delay_seconds: number
  api_key_auth_enabled: boolean
  notify_alert_node_id?: string
}

export interface ApplySystemSettingsResult {
  status: string
  restart_required?: boolean
  message?: string
}

export interface NotificationChannel {
  id: string
  name: string
  type: 'webhook' | 'email' | string
  provider: 'generic' | 'dingtalk' | 'feishu' | 'wecom' | string
  description?: string
  enabled: boolean
  config?: Record<string, any>
}

export interface NotificationConfig {
  channels: NotificationChannel[]
  updated_at?: string
}

export interface AlertRule {
  id: string
  name: string
  event_code: string
  threshold?: number
  window_sec?: number
  channel_ids: string[]
  title_template?: string
  body_template?: string
  enabled: boolean
}

export interface AlertConfig {
  rules: AlertRule[]
  updated_at?: string
}

export interface RbacUser {
  username: string
  password?: string
  nickname?: string
  phone?: string
  email?: string
  remark?: string
  role: string
  is_builtin?: boolean
}

export const getServices = (namespace = '') => api.get<ServiceSummary[]>(`/v1/catalog/services${namespace ? `?namespace=${encodeURIComponent(namespace)}` : ''}`)
export const getServiceInstances = (name: string, namespace = '') =>
  api.get<Instance[]>(`/v1/catalog/service/${encodeURIComponent(name)}${namespace ? `?namespace=${encodeURIComponent(namespace)}` : ''}`)
export const setInstanceStatus = (namespace: string, serviceName: string, instanceId: string, status: string) =>
  api.post('/v1/catalog/instance/status', { namespace, service_name: serviceName, instance_id: instanceId, status })
export const getSubscribers = (name: string, namespace = '') =>
  api.get<string[]>(`/v1/catalog/service/${encodeURIComponent(name)}/subscribers${namespace ? `?namespace=${encodeURIComponent(namespace)}` : ''}`)
export const getDependencyGraph = (namespace: string) => api.get<any>(`/v1/catalog/dependency-graph?namespace=${namespace}`)
export const getTopology = (namespace = '') =>
  api.get<TopologyGraph>(`/v1/catalog/topology${namespace ? `?namespace=${encodeURIComponent(namespace)}` : ''}`)

export const getNamespaces = () => api.get<Namespace[]>('/v1/namespaces')
export const createNamespace = (data: Partial<Namespace>) => api.post('/v1/namespace', data)
export const updateNamespace = (data: Partial<Namespace>) => api.put('/v1/namespace', data)
export const deleteNamespace = (name: string) => api.delete(`/v1/namespace?name=${name}`)

export const getRbacUsers = () => api.get<RbacUser[]>('/v1/rbac/users')
export const saveRbacUser = (data: Partial<RbacUser>) => api.post('/v1/rbac/user', data)
export const deleteRbacUser = (username: string) =>
  api.delete(`/v1/rbac/user/delete?username=${encodeURIComponent(username)}`)

export const getClusterMembers = () => api.get<ClusterMember[]>('/v1/cluster/members')
export const getClusterStats = () => api.get<ClusterStats>('/v1/cluster/stats')
export const addClusterMember = (data: { addresses: string[] }) => api.post('/v1/cluster/member', data)
export const removeClusterMember = (address: string, node_id?: string) => 
  api.delete(`/v1/cluster/member?address=${address}${node_id ? '&node_id=' + node_id : ''}`)
export const getEvents = (count = 100, offset = 0, query = '', date = '', type = '', service = '', startTime = '', endTime = '') =>
  api.get<{total: number, data: RegistryEvent[]}>(`/v1/events?count=${count}&offset=${offset}${query ? `&query=${encodeURIComponent(query)}` : ''}${date ? `&date=${date}` : ''}${type ? `&type=${type}` : ''}${service ? `&service=${service}` : ''}${startTime ? `&start_time=${startTime}` : ''}${endTime ? `&end_time=${endTime}` : ''}`)
export const getLogFiles = () => api.get<{name: string, file: string}[]>('/v1/cluster/log-files')
export const getLogs = (file = '', count = 100, offset = 0, query = '') =>
  api.get<{total: number, data: string[]}>(`/v1/cluster/logs?count=${count}&offset=${offset}${file ? `&file=${encodeURIComponent(file)}` : ''}${query ? `&query=${encodeURIComponent(query)}` : ''}`)
export const getStorage = () => api.get<any>('/v1/settings/storage')
export const updateStorage = (data: any) => api.post('/v1/settings/storage', data)
export const getSystemSettings = () => api.get<SystemSettings>('/v1/settings/system')
export const updateSystemSettings = (data: SystemSettings) => api.post<ApplySystemSettingsResult>('/v1/settings/system', data)
export const getNotificationConfig = (namespace = 'default') =>
  api.get<NotificationConfig>(`/v1/notify/config?namespace=${encodeURIComponent(namespace)}`)
export const updateNotificationConfig = (namespace: string, data: NotificationConfig) =>
  api.post<NotificationConfig>(`/v1/notify/config?namespace=${encodeURIComponent(namespace)}`, data)
export const getAlertConfig = (namespace = 'default') =>
  api.get<AlertConfig>(`/v1/alert/config?namespace=${encodeURIComponent(namespace)}`)
export const updateAlertConfig = (namespace: string, data: AlertConfig) =>
  api.post<AlertConfig>(`/v1/alert/config?namespace=${encodeURIComponent(namespace)}`, data)

export const getGuideStatus = () => api.get<{guide_completed: boolean}>('/v1/auth/guide-status')
export const updateGuideStatus = (completed: boolean) => api.post('/v1/auth/guide-status', { completed })
export const testNotification = (data: { rule: Partial<AlertRule> }) => api.post('/v1/notify/test', data)
export const testChannel = (data: { channel: Partial<NotificationChannel> }) => api.post('/v1/notify/channel/test', data)


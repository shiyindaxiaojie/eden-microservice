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

export const getClusterMembers = () => api.get<ClusterMember[]>('/v1/cluster/members')
export const getClusterStats = () => api.get<ClusterStats>('/v1/cluster/stats')
export const addClusterMember = (data: { addresses: string[] }) => api.post('/v1/cluster/member', data)
export const removeClusterMember = (address: string, node_id?: string) => 
  api.delete(`/v1/cluster/member?address=${address}${node_id ? '&node_id=' + node_id : ''}`)
export const getEvents = () => api.get<RegistryEvent[]>('/v1/events')

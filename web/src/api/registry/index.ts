import api from '../index'

export interface ServiceSummary {
  name: string
  instance_count: number
  healthy_count: number
}

export interface Instance {
  id: string
  service_name: string
  host: string
  port: number
  weight: number
  metadata: Record<string, string>
  status: 'passing' | 'critical'
  last_heartbeat: string
  registered_at: string
}

export interface ClusterMember {
  id: string
  address: string
  role: string
  status: 'Online' | 'Offline'
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
}

export interface RegistryEvent {
  id: number
  type: string
  service: string
  instance: string
  message: string
  timestamp: string
}

export const getServices = () => api.get<ServiceSummary[]>('/v1/catalog/services')
export const getServiceInstances = (name: string) => api.get<Instance[]>(`/v1/catalog/service/${name}`)
export const deregisterInstance = (serviceName: string, instanceId: string) =>
  api.post('/v1/catalog/deregister', { service_name: serviceName, instance_id: instanceId })

export const getClusterMembers = () => api.get<ClusterMember[]>('/v1/cluster/members')
export const getClusterStats = () => api.get<ClusterStats>('/v1/cluster/stats')
export const addClusterMember = (data: { node_id?: string, address: string }) => api.post('/v1/cluster/member', data)
export const removeClusterMember = (address: string, node_id?: string) => 
  api.delete(`/v1/cluster/member?address=${address}${node_id ? '&node_id=' + node_id : ''}`)
export const getEvents = () => api.get<RegistryEvent[]>('/v1/events')

import api from '../index'

export type ConfigContentType = 'text' | 'yaml' | 'json' | 'properties'
export type ConfigHistoryAction = 'publish' | 'delete' | 'rollback'

export interface ConfigIdentity {
  namespace: string
  group: string
  data_id: string
}

export interface ConfigResource extends ConfigIdentity {
  content: string
  type: ConfigContentType
  md5: string
  revision: number
  description: string
  tags: string[]
  created_at: string
  updated_at: string
  created_by: string
  updated_by: string
}

export interface ConfigHistoryEntry extends ConfigIdentity {
  content: string
  type: ConfigContentType
  md5: string
  revision: number
  action: ConfigHistoryAction
  operator: string
  summary: string
  created_at: string
}

export interface ConfigListQuery {
  namespace?: string
  group?: string
  type?: ConfigContentType
  query?: string
  page?: number
  page_size?: number
}

export interface ConfigListResult {
  total: number
  data: ConfigResource[]
}

export interface PublishConfigRequest extends ConfigIdentity {
  content: string
  type: ConfigContentType
  description?: string
  tags?: string[]
  expected_md5?: string
}

export const listConfigs = (params: ConfigListQuery = {}) =>
  api.get<ConfigListResult>('/v1/configs', { params }).then((response) => response.data)

export const getConfig = (identity: ConfigIdentity) =>
  api.get<ConfigResource>('/v1/config', { params: identity }).then((response) => response.data)

export const listConfigHistory = (identity: ConfigIdentity) =>
  api.get<ConfigHistoryEntry[]>('/v1/config/history', { params: identity }).then((response) => response.data)

export const publishConfig = (payload: PublishConfigRequest) =>
  api.post<ConfigResource>('/v1/config', payload).then((response) => response.data)

export const deleteConfig = (identity: ConfigIdentity) =>
  api.delete<ConfigHistoryEntry>('/v1/config', { params: identity }).then((response) => response.data)

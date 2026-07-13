import api from '../index'

export type GatewayTargetType = 'service' | 'static'
export type GatewayLoadBalance = 'round_robin' | 'random' | 'weighted'
export type GatewayTrafficMode = 'weighted' | 'canary' | 'blue_green'
export type GatewayFilterType = 'strip_prefix' | 'add_request_header' | 'set_response_header'

export interface GatewayIdentity {
  namespace: string
  id: string
}

export interface GatewayMatch {
  hosts?: string[]
  path_prefix?: string
  path?: string
  methods?: string[]
  headers?: Record<string, string>
}

export interface GatewayServiceTarget {
  namespace: string
  group: string
  service_name: string
}

export interface GatewayStaticEndpoint {
  url: string
  weight: number
}

export interface GatewayStaticTarget {
  endpoints: GatewayStaticEndpoint[]
}

export interface GatewayTarget {
  id: string
  name?: string
  labels?: Record<string, string>
  type: GatewayTargetType
  service?: GatewayServiceTarget
  static?: GatewayStaticTarget
  load_balance: GatewayLoadBalance
  healthy_only: boolean
}

export interface GatewayWeightedTarget {
  target_id: string
  weight: number
}

export interface GatewayBetaTarget {
  target_id: string
  users?: string[]
  tenants?: string[]
}

export interface GatewayTrafficPolicy {
  mode: GatewayTrafficMode
  default_target_id?: string
  weighted_targets?: GatewayWeightedTarget[]
  beta_targets?: GatewayBetaTarget[]
  active_target_id?: string
}

export interface GatewayFilter {
  type: GatewayFilterType
  name?: string
  value?: string
  parts?: number
}

export interface GatewayRoute extends GatewayIdentity {
  name: string
  enabled: boolean
  priority: number
  match: GatewayMatch
  targets: GatewayTarget[]
  traffic: GatewayTrafficPolicy
  filters?: GatewayFilter[]
  timeout_ms: number
  revision: number
  created_at: string
  updated_at: string
  created_by: string
  updated_by: string
}

export interface GatewayListResult {
  total: number
  data: GatewayRoute[]
}

export interface GatewayListQuery {
  namespace?: string
  query?: string
  enabled?: boolean
  page?: number
  page_size?: number
}

export interface GatewayHistoryEntry extends GatewayIdentity {
  route?: GatewayRoute
  revision: number
  action: 'create' | 'update' | 'delete' | 'enable' | 'disable'
  operator: string
  summary: string
  created_at: string
}

export interface GatewayRuntimeStatus extends GatewayIdentity {
  enabled: boolean
  data_plane_enabled: boolean
  requests: number
  errors: number
  last_status: number
  last_error?: string
  last_request_at?: string
  snapshot_revision: number
}

export type GatewayRouteInput = Omit<GatewayRoute, 'revision' | 'created_at' | 'updated_at' | 'created_by' | 'updated_by'> & Partial<Pick<GatewayRoute, 'revision'>>

export const listGatewayRoutes = (params: GatewayListQuery = {}) =>
  api.get<GatewayListResult>('/v1/gateway/routes', { params }).then((response) => response.data)

export const getGatewayRoute = (identity: GatewayIdentity) =>
  api.get<GatewayRoute>('/v1/gateway/route', { params: identity }).then((response) => response.data)

export const createGatewayRoute = (route: GatewayRouteInput) =>
  api.post<GatewayRoute>('/v1/gateway/route', route).then((response) => response.data)

export const updateGatewayRoute = (route: GatewayRouteInput & { revision: number }) =>
  api.put<GatewayRoute>('/v1/gateway/route', { ...route, expected_revision: route.revision }).then((response) => response.data)

export const saveGatewayRoute = (route: GatewayRouteInput | GatewayRoute) => {
  const revision = route.revision
  if (typeof revision === 'number' && revision > 0) {
    return updateGatewayRoute({ ...route, revision })
  }
  return createGatewayRoute(route as GatewayRouteInput)
}

export const deleteGatewayRoute = (route: Pick<GatewayRoute, 'namespace' | 'id' | 'revision'>) =>
  api.delete<GatewayHistoryEntry>('/v1/gateway/route', {
    params: { namespace: route.namespace, id: route.id, expected_revision: route.revision },
  }).then((response) => response.data)

export const setGatewayRouteEnabled = (route: Pick<GatewayRoute, 'namespace' | 'id' | 'revision'>, enabled: boolean) =>
  api.post<GatewayRoute>('/v1/gateway/route/status', {
    namespace: route.namespace,
    id: route.id,
    enabled,
    expected_revision: route.revision,
  }).then((response) => response.data)

export const listGatewayRouteHistory = (identity: GatewayIdentity) =>
  api.get<GatewayHistoryEntry[]>('/v1/gateway/history', { params: identity }).then((response) => response.data)

export const listGatewayRuntime = (namespace?: string) =>
  api.get<GatewayRuntimeStatus[]>('/v1/gateway/runtime', { params: namespace ? { namespace } : {} }).then((response) => response.data)

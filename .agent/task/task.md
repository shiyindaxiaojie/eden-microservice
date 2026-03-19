# Eden Go Registry - Feature Enhancement Plan

## 1. Namespace Isolation
- [x] Backend: Create `internal/store/namespace_store.go` with CRUD + persistence to `namespace.json`
- [x] Backend: Add `Namespace` field to `model.Instance` (optional, default empty = "default")
- [x] Backend: Update `ServiceStore` to support namespace-scoped operations
- [x] Backend: Create `namespace_handler.go` with REST API endpoints
- [x] Backend: Wire up routes in `handler.go`
- [x] Proto: Add `namespace` field to `ServiceInstance`, `DiscoverRequest`, `WatchRequest`
- [x] Frontend: Create `namespace.vue` page with CRUD UI
- [x] Frontend: Add namespace route and menu entry
- [x] Frontend: Add namespace API module
- [x] SDK: Ensure `pkg/eden/client.go` works unchanged (namespace is optional)

## 2. Service List UI Optimization
- [x] Backend: Enhance `ListServices` to return subscriber count per service
- [x] Backend: Add endpoint to query subscribers by service name
- [x] Backend: Add instance offline/online endpoints (replace deregister)
- [x] Backend: Implement gRPC notifications for offline/online commands to clients
- [x] Frontend: Redesign `services.vue` with search by name/IP/health, grid+list toggle
- [x] Frontend: Redesign `service-detail.vue` with offline/online buttons + subscriber count
- [x] Frontend: Add subscriber list dialog

## 3. Service Dependency Graph
- [x] Backend: Track service dependencies (provider/consumer) from subscribe data
- [x] Backend: Add `/v1/catalog/dependency-graph` endpoint
- [x] Frontend: Create `dependency-graph.vue` page with network visualization (e.g., ECharts/D3)
- [x] Frontend: Add visualization route and menu entry

## 4. Local Cache Mechanism (Client SDK)
- [x] SDK: Add local file cache for service discovery results
- [x] SDK: Load cached data on startup before connecting to registry
- [x] SDK: Fallback to cache when registry is unreachable
- [x] SDK: Periodic cache refresh in background
- [x] SDK: Ensure backward compatibility - cache is transparent/optional

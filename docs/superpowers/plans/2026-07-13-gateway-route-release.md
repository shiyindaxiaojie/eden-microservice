# Gateway Route Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a persistent HTTP gateway that binds routes to registry services or static endpoints and supports weighted, canary, BETA, and blue-green traffic releases.

**Architecture:** `internal/gateway` owns the route resource, Bolt persistence, validation, immutable runtime snapshots, matching, target selection, and reverse proxying. The HTTP transport exposes management APIs and `cmd/server` starts a dedicated data-plane listener. The Vue route page uses those APIs and lets an administrator edit multiple targets and release policy without exposing catalog compatibility keys.

**Tech Stack:** Go 1.25, net/http/httputil, bbolt, Vue 3, TypeScript, Element Plus, Vite.

## Global Constraints

- Preserve the user's in-progress config-center changes; do not move, reset, or rewrite them.
- Native management APIs live under `/v1/gateway/*`; the data plane uses a separate `gateway.http` listener.
- Route identity is `namespace + id`; default namespace is `default`.
- BETA user and tenant headers are trusted only from `gateway.trusted_proxy_cidrs`; default trust is loopback only.
- Publication target weight and service-instance weight are separate layers.
- The route runtime has no implicit fallback between release targets.
- Route writes use `expected_revision`; stale writes return HTTP `409`.
- Route modifications are admin-only; admin/developer/viewer are read-only according to existing role support.

---

## File Structure

- Create: `internal/gateway/model.go` — public route, target, traffic-policy, history, and runtime-status types.
- Create: `internal/gateway/service.go` — store and runtime interfaces, typed errors, and subscription contract.
- Create: `internal/gateway/validate.go` — identity, route, target, static URL, and policy validation.
- Create: `internal/gateway/store.go` — Bolt-backed CRUD, revision/history persistence, and change notifications.
- Create: `internal/gateway/runtime.go` — snapshot reload, matching, BETA/weight/blue-green selection, proxying, and metrics.
- Create: `internal/gateway/model_test.go`, `store_test.go`, `runtime_test.go` — behavior-first package tests.
- Modify: `internal/config/config.go` — typed gateway listener and trusted-proxy configuration plus defaults.
- Modify: `config/config.yaml.example` — disabled-by-default gateway example and identity-proxy configuration.
- Modify: `cmd/server/main.go` — open/close gateway store, construct runtime, start the dedicated listener.
- Create: `cmd/server/gateway_config_test.go` — gateway configuration default and enabled-listener helpers without touching the user's in-progress server tests.
- Modify: `internal/transport/http/server.go` — inject the gateway service/runtime and register protected management routes.
- Create: `internal/transport/http/gateway.go` — API request decoding, actor extraction, and JSON responses.
- Create: `internal/transport/http/gateway_handler_test.go` — management API method, validation, and revision-conflict tests.
- Create: `web/src/api/gateway/index.ts` — typed gateway management client.
- Modify: `web/src/views/routes.vue` — real route list/editor, catalog service selector, multi-target editor, and release-policy editor.
- Modify: `web/src/utils/i18n.ts` — rename the menu and screen title to `网关路由` / `Gateway Routes`.
- Modify: `specs/zh-CN/gateway/*`, `specs/zh-CN/http-api/api-spec.md` — authoritative behavior contract already introduced before implementation.

## Task 1: Define the Gateway Resource and Validate It

**Files:**
- Create: `internal/gateway/model.go`
- Create: `internal/gateway/service.go`
- Create: `internal/gateway/validate.go`
- Test: `internal/gateway/model_test.go`

**Interfaces:**
- Produces `gateway.Route`, `gateway.Target`, `gateway.TrafficPolicy`, `gateway.ValidateRoute(Route) error`, and `gateway.NormalizeIdentity(Identity) (Identity, error)`.
- Later tasks consume `Route.Identity`, `Route.Targets`, `Route.Traffic`, and errors `ErrInvalidRoute`, `ErrConflict`, and `ErrNotFound`.

- [ ] **Step 1: Write failing validation tests**

```go
func TestValidateRouteAcceptsCanaryWithBetaTarget(t *testing.T) {
    route := validRoute()
    route.Traffic = TrafficPolicy{
        Mode: TrafficCanary, DefaultTargetID: "stable",
        WeightedTargets: []WeightedTarget{{TargetID: "stable", Weight: 95}, {TargetID: "canary", Weight: 5}},
        BetaTargets: []BetaTarget{{TargetID: "canary", Users: []string{"u-1"}}},
    }
    if err := ValidateRoute(route); err != nil { t.Fatal(err) }
}

func TestValidateRouteRejectsOverlappingBetaUsers(t *testing.T) {
    route := validRoute()
    route.Traffic.BetaTargets = []BetaTarget{{TargetID: "stable", Users: []string{"u-1"}}, {TargetID: "canary", Users: []string{"u-1"}}}
    if err := ValidateRoute(route); !errors.Is(err, ErrInvalidRoute) { t.Fatalf("error = %v", err) }
}
```

- [ ] **Step 2: Run the package test to verify RED**

Run: `go test ./internal/gateway -run 'TestValidateRoute'`

Expected: FAIL because package and public gateway types do not exist.

- [ ] **Step 3: Implement model normalization and validation**

Implement the types shown by the design, with `TargetTypeService`, `TargetTypeStatic`, `TrafficWeighted`, `TrafficCanary`, and `TrafficBlueGreen`. Validate one exact/path-prefix matcher, target IDs, service identity, safe static URLs, matching target references, 100-point weighted sums, and blue-green active target.

- [ ] **Step 4: Run focused tests to verify GREEN**

Run: `go test ./internal/gateway -run 'TestValidateRoute'`

Expected: PASS.

- [ ] **Step 5: Add focused edge-case tests and rerun**

Cover invalid route IDs, missing targets, duplicate target IDs, insecure static URL userinfo, invalid weight sum, and inactive blue-green target.

Run: `go test ./internal/gateway`

Expected: PASS.

## Task 2: Persist Routes, Revisions, History, and Snapshot Notifications

**Files:**
- Create: `internal/gateway/store.go`
- Test: `internal/gateway/store_test.go`

**Interfaces:**
- Consumes `Route`, `Identity`, and validation from Task 1.
- Produces `gateway.Store` implementing:

```go
type Service interface {
    Get(Identity) (*Route, error)
    List(ListQuery) (ListResult, error)
    Create(CreateRequest) (*Route, error)
    Update(UpdateRequest) (*Route, error)
    Delete(Identity, uint64, string) (*HistoryEntry, error)
    SetEnabled(Identity, bool, uint64, string) (*Route, error)
    History(Identity) ([]HistoryEntry, error)
    Subscribe(func()) func()
    Close() error
}
```

- [ ] **Step 1: Write failing persistence tests**

```go
func TestStoreUpdatesRouteAndRejectsStaleRevision(t *testing.T) {
    service := openTestStore(t)
    created, err := service.Create(CreateRequest{Route: validRoute(), Operator: "admin"})
    if err != nil { t.Fatal(err) }
    updated, err := service.Update(UpdateRequest{Route: *created, ExpectedRevision: created.Revision, Operator: "admin"})
    if err != nil { t.Fatal(err) }
    _, err = service.Update(UpdateRequest{Route: *updated, ExpectedRevision: created.Revision, Operator: "admin"})
    if !errors.Is(err, ErrConflict) { t.Fatalf("error = %v", err) }
}
```

- [ ] **Step 2: Run the focused test to verify RED**

Run: `go test ./internal/gateway -run TestStoreUpdatesRouteAndRejectsStaleRevision`

Expected: FAIL because `Open` and `Create` do not exist.

- [ ] **Step 3: Implement the Bolt store**

Store routes in `dataDir/gateway/routes.db`, use a monotonic revision key, encode routes/history as JSON, normalize before persistence, and call subscribers after a committed mutation. Retain prior resource values as history for update/delete and append action history for enable/disable.

- [ ] **Step 4: Run persistence tests to verify GREEN**

Run: `go test ./internal/gateway -run 'TestStore|TestHistory|TestList'`

Expected: PASS.

- [ ] **Step 5: Add reload notification and reopen persistence tests**

Assert that a subscription fires after Create/Update/Delete and that a reopened store returns the current route and history.

Run: `go test ./internal/gateway`

Expected: PASS.

## Task 3: Build the Data-Plane Snapshot, Matching, Traffic Selection, and Proxy

**Files:**
- Create: `internal/gateway/runtime.go`
- Test: `internal/gateway/runtime_test.go`

**Interfaces:**
- Consumes Task 2 `Service` and a narrow discovery dependency:

```go
type Discovery interface {
    GetService(namespace, name string, healthyOnly bool) ([]*catalog.Instance, error)
}
type Runtime struct { /* immutable route snapshot and counters */ }
func NewRuntime(Service, Discovery, RuntimeConfig) (*Runtime, error)
func (r *Runtime) ServeHTTP(http.ResponseWriter, *http.Request)
func (r *Runtime) Status(namespace string) []RuntimeStatus
func (r *Runtime) Close()
```

- [ ] **Step 1: Write failing target-selection tests**

```go
func TestRuntimeBetaUserOverridesCanaryWeight(t *testing.T) {
    runtime := newRuntime(t, canaryRoute(), trustedLoopbackConfig())
    req := httptest.NewRequest(http.MethodGet, "http://gateway/orders", nil)
    req.RemoteAddr = "127.0.0.1:3000"
    req.Header.Set("X-Eden-User-ID", "u-beta")
    if got := runtime.selectTarget(req, route); got.ID != "canary" { t.Fatalf("target = %q", got.ID) }
}

func TestRuntimeBlueGreenUsesActiveTargetForNormalTraffic(t *testing.T) {
    runtime := newRuntime(t, blueGreenRoute(), trustedLoopbackConfig())
    if got := runtime.selectTarget(httptest.NewRequest(http.MethodGet, "http://gateway/orders", nil), route); got.ID != "blue" { t.Fatalf("target = %q", got.ID) }
}
```

- [ ] **Step 2: Run selection tests to verify RED**

Run: `go test ./internal/gateway -run 'TestRuntime(Beta|BlueGreen)'`

Expected: FAIL because `Runtime` does not exist.

- [ ] **Step 3: Implement immutable snapshot and matching**

Reload all enabled routes after each store notification, sort by priority, longer path, then creation time. Match hosts, exact/path-prefix, methods, and exact headers. Preserve one snapshot pointer for the lifetime of each request.

- [ ] **Step 4: Implement release and endpoint selection**

Read BETA headers only for trusted source IPs. Select user BETA first, tenant BETA second, then deterministic 0–99 weighted bucket or active blue-green target. Resolve service targets using `catalog.QualifiedServiceName`, and implement round-robin, random, and integer-weighted endpoint selection.

- [ ] **Step 5: Run focused selection tests to verify GREEN**

Run: `go test ./internal/gateway -run 'TestRuntime(Beta|BlueGreen|Weighted)'`

Expected: PASS.

- [ ] **Step 6: Write and run failing proxy behavior tests**

Add `httptest.NewServer` static targets and assert strip-prefix plus request/response filters. Add no-match, no-instance, timeout, and dial-error cases expecting `404`, `503`, `504`, and `502` respectively.

Run: `go test ./internal/gateway -run 'TestRuntime(Proxy|NoMatch|NoInstance|Timeout|BadGateway)'`

Expected before proxy implementation: FAIL for missing proxy behavior; after implementation: PASS.

## Task 4: Wire Configuration, Server Lifecycle, and Management HTTP APIs

**Files:**
- Modify: `internal/config/config.go`
- Modify: `config/config.yaml.example`
- Modify: `cmd/server/main.go`
- Modify: `internal/transport/http/server.go`
- Create: `internal/transport/http/gateway.go`
- Test: `internal/transport/http/gateway_handler_test.go`
- Test: `cmd/server/gateway_config_test.go`

**Interfaces:**
- Consumes `gateway.Service` and `gateway.Runtime` from Tasks 2–3.
- Produces management endpoints:

```text
GET    /v1/gateway/routes
GET    /v1/gateway/route?namespace=&id=
POST   /v1/gateway/route
PUT    /v1/gateway/route
DELETE /v1/gateway/route?namespace=&id=&expected_revision=
POST   /v1/gateway/route/status
GET    /v1/gateway/history?namespace=&id=
GET    /v1/gateway/runtime?namespace=
```

- [ ] **Step 1: Write failing configuration and handler tests**

```go
func TestGatewayRouteUpdateReturnsConflictForStaleRevision(t *testing.T) {
    h := newGatewayTestHandler(t)
    created := createGatewayRoute(t, h)
    response := performJSON(h, http.MethodPut, "/v1/gateway/route", map[string]any{
        "route": created, "expected_revision": created.Revision - 1,
    })
    if response.Code != http.StatusConflict { t.Fatalf("status = %d", response.Code) }
}
```

- [ ] **Step 2: Run HTTP tests to verify RED**

Run: `go test ./internal/transport/http -run TestGateway`

Expected: FAIL because gateway handlers are not registered.

- [ ] **Step 3: Add typed gateway configuration and lifecycle wiring**

Add `GatewayConfig{Enabled, HTTP, TrustedProxyCIDRs}` to process config, default `enabled=false`, `http=":8080"`, and loopback CIDRs. Open/close the gateway store with the process, construct a runtime from the catalog registry, and only start the independent data-plane listener when enabled.

- [ ] **Step 4: Implement protected native API handlers**

Decode JSON into gateway request types; use current authenticated username as operator; return `400` for validation, `404` for missing route, and `409` for revision conflicts. Apply `Auth(RBAC("admin", "developer", "viewer"))` for reads and `Auth(RBAC("admin"))` for writes.

- [ ] **Step 5: Run handler and bootstrap tests to verify GREEN**

Run: `go test ./internal/transport/http -run TestGateway; go test ./cmd/server -run Test.*Gateway`

Expected: PASS.

- [ ] **Step 6: Run affected Go packages**

Run: `go test ./internal/gateway ./internal/transport/http ./cmd/server`

Expected: PASS.

## Task 5: Replace the Mock Console With Gateway Route Management

**Files:**
- Create: `web/src/api/gateway/index.ts`
- Modify: `web/src/views/routes.vue`
- Modify: `web/src/utils/i18n.ts`

**Interfaces:**
- Consumes the HTTP contract from Task 4 and `ServiceSummary` from `web/src/api/registry/index.ts`.
- Produces a route form with typed `GatewayRoute`, `GatewayTarget`, and `TrafficPolicy` payloads; no route request may import `api/mock/control-plane`.

- [ ] **Step 1: Add a failing TypeScript usage surface**

Add the typed API client first and change `routes.vue` imports from mock functions to the expected `listGatewayRoutes`, `createGatewayRoute`, `updateGatewayRoute`, `deleteGatewayRoute`, `setGatewayRouteEnabled`, `listGatewayRouteHistory`, and `listGatewayRuntime` functions.

- [ ] **Step 2: Run the web type/build check to verify RED**

Run: `npm run build`

Expected: FAIL while the API module and form bindings are incomplete.

- [ ] **Step 3: Implement the typed client and list/status refresh**

Use `/v1/gateway/*` exclusively. Render saved state and runtime status separately; show namespace, path, targets, publication mode, revision, and updated time.

- [ ] **Step 4: Implement the multi-target and release-policy editor**

Support add/remove targets, a catalog-service dropdown carrying namespace/group/name, static endpoint rows, target-internal load balancing, weighted/canary percentages, blue-green active target, and BETA user/tenant comma-separated lists. Validate target IDs and 100-point weights client-side before submit, while treating the server as authoritative.

- [ ] **Step 5: Rename navigation copy**

Change Chinese `routes` navigation text to `网关路由`, English to `Gateway Routes`, and Japanese to `ゲートウェイルート`; retain `/routes` as the URL for compatibility.

- [ ] **Step 6: Run front-end checks to verify GREEN**

Run: `npm run check:i18n; npm run build`

Expected: both commands exit `0`.

## Task 6: Final Contract Review and Verification

**Files:**
- Modify only if verification exposes a concrete defect in the files above.

- [ ] **Step 1: Compare implementation to the gateway specs**

Check each required behavior: real persistence, service/static targets, route-to-target weighting, canary, trusted BETA matching, blue-green active switch, snapshot reload, errors, API RBAC, and console label/editor.

- [ ] **Step 2: Run full Go verification**

Run: `go test ./...`

Expected: exit `0` with all packages passing.

- [ ] **Step 3: Run full web verification**

Run: `npm run check:i18n; npm run build`

Expected: both commands exit `0`.

- [ ] **Step 4: Inspect the scoped diff**

Run: `git diff --check; git status --short; git diff -- specs/zh-CN/gateway internal/gateway internal/transport/http cmd/server internal/config config web/src`

Expected: no whitespace errors; all unrelated user changes remain untouched.

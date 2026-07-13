# Nacos Config Center Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a bbolt-backed Config center whose native API, Nacos Config API, console, and runnable Nacos client example share one durable source of truth.

**Architecture:** `internal/configcenter` owns identity normalization, durable state, revisions, history, CAS, and change waiters. HTTP and Nacos adapters translate their protocols into that service; the Vue view calls only the native management API. Server startup owns the lifecycle of the bbolt store.

**Tech Stack:** Go 1.25, bbolt, net/http, Nacos Go SDK v2, Vue 3, TypeScript, Axios, Vite.

## Global Constraints

- Persist all Config resources in `${data_dir}/config/configs.db`; do not add a memory-only store or fallback.
- Keep resource identity exactly `namespace -> group -> data_id`; normalize blank values to `default` and `DEFAULT_GROUP`.
- Treat configuration content as opaque; only calculate MD5 from the complete content.
- Keep Config replication, Apollo compatibility, import/export, and rollback out of this increment.
- Preserve uncommitted UI layout and style changes outside the Config API integration.
- Native management writes require existing admin/developer authorization; Nacos mutation routes require API-key authorization.

---

### Task 1: Create the durable Config domain with tests

**Files:**
- Create: `internal/configcenter/model.go`
- Create: `internal/configcenter/store.go`
- Create: `internal/configcenter/service.go`
- Create: `internal/configcenter/service_test.go`

**Interfaces:**
- Produces `type Service interface { Get(Identity) (*Resource, error); List(ListQuery) (ListResult, error); Publish(PublishRequest) (*Resource, error); Delete(Identity, string) (*HistoryEntry, error); History(Identity) ([]HistoryEntry, error); Wait([]WatchTarget, time.Duration) ([]Change, error); Close() error }`.
- Produces `func Open(dataDir string) (Service, error)` and sentinel errors `ErrNotFound`, `ErrConflict`, `ErrInvalidIdentity`.
- Consumes only the standard library and `go.etcd.io/bbolt`.

- [ ] **Step 1: Write a failing persistence-and-revision test**

```go
func TestPublishPersistsAndRecordsOnlyContentChanges(t *testing.T) {
    dir := t.TempDir()
    configs := openTestService(t, dir)
    first, err := configs.Publish(PublishRequest{Identity: Identity{DataID: "order.yaml"}, Content: "port: 8080", Type: "yaml", Operator: "admin"})
    if err != nil { t.Fatal(err) }
    again, err := configs.Publish(PublishRequest{Identity: first.Identity, Content: first.Content, Type: "yaml", Description: "display change", Operator: "admin"})
    if err != nil { t.Fatal(err) }
    if again.Revision != first.Revision { t.Fatalf("revision = %d, want %d", again.Revision, first.Revision) }
    if err := configs.Close(); err != nil { t.Fatal(err) }
    reopened := openTestService(t, dir)
    got, err := reopened.Get(first.Identity)
    if err != nil || got.MD5 != first.MD5 { t.Fatalf("Get() = %#v, %v", got, err) }
}
```

- [ ] **Step 2: Run the test and verify the domain is absent**

Run: `go test ./internal/configcenter -run TestPublishPersistsAndRecordsOnlyContentChanges -count=1`

Expected: FAIL because `internal/configcenter` does not exist.

- [ ] **Step 3: Implement resource identity, bbolt buckets, and `Publish`**

```go
type Identity struct { Namespace string `json:"namespace"`; Group string `json:"group"`; DataID string `json:"data_id"` }
type Resource struct { Identity; Content, Type, MD5, Description string; Tags []string; Revision uint64; CreatedAt, UpdatedAt time.Time; CreatedBy, UpdatedBy string }

func Open(dataDir string) (Service, error) {
    path := filepath.Join(dataDir, "config", "configs.db")
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { return nil, err }
    db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second})
    if err != nil { return nil, err }
    return newStore(db), db.Update(createBuckets)
}
```

Implement `Publish` as one write transaction: normalize the identity, reject an invalid data ID, load the active resource, enforce `ExpectedMD5`, MD5 the content, and update the `configs` record plus a global sequence key. Append the previous state to `config_history` only if content changed. Do not retain a process-local copy of Config resources.

- [ ] **Step 4: Add failing CAS, deletion, history, and watcher tests**

```go
func TestPublishRejectsStaleMD5(t *testing.T) { /* publish "v1", publish "v2", then expect ErrConflict for expected "v1" */ }
func TestDeleteHidesCurrentResourceAndKeepsHistory(t *testing.T) { /* delete then Get returns ErrNotFound and History includes delete */ }
func TestWaitReturnsChangedIdentity(t *testing.T) { /* wait with prior MD5, publish changed content, assert one Change */ }
```

- [ ] **Step 5: Implement the remaining `Service` methods and run the package tests**

```go
func (s *store) Wait(targets []WatchTarget, timeout time.Duration) ([]Change, error) {
    // return stale targets before registration; otherwise register a bounded buffered waiter,
    // remove it with defer, and return its changed identities or an empty slice on timeout.
}
```

Run: `go test ./internal/configcenter -count=1`

Expected: PASS with persistence, CAS, history, delete, list, and waiter tests green.

### Task 2: Wire the durable store into server lifecycle and native HTTP API

**Files:**
- Create: `internal/transport/http/config.go`
- Create: `internal/transport/http/config_handler_test.go`
- Modify: `internal/transport/http/server.go`
- Modify: `cmd/server/main.go`

**Interfaces:**
- Consumes `configcenter.Service` from Task 1.
- Produces native endpoints `/v1/config`, `/v1/configs`, `/v1/config/history`, and `/v1/config/listener`.
- Changes `NewHandler` to accept `configcenter.Service` after `catalogRegistry`.

- [ ] **Step 1: Write a failing native API test**

```go
func TestConfigPublishListAndHistory(t *testing.T) {
    h, closeStore := newConfigTestHandler(t)
    defer closeStore()
    publish := httptest.NewRequest(http.MethodPost, "/v1/config", strings.NewReader(`{"data_id":"demo.properties","content":"feature=true","type":"properties"}`))
    publish.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder(); h.config(publishRecorder, publish)
    // then GET /v1/configs and GET /v1/config/history; assert md5, revision, and one history entry.
}
```

- [ ] **Step 2: Run the handler test and verify it fails because the route is missing**

Run: `go test ./internal/transport/http -run TestConfigPublishListAndHistory -count=1`

Expected: FAIL because no Config handler or helper exists.

- [ ] **Step 3: Add request DTOs and exact method dispatch**

```go
func (h *Handler) config(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet: h.getConfig(w, r)
    case http.MethodPost: h.publishConfig(w, r)
    case http.MethodPut: h.publishConfig(w, r)
    case http.MethodDelete: h.deleteConfig(w, r)
    default: httpError(w, http.StatusMethodNotAllowed, "GET, POST, PUT or DELETE required")
    }
}
```

Decode JSON for writes; include `expected_md5`, `description`, `tags`, and `type`; call the Config service with the authenticated user name when available. Encode `ErrNotFound` as 404, `ErrConflict` as 409, input errors as 400, and persistence errors as 500. Keep list/history JSON as direct domain values so the console uses the same fields as the specification.

- [ ] **Step 4: Register routes and own store shutdown**

```go
h.mux.Handle("/v1/config", h.Auth(adminOrDev(http.HandlerFunc(h.config))))
h.mux.Handle("/v1/configs", h.Auth(adminOrDev(http.HandlerFunc(h.listConfigs))))
h.mux.Handle("/v1/config/history", h.Auth(adminOrDev(http.HandlerFunc(h.configHistory))))
h.mux.Handle("/v1/config/listener", h.APIKey(http.HandlerFunc(h.configListener)))
```

Open the service in `cmd/server/main.go` immediately after process configuration is loaded, pass it to `NewHandler`, fail startup when it cannot open the configured data directory, and close it after the shutdown signal. Do not place Config state in `cluster.RuntimeState` in this increment.

- [ ] **Step 5: Run focused API tests**

Run: `go test ./internal/transport/http -run TestConfig -count=1`

Expected: PASS for publish/list/history/CAS/delete/listener behavior.

### Task 3: Add Nacos Config HTTP compatibility with tests

**Files:**
- Create: `internal/adapter/nacos/config.go`
- Create: `internal/adapter/nacos/config_test.go`
- Modify: `internal/transport/http/server.go`

**Interfaces:**
- Consumes `configcenter.Service`.
- Produces `NewConfigHTTPAdapter(configcenter.Service) *ConfigHTTPAdapter` and `RegisterRoutes(*http.ServeMux, string, middleware)`.
- Supports both `/nacos/v1/cs/*` and `/v1/cs/*` registration prefixes.

- [ ] **Step 1: Write a failing Nacos contract test**

```go
func TestConfigAdapterServesNacosPublishQueryAndDelete(t *testing.T) {
    mux := nacosConfigMux(t)
    form := url.Values{"dataId":{"demo.properties"}, "group":{"DEFAULT_GROUP"}, "tenant":{"default"}, "content":{"feature=true"}}
    publish := httptest.NewRequest(http.MethodPost, "/nacos/v1/cs/configs", strings.NewReader(form.Encode()))
    publish.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    assertBody(t, serve(mux, publish), http.StatusOK, "true")
    assertBody(t, serve(mux, httptest.NewRequest(http.MethodGet, "/nacos/v1/cs/configs?dataId=demo.properties&group=DEFAULT_GROUP&tenant=default", nil)), http.StatusOK, "feature=true")
}
```

- [ ] **Step 2: Run it and verify Nacos Config paths are absent**

Run: `go test ./internal/adapter/nacos -run TestConfigAdapter -count=1`

Expected: FAIL because only Naming routes are registered.

- [ ] **Step 3: Implement form decoding and compatibility responses**

```go
func identityFromNacosForm(values url.Values) configcenter.Identity {
    return configcenter.Identity{Namespace: values.Get("tenant"), Group: values.Get("group"), DataID: values.Get("dataId")}
}

func writeNacosBoolean(w http.ResponseWriter) {
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    _, _ = io.WriteString(w, "true")
}
```

Parse the request form for every method. Return raw content for successful queries, literal `true` for successful POST/DELETE, and 404 for a missing active resource. Treat delete as idempotent success. Register POST listener requests with parsed `Listening-Configs`; return only changed Nacos keys and never content.

- [ ] **Step 4: Add listener parsing coverage and run adapter tests**

```go
func TestConfigAdapterListenerReturnsChangedKeyWithoutContent(t *testing.T) { /* long-poll then publish; assert dataId key and absence of content */ }
```

Run: `go test ./internal/adapter/nacos -run TestConfigAdapter -count=1`

Expected: PASS for publish, query, delete, defaults, not-found, and long-poll contracts.

### Task 4: Replace Config console mock calls with native API calls

**Files:**
- Create: `web/src/api/config/index.ts`
- Modify: `web/src/views/configs.vue`

**Interfaces:**
- Produces `listConfigs`, `listConfigHistory`, `publishConfig`, and `deleteConfig` typed Axios calls.
- Consumes `/v1/configs`, `/v1/config/history`, and `/v1/config` from Task 2.

- [ ] **Step 1: Replace the mocked Config interface import with a failing typed API import**

```ts
import {
  deleteConfig,
  listConfigHistory,
  listConfigs,
  publishConfig,
  type ConfigContentType,
  type ConfigHistoryEntry,
  type ConfigResource,
} from '../api/config'
```

- [ ] **Step 2: Run the web build and verify the module cannot resolve**

Run: `cd web; npm run build`

Expected: FAIL with `Cannot find module '../api/config'`.

- [ ] **Step 3: Implement the API module and update the four call sites**

```ts
export const listConfigs = (params: ConfigListQuery = {}) =>
  api.get<ConfigListResult>('/v1/configs', { params }).then((response) => response.data)
export const publishConfig = (payload: PublishConfigRequest) =>
  api.post<ConfigResource>('/v1/config', payload).then((response) => response.data)
export const deleteConfig = (target: ConfigIdentity) =>
  api.delete('/v1/config', { params: target })
```

Use server-side search/pagination when fetching data; retain current local type counters, selection behavior, confirmation modals, and error display. Remove only Config references to mock functions; leave Gateway mocks untouched.

- [ ] **Step 4: Run console checks**

Run: `cd web; npm run check:i18n; npm run build`

Expected: both commands exit 0 and TypeScript finds no mock Config imports.

### Task 5: Add the one-command Nacos Config example and documentation

**Files:**
- Create: `examples/config/nacos/cmd/listener/main.go`
- Create: `examples/config/nacos/start.sh`
- Create: `examples/config/nacos/start.bat`
- Create: `examples/config/nacos/README.md`
- Modify: `examples/service-discovery/README.md`
- Modify: `specs/zh-CN/config/config-publish-query-spec.md`
- Modify: `specs/zh-CN/config/config-listener-watch-spec.md`

**Interfaces:**
- Consumes the running local `/nacos/v1/cs/configs` endpoint.
- Produces a repeatable demo which publishes `demo.properties`, observes a listener notification, and exits with a non-zero status on failure.

- [ ] **Step 1: Write an executable smoke script assertion before the client implementation**

```sh
go run ./cmd/listener -server 127.0.0.1:8500 -data-id demo.properties -group DEFAULT_GROUP -namespace default
```

Expected: FAIL because `examples/config/nacos/cmd/listener` does not exist.

- [ ] **Step 2: Implement the Nacos Config listener example**

```go
client, err := config_client.NewConfigClient(vo.NacosClientParam{
    ServerConfigs: []constant.ServerConfig{{IpAddr: host, Port: port}},
    ClientConfig: &constant.ClientConfig{NamespaceId: namespace},
})
if err != nil { log.Fatal(err) }
_, err = client.PublishConfig(vo.ConfigParam{DataId: dataID, Group: group, Content: "feature.enabled=true"})
if err != nil { log.Fatal(err) }
```

Subscribe before the second publish, send the observed content or MD5 to stdout, and use a deadline so a lost notification fails quickly.

- [ ] **Step 3: Create parity launchers**

```sh
go run ./cmd/server/main.go -http :8500 -data-dir "$DEMO_DATA_DIR" &
server_pid=$!
trap 'kill "$server_pid"' EXIT
go run ./examples/config/nacos/cmd/listener -server 127.0.0.1:8500
```

The Windows script uses `Start-Process` and a `try/finally` block to stop only the process it started. Neither script deletes repository data; both use a temp/demo data directory.

- [ ] **Step 4: Document supported Nacos paths and run the demo**

Run: `go test ./...`

Run: `examples/config/nacos/start.sh`

Expected: all Go tests pass; the demo prints the initial publish, changed notification, and successful content reload.

### Task 6: Verify the integrated result

**Files:**
- Verify: `internal/configcenter`, `internal/transport/http`, `internal/adapter/nacos`, `web`, `examples/config/nacos`

- [ ] **Step 1: Format changed Go files**

Run: `gofmt -w internal/configcenter/*.go internal/transport/http/config*.go internal/transport/http/server.go internal/adapter/nacos/config*.go examples/config/nacos/cmd/listener/main.go`

- [ ] **Step 2: Run targeted backend tests**

Run: `go test ./internal/configcenter ./internal/transport/http ./internal/adapter/nacos -count=1`

Expected: PASS.

- [ ] **Step 3: Run repository and console verification**

Run: `go test ./...`

Run: `cd web; npm run check:i18n; npm run build`

Expected: all commands exit 0.

- [ ] **Step 4: Inspect the staged diff before committing**

Run: `git diff --check && git diff --stat`

Expected: no whitespace errors; only Config center, Nacos adapter, console, example, and Config specifications change.

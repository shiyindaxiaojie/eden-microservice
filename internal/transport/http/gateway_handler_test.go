package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shiyindaxiaojie/eden-registry/internal/catalog"
	"github.com/shiyindaxiaojie/eden-registry/internal/config"
	"github.com/shiyindaxiaojie/eden-registry/internal/gateway"
)

type gatewayRuntimeTestDiscovery struct{}

func (gatewayRuntimeTestDiscovery) GetService(string, string, bool) ([]*catalog.Instance, error) {
	return nil, nil
}

func newGatewayTestHandler(t *testing.T) (*Handler, func()) {
	t.Helper()
	service, err := gateway.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &Handler{gateway: service}, func() {
		if err := service.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}
}

func validGatewayRoutePayload() gateway.Route {
	return gateway.Route{
		Identity: gateway.Identity{Namespace: "default", ID: "orders"},
		Name:     "Orders",
		Enabled:  true,
		Match:    gateway.RouteMatch{PathPrefix: "/orders", Methods: []string{http.MethodGet}},
		Targets: []gateway.Target{{
			ID:          "stable",
			Type:        gateway.TargetStatic,
			Static:      &gateway.StaticTarget{Endpoints: []gateway.StaticEndpoint{{URL: "http://127.0.0.1:8081"}}},
			LoadBalance: gateway.LoadBalanceRoundRobin,
		}},
		Traffic: gateway.TrafficPolicy{
			Mode:            gateway.TrafficWeighted,
			DefaultTargetID: "stable",
			WeightedTargets: []gateway.WeightedTarget{{TargetID: "stable", Weight: 100}},
		},
	}
}

func gatewayRequest(t *testing.T, method, target string, payload any) *http.Request {
	t.Helper()
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequest(method, target, &body)
	request.Header.Set("Content-Type", "application/json")
	return request.WithContext(context.WithValue(request.Context(), UserContextKey, "admin"))
}

func TestGatewayRouteCreateGetListAndHistory(t *testing.T) {
	h, closeStore := newGatewayTestHandler(t)
	defer closeStore()

	createRecorder := httptest.NewRecorder()
	h.gatewayRoute(createRecorder, gatewayRequest(t, http.MethodPost, "/v1/gateway/route", validGatewayRoutePayload()))
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	var created gateway.Route
	if err := json.NewDecoder(createRecorder.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	getRecorder := httptest.NewRecorder()
	h.gatewayRoute(getRecorder, gatewayRequest(t, http.MethodGet, "/v1/gateway/route?namespace=default&id=orders", nil))
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s", getRecorder.Code, getRecorder.Body.String())
	}
	var got gateway.Route
	if err := json.NewDecoder(getRecorder.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Revision != created.Revision || got.Name != "Orders" {
		t.Fatalf("get route = %#v", got)
	}

	listRecorder := httptest.NewRecorder()
	h.gatewayRoutes(listRecorder, gatewayRequest(t, http.MethodGet, "/v1/gateway/routes?namespace=default", nil))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	var list gateway.ListResult
	if err := json.NewDecoder(listRecorder.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 || len(list.Data) != 1 {
		t.Fatalf("list = %#v", list)
	}

	historyRecorder := httptest.NewRecorder()
	h.gatewayHistory(historyRecorder, gatewayRequest(t, http.MethodGet, "/v1/gateway/history?namespace=default&id=orders", nil))
	if historyRecorder.Code != http.StatusOK {
		t.Fatalf("history status = %d body=%s", historyRecorder.Code, historyRecorder.Body.String())
	}
	var history []gateway.HistoryEntry
	if err := json.NewDecoder(historyRecorder.Body).Decode(&history); err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Action != gateway.HistoryCreate {
		t.Fatalf("history = %#v", history)
	}
}

func TestGatewayRouteUpdateReturnsConflictForStaleRevision(t *testing.T) {
	h, closeStore := newGatewayTestHandler(t)
	defer closeStore()

	created, err := h.gateway.Create(gateway.CreateRequest{Route: validGatewayRoutePayload(), Operator: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	stale := *created
	stale.Name = "Updated Orders"
	recorder := httptest.NewRecorder()
	h.gatewayRoute(recorder, gatewayRequest(t, http.MethodPut, "/v1/gateway/route", map[string]any{
		"route":             stale,
		"expected_revision": created.Revision - 1,
	}))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestGatewayRouteStatusChangesEnabledState(t *testing.T) {
	h, closeStore := newGatewayTestHandler(t)
	defer closeStore()
	created, err := h.gateway.Create(gateway.CreateRequest{Route: validGatewayRoutePayload(), Operator: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	h.gatewayRouteStatus(recorder, gatewayRequest(t, http.MethodPost, "/v1/gateway/route/status", map[string]any{
		"namespace":         created.Namespace,
		"id":                created.ID,
		"enabled":           false,
		"expected_revision": created.Revision,
	}))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status update code = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var updated gateway.Route
	if err := json.NewDecoder(recorder.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if updated.Enabled || updated.Revision <= created.Revision {
		t.Fatalf("updated = %#v", updated)
	}
}

func TestGatewayRoutesFiltersEnabledState(t *testing.T) {
	h, closeStore := newGatewayTestHandler(t)
	defer closeStore()
	if _, err := h.gateway.Create(gateway.CreateRequest{Route: validGatewayRoutePayload(), Operator: "admin"}); err != nil {
		t.Fatal(err)
	}
	disabled := validGatewayRoutePayload()
	disabled.ID = "orders-disabled"
	disabled.Enabled = false
	if _, err := h.gateway.Create(gateway.CreateRequest{Route: disabled, Operator: "admin"}); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	h.gatewayRoutes(recorder, gatewayRequest(t, http.MethodGet, "/v1/gateway/routes?enabled=false", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var result gateway.ListResult
	if err := json.NewDecoder(recorder.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Data) != 1 || result.Data[0].ID != disabled.ID {
		t.Fatalf("result = %#v", result)
	}
}

func TestGatewayRuntimeStatusReportsDataPlaneAvailability(t *testing.T) {
	service, err := gateway.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	runtime, err := gateway.NewRuntime(service, gatewayRuntimeTestDiscovery{}, gateway.RuntimeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	created, err := service.Create(gateway.CreateRequest{Route: validGatewayRoutePayload(), Operator: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{
		config:         &config.Config{Gateway: config.GatewayConfig{Enabled: false, HTTP: ":8080"}},
		gateway:        service,
		gatewayRuntime: runtime,
	}
	recorder := httptest.NewRecorder()
	h.gatewayRuntimeStatus(recorder, gatewayRequest(t, http.MethodGet, "/v1/gateway/runtime", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("disabled data plane status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var disabled []gateway.RuntimeStatus
	if err := json.NewDecoder(recorder.Body).Decode(&disabled); err != nil {
		t.Fatal(err)
	}
	if len(disabled) != 1 || disabled[0].ID != created.ID || disabled[0].DataPlaneEnabled {
		t.Fatalf("disabled data plane status = %#v", disabled)
	}

	h.config.Gateway.Enabled = true
	recorder = httptest.NewRecorder()
	h.gatewayRuntimeStatus(recorder, gatewayRequest(t, http.MethodGet, "/v1/gateway/runtime", nil))
	var enabled []gateway.RuntimeStatus
	if err := json.NewDecoder(recorder.Body).Decode(&enabled); err != nil {
		t.Fatal(err)
	}
	if len(enabled) != 1 || !enabled[0].DataPlaneEnabled {
		t.Fatalf("enabled data plane status = %#v", enabled)
	}
}

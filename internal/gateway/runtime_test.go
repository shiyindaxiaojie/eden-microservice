package gateway

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shiyindaxiaojie/eden-registry/internal/catalog"
)

type fakeDiscovery struct {
	mu        sync.Mutex
	instances map[string][]*catalog.Instance
	err       error
}

func (d *fakeDiscovery) GetService(namespace, name string, healthyOnly bool) ([]*catalog.Instance, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.err != nil {
		return nil, d.err
	}
	key := namespace + "/" + name
	instances := d.instances[key]
	result := make([]*catalog.Instance, 0, len(instances))
	for _, instance := range instances {
		copy := *instance
		result = append(result, &copy)
	}
	return result, nil
}

func testRuntime(t *testing.T, route Route, config RuntimeConfig) *Runtime {
	return testRuntimeWithDiscovery(t, route, &fakeDiscovery{instances: map[string][]*catalog.Instance{}}, config)
}

func testRuntimeWithDiscovery(t *testing.T, route Route, discovery Discovery, config RuntimeConfig) *Runtime {
	t.Helper()
	service := openTestStore(t)
	if _, err := service.Create(CreateRequest{Route: route, Operator: "admin"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	runtime, err := NewRuntime(service, discovery, config)
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}
	t.Cleanup(runtime.Close)
	return runtime
}

func trustedLoopbackConfig() RuntimeConfig {
	return RuntimeConfig{TrustedProxyCIDRs: []string{"127.0.0.1/32", "::1/128"}}
}

func TestRuntimeBetaUserOverridesCanaryWeight(t *testing.T) {
	route := validRoute()
	route.Traffic.BetaTargets = []BetaTarget{{TargetID: "canary", Users: []string{"u-beta"}}}
	runtime := testRuntime(t, route, trustedLoopbackConfig())
	request := httptest.NewRequest(http.MethodGet, "http://gateway/orders", nil)
	request.RemoteAddr = "127.0.0.1:3010"
	request.Header.Set("X-Eden-User-ID", "u-beta")

	target, ok := runtime.selectTarget(request, route)
	if !ok || target.ID != "canary" {
		t.Fatalf("selectTarget() = %#v, %v; want canary", target, ok)
	}
}

func TestRuntimeBetaTenantOverridesCanaryWeight(t *testing.T) {
	route := validRoute()
	route.Traffic.BetaTargets = []BetaTarget{{TargetID: "canary", Tenants: []string{"tenant-beta"}}}
	runtime := testRuntime(t, route, trustedLoopbackConfig())
	request := httptest.NewRequest(http.MethodGet, "http://gateway/orders", nil)
	request.RemoteAddr = "127.0.0.1:3010"
	request.Header.Set("X-Eden-Tenant-ID", "tenant-beta")

	target, ok := runtime.selectTarget(request, route)
	if !ok || target.ID != "canary" {
		t.Fatalf("selectTarget() = %#v, %v; want canary", target, ok)
	}
}

func TestRuntimeBetaUserTakesPrecedenceOverTenant(t *testing.T) {
	route := validRoute()
	route.Traffic.BetaTargets = []BetaTarget{
		{TargetID: "stable", Users: []string{"u-beta"}},
		{TargetID: "canary", Tenants: []string{"tenant-beta"}},
	}
	runtime := testRuntime(t, route, trustedLoopbackConfig())
	request := httptest.NewRequest(http.MethodGet, "http://gateway/orders", nil)
	request.RemoteAddr = "127.0.0.1:3010"
	request.Header.Set("X-Eden-User-ID", "u-beta")
	request.Header.Set("X-Eden-Tenant-ID", "tenant-beta")

	target, ok := runtime.selectTarget(request, route)
	if !ok || target.ID != "stable" {
		t.Fatalf("selectTarget() = %#v, %v; want stable", target, ok)
	}
}

func TestRuntimeIgnoresBetaHeaderFromUntrustedClient(t *testing.T) {
	route := validRoute()
	route.Traffic = TrafficPolicy{
		Mode:           TrafficBlueGreen,
		ActiveTargetID: "stable",
		BetaTargets:    []BetaTarget{{TargetID: "canary", Users: []string{"u-beta"}}},
	}
	runtime := testRuntime(t, route, trustedLoopbackConfig())
	request := httptest.NewRequest(http.MethodGet, "http://gateway/orders", nil)
	request.RemoteAddr = "203.0.113.44:3010"
	request.Header.Set("X-Eden-User-ID", "u-beta")

	target, ok := runtime.selectTarget(request, route)
	if !ok || target.ID != "stable" {
		t.Fatalf("selectTarget() = %#v, %v; want stable", target, ok)
	}
}

func TestRuntimeBlueGreenUsesActiveTargetForNormalTraffic(t *testing.T) {
	route := validRoute()
	route.Traffic = TrafficPolicy{Mode: TrafficBlueGreen, ActiveTargetID: "canary"}
	runtime := testRuntime(t, route, trustedLoopbackConfig())

	target, ok := runtime.selectTarget(httptest.NewRequest(http.MethodGet, "http://gateway/orders", nil), route)
	if !ok || target.ID != "canary" {
		t.Fatalf("selectTarget() = %#v, %v; want canary", target, ok)
	}
}

func TestRuntimeWeightedReleaseSelectsBothConfiguredTargets(t *testing.T) {
	route := validRoute()
	runtime := testRuntime(t, route, trustedLoopbackConfig())

	stableKey := ""
	canaryKey := ""
	for index := 0; index < 1_000 && (stableKey == "" || canaryKey == ""); index++ {
		key := fmt.Sprintf("request:%d", index)
		if selectionBucket(route, key) < 95 {
			stableKey = fmt.Sprintf("%d", index)
		} else {
			canaryKey = fmt.Sprintf("%d", index)
		}
	}
	if stableKey == "" || canaryKey == "" {
		t.Fatal("could not find deterministic release buckets")
	}
	for key, wanted := range map[string]string{stableKey: "stable", canaryKey: "canary"} {
		request := httptest.NewRequest(http.MethodGet, "http://gateway/orders", nil)
		request.Header.Set("X-Request-ID", key)
		target, ok := runtime.selectTarget(request, route)
		if !ok || target.ID != wanted {
			t.Fatalf("selectTarget(%q) = %#v, %v; want %q", key, target, ok, wanted)
		}
	}
}

func TestRuntimeDistributesWeightedTrafficWithoutStableIdentity(t *testing.T) {
	route := validRoute()
	runtime := testRuntime(t, route, trustedLoopbackConfig())
	counts := map[string]int{}
	for range 100 {
		request := httptest.NewRequest(http.MethodGet, "http://gateway/orders", nil)
		// A shared ingress address is deliberately not a stable release identity.
		request.RemoteAddr = "10.0.0.5:443"
		target, ok := runtime.selectTarget(request, route)
		if !ok {
			t.Fatal("selectTarget() returned no target")
		}
		counts[target.ID]++
	}
	if counts["stable"] != 95 || counts["canary"] != 5 {
		t.Fatalf("weighted distribution = %#v, want stable=95 canary=5", counts)
	}
}

func TestRuntimeReleaseCounterDoesNotCollideWithTargetLoadBalancer(t *testing.T) {
	route := staticRoute("http://127.0.0.1:1")
	releaseTarget := route.Targets[0]
	releaseTarget.ID = "release"
	route.Targets = append(route.Targets, releaseTarget)
	route.Traffic = TrafficPolicy{
		Mode:            TrafficCanary,
		DefaultTargetID: "static",
		WeightedTargets: []WeightedTarget{
			{TargetID: "static", Weight: 95},
			{TargetID: "release", Weight: 5},
		},
	}
	runtime := testRuntime(t, route, trustedLoopbackConfig())
	counts := map[string]int{}
	for range 100 {
		request := httptest.NewRequest(http.MethodGet, "http://gateway/api/orders", nil)
		request.RemoteAddr = "10.0.0.5:443"
		target, ok := runtime.selectTarget(request, route)
		if !ok {
			t.Fatal("selectTarget() returned no target")
		}
		counts[target.ID]++
		if runtime.selectStaticEndpoint(route, target) == nil {
			t.Fatal("selectStaticEndpoint() returned nil")
		}
	}
	if counts["static"] != 95 || counts["release"] != 5 {
		t.Fatalf("weighted distribution = %#v, want static=95 release=5", counts)
	}
}

func TestRuntimeProxiesRegistryServiceTarget(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/orders" {
			http.Error(w, "unexpected path", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte("service target"))
	}))
	defer upstream.Close()
	host, portText, err := net.SplitHostPort(strings.TrimPrefix(upstream.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	discovery := &fakeDiscovery{instances: map[string][]*catalog.Instance{
		"default/order-service": {{ID: "order-1", Host: host, Port: port, Weight: 1, Status: catalog.HealthPassing}},
	}}
	runtime := testRuntimeWithDiscovery(t, validRoute(), discovery, trustedLoopbackConfig())
	recorder := httptest.NewRecorder()
	runtime.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://gateway/orders", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "service target" {
		t.Fatalf("status/body = %d/%q", recorder.Code, recorder.Body.String())
	}
}

func TestRuntimeProxiesStaticTargetAndAppliesFilters(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/items" {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusBadRequest)
			return
		}
		if r.Header.Get("X-Gateway") != "eden" {
			http.Error(w, "missing gateway header", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("proxied"))
	}))
	defer upstream.Close()

	route := staticRoute(upstream.URL)
	route.Filters = []Filter{
		{Type: FilterStripPrefix, Parts: 2},
		{Type: FilterAddRequestHeader, Name: "X-Gateway", Value: "eden"},
		{Type: FilterSetResponseHeader, Name: "X-Release", Value: "preview"},
	}
	runtime := testRuntime(t, route, trustedLoopbackConfig())
	recorder := httptest.NewRecorder()
	runtime.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://gateway/api/orders/items", nil))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("X-Release") != "preview" || recorder.Body.String() != "proxied" {
		t.Fatalf("response headers/body = %#v / %q", recorder.Header(), recorder.Body.String())
	}
}

func TestRuntimeStripsUntrustedBetaIdentityHeadersBeforeProxy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Eden-User-ID") != "" || r.Header.Get("X-Eden-Tenant-ID") != "" {
			http.Error(w, "untrusted identity header reached upstream", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	runtime := testRuntime(t, staticRoute(upstream.URL), trustedLoopbackConfig())
	request := httptest.NewRequest(http.MethodGet, "http://gateway/api/orders", nil)
	request.RemoteAddr = "203.0.113.55:3000"
	request.Header.Set("X-Eden-User-ID", "spoofed")
	request.Header.Set("X-Eden-Tenant-ID", "spoofed-tenant")
	recorder := httptest.NewRecorder()
	runtime.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRuntimeReloadsSnapshotAfterRouteUpdate(t *testing.T) {
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("first"))
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("second"))
	}))
	defer second.Close()

	service := openTestStore(t)
	created, err := service.Create(CreateRequest{Route: staticRoute(first.URL), Operator: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewRuntime(service, &fakeDiscovery{instances: map[string][]*catalog.Instance{}}, trustedLoopbackConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()

	request := httptest.NewRequest(http.MethodGet, "http://gateway/api/orders", nil)
	recorder := httptest.NewRecorder()
	runtime.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "first" {
		t.Fatalf("before update = %d/%q", recorder.Code, recorder.Body.String())
	}

	updated := *created
	updated.Targets[0].Static.Endpoints[0].URL = second.URL
	if _, err := service.Update(UpdateRequest{Route: updated, ExpectedRevision: created.Revision, Operator: "admin"}); err != nil {
		t.Fatal(err)
	}
	recorder = httptest.NewRecorder()
	runtime.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://gateway/api/orders", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "second" {
		t.Fatalf("after update = %d/%q", recorder.Code, recorder.Body.String())
	}
}

func TestRuntimeReturnsGatewayErrorStatus(t *testing.T) {
	t.Run("no route", func(t *testing.T) {
		runtime := testRuntime(t, validRoute(), trustedLoopbackConfig())
		recorder := httptest.NewRecorder()
		runtime.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://gateway/not-found", nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d", recorder.Code)
		}
	})

	t.Run("no service instance", func(t *testing.T) {
		runtime := testRuntime(t, validRoute(), trustedLoopbackConfig())
		recorder := httptest.NewRecorder()
		runtime.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://gateway/orders", nil))
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d", recorder.Code)
		}
	})

	t.Run("upstream timeout", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			time.Sleep(100 * time.Millisecond)
		}))
		defer upstream.Close()
		route := staticRoute(upstream.URL)
		route.TimeoutMS = 10
		runtime := testRuntime(t, route, trustedLoopbackConfig())
		recorder := httptest.NewRecorder()
		runtime.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://gateway/api/orders", nil))
		if recorder.Code != http.StatusGatewayTimeout {
			t.Fatalf("status = %d", recorder.Code)
		}
	})

	t.Run("dial error", func(t *testing.T) {
		route := staticRoute("http://127.0.0.1:1")
		runtime := testRuntime(t, route, trustedLoopbackConfig())
		recorder := httptest.NewRecorder()
		runtime.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://gateway/api/orders", nil))
		if recorder.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
	})
}

func staticRoute(upstreamURL string) Route {
	route := validRoute()
	route.Match = RouteMatch{PathPrefix: "/api/orders", Methods: []string{http.MethodGet}}
	route.Targets = []Target{{
		ID:          "static",
		Type:        TargetStatic,
		Static:      &StaticTarget{Endpoints: []StaticEndpoint{{URL: upstreamURL, Weight: 1}}},
		LoadBalance: LoadBalanceRoundRobin,
	}}
	route.Traffic = TrafficPolicy{Mode: TrafficWeighted, DefaultTargetID: "static", WeightedTargets: []WeightedTarget{{TargetID: "static", Weight: 100}}}
	return route
}

func TestRuntimeReportsCurrentSnapshotRevision(t *testing.T) {
	runtime := testRuntime(t, validRoute(), trustedLoopbackConfig())
	statuses := runtime.Status("default")
	if len(statuses) != 1 || statuses[0].SnapshotRevision == 0 {
		t.Fatalf("Status() = %#v", statuses)
	}
	if got := fmt.Sprintf("%s/%s", statuses[0].Namespace, statuses[0].ID); got != "default/orders" {
		t.Fatalf("status identity = %s", got)
	}
}

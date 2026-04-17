package consul

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shiyindaxiaojie/eden-registry/internal/catalog"
	"github.com/shiyindaxiaojie/eden-registry/internal/config"
)

type fakeRegistry struct {
	statusCalls    []statusCall
	heartbeatCalls []heartbeatCall
}

type statusCall struct {
	namespace   string
	serviceName string
	instanceID  string
	status      string
}

type heartbeatCall struct {
	namespace   string
	serviceName string
	instanceID  string
}

func (f *fakeRegistry) Register(*catalog.Instance) error { return nil }

func (f *fakeRegistry) Deregister(namespace, serviceName, instanceID string) error { return nil }

func (f *fakeRegistry) SetInstanceStatus(namespace, serviceName, instanceID string, status string) error {
	f.statusCalls = append(f.statusCalls, statusCall{
		namespace:   namespace,
		serviceName: serviceName,
		instanceID:  instanceID,
		status:      status,
	})
	return nil
}

func (f *fakeRegistry) Heartbeat(namespace, serviceName, instanceID string) error {
	f.heartbeatCalls = append(f.heartbeatCalls, heartbeatCall{
		namespace:   namespace,
		serviceName: serviceName,
		instanceID:  instanceID,
	})
	return nil
}

func (f *fakeRegistry) ListServices(namespace string) ([]interface{}, error) { return nil, nil }

func (f *fakeRegistry) GetService(namespace, name string, healthyOnly bool) ([]*catalog.Instance, error) {
	return nil, nil
}

func (f *fakeRegistry) Subscribe(namespace, serviceName, consumerService string, ch chan catalog.WatchEvent) {
}

func (f *fakeRegistry) Unsubscribe(namespace, serviceName string, ch chan catalog.WatchEvent) {}

func (f *fakeRegistry) GetSubscribers(namespace, serviceName string) []string { return nil }

func (f *fakeRegistry) RecordDependency(namespace, consumerService, providerService string) {}

func (f *fakeRegistry) GetDependencyGraph(namespace string) map[string]interface{} { return nil }

func (f *fakeRegistry) ReportTopology(namespace, consumerService string, providers []string, checksum string) bool {
	return false
}

func (f *fakeRegistry) GetTopology(namespace string) *catalog.TopologyGraph { return nil }

func (f *fakeRegistry) ListNamespaces() []*catalog.Namespace { return nil }

func (f *fakeRegistry) CreateNamespace(ns *catalog.Namespace) bool { return false }

func (f *fakeRegistry) UpdateNamespace(ns *catalog.Namespace) bool { return false }

func (f *fakeRegistry) DeleteNamespace(name string) bool { return false }

func (f *fakeRegistry) Metrics() *catalog.MetricsStore { return nil }

func TestUpdateAgentCheckLegacyPassUsesInstanceIDRoute(t *testing.T) {
	t.Parallel()

	reg := &fakeRegistry{}
	adapter := NewHTTPAdapter(&config.Config{}, reg)

	req := httptest.NewRequest(http.MethodPut, "/v1/agent/check/pass/service:consul-auth-center-1?namespace=default", nil)
	rec := httptest.NewRecorder()

	adapter.UpdateAgentCheckLegacy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if len(reg.statusCalls) != 1 {
		t.Fatalf("expected 1 status call, got %d", len(reg.statusCalls))
	}
	if len(reg.heartbeatCalls) != 1 {
		t.Fatalf("expected 1 heartbeat call, got %d", len(reg.heartbeatCalls))
	}
	if reg.statusCalls[0].serviceName != "" {
		t.Fatalf("expected status update to avoid service-name lookup, got %q", reg.statusCalls[0].serviceName)
	}
	if reg.heartbeatCalls[0].serviceName != "" {
		t.Fatalf("expected heartbeat to avoid service-name lookup, got %q", reg.heartbeatCalls[0].serviceName)
	}
	if reg.heartbeatCalls[0].instanceID != "consul-auth-center-1" {
		t.Fatalf("expected heartbeat for instance id, got %q", reg.heartbeatCalls[0].instanceID)
	}
}

package httpclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shiyindaxiaojie/eden-registry/examples/service-discovery/custom/internal/registry"
)

func TestHeartbeatReRegistersMissingInstance(t *testing.T) {
	var (
		heartbeatCalls int
		registerCalls  int
		registerBody   map[string]interface{}
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/catalog/heartbeat":
			heartbeatCalls++
			http.Error(w, `{"error":"instance not found"}`, http.StatusNotFound)
		case "/v1/catalog/register":
			registerCalls++
			if err := json.NewDecoder(r.Body).Decode(&registerBody); err != nil {
				t.Fatalf("decode register body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &Client{
		targets:    []string{server.URL},
		namespace:  "default",
		datacenter: "dc1",
		httpClient: server.Client(),
	}
	instance := &registry.ServiceInstance{
		ID:          "custom-http-auth-center-1",
		ServiceName: "custom-http-auth-center",
		Host:        "127.0.0.1",
		Port:        24102,
		Weight:      100,
		Metadata:    map[string]string{"version": "v1"},
	}

	if err := client.Heartbeat(instance); err != nil {
		t.Fatalf("Heartbeat() error = %v", err)
	}
	if heartbeatCalls != 1 {
		t.Fatalf("heartbeat calls = %d, want 1", heartbeatCalls)
	}
	if registerCalls != 1 {
		t.Fatalf("register calls = %d, want 1", registerCalls)
	}
	if got := registerBody["service_name"]; got != instance.ServiceName {
		t.Fatalf("registered service = %v, want %q", got, instance.ServiceName)
	}
	if got := registerBody["namespace"]; got != "default" {
		t.Fatalf("registered namespace = %v, want default", got)
	}
}

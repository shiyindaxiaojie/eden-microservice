package nacos

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/shiyindaxiaojie/eden-registry/internal/catalog"
)

func TestRegisterInstanceStoresNacosGroupSeparately(t *testing.T) {
	state := catalog.NewState("")
	registry := catalog.NewRegistry(state, nil, nil, nil)
	adapter := NewHTTPAdapter(registry)
	mux := http.NewServeMux()
	adapter.RegisterRoutes(mux, "/nacos", func(next http.Handler) http.Handler { return next })

	query := url.Values{
		"serviceName": {"auth-service"},
		"groupName":   {"DEFAULT_GROUP"},
		"namespaceId": {"default"},
		"ip":          {"127.0.0.1"},
		"port":        {"8080"},
	}
	request := httptest.NewRequest(http.MethodPost, "/nacos/v1/ns/instance?"+query.Encode(), nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("register returned %d: %s", recorder.Code, recorder.Body.String())
	}

	instances := state.Instances.GetServiceNS("default", "DEFAULT_GROUP@@auth-service")
	if len(instances) != 1 {
		t.Fatalf("expected one registered instance, got %d", len(instances))
	}
	if instances[0].ServiceName != "auth-service" || instances[0].Group != "DEFAULT_GROUP" {
		t.Fatalf("expected separated Nacos identity, got service=%q group=%q", instances[0].ServiceName, instances[0].Group)
	}
}

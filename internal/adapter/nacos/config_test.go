package nacos

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/shiyindaxiaojie/eden-registry/internal/configcenter"
)

func newNacosConfigMux(t *testing.T) (*http.ServeMux, configcenter.Service) {
	t.Helper()
	service, err := configcenter.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	adapter := NewConfigHTTPAdapter(service)
	mux := http.NewServeMux()
	adapter.RegisterRoutes(mux, "/nacos", func(next http.Handler) http.Handler { return next })
	return mux, service
}

func serveNacosConfig(mux *http.ServeMux, request *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder
}

func nacosFormRequest(method, target string, values url.Values) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

func TestConfigAdapterServesNacosPublishQueryAndDelete(t *testing.T) {
	mux, _ := newNacosConfigMux(t)
	identity := url.Values{
		"dataId": {"demo.properties"},
		"group":  {"DEFAULT_GROUP"},
		"tenant": {"default"},
	}
	publishValues := url.Values{}
	for key, values := range identity {
		publishValues[key] = append([]string(nil), values...)
	}
	publishValues.Set("content", "feature.enabled=true")
	publishValues.Set("type", "properties")

	publish := serveNacosConfig(mux, nacosFormRequest(http.MethodPost, "/nacos/v1/cs/configs", publishValues))
	if publish.Code != http.StatusOK || publish.Body.String() != "true" {
		t.Fatalf("publish = %d %q", publish.Code, publish.Body.String())
	}

	query := serveNacosConfig(mux, httptest.NewRequest(http.MethodGet, "/nacos/v1/cs/configs?"+identity.Encode(), nil))
	if query.Code != http.StatusOK || query.Body.String() != "feature.enabled=true" {
		t.Fatalf("query = %d %q", query.Code, query.Body.String())
	}
	if contentType := query.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
		t.Fatalf("Content-Type = %q", contentType)
	}

	deleted := serveNacosConfig(mux, nacosFormRequest(http.MethodDelete, "/nacos/v1/cs/configs", identity))
	if deleted.Code != http.StatusOK || deleted.Body.String() != "true" {
		t.Fatalf("delete = %d %q", deleted.Code, deleted.Body.String())
	}
	missing := serveNacosConfig(mux, httptest.NewRequest(http.MethodGet, "/nacos/v1/cs/configs?"+identity.Encode(), nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d body=%q", missing.Code, missing.Body.String())
	}
	idempotentDelete := serveNacosConfig(mux, nacosFormRequest(http.MethodDelete, "/nacos/v1/cs/configs", identity))
	if idempotentDelete.Code != http.StatusOK || idempotentDelete.Body.String() != "true" {
		t.Fatalf("idempotent delete = %d %q", idempotentDelete.Code, idempotentDelete.Body.String())
	}
}

func TestConfigAdapterUsesNacosDefaults(t *testing.T) {
	mux, service := newNacosConfigMux(t)
	values := url.Values{"dataId": {"defaults.txt"}, "content": {"ok"}}
	recorder := serveNacosConfig(mux, nacosFormRequest(http.MethodPost, "/nacos/v1/cs/configs", values))
	if recorder.Code != http.StatusOK {
		t.Fatalf("publish status = %d body=%q", recorder.Code, recorder.Body.String())
	}
	resource, err := service.Get(configcenter.Identity{DataID: "defaults.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if resource.Namespace != configcenter.DefaultNamespace || resource.Group != configcenter.DefaultGroup {
		t.Fatalf("identity = %#v", resource.Identity)
	}
}

func TestConfigAdapterListenerReturnsChangedKeyWithoutContent(t *testing.T) {
	mux, service := newNacosConfigMux(t)
	resource, err := service.Publish(configcenter.PublishRequest{
		Identity: configcenter.Identity{Namespace: "default", Group: "DEFAULT_GROUP", DataID: "watch.properties"},
		Content:  "value=one",
	})
	if err != nil {
		t.Fatal(err)
	}

	listening := resource.DataID + "\x02" + resource.Group + "\x02" + resource.MD5 + "\x02" + resource.Namespace + "\x01"
	request := nacosFormRequest(http.MethodPost, "/nacos/v1/cs/configs/listener", url.Values{"Listening-Configs": {listening}})
	request.Header.Set("Long-Pulling-Timeout", "1000")
	result := make(chan *httptest.ResponseRecorder, 1)
	go func() { result <- serveNacosConfig(mux, request) }()

	time.Sleep(25 * time.Millisecond)
	if _, err := service.Publish(configcenter.PublishRequest{Identity: resource.Identity, Content: "value=two"}); err != nil {
		t.Fatal(err)
	}

	select {
	case recorder := <-result:
		want := resource.DataID + "\x02" + resource.Group + "\x02" + resource.Namespace + "\x01"
		if recorder.Code != http.StatusOK || recorder.Body.String() != want {
			t.Fatalf("listener = %d %q, want %q", recorder.Code, recorder.Body.String(), want)
		}
		if strings.Contains(recorder.Body.String(), "value=two") {
			t.Fatalf("listener leaked content: %q", recorder.Body.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("listener did not return")
	}
}

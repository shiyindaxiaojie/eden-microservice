package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shiyindaxiaojie/eden-registry/internal/configcenter"
)

func newConfigTestHandler(t *testing.T) (*Handler, func()) {
	t.Helper()
	service, err := configcenter.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &Handler{configs: service}, func() {
		if err := service.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}
}

func configRequest(t *testing.T, method, target string, payload any) *http.Request {
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

func TestConfigPublishListGetAndHistory(t *testing.T) {
	h, closeStore := newConfigTestHandler(t)
	defer closeStore()

	create := configRequest(t, http.MethodPost, "/v1/config", map[string]any{
		"data_id": "demo.properties",
		"content": "feature.enabled=false",
		"type":    "properties",
		"tags":    []string{"demo"},
	})
	createRecorder := httptest.NewRecorder()
	h.configResource(createRecorder, create)
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create status = %d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	var first configcenter.Resource
	if err := json.NewDecoder(createRecorder.Body).Decode(&first); err != nil {
		t.Fatal(err)
	}

	update := configRequest(t, http.MethodPut, "/v1/config", map[string]any{
		"namespace":    first.Namespace,
		"group":        first.Group,
		"data_id":      first.DataID,
		"content":      "feature.enabled=true",
		"type":         first.Type,
		"expected_md5": first.MD5,
	})
	updateRecorder := httptest.NewRecorder()
	h.configResource(updateRecorder, update)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}

	getRecorder := httptest.NewRecorder()
	h.configResource(getRecorder, configRequest(t, http.MethodGet, "/v1/config?data_id=demo.properties", nil))
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s", getRecorder.Code, getRecorder.Body.String())
	}
	var current configcenter.Resource
	if err := json.NewDecoder(getRecorder.Body).Decode(&current); err != nil {
		t.Fatal(err)
	}
	if current.Content != "feature.enabled=true" || current.Revision <= first.Revision {
		t.Fatalf("current = %#v, first revision=%d", current, first.Revision)
	}

	listRecorder := httptest.NewRecorder()
	h.listConfigs(listRecorder, configRequest(t, http.MethodGet, "/v1/configs?query=demo&page=1&page_size=12", nil))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	var list configcenter.ListResult
	if err := json.NewDecoder(listRecorder.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 || len(list.Data) != 1 || list.Data[0].MD5 != current.MD5 {
		t.Fatalf("list = %#v", list)
	}

	historyRecorder := httptest.NewRecorder()
	h.configHistory(historyRecorder, configRequest(t, http.MethodGet, "/v1/config/history?data_id=demo.properties", nil))
	if historyRecorder.Code != http.StatusOK {
		t.Fatalf("history status = %d body=%s", historyRecorder.Code, historyRecorder.Body.String())
	}
	var history []configcenter.HistoryEntry
	if err := json.NewDecoder(historyRecorder.Body).Decode(&history); err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Revision != first.Revision {
		t.Fatalf("history = %#v", history)
	}
}

func TestConfigPublishReturnsConflictForStaleMD5(t *testing.T) {
	h, closeStore := newConfigTestHandler(t)
	defer closeStore()

	if _, err := h.configs.Publish(configcenter.PublishRequest{Identity: configcenter.Identity{DataID: "cas.txt"}, Content: "current"}); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	h.configResource(recorder, configRequest(t, http.MethodPut, "/v1/config", map[string]any{
		"data_id":      "cas.txt",
		"content":      "stale",
		"expected_md5": "not-current",
	}))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestConfigListenerReturnsChangesAndEmptyTimeout(t *testing.T) {
	h, closeStore := newConfigTestHandler(t)
	defer closeStore()

	resource, err := h.configs.Publish(configcenter.PublishRequest{Identity: configcenter.Identity{DataID: "watch.txt"}, Content: "one"})
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	h.configListener(recorder, configRequest(t, http.MethodPost, "/v1/config/listener", map[string]any{
		"targets":    []map[string]any{{"data_id": resource.DataID, "md5": "stale"}},
		"timeout_ms": 100,
	}))
	if recorder.Code != http.StatusOK {
		t.Fatalf("listener status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var changes []configcenter.Change
	if err := json.NewDecoder(recorder.Body).Decode(&changes); err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].MD5 != resource.MD5 {
		t.Fatalf("changes = %#v", changes)
	}

	timeoutRecorder := httptest.NewRecorder()
	h.configListener(timeoutRecorder, configRequest(t, http.MethodPost, "/v1/config/listener", map[string]any{
		"targets":    []map[string]any{{"data_id": resource.DataID, "md5": resource.MD5}},
		"timeout_ms": 10,
	}))
	var timeoutChanges []configcenter.Change
	if err := json.NewDecoder(timeoutRecorder.Body).Decode(&timeoutChanges); err != nil {
		t.Fatal(err)
	}
	if len(timeoutChanges) != 0 {
		t.Fatalf("timeout changes = %#v", timeoutChanges)
	}
}

package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shiyindaxiaojie/eden-registry/internal/auth"
	"github.com/shiyindaxiaojie/eden-registry/internal/settings"
)

type handlerRuntimeStorage struct {
	eventMode   string
	metricsMode string
}

func (h *handlerRuntimeStorage) SetEventStorageMode(mode string) {
	h.eventMode = mode
}

func (h *handlerRuntimeStorage) SetMetricsStorageMode(mode string) {
	h.metricsMode = mode
}

type handlerEventCleaner struct{}

func (handlerEventCleaner) Cleanup(days int) {}

type handlerMetricsCleaner struct{}

func (handlerMetricsCleaner) Cleanup() {}

func newSettingsTestHandler(t *testing.T) (*Handler, settings.Controller, *handlerRuntimeStorage) {
	t.Helper()

	dataDir := t.TempDir()
	profile := settings.NewProfile(dataDir)
	store := auth.NewStore(dataDir)
	runtimeStorage := &handlerRuntimeStorage{}
	ctrl := settings.NewController(
		profile,
		store,
		nil,
		nil,
		handlerEventCleaner{},
		handlerMetricsCleaner{},
		runtimeStorage,
		settings.StartupState{Mode: "standalone", Consistency: "ap"},
	)
	return &Handler{settings: ctrl}, ctrl, runtimeStorage
}

func TestSystemSettingsGetReturnsCurrentSettings(t *testing.T) {
	t.Parallel()

	h, ctrl, _ := newSettingsTestHandler(t)
	if err := ctrl.SetEventStorageMode("persistent"); err != nil {
		t.Fatalf("SetEventStorageMode() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/settings/system", nil)
	rec := httptest.NewRecorder()

	h.systemSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var got settings.SystemSettings
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.EventStorageMode != "persistent" {
		t.Fatalf("EventStorageMode = %q, want persistent", got.EventStorageMode)
	}
}

func TestSystemSettingsPostAppliesValues(t *testing.T) {
	t.Parallel()

	h, ctrl, runtimeStorage := newSettingsTestHandler(t)
	body, err := json.Marshal(settings.SystemSettings{
		Mode:                        "standalone",
		Consistency:                 "ap",
		LogLevel:                    "debug",
		EventStorageMode:            "persistent",
		EventRetentionDays:          7,
		MetricsStorageMode:          "persistent",
		MetricsRetentionDays:        14,
		LogRetentionDays:            30,
		EventTypes:                  []string{"service.register", "service.offline"},
		HeartbeatMaxFailures:        4,
		InstanceRemovalDelaySeconds: 120,
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/settings/system", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.systemSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: body=%s", rec.Code, rec.Body.String())
	}
	if ctrl.GetLogLevel() != "DEBUG" {
		t.Fatalf("log level = %q, want DEBUG", ctrl.GetLogLevel())
	}
	if ctrl.GetEventStorageMode() != "persistent" || ctrl.GetMetricsStorageMode() != "persistent" {
		t.Fatalf("storage modes = (%q,%q), want persistent/persistent", ctrl.GetEventStorageMode(), ctrl.GetMetricsStorageMode())
	}
	if runtimeStorage.eventMode != "persistent" || runtimeStorage.metricsMode != "persistent" {
		t.Fatalf("runtime storage modes = (%q,%q), want persistent/persistent", runtimeStorage.eventMode, runtimeStorage.metricsMode)
	}
}

func TestSystemSettingsPostRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	h, _, _ := newSettingsTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/settings/system", bytes.NewBufferString("{"))
	rec := httptest.NewRecorder()

	h.systemSettings(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

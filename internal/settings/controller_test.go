package settings

import (
	"testing"

	"github.com/shiyindaxiaojie/eden-registry/internal/auth"
)

type fakeEventCleaner struct {
	calls []int
}

func (f *fakeEventCleaner) Cleanup(days int) {
	f.calls = append(f.calls, days)
}

type fakeMetricsCleaner struct {
	calls int
}

func (f *fakeMetricsCleaner) Cleanup() {
	f.calls++
}

type fakeRuntimeStorage struct {
	eventModes   []string
	metricsModes []string
}

func (f *fakeRuntimeStorage) SetEventStorageMode(mode string) {
	f.eventModes = append(f.eventModes, mode)
}

func (f *fakeRuntimeStorage) SetMetricsStorageMode(mode string) {
	f.metricsModes = append(f.metricsModes, mode)
}

func newTestController(t *testing.T, startup StartupState, runtimeStorage RuntimeStorage, events EventCleaner, metrics MetricsCleaner) Controller {
	t.Helper()

	dataDir := t.TempDir()
	profile := NewProfile(dataDir)
	store := auth.NewStore(dataDir)
	return NewController(profile, store, nil, nil, events, metrics, runtimeStorage, startup)
}

func TestControllerSetStorageModesStandaloneUpdatesRuntimeStorage(t *testing.T) {
	t.Parallel()

	runtimeStorage := &fakeRuntimeStorage{}
	ctrl := newTestController(t, StartupState{Mode: "standalone", Consistency: "ap"}, runtimeStorage, &fakeEventCleaner{}, &fakeMetricsCleaner{})

	if err := ctrl.SetEventStorageMode("persistent"); err != nil {
		t.Fatalf("SetEventStorageMode() error = %v", err)
	}
	if err := ctrl.SetMetricsStorageMode("persistent"); err != nil {
		t.Fatalf("SetMetricsStorageMode() error = %v", err)
	}

	if got := ctrl.GetEventStorageMode(); got != "persistent" {
		t.Fatalf("event storage mode = %q, want persistent", got)
	}
	if got := ctrl.GetMetricsStorageMode(); got != "persistent" {
		t.Fatalf("metrics storage mode = %q, want persistent", got)
	}
	if len(runtimeStorage.eventModes) != 1 || runtimeStorage.eventModes[0] != "persistent" {
		t.Fatalf("runtime event mode calls = %#v, want [persistent]", runtimeStorage.eventModes)
	}
	if len(runtimeStorage.metricsModes) != 1 || runtimeStorage.metricsModes[0] != "persistent" {
		t.Fatalf("runtime metrics mode calls = %#v, want [persistent]", runtimeStorage.metricsModes)
	}
}

func TestControllerSaveSettingLocalV2AppliesRuntimeStorageAndCleanup(t *testing.T) {
	t.Parallel()

	eventCleaner := &fakeEventCleaner{}
	metricsCleaner := &fakeMetricsCleaner{}
	runtimeStorage := &fakeRuntimeStorage{}
	ctrl := newTestController(t, StartupState{Mode: "cluster", Consistency: "ap"}, runtimeStorage, eventCleaner, metricsCleaner)

	ctrl.SaveSettingLocalV2("event_storage_mode", "persistent")
	ctrl.SaveSettingLocalV2("metrics_storage_mode", "persistent")
	ctrl.SaveSettingLocalV2("event_retention_days", float64(14))
	ctrl.SaveSettingLocalV2("metrics_retention_days", float64(21))

	if len(runtimeStorage.eventModes) != 1 || runtimeStorage.eventModes[0] != "persistent" {
		t.Fatalf("runtime event mode calls = %#v, want [persistent]", runtimeStorage.eventModes)
	}
	if len(runtimeStorage.metricsModes) != 1 || runtimeStorage.metricsModes[0] != "persistent" {
		t.Fatalf("runtime metrics mode calls = %#v, want [persistent]", runtimeStorage.metricsModes)
	}
	if len(eventCleaner.calls) != 1 || eventCleaner.calls[0] != 14 {
		t.Fatalf("event cleaner calls = %#v, want [14]", eventCleaner.calls)
	}
	if metricsCleaner.calls != 1 {
		t.Fatalf("metrics cleaner calls = %d, want 1", metricsCleaner.calls)
	}
	if got := ctrl.GetMetricsRetentionDays(); got != 21 {
		t.Fatalf("metrics retention days = %d, want 21", got)
	}
}

func TestControllerApplySystemSettingsReturnsRestartRequiredAndAppliesValues(t *testing.T) {
	t.Parallel()

	runtimeStorage := &fakeRuntimeStorage{}
	ctrl := newTestController(t, StartupState{
		Mode:        "standalone",
		Consistency: "ap",
		GRPCEnabled: true,
		QUICEnabled: false,
		RaftEnabled: false,
	}, runtimeStorage, &fakeEventCleaner{}, &fakeMetricsCleaner{})

	result, err := ctrl.ApplySystemSettings(&SystemSettings{
		Mode:                        "cluster",
		Consistency:                 "ap",
		LogLevel:                    "debug",
		EventStorageMode:            "persistent",
		EventRetentionDays:          7,
		MetricsStorageMode:          "persistent",
		MetricsRetentionDays:        15,
		LogRetentionDays:            9,
		EventTypes:                  []string{"service.register", "service.offline"},
		HeartbeatMaxFailures:        5,
		InstanceRemovalDelaySeconds: 120,
		APIKeyAuthEnabled:           true,
	})
	if err != nil {
		t.Fatalf("ApplySystemSettings() error = %v", err)
	}

	if !result.RestartRequired {
		t.Fatalf("RestartRequired = false, want true")
	}
	if result.Status != "ok" {
		t.Fatalf("Status = %q, want ok", result.Status)
	}
	if ctrl.GetEnvironment() != "cluster" {
		t.Fatalf("environment = %q, want cluster", ctrl.GetEnvironment())
	}
	if ctrl.GetMode() != "ap" {
		t.Fatalf("mode = %q, want ap", ctrl.GetMode())
	}
	if ctrl.GetLogLevel() != "DEBUG" {
		t.Fatalf("log level = %q, want DEBUG", ctrl.GetLogLevel())
	}
	if ctrl.GetEventStorageMode() != "persistent" || ctrl.GetMetricsStorageMode() != "persistent" {
		t.Fatalf("storage modes = (%q,%q), want persistent/persistent", ctrl.GetEventStorageMode(), ctrl.GetMetricsStorageMode())
	}
	if len(runtimeStorage.eventModes) == 0 || runtimeStorage.eventModes[len(runtimeStorage.eventModes)-1] != "persistent" {
		t.Fatalf("runtime event mode calls = %#v, want trailing persistent", runtimeStorage.eventModes)
	}
	if len(runtimeStorage.metricsModes) == 0 || runtimeStorage.metricsModes[len(runtimeStorage.metricsModes)-1] != "persistent" {
		t.Fatalf("runtime metrics mode calls = %#v, want trailing persistent", runtimeStorage.metricsModes)
	}
}

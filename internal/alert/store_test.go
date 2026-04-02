package alert

import "testing"

type mockProvider struct {
	configs map[string]*Config
}

func (m *mockProvider) GetAlertConfig(namespace string) *Config {
	return m.configs[namespace]
}

func (m *mockProvider) SaveAlertConfig(namespace string, cfg *Config) error {
	m.configs[namespace] = cfg
	return nil
}

func TestStoreLoadSave(t *testing.T) {
	t.Parallel()

	provider := &mockProvider{configs: make(map[string]*Config)}
	store := NewStore(provider)

	cfg, err := store.LoadConfig("prod")
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}
	if len(cfg.Rules) != 0 {
		t.Fatalf("expected empty config, got %#v", cfg)
	}

	cfg.Rules = append(cfg.Rules, Rule{
		ID:            "rule-1",
		Name:          "Service Offline Burst",
		EventCode:     "service_offline",
		Threshold:     3,
		WindowSec:     300,
		ChannelIDs:    []string{"channel-email"},
		TitleTemplate: "Eden Alert - {{ event_name }}",
		BodyTemplate:  "{{ event_code }} reached threshold {{ threshold }} in {{ window_sec }}s",
		Enabled:       true,
	})

	if err := store.SaveConfig("prod", cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	loaded, err := store.LoadConfig("prod")
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(loaded.Rules) != 1 || loaded.Rules[0].ID != "rule-1" {
		t.Fatalf("unexpected rules: %#v", loaded)
	}
	if len(loaded.Rules[0].ChannelIDs) != 1 || loaded.Rules[0].ChannelIDs[0] != "channel-email" {
		t.Fatalf("unexpected channel_ids: %#v", loaded.Rules[0].ChannelIDs)
	}
	if loaded.Rules[0].TitleTemplate == "" {
		t.Fatalf("expected title_template to be set")
	}
	if loaded.UpdatedAt == "" {
		t.Fatalf("expected updated_at to be set")
	}
}

package alert

import "testing"

func TestStoreLoadSave(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := NewStore(dir)

	cfg, err := store.LoadConfig("prod")
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}
	if len(cfg.Templates) != 0 || len(cfg.Rules) != 0 {
		t.Fatalf("expected empty config, got %#v", cfg)
	}

	cfg.Templates = append(cfg.Templates, Template{
		ID:            "tpl-1",
		Name:          "Offline Email",
		ChannelID:     "channel-email",
		TitleTemplate: "Service Alert",
		BodyTemplate:  "{{ event_code }} reached threshold",
		Enabled:       true,
	})
	cfg.Rules = append(cfg.Rules, Rule{
		ID:          "rule-1",
		Name:        "Service Offline Burst",
		EventCode:   "service_offline",
		Threshold:   3,
		WindowSec:   300,
		TemplateIDs: []string{"tpl-1"},
		Enabled:     true,
	})

	if err := store.SaveConfig("prod", cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	loaded, err := store.LoadConfig("prod")
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(loaded.Templates) != 1 || loaded.Templates[0].ID != "tpl-1" {
		t.Fatalf("unexpected templates: %#v", loaded)
	}
	if len(loaded.Rules) != 1 || loaded.Rules[0].ID != "rule-1" {
		t.Fatalf("unexpected rules: %#v", loaded)
	}
	if loaded.UpdatedAt == "" {
		t.Fatalf("expected updated_at to be set")
	}
}

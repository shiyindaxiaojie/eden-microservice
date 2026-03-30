package notify

import "testing"

func TestStoreLoadSave(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := NewStore(dir)

	cfg, err := store.Load("prod")
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}
	if len(cfg.Channels) != 0 {
		t.Fatalf("expected empty config, got %#v", cfg)
	}

	cfg.Channels = append(cfg.Channels, Channel{
		ID:       "chan-1",
		Name:     "Primary Webhook",
		Type:     "webhook",
		Provider: "feishu",
		Enabled:  true,
	})

	if err := store.Save("prod", cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	loaded, err := store.Load("prod")
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(loaded.Channels) != 1 || loaded.Channels[0].ID != "chan-1" {
		t.Fatalf("unexpected loaded config: %#v", loaded)
	}
	if loaded.UpdatedAt == "" {
		t.Fatalf("expected updated_at to be set")
	}
}

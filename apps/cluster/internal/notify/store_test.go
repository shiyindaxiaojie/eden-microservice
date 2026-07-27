package notify

import "testing"

type fakeConfigProvider struct {
	configs map[string]*Config
}

func (f *fakeConfigProvider) GetNotifyConfig(namespace string) *Config {
	if f == nil || f.configs == nil {
		return nil
	}
	return f.configs[namespace]
}

func (f *fakeConfigProvider) SaveNotifyConfig(namespace string, cfg *Config) error {
	if f.configs == nil {
		f.configs = make(map[string]*Config)
	}
	cp := *cfg
	cp.Channels = append([]Channel(nil), cfg.Channels...)
	f.configs[namespace] = &cp
	return nil
}

func TestStoreLoadSave(t *testing.T) {
	t.Parallel()

	provider := &fakeConfigProvider{}
	store := NewStore(provider)

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

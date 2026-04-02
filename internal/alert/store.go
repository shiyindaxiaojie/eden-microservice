package alert

import (
	"sync"
	"time"
)

const defaultNamespace = "default"

type Rule struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	EventCode     string   `json:"event_code"`
	Threshold     int      `json:"threshold,omitempty"`
	WindowSec     int      `json:"window_sec,omitempty"`
	ChannelIDs    []string `json:"channel_ids,omitempty"`
	TitleTemplate string   `json:"title_template,omitempty"`
	BodyTemplate  string   `json:"body_template,omitempty"`
	Enabled       bool     `json:"enabled"`
}

type Config struct {
	Rules     []Rule `json:"rules"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type ConfigProvider interface {
	GetAlertConfig(namespace string) *Config
	SaveAlertConfig(namespace string, cfg *Config) error
}

type Store struct {
	provider ConfigProvider
	mu       sync.RWMutex
}

func NewStore(provider ConfigProvider) *Store {
	return &Store{provider: provider}
}

func (s *Store) LoadConfig(namespace string) (*Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cfg := s.provider.GetAlertConfig(namespace)
	if cfg == nil {
		defaultCfg := defaultConfig()
		return &defaultCfg, nil
	}

	normalizeConfig(cfg)
	return cfg, nil
}

func (s *Store) SaveConfig(namespace string, cfg *Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cfg == nil {
		defaultCfg := defaultConfig()
		cfg = &defaultCfg
	}
	normalizeConfig(cfg)
	cfg.UpdatedAt = time.Now().Format(time.RFC3339)

	return s.provider.SaveAlertConfig(namespace, cfg)
}

func defaultConfig() Config {
	return Config{
		Rules: []Rule{},
	}
}

func normalizeConfig(cfg *Config) {
	if cfg.Rules == nil {
		cfg.Rules = []Rule{}
	}
	for i := range cfg.Rules {
		if cfg.Rules[i].ChannelIDs == nil {
			cfg.Rules[i].ChannelIDs = []string{}
		}
	}
}

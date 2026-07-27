package notify

import (
	"sync"
	"time"
)

const defaultNamespace = "default"

type Channel struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Provider    string                 `json:"provider"`
	Description string                 `json:"description,omitempty"`
	Enabled     bool                   `json:"enabled"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

type Config struct {
	Channels  []Channel `json:"channels"`
	UpdatedAt string    `json:"updated_at,omitempty"`
}

type ConfigProvider interface {
	GetNotifyConfig(namespace string) *Config
	SaveNotifyConfig(namespace string, cfg *Config) error
}

type Store struct {
	provider ConfigProvider
	mu       sync.RWMutex
}

func NewStore(provider ConfigProvider) *Store {
	return &Store{provider: provider}
}

func (s *Store) Load(namespace string) (*Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cfg := s.provider.GetNotifyConfig(namespace)
	if cfg == nil {
		defaultCfg := defaultConfig()
		return &defaultCfg, nil
	}

	normalizeConfig(cfg)
	return cfg, nil
}

func (s *Store) Save(namespace string, cfg *Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cfg == nil {
		defaultCfg := defaultConfig()
		cfg = &defaultCfg
	}
	normalizeConfig(cfg)
	cfg.UpdatedAt = time.Now().Format(time.RFC3339)

	return s.provider.SaveNotifyConfig(namespace, cfg)
}

func defaultConfig() Config {
	return Config{Channels: []Channel{}}
}

func normalizeConfig(cfg *Config) {
	if cfg.Channels == nil {
		cfg.Channels = []Channel{}
	}
}

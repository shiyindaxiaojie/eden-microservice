package alert

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const defaultNamespace = "default"

type Template struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ChannelID     string `json:"channel_id"`
	TitleTemplate string `json:"title_template,omitempty"`
	BodyTemplate  string `json:"body_template"`
	Enabled       bool   `json:"enabled"`
}

type Rule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	EventCode   string   `json:"event_code"`
	Threshold   int      `json:"threshold,omitempty"`
	WindowSec   int      `json:"window_sec,omitempty"`
	TemplateIDs []string `json:"template_ids,omitempty"`
	Enabled     bool     `json:"enabled"`
}

type Config struct {
	Templates []Template `json:"templates"`
	Rules     []Rule     `json:"rules"`
	UpdatedAt string     `json:"updated_at,omitempty"`
}

type Store struct {
	dataDir string
	mu      sync.RWMutex
}

func NewStore(dataDir string) *Store {
	return &Store{dataDir: dataDir}
}

func (s *Store) LoadConfig(namespace string) (*Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := s.configPath(namespace)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := defaultConfig()
			return &cfg, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	normalizeConfig(&cfg)
	return &cfg, nil
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

	path := s.configPath(namespace)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *Store) configPath(namespace string) string {
	return filepath.Join(s.dataDir, "alert", cleanNamespace(namespace), "config.json")
}

func cleanNamespace(namespace string) string {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return defaultNamespace
	}

	replacer := strings.NewReplacer("/", "_", "\\", "_", "..", "_")
	namespace = replacer.Replace(namespace)
	if namespace == "" {
		return defaultNamespace
	}
	return namespace
}

func defaultConfig() Config {
	return Config{
		Templates: []Template{},
		Rules:     []Rule{},
	}
}

func normalizeConfig(cfg *Config) {
	if cfg.Templates == nil {
		cfg.Templates = []Template{}
	}
	if cfg.Rules == nil {
		cfg.Rules = []Rule{}
	}
}

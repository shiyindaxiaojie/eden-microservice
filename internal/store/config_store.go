package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// ConfigStore handles cluster settings, mode, and environment.
type ConfigStore struct {
	mu          sync.RWMutex
	mode        string
	environment string
	seeds       []string
	logLevel    string
	dataPath    string
}

func NewConfigStore(dataPath string) *ConfigStore {
	s := &ConfigStore{
		mode:        "ap",
		environment: "standalone",
		seeds:       []string{},
		dataPath:    dataPath,
	}
	s.Load()
	return s
}

func (s *ConfigStore) GetMode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mode
}

func (s *ConfigStore) SetMode(m string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mode = m
}

func (s *ConfigStore) GetEnvironment() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.environment
}

func (s *ConfigStore) SetEnvironment(e string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.environment = e
}

func (s *ConfigStore) GetSeeds() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.seeds
}

func (s *ConfigStore) SetSeeds(seeds []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seeds = seeds
}

func (s *ConfigStore) GetLogLevel() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.logLevel
}

func (s *ConfigStore) SetLogLevel(l string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logLevel = l
}

func (s *ConfigStore) Restore(mode, env, logLevel string, seeds []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mode = mode
	s.environment = env
	s.logLevel = logLevel
	s.seeds = seeds
	s.Save()
}

func (s *ConfigStore) Load() {
	if s.dataPath == "" {
		return
	}
	// Load settings
	settingsFile := filepath.Join(s.dataPath, "settings.json")
	if data, err := os.ReadFile(settingsFile); err == nil {
		var meta struct {
			Mode        string `json:"mode"`
			Environment string `json:"environment"`
			LogLevel    string `json:"log_level"`
		}
		if err := json.Unmarshal(data, &meta); err == nil {
			s.mode = meta.Mode
			s.environment = meta.Environment
			s.logLevel = meta.LogLevel
		}
	}
	// Load nodes
	nodesFile := filepath.Join(s.dataPath, "nodes.json")
	if data, err := os.ReadFile(nodesFile); err == nil {
		var seeds []string
		if err := json.Unmarshal(data, &seeds); err == nil {
			s.seeds = seeds
		}
	}
}

func (s *ConfigStore) Save() {
	if s.dataPath == "" {
		return
	}
	os.MkdirAll(s.dataPath, 0755)
	
	// Save settings
	settingsFile := filepath.Join(s.dataPath, "settings.json")
	meta := struct {
		Mode        string `json:"mode"`
		Environment string `json:"environment"`
		LogLevel    string `json:"log_level"`
	}{
		Mode:        s.GetMode(),
		Environment: s.GetEnvironment(),
		LogLevel:    s.GetLogLevel(),
	}
	data, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(settingsFile, data, 0644)

	// Save nodes
	nodesFile := filepath.Join(s.dataPath, "nodes.json")
	nodesData, _ := json.MarshalIndent(s.GetSeeds(), "", "  ")
	_ = os.WriteFile(nodesFile, nodesData, 0644)
}

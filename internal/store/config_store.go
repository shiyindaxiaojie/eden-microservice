package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
)

// ConfigStore handles cluster settings, mode, and environment.
type ConfigStore struct {
	mu           sync.RWMutex
	mode         string
	environment  string
	seeds        []string
	logLevel     string
	eventRetDays int
	logRetDays   int
	eventTypes   []string
	hbMaxFail    int
	removalDelay int
	dataPath     string
}

func NewConfigStore(dataPath string) *ConfigStore {
	s := &ConfigStore{
		mode:         "ap",
		environment:  "standalone",
		seeds:        []string{},
		eventRetDays: 30,
		logRetDays:   30,
		eventTypes:   model.DefaultEventTypes(),
		hbMaxFail:    3,
		removalDelay: 600,
		dataPath:     dataPath,
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
	s.logLevel = model.NormalizeLogLevel(l)
}

func (s *ConfigStore) GetEventRetentionDays() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.eventRetDays
}

func (s *ConfigStore) SetEventRetentionDays(days int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventRetDays = days
}

func (s *ConfigStore) GetLogRetentionDays() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.logRetDays
}

func (s *ConfigStore) SetLogRetentionDays(days int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logRetDays = days
}

func (s *ConfigStore) GetEventTypes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, len(s.eventTypes))
	copy(result, s.eventTypes)
	return result
}

func (s *ConfigStore) SetEventTypes(types []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventTypes = model.NormalizeEventTypes(types)
}

func (s *ConfigStore) GetHeartbeatMaxFailures() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hbMaxFail
}

func (s *ConfigStore) SetHeartbeatMaxFailures(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hbMaxFail = n
}

func (s *ConfigStore) GetInstanceRemovalDelaySeconds() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.removalDelay
}

func (s *ConfigStore) SetInstanceRemovalDelaySeconds(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removalDelay = n
}

func (s *ConfigStore) Restore(mode, env, logLevel string, seeds []string, eventRet, logRet int, eventTypes []string, hbMaxFail, removalDelay int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mode = mode
	s.environment = env
	s.logLevel = model.NormalizeLogLevel(logLevel)
	s.seeds = seeds
	if eventRet > 0 {
		s.eventRetDays = eventRet
	}
	if logRet > 0 {
		s.logRetDays = logRet
	}
	if eventTypes != nil {
		s.eventTypes = model.NormalizeEventTypes(eventTypes)
	}
	if hbMaxFail > 0 {
		s.hbMaxFail = hbMaxFail
	}
	if removalDelay > 0 {
		s.removalDelay = removalDelay
	}
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
			Mode               string    `json:"mode"`
			Environment        string    `json:"environment"`
			LogLevel           string    `json:"log_level"`
			EventRetentionDays int       `json:"event_retention_days"`
			LogRetentionDays   int       `json:"log_retention_days"`
			EventTypes         *[]string `json:"event_types"`
			HBMaxFail          int       `json:"heartbeat_max_failures"`
			RemovalDelay       int       `json:"instance_removal_delay_seconds"`
		}
		if err := json.Unmarshal(data, &meta); err == nil {
			s.mode = meta.Mode
			s.environment = meta.Environment
			s.logLevel = model.NormalizeLogLevel(meta.LogLevel)
			if meta.EventRetentionDays > 0 {
				s.eventRetDays = meta.EventRetentionDays
			}
			if meta.LogRetentionDays > 0 {
				s.logRetDays = meta.LogRetentionDays
			}
			if meta.EventTypes != nil {
				s.eventTypes = model.NormalizeEventTypes(*meta.EventTypes)
			}
			if meta.HBMaxFail > 0 {
				s.hbMaxFail = meta.HBMaxFail
			}
			if meta.RemovalDelay > 0 {
				s.removalDelay = meta.RemovalDelay
			}
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
		Mode               string   `json:"mode"`
		Environment        string   `json:"environment"`
		LogLevel           string   `json:"log_level"`
		EventRetentionDays int      `json:"event_retention_days"`
		LogRetentionDays   int      `json:"log_retention_days"`
		EventTypes         []string `json:"event_types"`
		HBMaxFail          int      `json:"heartbeat_max_failures"`
		RemovalDelay       int      `json:"instance_removal_delay_seconds"`
	}{
		Mode:               s.GetMode(),
		Environment:        s.GetEnvironment(),
		LogLevel:           s.GetLogLevel(),
		EventRetentionDays: s.GetEventRetentionDays(),
		LogRetentionDays:   s.GetLogRetentionDays(),
		EventTypes:         s.GetEventTypes(),
		HBMaxFail:          s.GetHeartbeatMaxFailures(),
		RemovalDelay:       s.GetInstanceRemovalDelaySeconds(),
	}
	data, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(settingsFile, data, 0644)

	// Save nodes
	nodesFile := filepath.Join(s.dataPath, "nodes.json")
	nodesData, _ := json.MarshalIndent(s.GetSeeds(), "", "  ")
	_ = os.WriteFile(nodesFile, nodesData, 0644)
}

package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/alert"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/catalog"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/notify"
)

// Profile persists cluster mode, environment, and runtime
type Profile struct {
	mu            sync.RWMutex
	mode          string
	environment   string
	loaded        bool
	seeds         []string
	logLevel      string
	eventRetDays  int
	logRetDays    int
	eventTypes    []string
	hbMaxFail     int
	removalDelay  int
	apiKeyAuth    bool
	apiKeyAuthSet bool
	notifyAlertNodeID string
	dataPath      string
	alertConfigs      map[string]*alert.Config
	notifyConfigs     map[string]*notify.Config
}

func NewProfile(dataPath string) *Profile {
	s := &Profile{
		mode:         "ap",
		environment:  "standalone",
		seeds:        []string{},
		eventRetDays: 30,
		logRetDays:   30,
		eventTypes:   catalog.DefaultEventTypes(),
		hbMaxFail:    3,
		removalDelay: 600,
		dataPath:     dataPath,
		alertConfigs:  make(map[string]*alert.Config),
		notifyConfigs: make(map[string]*notify.Config),
	}
	s.Load()
	return s
}

func (s *Profile) GetMode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mode
}

func (s *Profile) SetMode(m string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mode = m
}

func (s *Profile) GetEnvironment() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.environment
}

func (s *Profile) LoadedFromDisk() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loaded
}

func (s *Profile) SetEnvironment(e string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.environment = e
}

func (s *Profile) GetSeeds() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.seeds
}

func (s *Profile) SetSeeds(seeds []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seeds = seeds
}

func (s *Profile) GetLogLevel() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.logLevel
}

func (s *Profile) SetLogLevel(l string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logLevel = NormalizeLogLevel(l)
}

func (s *Profile) GetEventRetentionDays() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.eventRetDays
}

func (s *Profile) SetEventRetentionDays(days int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventRetDays = days
}

func (s *Profile) GetLogRetentionDays() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.logRetDays
}

func (s *Profile) SetLogRetentionDays(days int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logRetDays = days
}

func (s *Profile) GetEventTypes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, len(s.eventTypes))
	copy(result, s.eventTypes)
	return result
}

func (s *Profile) SetEventTypes(types []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventTypes = catalog.NormalizeEventTypes(types)
}

func (s *Profile) GetHeartbeatMaxFailures() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hbMaxFail
}

func (s *Profile) SetHeartbeatMaxFailures(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hbMaxFail = n
}

func (s *Profile) GetInstanceRemovalDelaySeconds() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.removalDelay
}

func (s *Profile) SetInstanceRemovalDelaySeconds(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removalDelay = n
}

func (s *Profile) GetAPIKeyAuthEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.apiKeyAuth
}

func (s *Profile) HasAPIKeyAuthEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.apiKeyAuthSet
}

func (s *Profile) SetAPIKeyAuthEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.apiKeyAuth = enabled
	s.apiKeyAuthSet = true
}

func (s *Profile) GetNotifyAlertNodeID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.notifyAlertNodeID
}

func (s *Profile) SetNotifyAlertNodeID(nodeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notifyAlertNodeID = nodeID
}

func (s *Profile) Restore(mode, env, logLevel string, seeds []string, eventRet, logRet int, eventTypes []string, hbMaxFail, removalDelay int, apiKeyAuth bool, notifyAlertNodeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mode = mode
	s.environment = env
	s.logLevel = NormalizeLogLevel(logLevel)
	s.seeds = seeds
	if eventRet > 0 {
		s.eventRetDays = eventRet
	}
	if logRet > 0 {
		s.logRetDays = logRet
	}
	if eventTypes != nil {
		s.eventTypes = catalog.NormalizeEventTypes(eventTypes)
	}
	if hbMaxFail > 0 {
		s.hbMaxFail = hbMaxFail
	}
	if removalDelay > 0 {
		s.removalDelay = removalDelay
	}
	s.apiKeyAuth = apiKeyAuth
	s.apiKeyAuthSet = true
	s.notifyAlertNodeID = notifyAlertNodeID
	s.notifyAlertNodeID = notifyAlertNodeID
	s.Save()
}

func (s *Profile) GetAlertConfig(namespace string) *alert.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if cfg, ok := s.alertConfigs[namespace]; ok {
		return cfg
	}
	return nil
}

func (s *Profile) SaveAlertConfig(namespace string, cfg *alert.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.alertConfigs == nil {
		s.alertConfigs = make(map[string]*alert.Config)
	}
	s.alertConfigs[namespace] = cfg
	s.Save()
}

func (s *Profile) GetNotifyConfig(namespace string) *notify.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if cfg, ok := s.notifyConfigs[namespace]; ok {
		return cfg
	}
	return nil
}

func (s *Profile) SaveNotifyConfig(namespace string, cfg *notify.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.notifyConfigs == nil {
		s.notifyConfigs = make(map[string]*notify.Config)
	}
	s.notifyConfigs[namespace] = cfg
	s.Save()
}

func (s *Profile) Load() {
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
			APIKeyAuthEnabled  *bool                     `json:"api_key_auth_enabled"`
			NotifyAlertNodeID  string                    `json:"notify_alert_node_id"`
			AlertConfigs       map[string]*alert.Config  `json:"alert_configs,omitempty"`
			NotifyConfigs      map[string]*notify.Config `json:"notify_configs,omitempty"`
		}
		if err := json.Unmarshal(data, &meta); err == nil {
			s.mode = meta.Mode
			s.environment = meta.Environment
			s.logLevel = NormalizeLogLevel(meta.LogLevel)
			if meta.EventRetentionDays > 0 {
				s.eventRetDays = meta.EventRetentionDays
			}
			if meta.LogRetentionDays > 0 {
				s.logRetDays = meta.LogRetentionDays
			}
			if meta.EventTypes != nil {
				s.eventTypes = catalog.NormalizeEventTypes(*meta.EventTypes)
			}
			if meta.HBMaxFail > 0 {
				s.hbMaxFail = meta.HBMaxFail
			}
			if meta.RemovalDelay > 0 {
				s.removalDelay = meta.RemovalDelay
			}
			if meta.APIKeyAuthEnabled != nil {
				s.apiKeyAuth = *meta.APIKeyAuthEnabled
				s.apiKeyAuthSet = true
			}
			s.notifyAlertNodeID = meta.NotifyAlertNodeID
			if meta.AlertConfigs != nil {
				s.alertConfigs = meta.AlertConfigs
			}
			if meta.NotifyConfigs != nil {
				s.notifyConfigs = meta.NotifyConfigs
			}
			s.loaded = true
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

func (s *Profile) Save() {
	if s.dataPath == "" {
		return
	}
	os.MkdirAll(s.dataPath, 0755)
	s.mu.Lock()
	s.loaded = true
	s.mu.Unlock()

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
		APIKeyAuthEnabled  bool                      `json:"api_key_auth_enabled"`
		NotifyAlertNodeID  string                    `json:"notify_alert_node_id,omitempty"`
		AlertConfigs       map[string]*alert.Config  `json:"alert_configs,omitempty"`
		NotifyConfigs      map[string]*notify.Config `json:"notify_configs,omitempty"`
	}{
		Mode:               s.GetMode(),
		Environment:        s.GetEnvironment(),
		LogLevel:           s.GetLogLevel(),
		EventRetentionDays: s.GetEventRetentionDays(),
		LogRetentionDays:   s.GetLogRetentionDays(),
		EventTypes:         s.GetEventTypes(),
		HBMaxFail:          s.GetHeartbeatMaxFailures(),
		RemovalDelay:       s.GetInstanceRemovalDelaySeconds(),
		APIKeyAuthEnabled:  s.GetAPIKeyAuthEnabled(),
		NotifyAlertNodeID:  s.GetNotifyAlertNodeID(),
		AlertConfigs:       s.alertConfigs,
		NotifyConfigs:      s.notifyConfigs,
	}
	data, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(settingsFile, data, 0644)

	// Save nodes
	nodesFile := filepath.Join(s.dataPath, "nodes.json")
	nodesData, _ := json.MarshalIndent(s.GetSeeds(), "", "  ")
	_ = os.WriteFile(nodesFile, nodesData, 0644)
}

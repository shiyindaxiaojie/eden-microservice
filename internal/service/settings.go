package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/shiyindaxiaojie/eden-go-logger"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/cluster/cp"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/store"
)

type settingsService struct {
	store  *store.Registry
	cpNode CPNode
	apNode APNode
}

func NewSettingsService(s *store.Registry, cp CPNode, ap APNode) SettingsService {
	return &settingsService{
		store:  s,
		cpNode: cp,
		apNode: ap,
	}
}

func (s *settingsService) isCPCluster() bool {
	return s.store.GetEnvironment() == "cluster" && s.store.GetMode() == "cp" && s.cpNode != nil
}

func (s *settingsService) ensureClusterWritable() error {
	if !s.isCPCluster() {
		return nil
	}
	if s.cpNode.IsLeader() {
		return nil
	}
	if leader := s.cpNode.LeaderAddr(); leader != "" {
		return fmt.Errorf("not leader, redirect to %s", leader)
	}
	return fmt.Errorf("not leader")
}

func applyRuntimeLogLevel(level string) {
	if lg, ok := logger.GetLogger().(*logger.Logger); ok {
		lg.SetLevel(logger.ParseLevel(level))
	}
}

func (s *settingsService) AddUser(u *model.User) error {
	mode := s.store.GetMode()
	if mode == "cp" {
		cmd := cp.Command{Type: cp.CmdAddUser, User: u}
		return s.cpNode.Apply(cmd, 5*time.Second)
	}
	if s.apNode != nil {
		return s.apNode.Apply("add_user", u, false)
	}
	s.store.AddUser(u)
	return nil
}

func (s *settingsService) GetUser(username string) (*model.User, bool) {
	return s.store.GetUser(username)
}

func (s *settingsService) DeleteUser(username string) error {
	mode := s.store.GetMode()
	if mode == "cp" {
		cmd := cp.Command{Type: cp.CmdDeleteUser, Username: username}
		return s.cpNode.Apply(cmd, 5*time.Second)
	}
	if s.apNode != nil {
		return s.apNode.Apply("delete_user", username, false)
	}
	s.store.DeleteUser(username)
	return nil
}

func (s *settingsService) ListUsers() ([]*model.User, error) {
	return s.store.ListUsers(), nil
}

func (s *settingsService) AddAPIKey(key *model.APIKey) error {
	mode := s.store.GetMode()
	if mode == "cp" {
		cmd := cp.Command{Type: cp.CmdAddAPIKey, APIKey: key}
		return s.cpNode.Apply(cmd, 5*time.Second)
	}
	if s.apNode != nil {
		return s.apNode.Apply("add_api_key", key, false)
	}
	s.store.AddAPIKey(key)
	return nil
}

func (s *settingsService) DeleteAPIKey(key string) error {
	mode := s.store.GetMode()
	if mode == "cp" {
		cmd := cp.Command{Type: cp.CmdDeleteAPIKey, Key: key}
		return s.cpNode.Apply(cmd, 5*time.Second)
	}
	if s.apNode != nil {
		return s.apNode.Apply("delete_api_key", key, false)
	}
	s.store.DeleteAPIKey(key)
	return nil
}

func (s *settingsService) ListAPIKeys() ([]*model.APIKey, error) {
	return s.store.ListAPIKeys(), nil
}

func (s *settingsService) SetMode(mode string) error {
	if err := s.ensureClusterWritable(); err != nil {
		return err
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "ap" && mode != "cp" {
		return errors.New("invalid mode")
	}
	if mode == s.store.GetMode() {
		return nil
	}

	currentEnv := s.store.GetEnvironment()
	if mode == "cp" && currentEnv == "cluster" && s.cpNode != nil {
		cmd := cp.Command{Type: cp.CmdSetMode, Mode: mode}
		// If already in cluster mode, replicate but also continue to HTTP sync
		_ = s.cpNode.Apply(cmd, 5*time.Second)
	}

	s.store.SetMode(mode)

	// If switching to CP, automatically invite existing AP seeds into the Raft cluster
	if mode == "cp" && s.cpNode != nil {
		seeds := s.store.GetSeeds()
		for _, seedAddr := range seeds {
			go func(addr string) {
				// 1. Fetch node info to get RaftAddr and NodeID
				client := http.Client{Timeout: 5 * time.Second}
				resp, err := client.Get(addr + "/v1/node/info")
				if err != nil {
					return
				}
				defer resp.Body.Close()

				var remoteCfg struct {
					NodeID   string `json:"node_id"`
					RaftAddr string `json:"raft_addr"`
				}
				if json.NewDecoder(resp.Body).Decode(&remoteCfg) == nil {
					if remoteCfg.NodeID != "" && remoteCfg.RaftAddr != "" {
						_ = s.cpNode.Join(remoteCfg.NodeID, remoteCfg.RaftAddr)
					}
				}
			}(seedAddr)
		}
	}

	// Sync to peers via HTTP
	s.syncSettingsToPeers(map[string]string{"mode": mode})
	return nil
}

func (s *settingsService) SetEnvironment(env string) error {
	if err := s.ensureClusterWritable(); err != nil {
		return err
	}
	env = strings.ToLower(strings.TrimSpace(env))
	if env != "standalone" && env != "cluster" {
		return errors.New("invalid environment")
	}
	if env == s.store.GetEnvironment() {
		return nil
	}
	mode := s.store.GetMode()
	if mode == "cp" && env == "cluster" && s.cpNode != nil {
		cmd := cp.Command{Type: cp.CmdSetEnv, Environment: env}
		_ = s.cpNode.Apply(cmd, 5*time.Second)
	}
	s.store.SetEnvironment(env)
	// Sync to peers via HTTP
	s.syncSettingsToPeers(map[string]string{"environment": env})
	return nil
}

func (s *settingsService) SetLogLevel(level string) error {
	if err := s.ensureClusterWritable(); err != nil {
		return err
	}
	level = model.NormalizeLogLevel(level)
	mode := s.store.GetMode()
	if mode == "cp" && s.cpNode != nil {
		cmd := cp.Command{Type: cp.CmdSetLogLevel, LogLevel: level}
		if err := s.cpNode.Apply(cmd, 5*time.Second); err != nil {
			return err
		}
	}
	s.store.SetLogLevel(level)
	applyRuntimeLogLevel(level)
	// Sync to peers via HTTP
	s.syncSettingsToPeers(map[string]string{"log_level": level})
	return nil
}

func (s *settingsService) SetEventRetentionDays(days int) error {
	if err := s.ensureClusterWritable(); err != nil {
		return err
	}
	if days <= 0 {
		return errors.New("invalid event retention days")
	}
	mode := s.store.GetMode()
	if mode == "cp" && s.cpNode != nil {
		cmd := cp.Command{Type: cp.CmdSetEventRetentionDays, IntValue: days}
		if err := s.cpNode.Apply(cmd, 5*time.Second); err != nil {
			return err
		}
	}
	s.store.SetEventRetentionDays(days)
	s.syncSettingsToPeers(map[string]interface{}{"event_retention_days": days})
	return nil
}

func (s *settingsService) GetEventRetentionDays() int {
	return s.store.GetEventRetentionDays()
}

func (s *settingsService) SetLogRetentionDays(days int) error {
	if err := s.ensureClusterWritable(); err != nil {
		return err
	}
	if days <= 0 {
		return errors.New("invalid log retention days")
	}
	if s.store.GetMode() == "cp" && s.cpNode != nil {
		cmd := cp.Command{Type: cp.CmdSetLogRetentionDays, IntValue: days}
		if err := s.cpNode.Apply(cmd, 5*time.Second); err != nil {
			return err
		}
	}
	s.store.SetLogRetentionDays(days)
	s.syncSettingsToPeers(map[string]interface{}{"log_retention_days": days})
	return nil
}

func (s *settingsService) GetLogRetentionDays() int {
	return s.store.GetLogRetentionDays()
}

func (s *settingsService) SetEventTypes(types []string) error {
	if err := s.ensureClusterWritable(); err != nil {
		return err
	}
	normalized := model.NormalizeEventTypes(types)
	if types == nil {
		normalized = nil
	}
	if s.store.GetMode() == "cp" && s.cpNode != nil {
		cmd := cp.Command{Type: cp.CmdSetEventTypes, StringList: normalized}
		if err := s.cpNode.Apply(cmd, 5*time.Second); err != nil {
			return err
		}
	}
	s.store.SetEventTypes(normalized)
	s.syncSettingsToPeers(map[string]interface{}{"event_types": normalized})
	return nil
}

func (s *settingsService) GetEventTypes() []string {
	return s.store.GetEventTypes()
}

func (s *settingsService) SetHeartbeatMaxFailures(n int) error {
	if err := s.ensureClusterWritable(); err != nil {
		return err
	}
	if n <= 0 {
		return errors.New("invalid heartbeat max failures")
	}
	if s.store.GetMode() == "cp" && s.cpNode != nil {
		cmd := cp.Command{Type: cp.CmdSetHeartbeatMaxFailures, IntValue: n}
		if err := s.cpNode.Apply(cmd, 5*time.Second); err != nil {
			return err
		}
	}
	s.store.SetHeartbeatMaxFailures(n)
	s.syncSettingsToPeers(map[string]interface{}{"heartbeat_max_failures": n})
	return nil
}

func (s *settingsService) GetHeartbeatMaxFailures() int {
	return s.store.GetHeartbeatMaxFailures()
}

func (s *settingsService) SetInstanceRemovalDelaySeconds(n int) error {
	if err := s.ensureClusterWritable(); err != nil {
		return err
	}
	if n <= 0 {
		return errors.New("invalid instance removal delay seconds")
	}
	if s.store.GetMode() == "cp" && s.cpNode != nil {
		cmd := cp.Command{Type: cp.CmdSetInstanceRemovalDelaySeconds, IntValue: n}
		if err := s.cpNode.Apply(cmd, 5*time.Second); err != nil {
			return err
		}
	}
	s.store.SetInstanceRemovalDelaySeconds(n)
	s.syncSettingsToPeers(map[string]interface{}{"instance_removal_delay_seconds": n})
	return nil
}

func (s *settingsService) GetInstanceRemovalDelaySeconds() int {
	return s.store.GetInstanceRemovalDelaySeconds()
}

func (s *settingsService) GetMode() string {
	return s.store.GetMode()
}

func (s *settingsService) GetLogLevel() string {
	return s.store.GetLogLevel()
}

func (s *settingsService) GetEnvironment() string {
	return s.store.GetEnvironment()
}

func (s *settingsService) GetSeeds() []string {
	return s.store.GetSeeds()
}

func (s *settingsService) SaveSeedsLocal(seeds []string) {
	s.store.SetSeeds(seeds)
	if s.apNode != nil {
		s.apNode.SyncSeeds()
	}
}

func (s *settingsService) SaveSettingLocal(key, value string) {
	// Value is handled as interface{} in syncSettingsToPeers
	// but here we get raw strings from internal sync handlers usually.
	// We might need to handle different types if we use interface{} in handlers.
}

func (s *settingsService) GetSystemSettings() *model.SystemSettings {
	return &model.SystemSettings{
		Mode:                        s.store.GetMode(),
		Environment:                 s.store.GetEnvironment(),
		LogLevel:                    s.store.GetLogLevel(),
		EventRetentionDays:          s.store.GetEventRetentionDays(),
		LogRetentionDays:            s.store.GetLogRetentionDays(),
		EventTypes:                  s.store.GetEventTypes(),
		HeartbeatMaxFailures:        s.store.GetHeartbeatMaxFailures(),
		InstanceRemovalDelaySeconds: s.store.GetInstanceRemovalDelaySeconds(),
	}
}

func (s *settingsService) ApplySystemSettings(settings *model.SystemSettings) error {
	if settings == nil {
		return errors.New("settings required")
	}

	target := *settings
	if target.Mode == "" {
		target.Mode = s.store.GetMode()
	}
	if target.Environment == "" {
		target.Environment = s.store.GetEnvironment()
	}
	target.LogLevel = model.NormalizeLogLevel(target.LogLevel)
	if target.EventRetentionDays <= 0 {
		target.EventRetentionDays = s.store.GetEventRetentionDays()
	}
	if target.LogRetentionDays <= 0 {
		target.LogRetentionDays = s.store.GetLogRetentionDays()
	}
	if target.EventTypes == nil {
		target.EventTypes = s.store.GetEventTypes()
	} else {
		target.EventTypes = model.NormalizeEventTypes(target.EventTypes)
	}
	if target.HeartbeatMaxFailures <= 0 {
		target.HeartbeatMaxFailures = s.store.GetHeartbeatMaxFailures()
	}
	if target.InstanceRemovalDelaySeconds <= 0 {
		target.InstanceRemovalDelaySeconds = s.store.GetInstanceRemovalDelaySeconds()
	}

	if err := s.SetEnvironment(target.Environment); err != nil {
		return err
	}
	if err := s.SetMode(target.Mode); err != nil {
		return err
	}
	if err := s.SetEventRetentionDays(target.EventRetentionDays); err != nil {
		return err
	}
	if err := s.SetEventTypes(target.EventTypes); err != nil {
		return err
	}
	if err := s.SetLogLevel(target.LogLevel); err != nil {
		return err
	}
	if err := s.SetLogRetentionDays(target.LogRetentionDays); err != nil {
		return err
	}
	if err := s.SetHeartbeatMaxFailures(target.HeartbeatMaxFailures); err != nil {
		return err
	}
	if err := s.SetInstanceRemovalDelaySeconds(target.InstanceRemovalDelaySeconds); err != nil {
		return err
	}
	return nil
}

func coerceInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func coerceStringSlice(value interface{}) ([]string, bool) {
	switch v := value.(type) {
	case []string:
		return v, true
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			result = append(result, s)
		}
		return result, true
	default:
		return nil, false
	}
}

func (s *settingsService) SaveSettingLocalV2(key string, value interface{}) {
	switch key {
	case "mode":
		if v, ok := value.(string); ok {
			s.store.SetMode(strings.ToLower(v))
		}
	case "environment":
		if v, ok := value.(string); ok {
			s.store.SetEnvironment(strings.ToLower(v))
		}
	case "log_level":
		if v, ok := value.(string); ok {
			level := model.NormalizeLogLevel(v)
			s.store.SetLogLevel(level)
			applyRuntimeLogLevel(level)
		}
	case "event_retention_days":
		if v, ok := coerceInt(value); ok {
			s.store.SetEventRetentionDays(v)
		}
	case "log_retention_days":
		if v, ok := coerceInt(value); ok {
			s.store.SetLogRetentionDays(v)
		}
	case "event_types":
		if v, ok := coerceStringSlice(value); ok {
			s.store.SetEventTypes(v)
		}
	case "heartbeat_max_failures":
		if v, ok := coerceInt(value); ok {
			s.store.SetHeartbeatMaxFailures(v)
		}
	case "instance_removal_delay_seconds":
		if v, ok := coerceInt(value); ok {
			s.store.SetInstanceRemovalDelaySeconds(v)
		}
	}
}
func (s *settingsService) SetSeeds(seeds []string) error {
	s.store.SetSeeds(seeds)
	if s.apNode != nil {
		s.apNode.SyncSeeds()
	}

	// Sync seeds to each peer via HTTP API
	// Each peer should know about all OTHER nodes (including this node's HTTP addr)
	config := s.apNode.GetConfig()
	selfHTTPAddr := config.HTTPAddr
	// Normalize self address (e.g. ":8500" -> "http://127.0.0.1:8500")
	if strings.HasPrefix(selfHTTPAddr, ":") {
		selfHTTPAddr = "http://127.0.0.1" + selfHTTPAddr
	} else if !strings.HasPrefix(selfHTTPAddr, "http") {
		selfHTTPAddr = "http://" + selfHTTPAddr
	}

	// Build the full node list (self + all seeds)
	allNodes := make([]string, 0, len(seeds)+1)
	allNodes = append(allNodes, selfHTTPAddr)
	allNodes = append(allNodes, seeds...)

	for _, peer := range seeds {
		// Build per-peer seeds: all nodes except the peer itself
		peerSeeds := make([]string, 0, len(allNodes)-1)
		for _, n := range allNodes {
			if n != peer {
				peerSeeds = append(peerSeeds, n)
			}
		}
		go s.syncSeedsToPeerHTTP(peer, peerSeeds)
		// Ensure the new peer also knows it should be in the same mode and environment
		go s.syncSettingsToPeerHTTP(peer, map[string]string{
			"mode":        s.store.GetMode(),
			"environment": "cluster",
		})
	}
	return nil
}

func (s *settingsService) syncSettingsToPeerHTTP(peerAddr string, settings map[string]string) {
	body, err := json.Marshal(settings)
	if err != nil {
		return
	}

	url := peerAddr + "/internal/sync/settings"
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		logger.Warn("[SyncSettings] Failed to sync settings to %s: %v", peerAddr, err)
		return
	}
	resp.Body.Close()
	logger.Info("[SyncSettings] Synced settings to %s", peerAddr)
}

func (s *settingsService) syncSeedsToPeerHTTP(peerAddr string, seeds []string) {
	body, err := json.Marshal(map[string][]string{"seeds": seeds})
	if err != nil {
		return
	}

	url := peerAddr + "/internal/sync/seeds"
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		logger.Warn("[SetSeeds] Failed to sync seeds to %s: %v", peerAddr, err)
		return
	}
	resp.Body.Close()
	logger.Info("[SetSeeds] Synced seeds to %s", peerAddr)
}

func (s *settingsService) syncSettingsToPeers(settings interface{}) {
	if s.apNode == nil {
		return
	}
	seeds := s.store.GetSeeds()
	if len(seeds) == 0 {
		return
	}

	body, err := json.Marshal(settings)
	if err != nil {
		return
	}

	for _, peer := range seeds {
		go func(addr string) {
			url := addr + "/internal/sync/settings"
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				logger.Warn("[SyncSettings] Failed to sync to %s: %v", addr, err)
				return
			}
			resp.Body.Close()
			logger.Info("[SyncSettings] Synced to %s", addr)
		}(peer)
	}
}

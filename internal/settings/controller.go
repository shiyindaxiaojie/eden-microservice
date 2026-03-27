package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/shiyindaxiaojie/eden-go-logger"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/auth"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/catalog"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/cluster/replication"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/config"
)

type Controller interface {
	AddUser(u *auth.User) error
	GetUser(username string) (*auth.User, bool)
	DeleteUser(username string) error
	ListUsers() ([]*auth.User, error)
	AddAPIKey(key *auth.APIKey) error
	DeleteAPIKey(key string) error
	ListAPIKeys() ([]*auth.APIKey, error)
	SetMode(mode string) error
	SetEnvironment(env string) error
	SetLogLevel(level string) error
	GetLogLevel() string
	GetMode() string
	GetEnvironment() string
	GetSeeds() []string
	SetSeeds(seeds []string) error
	SaveSeedsLocal(seeds []string)
	SaveUserLocal(u *auth.User)
	DeleteUserLocal(username string)
	SaveAPIKeyLocal(key *auth.APIKey)
	DeleteAPIKeyLocal(key string)
	SaveSettingLocal(key, value string)
	SaveSettingLocalV2(key string, value interface{})
	GetSystemSettings() *SystemSettings
	ApplySystemSettings(settings *SystemSettings) (*ApplySystemSettingsResult, error)
	SetEventRetentionDays(days int) error
	GetEventRetentionDays() int
	SetLogRetentionDays(days int) error
	GetLogRetentionDays() int
	SetEventTypes(types []string) error
	GetEventTypes() []string
	SetHeartbeatMaxFailures(n int) error
	GetHeartbeatMaxFailures() int
	SetInstanceRemovalDelaySeconds(n int) error
	GetInstanceRemovalDelaySeconds() int
	SetAPIKeyAuthEnabled(enabled bool) error
}

type CPNode interface {
	Apply(cmd interface{}, timeout time.Duration) error
	IsLeader() bool
	LeaderAddr() string
	Join(nodeID, addr string) error
}

type APNode interface {
	Apply(cmdType string, data interface{}, isReplicate bool) error
	SyncSeeds()
	GetConfig() *config.Config
}

type EventCleaner interface {
	Cleanup(days int)
}

type controller struct {
	profile *Profile
	auth    *auth.Directory
	cpNode  CPNode
	apNode  APNode
	events  EventCleaner
	startup StartupState
}

func NewController(profile *Profile, directory *auth.Directory, cp CPNode, ap APNode, events EventCleaner, startup StartupState) Controller {
	return &controller{
		profile: profile,
		auth:    directory,
		cpNode:  cp,
		apNode:  ap,
		events:  events,
		startup: startup,
	}
}

func (c *controller) isCPCluster() bool {
	return c.profile.GetEnvironment() == "cluster" && c.profile.GetMode() == "cp" && c.cpNode != nil
}

func (c *controller) ensureClusterWritable() error {
	if !c.isCPCluster() {
		return nil
	}
	if c.cpNode.IsLeader() {
		return nil
	}
	if leader := c.cpNode.LeaderAddr(); leader != "" {
		return fmt.Errorf("not leader, redirect to %s", leader)
	}
	return fmt.Errorf("not leader")
}

func applyRuntimeLogLevel(level string) {
	if lg, ok := logger.GetLogger().(*logger.Logger); ok {
		lg.SetLevel(logger.ParseLevel(level))
	}
}

func normalizeTopologySetting(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), "cluster") {
		return "cluster"
	}
	return "standalone"
}

func normalizeConsistencySetting(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), "cp") {
		return "cp"
	}
	return "ap"
}

func normalizeRuntimeSelection(topology, consistency string) (string, string) {
	normalizedTopology := normalizeTopologySetting(topology)
	if normalizedTopology != "cluster" {
		return normalizedTopology, "ap"
	}
	return normalizedTopology, normalizeConsistencySetting(consistency)
}

func (c *controller) evaluateRestartRequirement(target *SystemSettings) (bool, string) {
	if target == nil {
		return false, ""
	}

	topology, consistency := normalizeRuntimeSelection(target.Mode, target.Consistency)

	if topology == "cluster" && normalizeTopologySetting(c.startup.Mode) != "cluster" {
		return true, "当前进程以单机模式启动，切换到集群模式需要重启"
	}
	if topology == "cluster" && consistency == "cp" && (normalizeConsistencySetting(c.startup.Consistency) != "cp" || !c.startup.RaftEnabled) {
		return true, "当前进程未以 CP 模式启动，切换到 CP 模式需要重启"
	}
	return false, ""
}

func (c *controller) AddUser(u *auth.User) error {
	mode := c.profile.GetMode()
	if mode == "cp" && c.cpNode != nil {
		cmd := replication.Command{Type: replication.CmdAddUser, User: toReplicatedUser(u)}
		return c.cpNode.Apply(cmd, 5*time.Second)
	}
	if c.apNode != nil {
		return c.apNode.Apply("add_user", u, false)
	}
	c.SaveUserLocal(u)
	return nil
}

func (c *controller) GetUser(username string) (*auth.User, bool) {
	return c.auth.GetUser(username)
}

func (c *controller) DeleteUser(username string) error {
	mode := c.profile.GetMode()
	if mode == "cp" && c.cpNode != nil {
		cmd := replication.Command{Type: replication.CmdDeleteUser, Username: username}
		return c.cpNode.Apply(cmd, 5*time.Second)
	}
	if c.apNode != nil {
		return c.apNode.Apply("delete_user", username, false)
	}
	c.DeleteUserLocal(username)
	return nil
}

func (c *controller) ListUsers() ([]*auth.User, error) {
	return c.auth.ListUsers(), nil
}

func (c *controller) AddAPIKey(key *auth.APIKey) error {
	mode := c.profile.GetMode()
	if mode == "cp" && c.cpNode != nil {
		cmd := replication.Command{Type: replication.CmdAddAPIKey, APIKey: toReplicatedAPIKey(key)}
		return c.cpNode.Apply(cmd, 5*time.Second)
	}
	if c.apNode != nil {
		return c.apNode.Apply("add_api_key", key, false)
	}
	c.SaveAPIKeyLocal(key)
	return nil
}

func (c *controller) DeleteAPIKey(key string) error {
	mode := c.profile.GetMode()
	if mode == "cp" && c.cpNode != nil {
		cmd := replication.Command{Type: replication.CmdDeleteAPIKey, Key: key}
		return c.cpNode.Apply(cmd, 5*time.Second)
	}
	if c.apNode != nil {
		return c.apNode.Apply("delete_api_key", key, false)
	}
	c.DeleteAPIKeyLocal(key)
	return nil
}

func (c *controller) ListAPIKeys() ([]*auth.APIKey, error) {
	return c.auth.ListAPIKeys(), nil
}

func toReplicatedUser(user *auth.User) *replication.User {
	if user == nil {
		return nil
	}
	return &replication.User{
		Username:  user.Username,
		Password:  user.Password,
		Nickname:  user.Nickname,
		Phone:     user.Phone,
		Email:     user.Email,
		Remark:    user.Remark,
		Role:      user.Role,
		IsBuiltIn: user.IsBuiltIn,
	}
}

func toReplicatedAPIKey(key *auth.APIKey) *replication.APIKey {
	if key == nil {
		return nil
	}
	return &replication.APIKey{
		Key:         key.Key,
		Label:       key.Label,
		Description: key.Description,
		CreatedBy:   key.CreatedBy,
		CreatedAt:   key.CreatedAt,
		ExpiresAt:   key.ExpiresAt,
		Status:      key.Status,
	}
}

func (c *controller) SetMode(mode string) error {
	if err := c.ensureClusterWritable(); err != nil {
		return err
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "ap" && mode != "cp" {
		return errors.New("invalid mode")
	}
	if c.profile.GetEnvironment() != "cluster" {
		mode = "ap"
	}
	if mode == c.profile.GetMode() {
		return nil
	}

	currentEnv := c.profile.GetEnvironment()
	if mode == "cp" && currentEnv == "cluster" && c.cpNode != nil {
		cmd := replication.Command{Type: replication.CmdSetMode, Mode: mode}
		_ = c.cpNode.Apply(cmd, 5*time.Second)
	}

	c.profile.SetMode(mode)
	c.profile.Save()

	if mode == "cp" && c.cpNode != nil {
		seeds := c.profile.GetSeeds()
		for _, seedAddr := range seeds {
			go func(addr string) {
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
						_ = c.cpNode.Join(remoteCfg.NodeID, remoteCfg.RaftAddr)
					}
				}
			}(seedAddr)
		}
	}

	c.syncSettingsToPeers(map[string]string{"consistency": mode})
	return nil
}

func (c *controller) SetEnvironment(env string) error {
	if err := c.ensureClusterWritable(); err != nil {
		return err
	}
	env = strings.ToLower(strings.TrimSpace(env))
	if env != "standalone" && env != "cluster" {
		return errors.New("invalid environment")
	}
	if env == c.profile.GetEnvironment() {
		return nil
	}
	mode := c.profile.GetMode()
	if mode == "cp" && env == "cluster" && c.cpNode != nil {
		cmd := replication.Command{Type: replication.CmdSetEnv, Environment: env}
		_ = c.cpNode.Apply(cmd, 5*time.Second)
	}
	c.profile.SetEnvironment(env)
	if env == "standalone" {
		c.profile.SetMode("ap")
	}
	c.profile.Save()
	settingsToSync := map[string]string{"mode": env}
	if env == "standalone" {
		settingsToSync["consistency"] = "ap"
	}
	c.syncSettingsToPeers(settingsToSync)
	return nil
}

func (c *controller) SetLogLevel(level string) error {
	if err := c.ensureClusterWritable(); err != nil {
		return err
	}
	level = NormalizeLogLevel(level)
	mode := c.profile.GetMode()
	if mode == "cp" && c.cpNode != nil {
		cmd := replication.Command{Type: replication.CmdSetLogLevel, LogLevel: level}
		if err := c.cpNode.Apply(cmd, 5*time.Second); err != nil {
			return err
		}
	}
	c.profile.SetLogLevel(level)
	c.profile.Save()
	applyRuntimeLogLevel(level)
	c.syncSettingsToPeers(map[string]string{"log_level": level})
	return nil
}

func (c *controller) SetEventRetentionDays(days int) error {
	if err := c.ensureClusterWritable(); err != nil {
		return err
	}
	if days <= 0 {
		return errors.New("invalid event retention days")
	}
	mode := c.profile.GetMode()
	if mode == "cp" && c.cpNode != nil {
		cmd := replication.Command{Type: replication.CmdSetEventRetentionDays, IntValue: days}
		if err := c.cpNode.Apply(cmd, 5*time.Second); err != nil {
			return err
		}
	}
	c.profile.SetEventRetentionDays(days)
	c.profile.Save()
	if c.events != nil {
		c.events.Cleanup(days)
	}
	c.syncSettingsToPeers(map[string]interface{}{"event_retention_days": days})
	return nil
}

func (c *controller) GetEventRetentionDays() int {
	return c.profile.GetEventRetentionDays()
}

func (c *controller) SetLogRetentionDays(days int) error {
	if err := c.ensureClusterWritable(); err != nil {
		return err
	}
	if days <= 0 {
		return errors.New("invalid log retention days")
	}
	if c.profile.GetMode() == "cp" && c.cpNode != nil {
		cmd := replication.Command{Type: replication.CmdSetLogRetentionDays, IntValue: days}
		if err := c.cpNode.Apply(cmd, 5*time.Second); err != nil {
			return err
		}
	}
	c.profile.SetLogRetentionDays(days)
	c.profile.Save()
	c.syncSettingsToPeers(map[string]interface{}{"log_retention_days": days})
	return nil
}

func (c *controller) GetLogRetentionDays() int {
	return c.profile.GetLogRetentionDays()
}

func (c *controller) SetEventTypes(types []string) error {
	if err := c.ensureClusterWritable(); err != nil {
		return err
	}
	normalized := catalog.NormalizeEventTypes(types)
	if types == nil {
		normalized = nil
	}
	if c.profile.GetMode() == "cp" && c.cpNode != nil {
		cmd := replication.Command{Type: replication.CmdSetEventTypes, StringList: normalized}
		if err := c.cpNode.Apply(cmd, 5*time.Second); err != nil {
			return err
		}
	}
	c.profile.SetEventTypes(normalized)
	c.profile.Save()
	c.syncSettingsToPeers(map[string]interface{}{"event_types": normalized})
	return nil
}

func (c *controller) GetEventTypes() []string {
	return c.profile.GetEventTypes()
}

func (c *controller) SetHeartbeatMaxFailures(n int) error {
	if err := c.ensureClusterWritable(); err != nil {
		return err
	}
	if n <= 0 {
		return errors.New("invalid heartbeat max failures")
	}
	if c.profile.GetMode() == "cp" && c.cpNode != nil {
		cmd := replication.Command{Type: replication.CmdSetHeartbeatMaxFailures, IntValue: n}
		if err := c.cpNode.Apply(cmd, 5*time.Second); err != nil {
			return err
		}
	}
	c.profile.SetHeartbeatMaxFailures(n)
	c.profile.Save()
	c.syncSettingsToPeers(map[string]interface{}{"heartbeat_max_failures": n})
	return nil
}

func (c *controller) GetHeartbeatMaxFailures() int {
	return c.profile.GetHeartbeatMaxFailures()
}

func (c *controller) SetInstanceRemovalDelaySeconds(n int) error {
	if err := c.ensureClusterWritable(); err != nil {
		return err
	}
	if n <= 0 {
		return errors.New("invalid instance removal delay seconds")
	}
	if c.profile.GetMode() == "cp" && c.cpNode != nil {
		cmd := replication.Command{Type: replication.CmdSetInstanceRemovalDelaySeconds, IntValue: n}
		if err := c.cpNode.Apply(cmd, 5*time.Second); err != nil {
			return err
		}
	}
	c.profile.SetInstanceRemovalDelaySeconds(n)
	c.profile.Save()
	c.syncSettingsToPeers(map[string]interface{}{"instance_removal_delay_seconds": n})
	return nil
}

func (c *controller) GetInstanceRemovalDelaySeconds() int {
	return c.profile.GetInstanceRemovalDelaySeconds()
}

func (c *controller) GetMode() string {
	return c.profile.GetMode()
}

func (c *controller) GetLogLevel() string {
	return c.profile.GetLogLevel()
}

func (c *controller) GetEnvironment() string {
	return c.profile.GetEnvironment()
}

func (c *controller) GetSeeds() []string {
	return c.profile.GetSeeds()
}

func (c *controller) SaveSeedsLocal(seeds []string) {
	c.profile.SetSeeds(seeds)
	c.profile.Save()
	if c.apNode != nil {
		c.apNode.SyncSeeds()
	}
}

func (c *controller) SaveUserLocal(u *auth.User) {
	c.auth.AddUser(u)
	c.auth.Save()
}

func (c *controller) DeleteUserLocal(username string) {
	c.auth.DeleteUser(username)
	c.auth.Save()
}

func (c *controller) SaveAPIKeyLocal(key *auth.APIKey) {
	c.auth.AddAPIKey(key)
	c.auth.Save()
}

func (c *controller) DeleteAPIKeyLocal(key string) {
	c.auth.DeleteAPIKey(key)
	c.auth.Save()
}

func (c *controller) SaveSettingLocal(key, value string) {
}

func (c *controller) GetSystemSettings() *SystemSettings {
	topology, consistency := normalizeRuntimeSelection(c.profile.GetEnvironment(), c.profile.GetMode())
	return &SystemSettings{
		Mode:                        topology,
		Consistency:                 consistency,
		LogLevel:                    c.profile.GetLogLevel(),
		EventRetentionDays:          c.profile.GetEventRetentionDays(),
		LogRetentionDays:            c.profile.GetLogRetentionDays(),
		EventTypes:                  c.profile.GetEventTypes(),
		HeartbeatMaxFailures:        c.profile.GetHeartbeatMaxFailures(),
		InstanceRemovalDelaySeconds: c.profile.GetInstanceRemovalDelaySeconds(),
		APIKeyAuthEnabled:           c.profile.GetAPIKeyAuthEnabled(),
	}
}

func (c *controller) ApplySystemSettings(systemSettings *SystemSettings) (*ApplySystemSettingsResult, error) {
	if systemSettings == nil {
		return nil, errors.New("settings required")
	}

	target := *systemSettings
	if target.Mode == "" {
		target.Mode = c.profile.GetEnvironment()
	}
	if target.Consistency == "" {
		target.Consistency = c.profile.GetMode()
	}
	target.Mode, target.Consistency = normalizeRuntimeSelection(target.Mode, target.Consistency)
	target.LogLevel = NormalizeLogLevel(target.LogLevel)
	if target.EventRetentionDays <= 0 {
		target.EventRetentionDays = c.profile.GetEventRetentionDays()
	}
	if target.LogRetentionDays <= 0 {
		target.LogRetentionDays = c.profile.GetLogRetentionDays()
	}
	if target.EventTypes == nil {
		target.EventTypes = c.profile.GetEventTypes()
	} else {
		target.EventTypes = catalog.NormalizeEventTypes(target.EventTypes)
	}
	if target.HeartbeatMaxFailures <= 0 {
		target.HeartbeatMaxFailures = c.profile.GetHeartbeatMaxFailures()
	}
	if target.InstanceRemovalDelaySeconds <= 0 {
		target.InstanceRemovalDelaySeconds = c.profile.GetInstanceRemovalDelaySeconds()
	}
	restartRequired, restartMessage := c.evaluateRestartRequirement(&target)

	if err := c.SetEnvironment(target.Mode); err != nil {
		return nil, err
	}
	if err := c.SetMode(target.Consistency); err != nil {
		return nil, err
	}
	if err := c.SetEventRetentionDays(target.EventRetentionDays); err != nil {
		return nil, err
	}
	if err := c.SetEventTypes(target.EventTypes); err != nil {
		return nil, err
	}
	if err := c.SetLogLevel(target.LogLevel); err != nil {
		return nil, err
	}
	if err := c.SetLogRetentionDays(target.LogRetentionDays); err != nil {
		return nil, err
	}
	if err := c.SetHeartbeatMaxFailures(target.HeartbeatMaxFailures); err != nil {
		return nil, err
	}
	if err := c.SetInstanceRemovalDelaySeconds(target.InstanceRemovalDelaySeconds); err != nil {
		return nil, err
	}
	if err := c.SetAPIKeyAuthEnabled(target.APIKeyAuthEnabled); err != nil {
		return nil, err
	}
	return &ApplySystemSettingsResult{
		Status:          "ok",
		RestartRequired: restartRequired,
		Message:         restartMessage,
	}, nil
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

func coerceBool(value interface{}) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	default:
		return false, false
	}
}

func (c *controller) SaveSettingLocalV2(key string, value interface{}) {
	switch key {
	case "consistency":
		if v, ok := value.(string); ok {
			c.profile.SetMode(strings.ToLower(v))
		}
	case "mode":
		if v, ok := value.(string); ok {
			value := strings.ToLower(v)
			if value == "standalone" || value == "cluster" {
				c.profile.SetEnvironment(value)
				if value == "standalone" {
					c.profile.SetMode("ap")
				}
			} else {
				c.profile.SetMode(value)
			}
		}
	case "environment":
		if v, ok := value.(string); ok {
			environment := strings.ToLower(v)
			c.profile.SetEnvironment(environment)
			if environment == "standalone" {
				c.profile.SetMode("ap")
			}
		}
	case "log_level":
		if v, ok := value.(string); ok {
			level := NormalizeLogLevel(v)
			c.profile.SetLogLevel(level)
			applyRuntimeLogLevel(level)
		}
	case "event_retention_days":
		if v, ok := coerceInt(value); ok {
			c.profile.SetEventRetentionDays(v)
			if c.events != nil {
				c.events.Cleanup(v)
			}
		}
	case "log_retention_days":
		if v, ok := coerceInt(value); ok {
			c.profile.SetLogRetentionDays(v)
		}
	case "event_types":
		if v, ok := coerceStringSlice(value); ok {
			c.profile.SetEventTypes(v)
		}
	case "heartbeat_max_failures":
		if v, ok := coerceInt(value); ok {
			c.profile.SetHeartbeatMaxFailures(v)
		}
	case "instance_removal_delay_seconds":
		if v, ok := coerceInt(value); ok {
			c.profile.SetInstanceRemovalDelaySeconds(v)
		}
	case "api_key_auth_enabled":
		if v, ok := coerceBool(value); ok {
			c.profile.SetAPIKeyAuthEnabled(v)
		}
	}
	c.profile.Save()
}

func (c *controller) SetAPIKeyAuthEnabled(enabled bool) error {
	if err := c.ensureClusterWritable(); err != nil {
		return err
	}
	if c.profile.GetMode() == "cp" && c.cpNode != nil {
		cmd := replication.Command{Type: replication.CmdSetAPIKeyAuthEnabled, BoolValue: enabled}
		if err := c.cpNode.Apply(cmd, 5*time.Second); err != nil {
			return err
		}
	}
	c.profile.SetAPIKeyAuthEnabled(enabled)
	c.profile.Save()
	c.syncSettingsToPeers(map[string]interface{}{"api_key_auth_enabled": enabled})
	return nil
}

func (c *controller) SetSeeds(seeds []string) error {
	c.profile.SetSeeds(seeds)
	c.profile.Save()
	if c.apNode != nil {
		c.apNode.SyncSeeds()
	}

	if c.apNode == nil {
		return nil
	}

	config := c.apNode.GetConfig()
	selfHTTPAddr := config.HTTPAddr
	if strings.HasPrefix(selfHTTPAddr, ":") {
		selfHTTPAddr = "http://127.0.0.1" + selfHTTPAddr
	} else if !strings.HasPrefix(selfHTTPAddr, "http") {
		selfHTTPAddr = "http://" + selfHTTPAddr
	}

	allNodes := make([]string, 0, len(seeds)+1)
	allNodes = append(allNodes, selfHTTPAddr)
	allNodes = append(allNodes, seeds...)

	for _, peer := range seeds {
		peerSeeds := make([]string, 0, len(allNodes)-1)
		for _, n := range allNodes {
			if n != peer {
				peerSeeds = append(peerSeeds, n)
			}
		}
		go c.syncSeedsToPeerHTTP(peer, peerSeeds)
		go c.syncSettingsToPeerHTTP(peer, map[string]string{
			"mode":        "cluster",
			"consistency": c.profile.GetMode(),
		})
	}
	return nil
}

func (c *controller) syncSettingsToPeerHTTP(peerAddr string, settings map[string]string) {
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

func (c *controller) syncSeedsToPeerHTTP(peerAddr string, seeds []string) {
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

func (c *controller) syncSettingsToPeers(settings interface{}) {
	if c.apNode == nil {
		return
	}
	seeds := c.profile.GetSeeds()
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

package service

import (
	"bytes"
	"encoding/json"
	"errors"
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
	if mode != "ap" && mode != "cp" {
		return errors.New("invalid mode")
	}
	currentMode := s.store.GetMode()
	if currentMode == "cp" && s.cpNode != nil {
		cmd := cp.Command{Type: cp.CmdSetMode, Mode: mode}
		return s.cpNode.Apply(cmd, 5*time.Second)
	}
	s.store.SetMode(mode)
	// Sync to peers via HTTP
	s.syncSettingsToPeers(map[string]string{"mode": mode})
	return nil
}

func (s *settingsService) SetEnvironment(env string) error {
	if env != "standalone" && env != "cluster" {
		return errors.New("invalid environment")
	}
	mode := s.store.GetMode()
	if mode == "cp" && s.cpNode != nil {
		cmd := cp.Command{Type: cp.CmdSetEnv, Environment: env}
		return s.cpNode.Apply(cmd, 5*time.Second)
	}
	s.store.SetEnvironment(env)
	// Sync to peers via HTTP
	s.syncSettingsToPeers(map[string]string{"environment": env})
	return nil
}

func (s *settingsService) SetLogLevel(level string) error {
	mode := s.store.GetMode()
	if mode == "cp" && s.cpNode != nil {
		cmd := cp.Command{Type: cp.CmdSetLogLevel, LogLevel: level}
		return s.cpNode.Apply(cmd, 5*time.Second)
	}
	s.store.SetLogLevel(level)
	// Sync to peers via HTTP
	s.syncSettingsToPeers(map[string]string{"log_level": level})
	return nil
}

func (s *settingsService) GetMode() string {
	return s.store.GetMode()
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
	switch key {
	case "mode":
		s.store.SetMode(value)
	case "environment":
		s.store.SetEnvironment(value)
	case "log_level":
		s.store.SetLogLevel(value)
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
	}
	return nil
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

func (s *settingsService) syncSettingsToPeers(settings map[string]string) {
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

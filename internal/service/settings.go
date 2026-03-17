package service

import (
	"context"
	"errors"
	"time"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/cluster"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/cluster/cp"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/store"
	pb "github.com/shiyindaxiaojie/eden-go-registry/api/proto/cluster/v1"
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
	if s.apNode != nil {
		return s.apNode.Apply("set_mode", mode, false)
	}
	s.store.SetMode(mode)
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
	return nil
}

func (s *settingsService) SetLogLevel(level string) error {
	mode := s.store.GetMode()
	if mode == "cp" && s.cpNode != nil {
		cmd := cp.Command{Type: cp.CmdSetLogLevel, LogLevel: level}
		return s.cpNode.Apply(cmd, 5*time.Second)
	}
	if s.apNode != nil {
		return s.apNode.Apply("set_log_level", level, false)
	}
	s.store.SetLogLevel(level)
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

func (s *settingsService) SetSeeds(seeds []string) error {
	s.store.SetSeeds(seeds)
	if s.apNode != nil {
		s.apNode.SyncSeeds()
		config := s.apNode.GetConfig()
		selfAddr := config.GRPCAddr
		pm, ok := s.apNode.GetPM().(*cluster.PeerManager)
		if ok {
			allNodes := make([]string, 0, len(seeds)+1)
			allNodes = append(allNodes, selfAddr)
			allNodes = append(allNodes, seeds...)

			for _, peer := range seeds {
				peerSeeds := make([]string, 0, len(allNodes)-1)
				for _, n := range allNodes {
					if n != peer {
						peerSeeds = append(peerSeeds, n)
					}
				}
				go s.syncSeedsToPeerGRPC(pm, peer, peerSeeds)
			}
		}
	}
	return nil
}

func (s *settingsService) syncSeedsToPeerGRPC(pm *cluster.PeerManager, peerAddr string, seeds []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pm.Range(func(p *cluster.Peer) bool {
		if p.Addr == peerAddr {
			if client, err := p.GetClient(); err == nil {
				client.SyncSeeds(ctx, &pb.SyncSeedsRequest{Seeds: seeds})
				return false
			}
		}
		return true
	})
}

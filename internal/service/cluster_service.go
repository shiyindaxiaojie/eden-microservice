package service

import (
	"errors"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/store"
)

type clusterService struct {
	store  *store.Registry
	cpNode CPNode
	apNode APNode
}

func NewClusterService(s *store.Registry, cp CPNode, ap APNode) ClusterService {
	return &clusterService{
		store:  s,
		cpNode: cp,
		apNode: ap,
	}
}

func (s *clusterService) JoinCluster(nodeID, addr string) error {
	if s.cpNode == nil {
		return errors.New("CP node not initialized")
	}
	return s.cpNode.Join(nodeID, addr)
}

func (s *clusterService) GetMembers() (interface{}, error) {
	if s.cpNode == nil {
		return nil, errors.New("CP node not initialized")
	}
	return s.cpNode.Members()
}

func (s *clusterService) RemoveMember(nodeID string) error {
	if s.cpNode == nil {
		return errors.New("CP node not initialized")
	}
	return s.cpNode.RemoveServer(nodeID)
}

func (s *clusterService) ListEvents() ([]*model.Event, error) {
	return s.store.ListEvents(), nil
}

func (s *clusterService) GetStats() (*store.Stats, error) {
	stats := s.store.Stats()
	return &stats, nil
}

func (s *clusterService) IsLeader() bool {
	if s.cpNode == nil {
		return true
	}
	return s.cpNode.IsLeader()
}

func (s *clusterService) LeaderAddr() string {
	if s.cpNode == nil {
		return ""
	}
	return s.cpNode.LeaderAddr()
}

func (s *clusterService) ReplicateData(cmdType string, payload []byte) {
	// Implementation for replication if needed in future
}

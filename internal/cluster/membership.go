package cluster

import (
	"errors"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/catalog"
)

// ClusterMember represents a node in the Raft cluster.
type ClusterMember struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Role    string `json:"role"`   // "Leader", "Follower", "Candidate"
	Status  string `json:"status"` // "Online", "Offline"
}

type Membership interface {
	JoinCluster(nodeID, addr string) error
	GetMembers() (interface{}, error)
	RemoveMember(nodeID string) error
	ListEvents() ([]*catalog.Event, error)
	QueryEvents(count, offset int, query, date, startTime, endTime, eventType, service string) ([]*catalog.Event, int, error)
	GetStats() (*catalog.Stats, error)
	IsLeader() bool
	LeaderAddr() string
	ReplicateData(cmdType string, payload []byte)
}

type ConsensusNode interface {
	Join(nodeID, addr string) error
	Members() (interface{}, error)
	RemoveServer(nodeID string) error
	IsLeader() bool
	LeaderAddr() string
}

type membership struct {
	catalog *catalog.State
	cpNode  ConsensusNode
}

func NewMembership(catalogState *catalog.State, cp ConsensusNode) Membership {
	return &membership{
		catalog: catalogState,
		cpNode:  cp,
	}
}

func (m *membership) JoinCluster(nodeID, addr string) error {
	if m.cpNode == nil {
		return errors.New("CP node not initialized")
	}
	return m.cpNode.Join(nodeID, addr)
}

func (m *membership) GetMembers() (interface{}, error) {
	if m.cpNode == nil {
		return nil, errors.New("CP node not initialized")
	}
	return m.cpNode.Members()
}

func (m *membership) RemoveMember(nodeID string) error {
	if m.cpNode == nil {
		return errors.New("CP node not initialized")
	}
	return m.cpNode.RemoveServer(nodeID)
}

func (m *membership) ListEvents() ([]*catalog.Event, error) {
	return m.catalog.ListEvents(), nil
}

func (m *membership) QueryEvents(count, offset int, query, date, startTime, endTime, eventType, service string) ([]*catalog.Event, int, error) {
	events, total := m.catalog.QueryEvents(count, offset, query, date, startTime, endTime, eventType, service)
	return events, total, nil
}

func (m *membership) GetStats() (*catalog.Stats, error) {
	stats := m.catalog.Stats()
	return &stats, nil
}

func (m *membership) IsLeader() bool {
	if m.cpNode == nil {
		return true
	}
	return m.cpNode.IsLeader()
}

func (m *membership) LeaderAddr() string {
	if m.cpNode == nil {
		return ""
	}
	return m.cpNode.LeaderAddr()
}

func (m *membership) ReplicateData(cmdType string, payload []byte) {
}

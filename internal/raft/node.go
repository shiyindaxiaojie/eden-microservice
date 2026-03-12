package raft

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	hraft "github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/store"
)

// Config holds Raft cluster configuration.
type Config struct {
	NodeID   string
	BindAddr string // Raft transport bind address, e.g. "127.0.0.1:7000"
	DataDir  string // directory for Raft logs and snapshots
	// Bootstrap as a single-node cluster if true (first node).
	Bootstrap bool
}

// Node wraps a hashicorp/raft instance and the registry.
type Node struct {
	Raft     *hraft.Raft
	fsm      *FSM
	Registry *store.Registry
	config   Config
}

// NewNode creates and starts a Raft node.
func NewNode(cfg Config, registry *store.Registry) (*Node, error) {
	fsm := NewFSM(registry)

	raftConfig := hraft.DefaultConfig()
	raftConfig.LocalID = hraft.ServerID(cfg.NodeID)
	raftConfig.SnapshotThreshold = 1024
	raftConfig.SnapshotInterval = 30 * time.Second

	// Transport
	addr, err := net.ResolveTCPAddr("tcp", cfg.BindAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve addr: %w", err)
	}
	transport, err := hraft.NewTCPTransport(cfg.BindAddr, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("tcp transport: %w", err)
	}

	// Log store and stable store
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir data dir: %w", err)
	}
	boltStore, err := raftboltdb.NewBoltStore(filepath.Join(cfg.DataDir, "raft.db"))
	if err != nil {
		return nil, fmt.Errorf("bolt store: %w", err)
	}

	// Snapshot store
	snapshotStore, err := hraft.NewFileSnapshotStore(cfg.DataDir, 2, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("snapshot store: %w", err)
	}

	r, err := hraft.NewRaft(raftConfig, fsm, boltStore, boltStore, snapshotStore, transport)
	if err != nil {
		return nil, fmt.Errorf("new raft: %w", err)
	}

	if cfg.Bootstrap {
		configuration := hraft.Configuration{
			Servers: []hraft.Server{
				{
					ID:      hraft.ServerID(cfg.NodeID),
					Address: hraft.ServerAddress(cfg.BindAddr),
				},
			},
		}
		f := r.BootstrapCluster(configuration)
		if f.Error() != nil && f.Error() != hraft.ErrCantBootstrap {
			log.Printf("[Raft] bootstrap warning: %v", f.Error())
		}
	}

	return &Node{
		Raft:     r,
		fsm:      fsm,
		Registry: registry,
		config:   cfg,
	}, nil
}

// Apply submits a command through Raft consensus.
func (n *Node) Apply(cmd Command, timeout time.Duration) error {
	if n.Raft.State() != hraft.Leader {
		leader := n.Raft.Leader()
		if leader == "" {
			return fmt.Errorf("no leader available")
		}
		return fmt.Errorf("not leader, redirect to %s", leader)
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("marshal command: %w", err)
	}

	f := n.Raft.Apply(data, timeout)
	if f.Error() != nil {
		return f.Error()
	}
	// Check for application-level error
	if resp, ok := f.Response().(error); ok {
		return resp
	}
	return nil
}

// Join adds a new voter to the cluster.
func (n *Node) Join(nodeID, addr string) error {
	if n.Raft.State() != hraft.Leader {
		return fmt.Errorf("not leader")
	}

	configFuture := n.Raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return err
	}

	for _, srv := range configFuture.Configuration().Servers {
		if srv.ID == hraft.ServerID(nodeID) && srv.Address == hraft.ServerAddress(addr) {
			// Already joined
			return nil
		}
		if srv.ID == hraft.ServerID(nodeID) || srv.Address == hraft.ServerAddress(addr) {
			// Remove the old configuration first
			removeFuture := n.Raft.RemoveServer(srv.ID, 0, 0)
			if removeFuture.Error() != nil {
				return fmt.Errorf("remove existing server: %w", removeFuture.Error())
			}
		}
	}

	f := n.Raft.AddVoter(hraft.ServerID(nodeID), hraft.ServerAddress(addr), 0, 0)
	if f.Error() != nil {
		return f.Error()
	}
	return nil
}

// ClusterMember represents a node in the Raft cluster.
type ClusterMember struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Role    string `json:"role"` // "Leader", "Follower", "Candidate"
}

// Members returns the current cluster membership.
func (n *Node) Members() ([]ClusterMember, error) {
	configFuture := n.Raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return nil, err
	}

	leaderAddr, _ := n.Raft.LeaderWithID()
	var members []ClusterMember
	for _, srv := range configFuture.Configuration().Servers {
		role := "Follower"
		if srv.Address == leaderAddr {
			role = "Leader"
		}
		members = append(members, ClusterMember{
			ID:      string(srv.ID),
			Address: string(srv.Address),
			Role:    role,
		})
	}
	return members, nil
}

// IsLeader returns whether this node is the Raft leader.
func (n *Node) IsLeader() bool {
	return n.Raft.State() == hraft.Leader
}

// LeaderAddr returns the current leader address.
func (n *Node) LeaderAddr() string {
	addr, _ := n.Raft.LeaderWithID()
	return string(addr)
}

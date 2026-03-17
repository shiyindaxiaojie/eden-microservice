package cluster

import (
	"context"
	"sync"
	"time"

	"github.com/shiyindaxiaojie/eden-go-logger"
	pb "github.com/shiyindaxiaojie/eden-go-registry/api/proto/cluster/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Peer represents a remote node in the cluster.
type Peer struct {
	Addr       string
	conn       *grpc.ClientConn
	client     pb.ClusterServiceClient
	mu         sync.RWMutex
	lastSeen   time.Time
}

// PeerManager manages gRPC connections to cluster peers.
type PeerManager struct {
	mu    sync.RWMutex
	peers map[string]*Peer
}

// NewPeerManager creates a new peer manager.
func NewPeerManager() *PeerManager {
	return &PeerManager{
		peers: make(map[string]*Peer),
	}
}

// UpdatePeers synchronizes the peer list.
func (pm *PeerManager) UpdatePeers(addrs []string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	newPeers := make(map[string]*Peer)
	for _, addr := range addrs {
		if addr == "" {
			continue
		}
		if p, ok := pm.peers[addr]; ok {
			newPeers[addr] = p
		} else {
			newPeers[addr] = &Peer{Addr: addr}
		}
	}

	// Close connections for removed peers
	for addr, p := range pm.peers {
		if _, ok := newPeers[addr]; !ok {
			if p.conn != nil {
				p.conn.Close()
			}
		}
	}

	pm.peers = newPeers
}

// GetClient returns a ClusterServiceClient for the given address.
// It lazily creates the connection if needed.
func (p *Peer) GetClient() (pb.ClusterServiceClient, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.client != nil {
		return p.client, nil
	}

	conn, err := grpc.Dial(p.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	p.conn = conn
	p.client = pb.NewClusterServiceClient(conn)
	return p.client, nil
}

// Range iterates over all peers.
func (pm *PeerManager) Range(f func(p *Peer) bool) {
	pm.mu.RLock()
	peers := make([]*Peer, 0, len(pm.peers))
	for _, p := range pm.peers {
		peers = append(peers, p)
	}
	pm.mu.RUnlock()

	for _, p := range peers {
		if !f(p) {
			break
		}
	}
}

// Broadcast calls the given function for all peers in parallel.
func (pm *PeerManager) Broadcast(ctx context.Context, f func(ctx context.Context, client pb.ClusterServiceClient) error) {
	pm.Range(func(p *Peer) bool {
		go func(p *Peer) {
			client, err := p.GetClient()
			if err != nil {
				logger.Error("[PeerManager] failed to get client for %s: %v", p.Addr, err)
				return
			}
			
			if err := f(ctx, client); err != nil {
				logger.Warn("[PeerManager] broadcast to %s failed: %v", p.Addr, err)
			}
		}(p)
		return true
	})
}

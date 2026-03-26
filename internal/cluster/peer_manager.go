package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/shiyindaxiaojie/eden-go-logger"
	pb "github.com/shiyindaxiaojie/eden-go-registry/api/proto/cluster/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Peer represents a remote node in the cluster.
type Peer struct {
	ID       string // HTTP address (identifier)
	grpcAddr string // Resolved gRPC address
	conn     *grpc.ClientConn
	client   pb.ClusterServiceClient
	mu       sync.RWMutex
	lastSeen time.Time
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
			newPeers[addr] = &Peer{ID: addr}
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
	p.mu.RLock()
	c := p.client
	p.mu.RUnlock()
	if c != nil {
		return c, nil
	}

	// Resolve gRPC address outside any lock (involves network IO)
	if err := p.Resolve(); err != nil {
		return nil, err
	}

	// Now create connection under lock
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double check after acquiring lock
	if p.client != nil {
		return p.client, nil
	}

	addr := p.grpcAddr
	if addr == "" {
		return nil, fmt.Errorf("could not resolve gRPC address for %s", p.ID)
	}

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	p.conn = conn
	p.client = pb.NewClusterServiceClient(conn)
	return p.client, nil
}

// GetConn returns the underlying gRPC connection for multiplexing other services.
func (p *Peer) GetConn() (*grpc.ClientConn, error) {
	if _, err := p.GetClient(); err != nil {
		return nil, err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.conn, nil
}

// resolveGRPCAddr fetches gRPC address from the peer's HTTP endpoint.
// This performs network IO and must NOT be called with any lock held.
func (p *Peer) resolveGRPCAddr() (string, error) {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(p.ID + "/v1/node/info")
	if err != nil {
		return "", fmt.Errorf("failed to call info for %s: %w", p.ID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to call info for %s: status %d", p.ID, resp.StatusCode)
	}

	var info struct {
		GRPCAddr string `json:"grpc_addr"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", fmt.Errorf("failed to decode info for %s: %w", p.ID, err)
	}

	addr := info.GRPCAddr
	// Normalize if needed (if it's just :port)
	if strings.Contains(addr, ":") {
		host, port, _ := net.SplitHostPort(strings.TrimPrefix(addr, ":"))
		if host == "" {
			// Extract host from ID
			idHost := strings.TrimPrefix(strings.TrimPrefix(p.ID, "http://"), "https://")
			if h, _, err := net.SplitHostPort(idHost); err == nil {
				idHost = h
			}
			if port == "" {
				port = strings.TrimPrefix(addr, ":")
			}
			addr = idHost + ":" + port
		}
	}
	return addr, nil
}

// Resolve fetches node info to get the gRPC address.
func (p *Peer) Resolve() error {
	p.mu.RLock()
	alreadyResolved := p.grpcAddr != ""
	p.mu.RUnlock()
	if alreadyResolved {
		return nil
	}

	addr, err := p.resolveGRPCAddr()
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.grpcAddr == "" {
		p.grpcAddr = addr
	}
	return nil
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
// Fire-and-forget replication for AP mode to ensure the service stays responsive.
func (pm *PeerManager) Broadcast(ctx context.Context, f func(ctx context.Context, client pb.ClusterServiceClient) error) {
	pm.Range(func(p *Peer) bool {
		// Do not wait for peers, do not block the caller!
		go func(p *Peer) {
			// Detach from the caller's context to avoid "context cancelled" on early return.
			broadcastCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			client, err := p.GetClient()
			if err != nil {
				// Tracking failures is good for debugging sync issues
				logger.Debug("[PeerManager] skip broadcast to %s: could not get client: %v", p.ID, err)
				return
			}

			// Best-effort replication
			if err := f(broadcastCtx, client); err != nil {
				logger.Warn("[PeerManager] replication to %s failed: %v", p.ID, err)
			}
		}(p)
		return true
	})
}

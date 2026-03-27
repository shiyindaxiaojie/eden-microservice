package ap

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shiyindaxiaojie/eden-go-logger"
	pb "github.com/shiyindaxiaojie/eden-go-registry/api/proto/cluster/v1"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/auth"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/catalog"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/cluster"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/config"
	"google.golang.org/grpc"
)

// Node represents an AP mode node that broadcasts changes to seed peers.
type Node struct {
	Config *config.Config
	State  *cluster.RuntimeState
	PM     *cluster.PeerManager
}

type syncDiscoveryPayload struct {
	ServicesByNamespace map[string]map[string]map[string]*catalog.Instance `json:"services_by_namespace"`
	Namespaces          []*catalog.Namespace                               `json:"namespaces"`
	TopologyReports     map[string]map[string]*catalog.TopologyReport      `json:"topology_reports"`
}

// NewNode creates an AP mode node.
func NewNode(cfg *config.Config, runtimeState *cluster.RuntimeState) *Node {
	n := &Node{
		Config: cfg,
		State:  runtimeState,
		PM:     cluster.NewPeerManager(),
	}
	// Initial peers from seeds
	n.SyncSeeds()
	// Start background sync
	n.startAntiEntropy()
	return n
}

func (n *Node) startAntiEntropy() {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for range ticker.C {
			n.fullSync()
		}
	}()
}

func (n *Node) fullSync() {
	peers := make([]*cluster.Peer, 0)
	n.PM.Range(func(p *cluster.Peer) bool {
		peers = append(peers, p)
		return true
	})

	if len(peers) == 0 {
		return
	}

	// Pick a random peer
	target := peers[time.Now().UnixNano()%int64(len(peers))]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := target.GetClient()
	if err != nil {
		logger.Error("[AP Node] failed to get client for full sync with %s: %v", target.ID, err)
		return
	}

	resp, err := client.SyncDiscovery(ctx, &pb.SyncDiscoveryRequest{})
	if err != nil {
		logger.Warn("[AP Node] full sync with %s failed: %v", target.ID, err)
		return
	}

	var payload syncDiscoveryPayload
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		logger.Error("[AP Node] unmarshal error during full sync: %v", err)
		return
	}

	if len(payload.ServicesByNamespace) > 0 {
		n.State.Catalog.Instances.MergeNS(payload.ServicesByNamespace)
	}
	if len(payload.Namespaces) > 0 {
		n.State.Catalog.Namespaces.Restore(payload.Namespaces)
	}
	if len(payload.TopologyReports) > 0 {
		n.State.Catalog.Topology.Restore(payload.TopologyReports)
	}
	n.State.AppendEvent(catalog.EventTypeRegistryNodeSync, "Cluster", target.ID, "Full sync completed")
	logger.Info("[AP Node] Full sync completed with %s", target.ID)
}

// SyncSeeds updates the internal peer map from registry seeds.
func (n *Node) SyncSeeds() {
	seeds := n.State.GetSeeds()
	n.PM.UpdatePeers(seeds)
}

// GetConfig returns the node configuration.
func (n *Node) GetConfig() *config.Config {
	return n.Config
}

// GetPM returns the peer manager.
func (n *Node) GetPM() interface{} {
	return n.PM
}

func (n *Node) GetLeaderConn(leaderID string) (*grpc.ClientConn, error) {
	var conn *grpc.ClientConn
	var connErr error
	n.PM.Range(func(peer *cluster.Peer) bool {
		if peer.ID == leaderID {
			conn, connErr = peer.GetConn()
			return false
		}
		return true
	})
	if conn == nil {
		if connErr != nil {
			return nil, fmt.Errorf("failed to proxy to leader: %w", connErr)
		}
		return nil, fmt.Errorf("leader connection not found in peer manager for %s", leaderID)
	}
	return conn, nil
}

// Apply executes the command locally and broadcasts it.
// isReplicate flag prevents infinite broadcast loops.
func (n *Node) Apply(cmdType string, data interface{}, isReplicate bool) error {
	// Local execution
	switch cmdType {
	case "register":
		if inst, ok := data.(*catalog.Instance); ok {
			n.State.Register(inst)
		}
	case "deregister":
		if d, ok := data.(map[string]string); ok {
			n.State.Catalog.Instances.DeregisterNS(d["namespace"], d["service_name"], d["instance_id"])
		}
	case "heartbeat":
		if d, ok := data.(map[string]string); ok {
			inst, _ := n.State.HeartbeatNS(d["namespace"], d["service_name"], d["instance_id"])
			if inst == nil {
				return fmt.Errorf("instance not found")
			}
		}
	case "add_api_key":
		if k, ok := data.(*auth.APIKey); ok {
			n.State.AddAPIKey(k)
		}
	case "delete_api_key":
		if key, ok := data.(string); ok {
			n.State.DeleteAPIKey(key)
		}
	case "add_user":
		if u, ok := data.(*auth.User); ok {
			n.State.AddUser(u)
		}
	case "delete_user":
		if username, ok := data.(string); ok {
			n.State.DeleteUser(username)
		}
	case "set_mode":
		if mode, ok := data.(string); ok {
			n.State.SetMode(mode)
		}
	case "set_seeds":
		if seeds, ok := data.([]string); ok {
			n.State.SetSeeds(seeds)
			n.SyncSeeds()
		}
	case "set_log_level":
		if level, ok := data.(string); ok {
			n.State.SetLogLevel(level)
		}
	}

	// Broadcast to peers if not already a replicated request
	if !isReplicate {
		go n.broadcast(cmdType, data)
	}

	return nil
}

// broadcast sends the command to other seeds/peers via gRPC
func (n *Node) broadcast(cmdType string, data interface{}) {
	payload, err := json.Marshal(data)
	if err != nil {
		logger.Error("[AP Node] marshal error: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	n.PM.Broadcast(ctx, func(ctx context.Context, client pb.ClusterServiceClient) error {
		_, err := client.ReplicateLog(ctx, &pb.ReplicateLogRequest{
			CommandType: cmdType,
			Data:        payload,
		})
		return err
	})
}

package ap

import (
	"context"
	"encoding/json"
	"time"

	"github.com/shiyindaxiaojie/eden-go-logger"
	pb "github.com/shiyindaxiaojie/eden-go-registry/api/proto/cluster/v1"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/cluster"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/config"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/store"
)

// Node represents an AP mode node that broadcasts changes to seed peers.
type Node struct {
	Config   *config.Config
	Registry *store.Registry
	PM       *cluster.PeerManager
}

type syncDiscoveryPayload struct {
	ServicesByNamespace map[string]map[string]map[string]*model.Instance `json:"services_by_namespace"`
	Namespaces          []*model.Namespace                               `json:"namespaces"`
	TopologyReports     map[string]map[string]*model.TopologyReport      `json:"topology_reports"`
}

// NewNode creates an AP mode node.
func NewNode(cfg *config.Config, registry *store.Registry) *Node {
	n := &Node{
		Config:   cfg,
		Registry: registry,
		PM:       cluster.NewPeerManager(),
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
		n.Registry.Services.MergeNS(payload.ServicesByNamespace)
	}
	if len(payload.Namespaces) > 0 {
		n.Registry.Namespaces.Restore(payload.Namespaces)
	}
	if len(payload.TopologyReports) > 0 {
		n.Registry.Topology.Restore(payload.TopologyReports)
	}
	logger.Info("[AP Node] Full sync completed with %s", target.ID)
}

// SyncSeeds updates the internal peer map from registry seeds.
func (n *Node) SyncSeeds() {
	seeds := n.Registry.GetSeeds()
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

// Apply executes the command locally and broadcasts it.
// isReplicate flag prevents infinite broadcast loops.
func (n *Node) Apply(cmdType string, data interface{}, isReplicate bool) error {
	// Local execution
	switch cmdType {
	case "register":
		if inst, ok := data.(*model.Instance); ok {
			n.Registry.Register(inst)
		}
	case "deregister":
		if d, ok := data.(map[string]string); ok {
			n.Registry.Services.DeregisterNS(d["namespace"], d["service_name"], d["instance_id"])
		}
	case "heartbeat":
		if d, ok := data.(map[string]string); ok {
			n.Registry.HeartbeatNS(d["namespace"], d["service_name"], d["instance_id"])
		}
	case "add_api_key":
		if k, ok := data.(*model.APIKey); ok {
			n.Registry.AddAPIKey(k)
		}
	case "delete_api_key":
		if key, ok := data.(string); ok {
			n.Registry.DeleteAPIKey(key)
		}
	case "add_user":
		if u, ok := data.(*model.User); ok {
			n.Registry.AddUser(u)
		}
	case "delete_user":
		if username, ok := data.(string); ok {
			n.Registry.DeleteUser(username)
		}
	case "set_mode":
		if mode, ok := data.(string); ok {
			n.Registry.SetMode(mode)
		}
	case "set_seeds":
		if seeds, ok := data.([]string); ok {
			n.Registry.SetSeeds(seeds)
			n.SyncSeeds()
		}
	case "set_log_level":
		if level, ok := data.(string); ok {
			n.Registry.SetLogLevel(level)
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

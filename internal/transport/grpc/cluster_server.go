package grpcapi

import (
	"context"
	"encoding/json"

	"github.com/shiyindaxiaojie/eden-go-logger"
	pb "github.com/shiyindaxiaojie/eden-go-registry/api/proto/cluster/v1"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/auth"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/catalog"
	clusterpkg "github.com/shiyindaxiaojie/eden-go-registry/internal/cluster"
)

// Replicator represents a component that can apply replicated commands (e.g. ap.Node).
type Replicator interface {
	Apply(cmdType string, data interface{}, isReplicate bool) error
}

// ClusterServer implements the ClusterService gRPC interface.
type ClusterServer struct {
	pb.UnimplementedClusterServiceServer
	state      *clusterpkg.RuntimeState
	replicator Replicator
}

type syncDiscoveryPayload struct {
	ServicesByNamespace map[string]map[string]map[string]*catalog.Instance `json:"services_by_namespace"`
	Namespaces          []*catalog.Namespace                               `json:"namespaces"`
	TopologyReports     map[string]map[string]*catalog.TopologyReport      `json:"topology_reports"`
}

// NewClusterServer creates a new gRPC cluster server.
func NewClusterServer(runtimeState *clusterpkg.RuntimeState, replicator Replicator) *ClusterServer {
	return &ClusterServer{
		state:      runtimeState,
		replicator: replicator,
	}
}

func (s *ClusterServer) SyncSeeds(ctx context.Context, req *pb.SyncSeedsRequest) (*pb.SyncResponse, error) {
	logger.Info("[gRPC] Received seed sync: %v", req.Seeds)
	if s.replicator != nil {
		s.replicator.Apply("set_seeds", req.Seeds, true)
	} else {
		s.state.SetSeeds(req.Seeds)
	}
	return &pb.SyncResponse{Success: true}, nil
}

func (s *ClusterServer) SyncUser(ctx context.Context, req *pb.SyncUserRequest) (*pb.SyncResponse, error) {
	u := &auth.User{
		Username:  req.Username,
		Password:  req.Password,
		Nickname:  req.Nickname,
		Role:      req.Role,
		Remark:    req.Remark,
		IsBuiltIn: req.IsBuiltIn,
	}
	if s.replicator != nil {
		s.replicator.Apply("add_user", u, true)
	} else {
		s.state.AddUser(u)
	}
	return &pb.SyncResponse{Success: true}, nil
}

func (s *ClusterServer) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*pb.SyncResponse, error) {
	if s.replicator != nil {
		s.replicator.Apply("delete_user", req.Username, true)
	} else {
		s.state.DeleteUser(req.Username)
	}
	return &pb.SyncResponse{Success: true}, nil
}

func (s *ClusterServer) SyncAPIKey(ctx context.Context, req *pb.SyncAPIKeyRequest) (*pb.SyncResponse, error) {
	k := &auth.APIKey{
		Key:       req.Key,
		Label:     req.Label,
		CreatedAt: req.CreatedAt,
		ExpiresAt: req.ExpiresAt,
	}
	if s.replicator != nil {
		s.replicator.Apply("add_api_key", k, true)
	} else {
		s.state.AddAPIKey(k)
	}
	return &pb.SyncResponse{Success: true}, nil
}

func (s *ClusterServer) DeleteAPIKey(ctx context.Context, req *pb.DeleteAPIKeyRequest) (*pb.SyncResponse, error) {
	if s.replicator != nil {
		s.replicator.Apply("delete_api_key", req.Key, true)
	} else {
		s.state.DeleteAPIKey(req.Key)
	}
	return &pb.SyncResponse{Success: true}, nil
}

func (s *ClusterServer) SyncSettings(ctx context.Context, req *pb.SyncSettingsRequest) (*pb.SyncResponse, error) {
	if req.Mode != "" {
		if s.replicator != nil {
			s.replicator.Apply("set_mode", req.Mode, true)
		} else {
			s.state.SetMode(req.Mode)
		}
	}
	// Note: set_environment is not explicitly in ap.Node.Apply, using direct registry call if needed
	if req.Environment != "" {
		s.state.SetEnvironment(req.Environment)
	}
	if req.LogLevel != "" {
		s.state.SetLogLevel(req.LogLevel)
	}
	return &pb.SyncResponse{Success: true}, nil
}

func (s *ClusterServer) ReplicateLog(ctx context.Context, req *pb.ReplicateLogRequest) (*pb.SyncResponse, error) {
	if s.replicator == nil {
		return &pb.SyncResponse{Success: false, Message: "No replicator configured"}, nil
	}

	logger.Info("[gRPC] Received replication log: type=%s", req.CommandType)

	var data interface{}
	switch req.CommandType {
	case "register":
		var inst catalog.Instance
		if err := json.Unmarshal(req.Data, &inst); err != nil {
			return nil, err
		}
		data = &inst
	case "deregister", "heartbeat":
		var d map[string]string
		if err := json.Unmarshal(req.Data, &d); err != nil {
			return nil, err
		}
		data = d
	default:
		// Other types might have different structures, but many use JSON
		var d interface{}
		if err := json.Unmarshal(req.Data, &d); err != nil {
			return nil, err
		}
		data = d
	}

	if err := s.replicator.Apply(req.CommandType, data, true); err != nil {
		return &pb.SyncResponse{Success: false, Message: err.Error()}, nil
	}

	return &pb.SyncResponse{Success: true}, nil
}

func (s *ClusterServer) SyncDiscovery(ctx context.Context, req *pb.SyncDiscoveryRequest) (*pb.SyncDiscoveryResponse, error) {
	payload := syncDiscoveryPayload{
		ServicesByNamespace: s.state.Catalog.Instances.GetAllNS(),
		Namespaces:          s.state.Catalog.Namespaces.List(),
		TopologyReports:     s.state.Catalog.Topology.Snapshot(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &pb.SyncDiscoveryResponse{Data: data}, nil
}

func (s *ClusterServer) ForwardToLeader(ctx context.Context, req *pb.ForwardRequest) (*pb.ForwardResponse, error) {
	// Real implementation would forward the request to the leader node
	// This is used in CP mode when a follower receives a write request
	return &pb.ForwardResponse{Success: false, StatusCode: 501, Body: []byte("Not implemented")}, nil
}

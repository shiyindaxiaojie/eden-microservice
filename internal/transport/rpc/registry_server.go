package rpcapi

import (
	"context"
	"sync"

	logger "github.com/shiyindaxiaojie/eden-go-logger"
	pb "github.com/shiyindaxiaojie/eden-registry/api/proto/registry/v1"
	"github.com/shiyindaxiaojie/eden-registry/internal/catalog"
	"github.com/shiyindaxiaojie/eden-registry/internal/cluster"
	"github.com/shiyindaxiaojie/eden-registry/internal/config"
	"github.com/shiyindaxiaojie/eden-registry/internal/settings"
	"google.golang.org/grpc/metadata"
)

// RegistryServer implements the RegistryService gRPC interface.
type RegistryServer struct {
	pb.UnimplementedRegistryServiceServer
	config    *config.Config
	catalog   catalog.Registry
	settings  settings.Controller
	cluster   cluster.Membership
	nodeCache sync.Map // map[string]config.Config
}

// NewRegistryServer creates a new gRPC registry server.
func NewRegistryServer(cfg *config.Config, catalogRegistry catalog.Registry, settingsController settings.Controller, clusterMembership cluster.Membership) *RegistryServer {
	return &RegistryServer{
		config:   cfg,
		catalog:  catalogRegistry,
		settings: settingsController,
		cluster:  clusterMembership,
	}
}

func (s *RegistryServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	pbi := req.GetInstance()
	inst := &catalog.Instance{
		ID:          pbi.Id,
		ServiceName: pbi.ServiceName,
		Namespace:   pbi.Namespace,
		Host:        pbi.Host,
		Port:        int(pbi.Port),
		Weight:      int(pbi.Weight),
		Metadata:    pbi.Metadata,
		Datacenter:  pbi.Datacenter,
	}

	if err := s.catalog.Register(inst); err != nil {
		return nil, err
	}

	logger.Info("[gRPC] Registered service: %s (%s)", inst.ServiceName, inst.ID)
	return &pb.RegisterResponse{Success: true}, nil
}

func (s *RegistryServer) Deregister(ctx context.Context, req *pb.DeregisterRequest) (*pb.DeregisterResponse, error) {
	err := s.catalog.Deregister(req.Namespace, req.ServiceName, req.InstanceId)
	success := err == nil

	logger.Info("[gRPC] Deregistered instance: %s (%s) namespace=%s success=%v", req.ServiceName, req.InstanceId, req.Namespace, success)
	if err != nil {
		return &pb.DeregisterResponse{Success: false}, err
	}
	return &pb.DeregisterResponse{Success: true}, nil
}

func (s *RegistryServer) SetInstanceStatus(ctx context.Context, req *pb.SetInstanceStatusRequest) (*pb.SetInstanceStatusResponse, error) {
	err := s.catalog.SetInstanceStatus(req.Namespace, req.ServiceName, req.InstanceId, req.Status)
	success := err == nil

	logger.Info("[gRPC] Set instance status %s: %s (%s) namespace=%s success=%v", req.Status, req.ServiceName, req.InstanceId, req.Namespace, success)
	if err != nil {
		return &pb.SetInstanceStatusResponse{Success: false}, err
	}
	return &pb.SetInstanceStatusResponse{Success: true}, nil
}

func (s *RegistryServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	err := s.catalog.Heartbeat(req.Namespace, req.ServiceName, req.InstanceId)
	if err != nil {
		return &pb.HeartbeatResponse{Success: false}, err
	}
	return &pb.HeartbeatResponse{Success: true}, nil
}

func (s *RegistryServer) Discover(ctx context.Context, req *pb.DiscoverRequest) (*pb.DiscoverResponse, error) {
	if consumer := consumerServiceFromContext(ctx); consumer != "" {
		s.catalog.RecordDependency(req.Namespace, consumer, req.ServiceName)
	}

	instances, err := s.catalog.GetService(req.Namespace, req.ServiceName, req.HealthyOnly)
	if err != nil {
		return nil, err
	}

	return &pb.DiscoverResponse{Instances: toProtoInstances(instances)}, nil
}

func (s *RegistryServer) Watch(req *pb.WatchRequest, stream pb.RegistryService_WatchServer) error {
	ch := make(chan catalog.WatchEvent, 10)
	consumerService := consumerServiceFromContext(stream.Context())

	s.catalog.Subscribe(req.Namespace, req.ServiceName, consumerService, ch)
	defer s.catalog.Unsubscribe(req.Namespace, req.ServiceName, ch)

	initial, _ := s.catalog.GetService(req.Namespace, req.ServiceName, false)
	if err := stream.Send(&pb.WatchResponse{Action: "update", Instances: toProtoInstances(initial)}); err != nil {
		return err
	}

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case evt := <-ch:
			if err := stream.Send(&pb.WatchResponse{Action: evt.Action, Instances: toProtoInstances(evt.Instances)}); err != nil {
				return err
			}
		}
	}
}

func (s *RegistryServer) GetMembers(ctx context.Context, req *pb.GetMembersRequest) (*pb.GetMembersResponse, error) {
	members, err := cluster.BuildClusterMemberViews(s.config, s.settings, s.cluster, &s.nodeCache)
	if err != nil {
		return nil, err
	}

	resp := &pb.GetMembersResponse{Members: make([]*pb.ClusterMember, 0, len(members))}
	for _, member := range members {
		resp.Members = append(resp.Members, &pb.ClusterMember{
			Id:       member.ID,
			Address:  member.Address,
			Status:   member.Status,
			Role:     member.Role,
			IsLocal:  member.IsLocal,
			HttpAddr: member.HTTPAddr,
			GrpcAddr: member.GRPCAddr,
			QuicAddr: member.QUICAddr,
			RaftAddr: member.RaftAddr,
		})
	}
	return resp, nil
}

func (s *RegistryServer) ReportTopology(ctx context.Context, req *pb.ReportTopologyRequest) (*pb.ReportTopologyResponse, error) {
	changed := s.catalog.ReportTopology(req.Namespace, req.ConsumerService, req.Providers, req.Checksum)
	return &pb.ReportTopologyResponse{
		Success: true,
		Changed: changed,
	}, nil
}

func consumerServiceFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get("x-consumer-service")
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func toProtoInstances(instances []*catalog.Instance) []*pb.ServiceInstance {
	pbInstances := make([]*pb.ServiceInstance, 0, len(instances))
	for _, inst := range instances {
		pbInstances = append(pbInstances, &pb.ServiceInstance{
			Id:          inst.ID,
			ServiceName: inst.ServiceName,
			Namespace:   inst.Namespace,
			Host:        inst.Host,
			Port:        int32(inst.Port),
			Weight:      int32(inst.Weight),
			Status:      string(inst.Status),
			Datacenter:  inst.Datacenter,
			Metadata:    inst.Metadata,
		})
	}
	return pbInstances
}

package grpc

import (
	"context"

	"github.com/shiyindaxiaojie/eden-go-logger"
	pb "github.com/shiyindaxiaojie/eden-go-registry/api/proto/registry/v1"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/service"
)

// RegistryServer implements the RegistryService gRPC interface.
type RegistryServer struct {
	pb.UnimplementedRegistryServiceServer
	catalog service.CatalogService
}

// NewRegistryServer creates a new gRPC registry server.
func NewRegistryServer(catalog service.CatalogService) *RegistryServer {
	return &RegistryServer{
		catalog: catalog,
	}
}

func (s *RegistryServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	pbi := req.GetInstance()
	inst := &model.Instance{
		ID:          pbi.Id,
		ServiceName: pbi.ServiceName,
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
	err := s.catalog.Deregister(req.ServiceName, req.InstanceId)
	success := err == nil
	
	logger.Info("[gRPC] Deregistered service: %s (%s) success=%v", req.ServiceName, req.InstanceId, success)
	if err != nil {
		return &pb.DeregisterResponse{Success: false}, nil
	}
	return &pb.DeregisterResponse{Success: true}, nil
}

func (s *RegistryServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	err := s.catalog.Heartbeat(req.ServiceName, req.InstanceId)
	if err != nil {
		return &pb.HeartbeatResponse{Success: false}, nil
	}
	return &pb.HeartbeatResponse{Success: true}, nil
}

func (s *RegistryServer) Discover(ctx context.Context, req *pb.DiscoverRequest) (*pb.DiscoverResponse, error) {
	instances, err := s.catalog.GetService(req.ServiceName, req.HealthyOnly)
	if err != nil {
		return nil, err
	}
	
	return &pb.DiscoverResponse{Instances: toProtoInstances(instances)}, nil
}

func (s *RegistryServer) Watch(req *pb.WatchRequest, stream pb.RegistryService_WatchServer) error {
	ch := make(chan []*model.Instance, 10)
	
	s.catalog.Subscribe(req.ServiceName, ch)
	defer s.catalog.Unsubscribe(req.ServiceName, ch)

	// Send initial state
	initial, _ := s.catalog.GetService(req.ServiceName, false)
	if err := stream.Send(&pb.WatchResponse{Instances: toProtoInstances(initial)}); err != nil {
		return err
	}

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case insts := <-ch:
			if err := stream.Send(&pb.WatchResponse{Instances: toProtoInstances(insts)}); err != nil {
				return err
			}
		}
	}
}

func toProtoInstances(instances []*model.Instance) []*pb.ServiceInstance {
	pbInstances := make([]*pb.ServiceInstance, 0, len(instances))
	for _, inst := range instances {
		pbInstances = append(pbInstances, &pb.ServiceInstance{
			Id:          inst.ID,
			ServiceName: inst.ServiceName,
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

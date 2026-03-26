package grpcclient

import (
	"context"
	"testing"

	"github.com/shiyindaxiaojie/eden-go-registry/examples/service-discovery/custom/internal/registry"
	pb "github.com/shiyindaxiaojie/eden-go-registry/examples/service-discovery/custom/internal/registryv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestHeartbeatReRegistersMissingInstance(t *testing.T) {
	fake := &fakeRegistryServiceClient{
		heartbeatErr: status.Error(codes.NotFound, "instance not found"),
	}
	client := &Client{
		targets: []*grpcTarget{
			{client: fake},
		},
		namespace:  "default",
		datacenter: "dc1",
	}

	instance := &registry.ServiceInstance{
		ID:          "custom-grpc-auth-center-1",
		ServiceName: "custom-grpc-auth-center",
		Host:        "127.0.0.1",
		Port:        24002,
		Weight:      100,
		Metadata:    map[string]string{"version": "v1"},
	}

	if err := client.Heartbeat(instance); err != nil {
		t.Fatalf("Heartbeat() error = %v", err)
	}
	if fake.heartbeatCalls != 1 {
		t.Fatalf("Heartbeat() calls = %d, want 1", fake.heartbeatCalls)
	}
	if fake.registerCalls != 1 {
		t.Fatalf("Register() calls = %d, want 1", fake.registerCalls)
	}
	if fake.lastRegister == nil || fake.lastRegister.Instance == nil {
		t.Fatal("Register() request was not captured")
	}
	if got := fake.lastRegister.Instance.GetServiceName(); got != instance.ServiceName {
		t.Fatalf("registered service = %q, want %q", got, instance.ServiceName)
	}
	if got := fake.lastRegister.Instance.GetNamespace(); got != "default" {
		t.Fatalf("registered namespace = %q, want default", got)
	}
}

type fakeRegistryServiceClient struct {
	heartbeatErr error
	heartbeatRes *pb.HeartbeatResponse
	registerErr  error
	registerRes  *pb.RegisterResponse

	heartbeatCalls int
	registerCalls  int
	lastRegister   *pb.RegisterRequest
}

func (f *fakeRegistryServiceClient) Register(_ context.Context, in *pb.RegisterRequest, _ ...grpc.CallOption) (*pb.RegisterResponse, error) {
	f.registerCalls++
	f.lastRegister = in
	if f.registerRes != nil || f.registerErr != nil {
		return f.registerRes, f.registerErr
	}
	return &pb.RegisterResponse{Success: true}, nil
}

func (f *fakeRegistryServiceClient) Deregister(context.Context, *pb.DeregisterRequest, ...grpc.CallOption) (*pb.DeregisterResponse, error) {
	return &pb.DeregisterResponse{Success: true}, nil
}

func (f *fakeRegistryServiceClient) SetInstanceStatus(context.Context, *pb.SetInstanceStatusRequest, ...grpc.CallOption) (*pb.SetInstanceStatusResponse, error) {
	return &pb.SetInstanceStatusResponse{Success: true}, nil
}

func (f *fakeRegistryServiceClient) Heartbeat(_ context.Context, _ *pb.HeartbeatRequest, _ ...grpc.CallOption) (*pb.HeartbeatResponse, error) {
	f.heartbeatCalls++
	if f.heartbeatRes != nil || f.heartbeatErr != nil {
		return f.heartbeatRes, f.heartbeatErr
	}
	return &pb.HeartbeatResponse{Success: true}, nil
}

func (f *fakeRegistryServiceClient) Discover(context.Context, *pb.DiscoverRequest, ...grpc.CallOption) (*pb.DiscoverResponse, error) {
	return &pb.DiscoverResponse{}, nil
}

func (f *fakeRegistryServiceClient) Watch(context.Context, *pb.WatchRequest, ...grpc.CallOption) (grpc.ServerStreamingClient[pb.WatchResponse], error) {
	return nil, nil
}

func (f *fakeRegistryServiceClient) GetMembers(context.Context, *pb.GetMembersRequest, ...grpc.CallOption) (*pb.GetMembersResponse, error) {
	return &pb.GetMembersResponse{}, nil
}

func (f *fakeRegistryServiceClient) ReportTopology(context.Context, *pb.ReportTopologyRequest, ...grpc.CallOption) (*pb.ReportTopologyResponse, error) {
	return &pb.ReportTopologyResponse{Success: true}, nil
}

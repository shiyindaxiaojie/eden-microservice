package grpcclient

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/shiyindaxiaojie/eden-go-registry/examples/service-discovery/custom/internal/registry"
	pb "github.com/shiyindaxiaojie/eden-go-registry/examples/service-discovery/custom/internal/registryv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type Client struct {
	targets []*grpcTarget

	namespace       string
	datacenter      string
	consumerService string

	mu      sync.Mutex
	cancels []context.CancelFunc
}

type grpcTarget struct {
	addr   string
	conn   *grpc.ClientConn
	client pb.RegistryServiceClient
}

func NewFromEnv() (registry.Registry, error) {
	addrs := splitAddrs(envOr("CUSTOM_GRPC_ADDRS", "127.0.0.1:9000"))
	if len(addrs) == 0 {
		return nil, fmt.Errorf("CUSTOM_GRPC_ADDRS is empty")
	}

	targets := make([]*grpcTarget, 0, len(addrs))
	for _, addr := range addrs {
		conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			for _, target := range targets {
				target.conn.Close()
			}
			return nil, err
		}
		targets = append(targets, &grpcTarget{
			addr:   addr,
			conn:   conn,
			client: pb.NewRegistryServiceClient(conn),
		})
	}

	return &Client{
		targets:    targets,
		namespace:  envOr("CUSTOM_NAMESPACE", "default"),
		datacenter: envOr("CUSTOM_DATACENTER", "dc1"),
	}, nil
}

func (c *Client) Register(instance *registry.ServiceInstance) error {
	c.mu.Lock()
	if c.consumerService == "" {
		c.consumerService = instance.ServiceName
	}
	c.mu.Unlock()

	return c.withClient(func(client pb.RegistryServiceClient) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := client.Register(ctx, &pb.RegisterRequest{
			Instance: &pb.ServiceInstance{
				Id:          instance.ID,
				ServiceName: instance.ServiceName,
				Host:        instance.Host,
				Port:        int32(instance.Port),
				Weight:      int32(instance.Weight),
				Datacenter:  c.datacenter,
				Metadata:    instance.Metadata,
				Namespace:   c.namespace,
			},
		})
		if err != nil {
			return err
		}
		if !resp.Success {
			return fmt.Errorf("grpc register failed")
		}
		return nil
	})
}

func (c *Client) Deregister(instance *registry.ServiceInstance) error {
	return c.withClient(func(client pb.RegistryServiceClient) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := client.SetInstanceStatus(ctx, &pb.SetInstanceStatusRequest{
			Namespace:   c.namespace,
			ServiceName: instance.ServiceName,
			InstanceId:  instance.ID,
			Status:      "offline",
		})
		if err != nil {
			return err
		}
		if !resp.Success {
			return fmt.Errorf("grpc deregister failed")
		}
		return nil
	})
}

func (c *Client) Discovery(serviceName string) ([]*registry.ServiceInstance, error) {
	var items []*registry.ServiceInstance
	err := c.withClient(func(client pb.RegistryServiceClient) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ctx = c.withConsumerService(ctx)

		resp, err := client.Discover(ctx, &pb.DiscoverRequest{
			ServiceName: serviceName,
			HealthyOnly: true,
			Datacenter:  c.datacenter,
			Namespace:   c.namespace,
		})
		if err != nil {
			return err
		}

		items = protoToInstances(resp.Instances)
		return nil
	})
	return items, err
}

func (c *Client) Subscribe(serviceName string, callback func([]*registry.ServiceInstance)) error {
	target := c.targets[0]

	ctx, cancel := context.WithCancel(context.Background())
	ctx = c.withConsumerService(ctx)
	c.addCancel(cancel)

	stream, err := target.client.Watch(ctx, &pb.WatchRequest{
		ServiceName: serviceName,
		Namespace:   c.namespace,
	})
	if err != nil {
		cancel()
		return err
	}

	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				return
			}
			callback(protoToInstances(msg.Instances))
		}
	}()
	return nil
}

func (c *Client) Heartbeat(instance *registry.ServiceInstance) error {
	return c.withClient(func(client pb.RegistryServiceClient) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := client.Heartbeat(ctx, &pb.HeartbeatRequest{
			Namespace:   c.namespace,
			ServiceName: instance.ServiceName,
			InstanceId:  instance.ID,
		})
		if err != nil {
			return err
		}
		if !resp.Success {
			return fmt.Errorf("grpc heartbeat failed")
		}
		return nil
	})
}

func (c *Client) Close() error {
	c.mu.Lock()
	cancels := append([]context.CancelFunc(nil), c.cancels...)
	c.cancels = nil
	c.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}

	var firstErr error
	for _, target := range c.targets {
		if err := target.conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (c *Client) withClient(fn func(pb.RegistryServiceClient) error) error {
	var lastErr error
	for _, target := range c.targets {
		if err := fn(target.client); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

func (c *Client) addCancel(cancel context.CancelFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cancels = append(c.cancels, cancel)
}

func (c *Client) withConsumerService(ctx context.Context) context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.consumerService == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "x-consumer-service", c.consumerService)
}

func protoToInstances(items []*pb.ServiceInstance) []*registry.ServiceInstance {
	result := make([]*registry.ServiceInstance, 0, len(items))
	for _, item := range items {
		healthy := item.Status == "" || item.Status == "passing" || item.Status == "online"
		result = append(result, &registry.ServiceInstance{
			ID:          item.Id,
			ServiceName: item.ServiceName,
			Host:        item.Host,
			Port:        int(item.Port),
			Weight:      int(item.Weight),
			Metadata:    item.Metadata,
			Healthy:     healthy,
			Datacenter:  item.Datacenter,
		})
	}
	return result
}

func envOr(key, def string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return def
}

func splitAddrs(raw string) []string {
	parts := strings.Split(raw, ",")
	addrs := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			addrs = append(addrs, part)
		}
	}
	return addrs
}

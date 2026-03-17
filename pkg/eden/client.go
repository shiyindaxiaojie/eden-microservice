package eden

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	pb "github.com/shiyindaxiaojie/eden-go-registry/api/proto/registry/v1"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/registry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"crypto/tls"
	"net"
	"github.com/quic-go/quic-go"
)

// Ensure Client implements registry.Registry.
var _ registry.Registry = (*Client)(nil)

// Client implements the Eden native registry client.
type Client struct {
	addresses  []string
	apiKey     string
	datacenter string
	client     *http.Client
	
	// gRPC support
	grpcConn   *grpc.ClientConn
	grpcClient pb.RegistryServiceClient
	transport  string
	
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// Config represents the Eden client configuration.
type Config struct {
	Addresses  []string
	APIKey     string
	Datacenter string
	Transport  string // "http", "grpc" (default), "quic"
}

// New creates a new Eden registry client with default settings.
func New(addresses []string, apiKey, datacenter string) (*Client, error) {
	return NewWithConfig(&Config{
		Addresses:  addresses,
		APIKey:     apiKey,
		Datacenter: datacenter,
		Transport:  "grpc",
	})
}

// NewWithConfig creates a new Eden registry client with the given config.
func NewWithConfig(cfg *Config) (*Client, error) {
	if len(cfg.Addresses) == 0 {
		return nil, fmt.Errorf("eden: at least one address required")
	}

	c := &Client{
		addresses:  cfg.Addresses,
		apiKey:     cfg.APIKey,
		datacenter: cfg.Datacenter,
		transport:  cfg.Transport,
		client:     &http.Client{Timeout: 5 * time.Second},
		stopCh:     make(chan struct{}),
	}

	addr := cfg.Addresses[0]
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		var conn *grpc.ClientConn
		var err error
		
		if cfg.Transport == "quic" {
			conn, err = c.dialQUIC(addr)
		} else {
			conn, err = grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		}
		
		if err == nil {
			c.grpcConn = conn
			c.grpcClient = pb.NewRegistryServiceClient(conn)
		}
	}

	return c, nil
}

func (c *Client) Register(instance *registry.ServiceInstance) error {
	if c.grpcClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		_, err := c.grpcClient.Register(ctx, &pb.RegisterRequest{
			Instance: toProtoInstance(instance),
		})
		return err
	}

	body := map[string]interface{}{
		"id":           instance.ID,
		"service_name": instance.ServiceName,
		"host":         instance.Host,
		"port":         instance.Port,
		"weight":       instance.Weight,
		"metadata":     instance.Metadata,
		"datacenter":   c.datacenter,
	}
	return c.post("/v1/catalog/register", body)
}

func (c *Client) Deregister(instance *registry.ServiceInstance) error {
	if c.grpcClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		_, err := c.grpcClient.Deregister(ctx, &pb.DeregisterRequest{
			ServiceName: instance.ServiceName,
			InstanceId:  instance.ID,
		})
		return err
	}

	body := map[string]string{
		"service_name": instance.ServiceName,
		"instance_id":  instance.ID,
	}
	return c.post("/v1/catalog/deregister", body)
}

func (c *Client) Discovery(serviceName string) ([]*registry.ServiceInstance, error) {
	if c.grpcClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		resp, err := c.grpcClient.Discover(ctx, &pb.DiscoverRequest{
			ServiceName: serviceName,
			HealthyOnly: true,
			Datacenter:  c.datacenter,
		})
		if err != nil {
			return nil, err
		}
		
		return fromProtoInstances(resp.Instances), nil
	}

	url := fmt.Sprintf("%s/v1/catalog/service/%s?passing=true", c.pickHttpAddr(), serviceName)
	req, _ := http.NewRequest("GET", url, nil)
	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("eden discovery: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("eden discovery: status %d, body: %s", resp.StatusCode, string(body))
	}

	var edenInstances []struct {
		ID          string            `json:"id"`
		ServiceName string            `json:"service_name"`
		Host        string            `json:"host"`
		Port        int               `json:"port"`
		Weight      int               `json:"weight"`
		Status      string            `json:"status"`
		Metadata    map[string]string `json:"metadata"`
		Datacenter  string            `json:"datacenter"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&edenInstances); err != nil {
		return nil, fmt.Errorf("eden discovery decode: %w", err)
	}

	result := make([]*registry.ServiceInstance, 0, len(edenInstances))
	for _, ei := range edenInstances {
		result = append(result, &registry.ServiceInstance{
			ID:          ei.ID,
			ServiceName: ei.ServiceName,
			Host:        ei.Host,
			Port:        ei.Port,
			Weight:      ei.Weight,
			Metadata:    ei.Metadata,
			Healthy:     ei.Status == "passing",
			Datacenter:  ei.Datacenter,
		})
	}
	return result, nil
}

func (c *Client) Subscribe(serviceName string, callback func([]*registry.ServiceInstance)) error {
	if c.grpcClient != nil {
		s, err := c.grpcClient.Watch(context.Background(), &pb.WatchRequest{
			ServiceName: serviceName,
		})
		if err != nil {
			return err
		}
		
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			for {
				resp, err := s.Recv()
				if err != nil {
					return // End of stream or error
				}
				callback(fromProtoInstances(resp.Instances))
			}
		}()
		return nil
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		var lastHash string
		for {
			select {
			case <-c.stopCh:
				return
			case <-ticker.C:
				instances, err := c.Discovery(serviceName)
				if err != nil {
					continue
				}
				hash := instancesHash(instances)
				if hash != lastHash {
					lastHash = hash
					callback(instances)
				}
			}
		}
	}()
	return nil
}

func (c *Client) Heartbeat(instance *registry.ServiceInstance) error {
	if c.grpcClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		
		_, err := c.grpcClient.Heartbeat(ctx, &pb.HeartbeatRequest{
			ServiceName: instance.ServiceName,
			InstanceId:  instance.ID,
		})
		return err
	}

	body := map[string]string{
		"service_name": instance.ServiceName,
		"instance_id":  instance.ID,
	}
	return c.post("/v1/catalog/heartbeat", body)
}

func (c *Client) Close() error {
	close(c.stopCh)
	if c.grpcConn != nil {
		c.grpcConn.Close()
	}
	c.wg.Wait()
	return nil
}

// --- helpers ---

func (c *Client) pickHttpAddr() string {
	addr := c.addresses[0]
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		return "http://" + addr
	}
	return addr
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
}

func (c *Client) post(path string, body interface{}) error {
	data, _ := json.Marshal(body)
	url := c.pickHttpAddr() + path

	req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("eden %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("eden %s: status %d, body: %s", path, resp.StatusCode, string(respBody))
	}
	return nil
}

func toProtoInstance(inst *registry.ServiceInstance) *pb.ServiceInstance {
	return &pb.ServiceInstance{
		Id:          inst.ID,
		ServiceName: inst.ServiceName,
		Host:        inst.Host,
		Port:        int32(inst.Port),
		Weight:      int32(inst.Weight),
		Metadata:    inst.Metadata,
		Datacenter:  inst.Datacenter,
	}
}

func fromProtoInstances(pbList []*pb.ServiceInstance) []*registry.ServiceInstance {
	result := make([]*registry.ServiceInstance, 0, len(pbList))
	for _, pbi := range pbList {
		result = append(result, &registry.ServiceInstance{
			ID:          pbi.Id,
			ServiceName: pbi.ServiceName,
			Host:        pbi.Host,
			Port:        int(pbi.Port),
			Weight:      int(pbi.Weight),
			Metadata:    pbi.Metadata,
			Healthy:     pbi.Status == "passing",
			Datacenter:  pbi.Datacenter,
		})
	}
	return result
}

func (c *Client) dialQUIC(addr string) (*grpc.ClientConn, error) {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"h3"},
	}
	
	dialer := func(ctx context.Context, addr string) (net.Conn, error) {
		conn, err := quic.DialAddr(ctx, addr, tlsConf, nil)
		if err != nil {
			return nil, err
		}
		stream, err := conn.OpenStreamSync(ctx)
		if err != nil {
			return nil, err
		}
		return &quicStreamConn{conn: conn, Stream: stream}, nil
	}
	
	return grpc.Dial(addr, grpc.WithContextDialer(dialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
}

type quicStreamConn struct {
	conn *quic.Conn
	*quic.Stream
}

func (c *quicStreamConn) LocalAddr() net.Addr  { return c.conn.LocalAddr() }
func (c *quicStreamConn) RemoteAddr() net.Addr { return c.conn.RemoteAddr() }

func instancesHash(instances []*registry.ServiceInstance) string {
	data, _ := json.Marshal(instances)
	return string(data)
}

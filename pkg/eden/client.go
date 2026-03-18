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

type RegistryNode struct {
	ID       string
	HTTPAddr string
	GRPCAddr string
	QUICAddr string
	Status   string
}

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
	
	// Cluster state
	allNodes   []*RegistryNode
	mu         sync.RWMutex
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

	initialNodes := make([]*RegistryNode, 0)
	for _, addr := range cfg.Addresses {
		node := &RegistryNode{HTTPAddr: addr}
		if !strings.HasPrefix(addr, "http") {
			node.GRPCAddr = addr
		}
		initialNodes = append(initialNodes, node)
	}

	c := &Client{
		addresses:  cfg.Addresses,
		allNodes:   initialNodes,
		apiKey:     cfg.APIKey,
		datacenter: cfg.Datacenter,
		transport:  cfg.Transport,
		client:     &http.Client{Timeout: 5 * time.Second},
		stopCh:     make(chan struct{}),
	}

	// Start background node discovery
	c.startNodeDiscovery()

	return c, nil
}

func (c *Client) startNodeDiscovery() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		
		// Immediate check
		c.syncNodes()

		for {
			select {
			case <-ticker.C:
				c.syncNodes()
			case <-c.stopCh:
				return
			}
		}
	}()
}

func (c *Client) syncNodes() {
	var members []map[string]interface{}

	// Try all known addresses until one responds
	err := c.executeWithFailover(func(node *RegistryNode) error {
		addr := node.HTTPAddr
		if addr == "" { addr = node.GRPCAddr } // Fallback to guess if only gRPC addr known
		
		url := addr
		if !strings.HasPrefix(url, "http") {
			url = "http://" + url
		}
		if !strings.Contains(url, ":") || strings.HasSuffix(url, "://") {
			// Append port if missing (heuristic)
			url += "8500"
		}
		url += "/v1/cluster/members"

		resp, err := c.client.Get(url)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status %d", resp.StatusCode)
		}
		return json.NewDecoder(resp.Body).Decode(&members)
	})

	if err == nil {
		newNodes := make([]*RegistryNode, 0)
		for _, m := range members {
			if m["status"] == "Online" {
				node := &RegistryNode{
					ID:       fmt.Sprintf("%v", m["id"]),
					HTTPAddr: fmt.Sprintf("%v", m["http_addr"]),
					GRPCAddr: fmt.Sprintf("%v", m["grpc_addr"]),
					QUICAddr: fmt.Sprintf("%v", m["quic_addr"]),
					Status:   "Online",
				}
				newNodes = append(newNodes, node)
			}
		}
		if len(newNodes) > 0 {
			c.mu.Lock()
			c.allNodes = newNodes
			c.mu.Unlock()
		}
	}
}

func (c *Client) executeWithFailover(fn func(node *RegistryNode) error) error {
	c.mu.RLock()
	nodes := make([]*RegistryNode, len(c.allNodes))
	copy(nodes, c.allNodes)
	c.mu.RUnlock()

	var lastErr error
	for _, node := range nodes {
		if err := fn(node); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

func (c *Client) Register(instance *registry.ServiceInstance) error {
	return c.executeWithFailover(func(node *RegistryNode) error {
		if c.transport == "grpc" || c.transport == "quic" {
			addr := node.GRPCAddr
			if addr == "" { addr = node.HTTPAddr } // Fallback
			
			var conn *grpc.ClientConn
			var err error
			if c.transport == "quic" {
				conn, err = c.dialQUIC(addr)
			} else {
				conn, err = grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			}
			if err != nil { return err }
			defer conn.Close()
			
			client := pb.NewRegistryServiceClient(conn)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err = client.Register(ctx, &pb.RegisterRequest{
				Instance: toProtoInstance(instance),
			})
			return err
		}

		// HTTP
		body := map[string]interface{}{
			"id":           instance.ID,
			"service_name": instance.ServiceName,
			"host":         instance.Host,
			"port":         instance.Port,
			"weight":       instance.Weight,
			"metadata":     instance.Metadata,
			"datacenter":   c.datacenter,
		}
		data, _ := json.Marshal(body)
		url := node.HTTPAddr
		if !strings.HasPrefix(url, "http") { url = "http://" + url }
		url += "/v1/catalog/register"

		req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
		c.setHeaders(req)
		resp, err := c.client.Do(req)
		if err != nil { return err }
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status %d", resp.StatusCode)
		}
		return nil
	})
}

func (c *Client) Deregister(instance *registry.ServiceInstance) error {
	return c.executeWithFailover(func(node *RegistryNode) error {
		if c.transport == "grpc" || c.transport == "quic" {
			addr := node.GRPCAddr
			if addr == "" { addr = node.HTTPAddr }

			var conn *grpc.ClientConn
			var err error
			if c.transport == "quic" {
				conn, err = c.dialQUIC(addr)
			} else {
				conn, err = grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			}
			if err != nil { return err }
			defer conn.Close()

			client := pb.NewRegistryServiceClient(conn)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err = client.Deregister(ctx, &pb.DeregisterRequest{
				ServiceName: instance.ServiceName,
				InstanceId:  instance.ID,
			})
			return err
		}

		body := map[string]string{
			"service_name": instance.ServiceName,
			"instance_id":  instance.ID,
		}
		data, _ := json.Marshal(body)
		url := node.HTTPAddr
		if !strings.HasPrefix(url, "http") { url = "http://" + url }
		url += "/v1/catalog/deregister"

		req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
		c.setHeaders(req)
		resp, err := c.client.Do(req)
		if err != nil { return err }
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status %d", resp.StatusCode)
		}
		return nil
	})
}

func (c *Client) Discovery(serviceName string) ([]*registry.ServiceInstance, error) {
	var instances []*registry.ServiceInstance
	err := c.executeWithFailover(func(node *RegistryNode) error {
		if c.transport == "grpc" || c.transport == "quic" {
			addr := node.GRPCAddr
			if addr == "" { addr = node.HTTPAddr }
			
			var conn *grpc.ClientConn
			var err error
			if c.transport == "quic" {
				conn, err = c.dialQUIC(addr)
			} else {
				conn, err = grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			}
			if err != nil { return err }
			defer conn.Close()

			client := pb.NewRegistryServiceClient(conn)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			resp, err := client.Discover(ctx, &pb.DiscoverRequest{
				ServiceName: serviceName,
				HealthyOnly: true,
				Datacenter:  c.datacenter,
			})
			if err != nil { return err }
			instances = fromProtoInstances(resp.Instances)
			return nil
		}

		url := fmt.Sprintf("%s/v1/catalog/service/%s?passing=true", node.HTTPAddr, serviceName)
		if !strings.HasPrefix(url, "http") { url = "http://" + url }
		req, _ := http.NewRequest("GET", url, nil)
		c.setHeaders(req)
		resp, err := c.client.Do(req)
		if err != nil { return err }
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status %d", resp.StatusCode)
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
		if err := json.NewDecoder(resp.Body).Decode(&edenInstances); err != nil { return err }

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
		instances = result
		return nil
	})

	return instances, err
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
	return c.executeWithFailover(func(node *RegistryNode) error {
		if c.transport == "grpc" || c.transport == "quic" {
			addr := node.GRPCAddr
			if addr == "" { addr = node.HTTPAddr }
			
			var conn *grpc.ClientConn
			var err error
			if c.transport == "quic" {
				conn, err = c.dialQUIC(addr)
			} else {
				conn, err = grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			}
			if err != nil { return err }
			defer conn.Close()

			client := pb.NewRegistryServiceClient(conn)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, err = client.Heartbeat(ctx, &pb.HeartbeatRequest{
				ServiceName: instance.ServiceName,
				InstanceId:  instance.ID,
			})
			return err
		}

		body := map[string]string{
			"service_name": instance.ServiceName,
			"instance_id":  instance.ID,
		}
		data, _ := json.Marshal(body)
		url := node.HTTPAddr
		if !strings.HasPrefix(url, "http") { url = "http://" + url }
		url += "/v1/catalog/heartbeat"

		req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
		c.setHeaders(req)
		resp, err := c.client.Do(req)
		if err != nil { return err }
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status %d", resp.StatusCode)
		}
		return nil
	})
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
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.allNodes) == 0 {
		return ""
	}
	node := c.allNodes[0]
	addr := node.HTTPAddr
	if addr == "" { addr = node.GRPCAddr } // Guestimate
	if !strings.HasPrefix(addr, "http") {
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

package eden

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	neturl "net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	logger "github.com/shiyindaxiaojie/eden-go-logger"
	pb "github.com/shiyindaxiaojie/eden-go-registry/api/proto/registry/v1"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/registry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// RegistryNode represents a single registry server node with pooled connections.
type RegistryNode struct {
	ID       string
	HTTPAddr string
	GRPCAddr string
	QUICAddr string

	// Connection pool (per-node long-lived connection)
	mu     sync.Mutex
	conn   *grpc.ClientConn
	client pb.RegistryServiceClient

	// Health tracking
	healthy     bool
	lastFailure time.Time
}

func (n *RegistryNode) endpointForTransport(transport string) string {
	switch transport {
	case "http":
		if n.HTTPAddr != "" {
			return n.HTTPAddr
		}
	case "quic":
		if n.QUICAddr != "" {
			return n.QUICAddr
		}
		if n.GRPCAddr != "" {
			return n.GRPCAddr
		}
	default:
		if n.GRPCAddr != "" {
			return n.GRPCAddr
		}
		if n.QUICAddr != "" {
			return n.QUICAddr
		}
	}
	return n.HTTPAddr
}

// getClient returns a cached gRPC client, creating the connection if needed.
func (n *RegistryNode) getClient(transport string) (pb.RegistryServiceClient, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.client != nil {
		return n.client, nil
	}

	var conn *grpc.ClientConn
	var err error
	if transport == "quic" {
		if n.QUICAddr == "" {
			return nil, fmt.Errorf("no QUIC address for node %s", n.ID)
		}
		conn, err = dialQUIC(n.QUICAddr)
	} else {
		if n.GRPCAddr == "" {
			return nil, fmt.Errorf("no gRPC address for node %s", n.ID)
		}
		conn, err = grpc.Dial(n.GRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	if err != nil {
		return nil, err
	}

	n.conn = conn
	n.client = pb.NewRegistryServiceClient(conn)
	return n.client, nil
}

// closeConn closes the pooled connection and resets the client.
func (n *RegistryNode) closeConn() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.conn != nil {
		n.conn.Close()
		n.conn = nil
		n.client = nil
	}
}

// markUnhealthy marks the node as unhealthy and closes its connection.
func (n *RegistryNode) markUnhealthy() {
	n.mu.Lock()
	n.healthy = false
	n.lastFailure = time.Now()
	n.mu.Unlock()
	n.closeConn()
}

// isHealthy returns whether the node is considered healthy.
func (n *RegistryNode) isHealthy() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.healthy
}

// Ensure Client implements registry.Registry.
var _ registry.Registry = (*Client)(nil)

// Client implements the Eden native registry client with connection pooling and failover.
type Client struct {
	addresses  []string
	apiKey     string
	datacenter string
	namespace  string
	client     *http.Client
	transport  string // "http", "grpc" (default), "quic"
	discovery  string // "auto" (default) or "static"

	// Cache
	cache *LocalCache

	// Cluster state
	allNodes         []*RegistryNode
	lastTopology     string
	httpControlPlane bool
	mu               sync.RWMutex
	stopCh           chan struct{}
	wg               sync.WaitGroup
	identityMu       sync.RWMutex
	consumerService  string
	subscriptions    map[string]struct{}
	topologyChecksum string
}

// Config represents the Eden client configuration.
type Config struct {
	Addresses     []string
	APIKey        string
	Datacenter    string
	Namespace     string
	CacheDir      string
	Transport     string // "http", "grpc" (default), "quic"
	DiscoveryMode string // "auto" (default) or "static"
}

type clusterMemberSnapshot struct {
	ID       string `json:"id"`
	Address  string `json:"address"`
	Status   string `json:"status"`
	Role     string `json:"role"`
	IsLocal  bool   `json:"is_local"`
	HTTPAddr string `json:"http_addr"`
	GRPCAddr string `json:"grpc_addr"`
	QUICAddr string `json:"quic_addr"`
	RaftAddr string `json:"raft_addr"`
}

// New creates a new Eden registry client with default settings.
func New(addresses []string, apiKey, datacenter string) (*Client, error) {
	return NewWithConfig(&Config{
		Addresses:     addresses,
		APIKey:        apiKey,
		Datacenter:    datacenter,
		Namespace:     "",
		CacheDir:      "",
		Transport:     "grpc",
		DiscoveryMode: "auto",
	})
}

// NewWithConfig creates a new Eden registry client with the given config.
func NewWithConfig(cfg *Config) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("eden: config required")
	}
	if len(cfg.Addresses) == 0 {
		return nil, fmt.Errorf("eden: at least one address required")
	}

	transport := strings.ToLower(strings.TrimSpace(cfg.Transport))
	if transport == "" {
		transport = "grpc"
	}
	if transport != "http" && transport != "grpc" && transport != "quic" {
		return nil, fmt.Errorf("eden: unsupported transport %q", cfg.Transport)
	}

	discovery := strings.ToLower(strings.TrimSpace(cfg.DiscoveryMode))
	if discovery == "" {
		discovery = "auto"
	}
	if discovery != "auto" && discovery != "static" {
		return nil, fmt.Errorf("eden: unsupported discovery mode %q", cfg.DiscoveryMode)
	}

	initialNodes := make([]*RegistryNode, 0, len(cfg.Addresses))
	httpControlPlane := transport == "http"
	for _, addr := range cfg.Addresses {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}

		isHTTPAddr := strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://")
		node := &RegistryNode{ID: addr, healthy: true}

		switch transport {
		case "http":
			node.HTTPAddr = addr
		case "grpc":
			if discovery == "static" {
				if isHTTPAddr {
					return nil, fmt.Errorf("eden: grpc static mode requires direct gRPC addresses, got %s", addr)
				}
				node.GRPCAddr = addr
			} else if isHTTPAddr {
				node.HTTPAddr = addr
				httpControlPlane = true
			} else {
				node.GRPCAddr = addr
			}
		case "quic":
			if discovery == "static" {
				if isHTTPAddr {
					return nil, fmt.Errorf("eden: quic static mode requires direct QUIC addresses, got %s", addr)
				}
				node.QUICAddr = addr
			} else if isHTTPAddr {
				node.HTTPAddr = addr
				httpControlPlane = true
			} else {
				node.QUICAddr = addr
			}
		}

		initialNodes = append(initialNodes, node)
	}
	if len(initialNodes) == 0 {
		return nil, fmt.Errorf("eden: at least one non-empty address required")
	}

	c := &Client{
		addresses:        cfg.Addresses,
		allNodes:         initialNodes,
		apiKey:           cfg.APIKey,
		datacenter:       cfg.Datacenter,
		namespace:        cfg.Namespace,
		transport:        transport,
		discovery:        discovery,
		cache:            NewLocalCache(cfg.CacheDir),
		client:           &http.Client{Timeout: 5 * time.Second},
		httpControlPlane: httpControlPlane,
		stopCh:           make(chan struct{}),
		subscriptions:    make(map[string]struct{}),
	}

	// When HTTP control-plane addresses are available, resolve transport endpoints
	// and keep membership refreshed in the background. Otherwise the client stays
	// in static direct-endpoint mode and performs no HTTP requests.
	if c.discovery == "auto" {
		c.syncNodes()
	}

	logger.Info("[Registry Init] Client init, transport: %s, discovery: %s, httpControlPlane: %v, seeds: %v, namespace: %s, cacheDir: %s",
		transport, discovery, c.httpControlPlane, cfg.Addresses, cfg.Namespace, cfg.CacheDir)

	if c.discovery == "auto" {
		c.startNodeDiscovery()
	}
	c.startCacheRefresh()

	return c, nil
}

// startNodeDiscovery runs background node sync every 30 seconds.
func (c *Client) startNodeDiscovery() {
	if c.discovery == "static" {
		return
	}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.syncNodes()
			case <-c.stopCh:
				return
			}

			// If all nodes are unhealthy, be more aggressive
			c.mu.RLock()
			allDown := true
			for _, n := range c.allNodes {
				if n.isHealthy() {
					allDown = false
					break
				}
			}
			c.mu.RUnlock()

			if allDown {
				// Re-sync after 5s if everything is down
				time.Sleep(5 * time.Second)
				c.syncNodes()
			}
		}
	}()
}

// syncNodes fetches the cluster member list and updates node state.
func (c *Client) syncNodes() {
	if c.discovery == "static" {
		return
	}
	var members []clusterMemberSnapshot

	err := c.tryAny("SyncNodes", func(node *RegistryNode) error {
		var grpcErr error
		if c.transport == "grpc" || c.transport == "quic" {
			client, err := node.getClient(c.transport)
			if err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()

				resp, err := client.GetMembers(ctx, &pb.GetMembersRequest{})
				if err == nil {
					members = make([]clusterMemberSnapshot, 0, len(resp.Members))
					for _, member := range resp.Members {
						members = append(members, clusterMemberSnapshot{
							ID:       member.Id,
							Address:  member.Address,
							Status:   member.Status,
							Role:     member.Role,
							IsLocal:  member.IsLocal,
							HTTPAddr: member.HttpAddr,
							GRPCAddr: member.GrpcAddr,
							QUICAddr: member.QuicAddr,
							RaftAddr: member.RaftAddr,
						})
					}
					return nil
				}
				grpcErr = err
			} else {
				grpcErr = err
			}
		}

		if node.HTTPAddr == "" {
			if grpcErr != nil {
				return grpcErr
			}
			return fmt.Errorf("no control-plane endpoint for node %s", node.ID)
		}

		url := node.HTTPAddr
		if !strings.HasPrefix(url, "http") {
			url = "http://" + url
		}
		if !strings.Contains(url, ":") || strings.HasSuffix(url, "://") {
			url += "8500"
		}
		url += "/v1/cluster/members"

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}
		c.setHeaders(req)
		resp, err := c.client.Do(req)
		if err != nil {
			if grpcErr != nil {
				return fmt.Errorf("grpc discovery failed: %v; http discovery failed: %w", grpcErr, err)
			}
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			if grpcErr != nil {
				return fmt.Errorf("grpc discovery failed: %v; http discovery status %d", grpcErr, resp.StatusCode)
			}
			return fmt.Errorf("status %d", resp.StatusCode)
		}
		return json.NewDecoder(resp.Body).Decode(&members)
	})

	if err != nil {
		return
	}

	newNodes := make([]*RegistryNode, 0, len(members))
	c.mu.RLock()
	existingNodes := make([]*RegistryNode, len(c.allNodes))
	copy(existingNodes, c.allNodes)
	addressMap := make(map[string]*RegistryNode, len(existingNodes)*3)
	for _, n := range existingNodes {
		for _, key := range []string{n.HTTPAddr, n.GRPCAddr, n.QUICAddr} {
			if key != "" {
				addressMap[key] = n
			}
		}
	}
	c.mu.RUnlock()
	reused := make(map[*RegistryNode]bool, len(existingNodes))

	for _, m := range members {
		httpAddr := cleanMemberAddr(m.HTTPAddr)
		grpcAddr := cleanMemberAddr(m.GRPCAddr)
		quicAddr := cleanMemberAddr(m.QUICAddr)
		status := cleanMemberAddr(m.Status)

		var existing *RegistryNode
		for _, key := range []string{httpAddr, grpcAddr, quicAddr} {
			if key == "" {
				continue
			}
			if candidate, ok := addressMap[key]; ok {
				existing = candidate
				break
			}
		}

		if existing != nil {
			existing.mu.Lock()
			existing.ID = cleanMemberAddr(m.ID)
			existing.HTTPAddr = httpAddr
			existing.GRPCAddr = grpcAddr
			existing.QUICAddr = quicAddr
			existing.healthy = (status == "Online")
			existing.mu.Unlock()
			newNodes = append(newNodes, existing)
			reused[existing] = true
		} else {
			newNodes = append(newNodes, &RegistryNode{
				ID:       cleanMemberAddr(m.ID),
				HTTPAddr: httpAddr,
				GRPCAddr: grpcAddr,
				QUICAddr: quicAddr,
				healthy:  (status == "Online"),
			})
		}
	}

	for _, old := range existingNodes {
		if !reused[old] {
			old.closeConn()
		}
	}

	if len(newNodes) > 0 {
		c.mu.Lock()
		c.allNodes = newNodes

		var nodeDesc []string
		for _, n := range newNodes {
			nodeDesc = append(nodeDesc, fmt.Sprintf("%s(healthy=%v)", n.ID, n.healthy))
		}
		descStr := strings.Join(nodeDesc, ", ")
		changed := false
		if c.lastTopology != descStr {
			c.lastTopology = descStr
			changed = true
		}
		c.mu.Unlock()

		if changed {
			logger.Info("[Registry Nodes] Cluster topology updated. Total nodes: %d -> [%s]", len(newNodes), descStr)
		} else {
			logger.Debug("[Registry Nodes] Cluster topology remains unchanged. Total nodes: %d", len(newNodes))
		}
	}
}

// tryAny is a simple failover helper for syncNodes itself (avoids circular dependency).
func (c *Client) tryAny(opName string, fn func(node *RegistryNode) error) error {
	c.mu.RLock()
	nodes := make([]*RegistryNode, len(c.allNodes))
	copy(nodes, c.allNodes)
	c.mu.RUnlock()

	var lastErr error
	for _, node := range nodes {
		logger.Debug("[Registry Nodes] Executing '%s' via node %s (%s)", opName, node.ID, node.endpointForTransport(c.transport))
		if err := fn(node); err == nil {
			return nil
		} else {
			lastErr = err
			logger.Warn("[Registry Nodes] '%s' failed on node %s: %v", opName, node.ID, err)
		}
	}
	return lastErr
}

// sortedNodes returns a copy of allNodes with healthy nodes first.
func (c *Client) sortedNodes() []*RegistryNode {
	c.mu.RLock()
	nodes := make([]*RegistryNode, len(c.allNodes))
	copy(nodes, c.allNodes)
	c.mu.RUnlock()

	// Stable partition: healthy nodes first, unhealthy last
	healthy := make([]*RegistryNode, 0, len(nodes))
	unhealthy := make([]*RegistryNode, 0)
	for _, n := range nodes {
		if n.isHealthy() {
			healthy = append(healthy, n)
		} else {
			unhealthy = append(unhealthy, n)
		}
	}
	return append(healthy, unhealthy...)
}

// executeWithFailover tries each node in order (healthy first), marking failures.
func (c *Client) executeWithFailover(opName string, fn func(node *RegistryNode) error) error {
	nodes := c.sortedNodes()

	var lastErr error
	for _, node := range nodes {
		addr := node.endpointForTransport(c.transport)
		logger.Info("[Registry %s] Executing via node %s (%s)", opName, node.ID, addr)

		if err := fn(node); err == nil {
			return nil
		} else {
			lastErr = err
			logger.Warn("[Registry %s] Failed on node %s: %v. Marking unhealthy and failing over...", opName, node.ID, err)
			// Mark node as unhealthy on connection failure to trigger fast failover
			node.markUnhealthy()
		}
	}

	logger.Error("[Registry %s] All available nodes failed", opName)
	// All nodes failed — trigger immediate refresh in background
	go c.syncNodes()

	return lastErr
}

func (c *Client) Register(instance *registry.ServiceInstance) error {
	err := c.executeWithFailover("Register", func(node *RegistryNode) error {
		if c.transport == "grpc" || c.transport == "quic" {
			client, err := node.getClient(c.transport)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			pi := toProtoInstance(instance)
			if c.namespace != "" {
				pi.Namespace = c.namespace
			}

			resp, err := client.Register(ctx, &pb.RegisterRequest{
				Instance: pi,
			})
			if err != nil {
				return err
			}
			if !resp.Success {
				return fmt.Errorf("register failed")
			}
			return nil
		}

		// HTTP fallback
		body := map[string]interface{}{
			"id":           instance.ID,
			"service_name": instance.ServiceName,
			"namespace":    c.namespace,
			"host":         instance.Host,
			"port":         instance.Port,
			"weight":       instance.Weight,
			"metadata":     instance.Metadata,
			"datacenter":   c.datacenter,
		}
		data, _ := json.Marshal(body)
		url := node.HTTPAddr
		if !strings.HasPrefix(url, "http") {
			url = "http://" + url
		}
		url += "/v1/catalog/register"

		req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
		c.setHeaders(req)
		resp, err := c.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status %d", resp.StatusCode)
		}
		return nil
	})
	if err == nil {
		c.identityMu.Lock()
		if c.consumerService == "" {
			c.consumerService = instance.ServiceName
		}
		c.identityMu.Unlock()
		go c.reportTopology()
	}
	return err
}

func (c *Client) Deregister(instance *registry.ServiceInstance) error {
	return c.executeWithFailover("Deregister", func(node *RegistryNode) error {
		if c.transport == "grpc" || c.transport == "quic" {
			client, err := node.getClient(c.transport)
			if err != nil {
				return err
			}

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
				return fmt.Errorf("deregister failed")
			}
			return nil
		}

		// HTTP fallback
		body := map[string]string{
			"namespace":    c.namespace,
			"service_name": instance.ServiceName,
			"instance_id":  instance.ID,
			"status":       "offline",
		}
		data, _ := json.Marshal(body)
		url := node.HTTPAddr
		if !strings.HasPrefix(url, "http") {
			url = "http://" + url
		}
		url += "/v1/catalog/instance/status"

		req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
		c.setHeaders(req)
		resp, err := c.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status %d", resp.StatusCode)
		}
		return nil
	})
}

func (c *Client) Discovery(serviceName string) ([]*registry.ServiceInstance, error) {
	var instances []*registry.ServiceInstance
	err := c.executeWithFailover("Discovery", func(node *RegistryNode) error {
		if c.transport == "grpc" || c.transport == "quic" {
			client, err := node.getClient(c.transport)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ctx = c.withConsumerService(ctx)
			resp, err := client.Discover(ctx, &pb.DiscoverRequest{
				Namespace:   c.namespace,
				ServiceName: serviceName,
				HealthyOnly: true,
				Datacenter:  c.datacenter,
			})
			if err != nil {
				return err
			}
			instances = fromProtoInstances(resp.Instances)
			return nil
		}

		reqURL := fmt.Sprintf("%s/v1/catalog/service/%s?passing=true", node.HTTPAddr, serviceName)
		if c.namespace != "" {
			reqURL += "&namespace=" + c.namespace
		}
		if consumerService := c.getConsumerService(); consumerService != "" {
			reqURL += "&consumer_service=" + neturl.QueryEscape(consumerService)
		}
		if !strings.HasPrefix(reqURL, "http") {
			reqURL = "http://" + reqURL
		}
		req, _ := http.NewRequest("GET", reqURL, nil)
		c.setHeaders(req)
		resp, err := c.client.Do(req)
		if err != nil {
			return err
		}
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
		if err := json.NewDecoder(resp.Body).Decode(&edenInstances); err != nil {
			return err
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
		instances = result
		return nil
	})

	if err != nil {
		// Fallback to cache if discovery fails
		if cachedInstances, ok := c.cache.Get(serviceName); ok {
			logger.Warn("[Registry SDK] Discovery failed, using local cache for %s", serviceName)
			return cachedInstances, nil
		}
	} else if len(instances) > 0 {
		c.cache.Update(serviceName, instances)
	}

	return instances, err
}

// Subscribe watches for service changes using gRPC Watch with auto-reconnect.
// Falls back to polling if gRPC Watch is unavailable.
func (c *Client) Subscribe(serviceName string, callback func([]*registry.ServiceInstance)) error {
	c.rememberSubscription(serviceName)
	go c.reportTopology()

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for {
			select {
			case <-c.stopCh:
				return
			default:
			}

			// 1. Try establishing gRPC Watch (Real-time)
			if c.transport == "grpc" || c.transport == "quic" {
				if c.tryWatch(serviceName, callback) {
					// tryWatch returns true if it successfully connected and then disconnected.
					// We loop back to try reconnecting Watch.
					time.Sleep(1 * time.Second) // Avoid tight loop
					continue
				}
			}

			// 2. Fallback to Polling (If Watch fails or transport is HTTP)
			// pollSubscribe will now run for a few cycles and then exit to let us try Watch again.
			c.pollSubscribeThenRetry(serviceName, callback)
		}
	}()
	return nil
}

// tryWatch attempts to establish a gRPC Watch stream. Returns true if it connected
// (and the stream ended), false if it could not connect at all.
func (c *Client) tryWatch(serviceName string, callback func([]*registry.ServiceInstance)) bool {
	nodes := c.sortedNodes()
	for _, node := range nodes {
		logger.Debug("[Registry Subscribe] Attempting for service '%s' via node %s", serviceName, node.ID)
		client, err := node.getClient(c.transport)
		if err != nil {
			logger.Warn("[Registry Subscribe] Failed to get client for node %s: %v", node.ID, err)
			continue
		}

		ctx, cancel := context.WithCancel(context.Background())
		ctx = c.withConsumerService(ctx)
		// Monitor stopCh to cancel the watch
		go func() {
			select {
			case <-c.stopCh:
				cancel()
			case <-ctx.Done():
			}
		}()

		stream, err := client.Watch(ctx, &pb.WatchRequest{
			Namespace:   c.namespace,
			ServiceName: serviceName,
		})
		if err != nil {
			logger.Warn("[Registry Subscribe] Failed to establish stream on node %s: %v", node.ID, err)
			cancel()
			node.markUnhealthy()
			continue
		}

		logger.Info("[Registry Subscribe] Successfully established for '%s' via node %s", serviceName, node.ID)
		// Successfully connected — read events
		for {
			resp, err := stream.Recv()
			if err != nil {
				logger.Warn("[Registry Subscribe] Stream for '%s' on node %s disconnected: %v", serviceName, node.ID, err)
				cancel()
				node.markUnhealthy()
				// Trigger immediate node refresh
				go c.syncNodes()
				break
			}
			insts := fromProtoInstances(resp.Instances)
			c.cache.Update(serviceName, insts)
			callback(insts)
		}
		return true
	}
	return false
}

// pollSubscribeThenRetry polls Discovery for a while, then returns to allow attempting Watch upgrade.
func (c *Client) pollSubscribeThenRetry(serviceName string, callback func([]*registry.ServiceInstance)) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	// Try polling for 30 seconds before attempting to switch back to gRPC Watch
	retryWatchTimer := time.NewTimer(30 * time.Second)
	defer retryWatchTimer.Stop()

	var lastHash string
	for {
		select {
		case <-c.stopCh:
			return
		case <-retryWatchTimer.C:
			return // Time to try gRPC Watch again
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
}

func (c *Client) Heartbeat(instance *registry.ServiceInstance) error {
	err := c.executeWithFailover("Heartbeat", func(node *RegistryNode) error {
		if c.transport == "grpc" || c.transport == "quic" {
			client, err := node.getClient(c.transport)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
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
				return fmt.Errorf("instance not found")
			}
			return nil
		}

		body := map[string]string{
			"namespace":    c.namespace,
			"service_name": instance.ServiceName,
			"instance_id":  instance.ID,
		}
		data, _ := json.Marshal(body)
		url := node.HTTPAddr
		if !strings.HasPrefix(url, "http") {
			url = "http://" + url
		}
		url += "/v1/catalog/heartbeat"

		req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
		c.setHeaders(req)
		resp, err := c.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == http.StatusNotFound {
				return fmt.Errorf("instance not found")
			}
			return fmt.Errorf("status %d", resp.StatusCode)
		}
		return nil
	})

	// Self-repair: If the registry lost our registration (e.g. restart without persistence), re-register.
	if err != nil && strings.Contains(err.Error(), "not found") {
		logger.Warn("[Registry SDK] Heartbeat failed (not found), attempting auto-recovery registration for %s/%s", instance.ServiceName, instance.ID)
		return c.Register(instance)
	}

	return err
}

func (c *Client) startCacheRefresh() {
	if c.cache == nil {
		return
	}

	c.cache.StartBackgroundSyncer(30*time.Second, func() map[string][]*registry.ServiceInstance {
		result := make(map[string][]*registry.ServiceInstance)
		for _, serviceName := range c.cache.ServiceNames() {
			instances, err := c.Discovery(serviceName)
			if err == nil {
				result[serviceName] = instances
			}
		}
		return result
	}, c.stopCh)
}

func (c *Client) getConsumerService() string {
	c.identityMu.RLock()
	defer c.identityMu.RUnlock()
	return c.consumerService
}

func (c *Client) rememberSubscription(serviceName string) {
	if serviceName == "" {
		return
	}

	c.identityMu.Lock()
	defer c.identityMu.Unlock()
	c.subscriptions[serviceName] = struct{}{}
}

func (c *Client) topologySnapshot() (string, []string, string) {
	c.identityMu.RLock()
	defer c.identityMu.RUnlock()

	if c.consumerService == "" || len(c.subscriptions) == 0 {
		return "", nil, ""
	}

	providers := make([]string, 0, len(c.subscriptions))
	for serviceName := range c.subscriptions {
		if serviceName == "" || serviceName == c.consumerService {
			continue
		}
		providers = append(providers, serviceName)
	}
	if len(providers) == 0 {
		return c.consumerService, nil, ""
	}
	sort.Strings(providers)
	payload, _ := json.Marshal(map[string]interface{}{
		"namespace": c.namespace,
		"consumer":  c.consumerService,
		"providers": providers,
	})
	sum := sha256.Sum256(payload)
	return c.consumerService, providers, fmt.Sprintf("%x", sum[:])
}

func (c *Client) reportTopology() {
	consumerService, providers, checksum := c.topologySnapshot()
	if consumerService == "" || checksum == "" {
		return
	}

	c.identityMu.RLock()
	lastChecksum := c.topologyChecksum
	c.identityMu.RUnlock()
	if checksum == lastChecksum {
		return
	}

	err := c.executeWithFailover("TopologyReport", func(node *RegistryNode) error {
		var grpcErr error
		if c.transport == "grpc" || c.transport == "quic" {
			client, err := node.getClient(c.transport)
			if err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()

				resp, err := client.ReportTopology(ctx, &pb.ReportTopologyRequest{
					Namespace:       c.namespace,
					ConsumerService: consumerService,
					Providers:       providers,
					Checksum:        checksum,
				})
				if err == nil {
					if !resp.Success {
						return fmt.Errorf("topology report rejected")
					}
					return nil
				}
				grpcErr = err
			} else {
				grpcErr = err
			}
		}

		if node.HTTPAddr == "" {
			if grpcErr != nil {
				return grpcErr
			}
			return fmt.Errorf("no topology-report endpoint for node %s", node.ID)
		}

		body := map[string]interface{}{
			"namespace":        c.namespace,
			"consumer_service": consumerService,
			"providers":        providers,
			"checksum":         checksum,
		}
		data, _ := json.Marshal(body)
		reqURL := node.HTTPAddr
		if !strings.HasPrefix(reqURL, "http") {
			reqURL = "http://" + reqURL
		}
		reqURL += "/v1/catalog/topology/report"

		req, _ := http.NewRequest("POST", reqURL, bytes.NewReader(data))
		c.setHeaders(req)
		resp, err := c.client.Do(req)
		if err != nil {
			if grpcErr != nil {
				return fmt.Errorf("grpc topology report failed: %v; http topology report failed: %w", grpcErr, err)
			}
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			if grpcErr != nil {
				return fmt.Errorf("grpc topology report failed: %v; http topology report status %d", grpcErr, resp.StatusCode)
			}
			return fmt.Errorf("status %d", resp.StatusCode)
		}
		return nil
	})
	if err == nil {
		c.identityMu.Lock()
		c.topologyChecksum = checksum
		c.identityMu.Unlock()
	}
}

func (c *Client) withConsumerService(ctx context.Context) context.Context {
	consumerService := c.getConsumerService()
	if consumerService == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "x-consumer-service", consumerService)
}

// Close shuts down the client, closing all pooled connections.
func (c *Client) Close() error {
	close(c.stopCh)

	c.mu.RLock()
	nodes := c.allNodes
	c.mu.RUnlock()
	for _, n := range nodes {
		n.closeConn()
	}

	c.wg.Wait()
	return nil
}

// --- helpers ---

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

func (c *Client) pickHttpAddr() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.allNodes) == 0 {
		return ""
	}
	node := c.allNodes[0]
	addr := node.HTTPAddr
	if addr == "" {
		addr = node.GRPCAddr
	}
	if !strings.HasPrefix(addr, "http") {
		return "http://" + addr
	}
	return addr
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

func dialQUIC(addr string) (*grpc.ClientConn, error) {
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

func cleanMemberAddr(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "<nil>" {
		return ""
	}
	return v
}

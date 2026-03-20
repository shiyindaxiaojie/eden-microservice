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

// getClient returns a cached gRPC client, creating the connection if needed.
func (n *RegistryNode) getClient(transport string) (pb.RegistryServiceClient, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.client != nil {
		return n.client, nil
	}

	addr := n.GRPCAddr
	if addr == "" {
		return nil, fmt.Errorf("no gRPC address for node %s", n.ID)
	}

	var conn *grpc.ClientConn
	var err error
	if transport == "quic" && n.QUICAddr != "" {
		conn, err = dialQUIC(n.QUICAddr)
	} else {
		conn, err = grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
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

	// Cache
	cache *LocalCache

	// Cluster state
	allNodes         []*RegistryNode
	lastTopology     string
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
	Addresses  []string
	APIKey     string
	Datacenter string
	Namespace  string
	CacheDir   string
	Transport  string // "http", "grpc" (default), "quic"
}

// New creates a new Eden registry client with default settings.
func New(addresses []string, apiKey, datacenter string) (*Client, error) {
	return NewWithConfig(&Config{
		Addresses:  addresses,
		APIKey:     apiKey,
		Datacenter: datacenter,
		Namespace:  "",
		CacheDir:   "",
		Transport:  "grpc",
	})
}

// NewWithConfig creates a new Eden registry client with the given config.
func NewWithConfig(cfg *Config) (*Client, error) {
	if len(cfg.Addresses) == 0 {
		return nil, fmt.Errorf("eden: at least one address required")
	}

	initialNodes := make([]*RegistryNode, 0, len(cfg.Addresses))
	for _, addr := range cfg.Addresses {
		node := &RegistryNode{
			HTTPAddr: addr,
			healthy:  true,
		}
		if !strings.HasPrefix(addr, "http") {
			node.GRPCAddr = addr
		}
		initialNodes = append(initialNodes, node)
	}

	c := &Client{
		addresses:     cfg.Addresses,
		allNodes:      initialNodes,
		apiKey:        cfg.APIKey,
		datacenter:    cfg.Datacenter,
		namespace:     cfg.Namespace,
		transport:     cfg.Transport,
		cache:         NewLocalCache(cfg.CacheDir),
		client:        &http.Client{Timeout: 5 * time.Second},
		stopCh:        make(chan struct{}),
		subscriptions: make(map[string]struct{}),
	}

	// Synchronous initial node discovery to resolve gRPC addresses.
	// If it fails, and we have cache, we'll rely on cache.
	c.syncNodes()

	logger.Info("[Registry Init] Client init, transport: %s, seeds: %v, namespace: %s, cacheDir: %s",
		cfg.Transport, cfg.Addresses, cfg.Namespace, cfg.CacheDir)

	// Start background node discovery
	c.startNodeDiscovery()
	c.startCacheRefresh()

	return c, nil
}

// startNodeDiscovery runs background node sync every 30 seconds.
func (c *Client) startNodeDiscovery() {
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
		}
	}()
}

// syncNodes fetches the cluster member list and updates node state.
func (c *Client) syncNodes() {
	var members []map[string]interface{}

	err := c.tryAny("SyncNodes", func(node *RegistryNode) error {
		addr := node.HTTPAddr
		if addr == "" {
			addr = node.GRPCAddr
		}

		url := addr
		if !strings.HasPrefix(url, "http") {
			url = "http://" + url
		}
		if !strings.Contains(url, ":") || strings.HasSuffix(url, "://") {
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

	if err != nil {
		return
	}

	newNodes := make([]*RegistryNode, 0, len(members))
	c.mu.RLock()
	existingMap := make(map[string]*RegistryNode, len(c.allNodes))
	for _, n := range c.allNodes {
		key := n.HTTPAddr
		if key == "" {
			key = n.GRPCAddr
		}
		existingMap[key] = n
	}
	c.mu.RUnlock()

	for _, m := range members {
		status, _ := m["status"].(string)
		httpAddr := fmt.Sprintf("%v", m["http_addr"])
		grpcAddr := fmt.Sprintf("%v", m["grpc_addr"])
		quicAddr := fmt.Sprintf("%v", m["quic_addr"])
		nodeID := fmt.Sprintf("%v", m["id"])

		// Reuse existing node to preserve connection pool
		if existing, ok := existingMap[httpAddr]; ok {
			existing.mu.Lock()
			existing.ID = nodeID
			existing.GRPCAddr = grpcAddr
			existing.QUICAddr = quicAddr
			existing.healthy = (status == "Online")
			existing.mu.Unlock()
			newNodes = append(newNodes, existing)
			delete(existingMap, httpAddr)
		} else {
			newNodes = append(newNodes, &RegistryNode{
				ID:       nodeID,
				HTTPAddr: httpAddr,
				GRPCAddr: grpcAddr,
				QUICAddr: quicAddr,
				healthy:  (status == "Online"),
			})
		}
	}

	// Close connections for removed nodes
	for _, old := range existingMap {
		old.closeConn()
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
		logger.Debug("[Registry Nodes] Executing '%s' via node %s (%s)", opName, node.ID, node.HTTPAddr)
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
		addr := node.GRPCAddr
		if c.transport == "http" || addr == "" {
			addr = node.HTTPAddr
		}
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

			_, err = client.Register(ctx, &pb.RegisterRequest{
				Instance: pi,
			})
			return err
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
			_, err = client.SetInstanceStatus(ctx, &pb.SetInstanceStatusRequest{
				Namespace:   c.namespace,
				ServiceName: instance.ServiceName,
				InstanceId:  instance.ID,
				Status:      "offline",
			})
			return err
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

			// Try gRPC Watch on the first healthy node
			if c.transport == "grpc" || c.transport == "quic" {
				if c.tryWatch(serviceName, callback) {
					continue // Watch ended, retry
				}
			}

			// Fallback to polling
			c.pollSubscribe(serviceName, callback)
			return
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

// pollSubscribe polls Discovery at regular intervals as a Watch fallback.
func (c *Client) pollSubscribe(serviceName string, callback func([]*registry.ServiceInstance)) {
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
}

func (c *Client) Heartbeat(instance *registry.ServiceInstance) error {
	return c.executeWithFailover("Heartbeat", func(node *RegistryNode) error {
		if c.transport == "grpc" || c.transport == "quic" {
			client, err := node.getClient(c.transport)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, err = client.Heartbeat(ctx, &pb.HeartbeatRequest{
				Namespace:   c.namespace,
				ServiceName: instance.ServiceName,
				InstanceId:  instance.ID,
			})
			return err
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
			return fmt.Errorf("status %d", resp.StatusCode)
		}
		return nil
	})
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

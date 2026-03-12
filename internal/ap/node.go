package ap

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/configs"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/store"
)

// Node represents an AP mode node that broadcasts changes to seed peers.
type Node struct {
	Config   *configs.Config
	Registry *store.Registry
	client   *http.Client
	peerMap  sync.Map // string (addr) -> bool
}

// NewNode creates an AP mode node.
func NewNode(cfg *configs.Config, registry *store.Registry) *Node {
	n := &Node{
		Config:   cfg,
		Registry: registry,
		client: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
	// Initial peers from seeds
	for _, seed := range cfg.Seeds {
		if seed != "" {
			n.peerMap.Store(seed, true)
		}
	}
	return n
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
			n.Registry.Deregister(d["service_name"], d["instance_id"])
		}
	case "heartbeat":
		if d, ok := data.(map[string]string); ok {
			n.Registry.Heartbeat(d["service_name"], d["instance_id"])
		}
	}

	// Broadcast to peers if not already a replicated request
	if !isReplicate {
		go n.broadcast(cmdType, data)
	}

	return nil
}

// broadcast sends the command to other seeds/peers
func (n *Node) broadcast(cmdType string, data interface{}) {
	payload, err := json.Marshal(data)
	if err != nil {
		log.Printf("[AP Node] marshal error: %v", err)
		return
	}

	path := ""
	switch cmdType {
	case "register":
		path = "/v1/catalog/register?replicate=true"
	case "deregister":
		path = "/v1/catalog/deregister?replicate=true"
	case "heartbeat":
		path = "/v1/catalog/heartbeat?replicate=true"
	}

	n.peerMap.Range(func(key, value interface{}) bool {
		peer := key.(string)
		go func(addr string) {
			url := addr + path
			resp, err := n.client.Post(url, "application/json", bytes.NewReader(payload))
			if err != nil {
				// log.Printf("[AP Node] broadcast to %s failed: %v", addr, err)
				return // suppress errors to avoid log spam, it's AP mode
			}
			resp.Body.Close()
		}(peer)
		return true
	})
}

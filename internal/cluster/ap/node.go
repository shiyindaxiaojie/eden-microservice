package ap

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/config"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/store"
)

// Node represents an AP mode node that broadcasts changes to seed peers.
type Node struct {
	Config   *config.Config
	Registry *store.Registry
	client   *http.Client
	peerMap  sync.Map // string (addr) -> bool
}

// NewNode creates an AP mode node.
func NewNode(cfg *config.Config, registry *store.Registry) *Node {
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
	case "add_api_key":
		if k, ok := data.(*model.APIKey); ok {
			n.Registry.AddAPIKey(k)
		}
	case "delete_api_key":
		if key, ok := data.(string); ok {
			n.Registry.DeleteAPIKey(key)
		}
	case "add_user":
		if u, ok := data.(*model.User); ok {
			n.Registry.AddUser(u)
		}
	case "delete_user":
		if username, ok := data.(string); ok {
			n.Registry.DeleteUser(username)
		}
	case "set_mode":
		if mode, ok := data.(string); ok {
			n.Registry.SetMode(mode)
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
	case "add_api_key":
		path = "/v1/settings/apikey?replicate=true"
	case "delete_api_key":
		path = "/v1/settings/apikey/delete?replicate=true" // Note: server handler might needs updating to handle Query in broadcast
	case "add_user":
		path = "/v1/rbac/user?replicate=true"
	case "delete_user":
		path = "/v1/rbac/user/delete?replicate=true"
	case "set_mode":
		path = "/v1/settings/mode?replicate=true"
	}

	n.peerMap.Range(func(key, value interface{}) bool {
		peer := key.(string)
		go func(addr string) {
			targetURL := addr + path
			// For delete operations, we need to append the query param from data
			if cmdType == "delete_api_key" {
				if k, ok := data.(string); ok {
					targetURL += "&key=" + k
				}
			} else if cmdType == "delete_user" {
				if u, ok := data.(string); ok {
					targetURL += "&username=" + u
				}
			} else if cmdType == "set_mode" {
				if m, ok := data.(string); ok {
					targetURL += "&mode=" + m
				}
			}

			resp, err := n.client.Post(targetURL, "application/json", bytes.NewReader(payload))
			if err != nil {
				return
			}
			resp.Body.Close()
		}(peer)
		return true
	})
}

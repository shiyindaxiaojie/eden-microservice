package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/cluster/cp"
)

// ---------- Cluster Handlers (Membership & Stats) ----------

func (h *Handler) handleJoin(w http.ResponseWriter, r *http.Request) {
	if h.config.Mode == "ap" {
		jsonOK(w, map[string]string{"status": "ignored_in_ap_mode"})
		return
	}
	var req struct {
		NodeID  string `json:"node_id"`
		Address string `json:"address"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if err := h.cpNode.Join(req.NodeID, req.Address); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleMembers(w http.ResponseWriter, r *http.Request) {
	mode := h.registry.GetMode()
	env := h.registry.GetEnvironment()

	if env == "standalone" {
		localAddr := h.normalizeAddr(h.config.HTTPAddr)
		members := []map[string]string{
			{"id": h.config.NodeID, "address": localAddr, "role": "Standalone", "status": "Online"},
		}
		// In standalone, also show persistent seeds if any, so user can manage node list before switching
		seeds := h.registry.GetSeeds()
		for i, seed := range seeds {
			if seed != "" {
				members = append(members, map[string]string{
					"id":      fmt.Sprintf("peer-node-%d", i+1),
					"address": seed,
					"role":    "Peer",
					"status":  "Offline", // Likely offline in standalone, or we could ping it
				})
			}
		}
		jsonOK(w, members)
		return
	}

	if mode == "ap" {
		localAddr := h.normalizeAddr(h.config.HTTPAddr)

		members := []map[string]string{
			{"id": h.config.NodeID, "address": localAddr, "role": "Local", "status": "Online"},
		}
		seeds := h.registry.GetSeeds()
		for i, seed := range seeds {
			if seed != "" {
				status := "Offline"
				// Try to ping the seed (assuming it's an HTTP address)
				client := http.Client{Timeout: 300 * time.Millisecond}
				resp, err := client.Get(seed + "/health")
				if err == nil {
					status = "Online"
					resp.Body.Close()
				}

				members = append(members, map[string]string{
					"id":      fmt.Sprintf("peer-node-%d", i+1),
					"address": seed,
					"role":    "Peer",
					"status":  status,
				})
			}
		}
		jsonOK(w, members)
		return
	}
	members, err := h.cpNode.Members()
	if err != nil || members == nil {
		// Fallback: If Raft is leaderless or uninitialized, show local node
		localAddr := h.normalizeAddr(h.config.HTTPAddr)
		members = []map[string]string{
			{"id": h.config.NodeID, "address": localAddr, "role": "Local", "status": "Online"},
		}
	} else {
		// Ensure it's not an empty slice
		if m, ok := members.([]cp.ClusterMember); ok && len(m) == 0 {
			localAddr := h.normalizeAddr(h.config.HTTPAddr)
			members = []map[string]string{
				{"id": h.config.NodeID, "address": localAddr, "role": "Local", "status": "Online"},
			}
		}
	}
	jsonOK(w, members)
}

func (h *Handler) handleMember(w http.ResponseWriter, r *http.Request) {
	mode := h.registry.GetMode()
	if r.Method == http.MethodPost {
		var req struct {
			NodeID  string   `json:"node_id"`
			Address string   `json:"address"`
			Seeds   []string `json:"seeds"` // Optional batch update
		}
		json.NewDecoder(r.Body).Decode(&req)

		// Handle batch seeds update if provided
		if len(req.Seeds) > 0 {
			if err := h.SetSeedsWithReplication(req.Seeds); err != nil {
				httpError(w, http.StatusInternalServerError, "failed to sync seeds: "+err.Error())
				return
			}
			jsonOK(w, map[string]string{"status": "ok"})
			return
		}

		if req.Address == "" {
			httpError(w, http.StatusBadRequest, "address required")
			return
		}

		env := h.registry.GetEnvironment()

		// If in standalone, we ONLY manage seeds (persistent node list), no protocol actions
		if env == "standalone" || mode == "ap" {
			seeds := h.registry.GetSeeds()
			// Check if already exists
			for _, s := range seeds {
				if s == req.Address {
					jsonOK(w, map[string]string{"status": "ok"})
					return
				}
			}
			seeds = append(seeds, req.Address)
			if err := h.SetSeedsWithReplication(seeds); err != nil {
				httpError(w, http.StatusInternalServerError, "failed to sync seeds: "+err.Error())
				return
			}

			jsonOK(w, map[string]string{"status": "ok"})
			return
		}

		if mode == "cp" {
			if req.NodeID == "" {
				httpError(w, http.StatusBadRequest, "node_id required for CP mode")
				return
			}
			if err := h.cpNode.Join(req.NodeID, req.Address); err != nil {
				httpError(w, http.StatusInternalServerError, "failed to join raft cluster: "+err.Error())
				return
			}
		}

		jsonOK(w, map[string]string{"status": "ok"})
		return
	}

	if r.Method == http.MethodDelete {
		addr := r.URL.Query().Get("address")
		nodeID := r.URL.Query().Get("node_id")
		if addr == "" {
			httpError(w, http.StatusBadRequest, "address required")
			return
		}

		env := h.registry.GetEnvironment()
		if env == "standalone" || mode == "ap" {
			seeds := h.registry.GetSeeds()
			newSeeds := []string{}
			for _, s := range seeds {
				if s != addr {
					newSeeds = append(newSeeds, s)
				}
			}
			if err := h.SetSeedsWithReplication(newSeeds); err != nil {
				httpError(w, http.StatusInternalServerError, "failed to sync seeds: "+err.Error())
				return
			}

			jsonOK(w, map[string]string{"status": "ok"})
			return
		}

		if mode == "cp" {
			// In CP mode, we use NodeID to remove server
			// hashicorp/raft RemoveServer needs ServerID
			if nodeID == "" {
				httpError(w, http.StatusBadRequest, "node_id required for CP mode removal")
				return
			}
			if err := h.cpNode.RemoveServer(nodeID); err != nil {
				httpError(w, http.StatusInternalServerError, "failed to remove from raft: "+err.Error())
				return
			}
		}

		jsonOK(w, map[string]string{"status": "ok"})
		return
	}

	httpError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.registry.Stats()
	mode := h.registry.GetMode()
	env := h.registry.GetEnvironment()
	isLeader := true
	leaderAddr := h.normalizeAddr(h.config.HTTPAddr)
	nodeCount := 1

	if env == "cluster" {
		if mode == "cp" {
			isLeader = h.cpNode.IsLeader()
			if !isLeader {
				leaderAddr = h.cpNode.LeaderAddr()
			}
			
			m, _ := h.cpNode.Members()
			// Handle different possible slice types or nil
			if m != nil {
				// Try type assertion to both possible forms (slice of structs or slice of maps)
				if sm, ok := m.([]cp.ClusterMember); ok && len(sm) > 0 {
					nodeCount = len(sm)
				} else if mm, ok := m.([]map[string]string); ok && len(mm) > 0 {
					nodeCount = len(mm)
				}
			}
		} else {
			nodeCount = len(h.registry.GetSeeds()) + 1
		}
	}

	// Double check nodeCount is never less than 1
	if nodeCount < 1 { nodeCount = 1 }

	result := map[string]interface{}{
		"node_count":     nodeCount,
		"service_count":  stats.ServiceCount,
		"instance_count": stats.InstanceCount,
		"healthy_count":  stats.HealthyCount,
		"health_rate":    stats.HealthRate,
		"memory_usage":   stats.MemoryUsage,
		"is_leader":      isLeader,
		"leader_addr":    leaderAddr,
		"mode":           mode,
		"environment":    env,
	}
	jsonOK(w, result)
}

func (h *Handler) handleEvents(w http.ResponseWriter, r *http.Request) {
	events := h.registry.GetEvents(50)
	if events == nil {
		events = make([]*model.Event, 0)
	}
	jsonOK(w, events)
}

// SetSeedsWithReplication updates seeds and propagates to cluster
func (h *Handler) SetSeedsWithReplication(seeds []string) error {
	mode := h.registry.GetMode()
	
	// Local update
	h.registry.SetSeeds(seeds)
	
	if mode == "cp" {
		if h.cpNode != nil {
			cmd := cp.Command{Type: cp.CmdSetSeeds, Seeds: seeds}
			return h.cpNode.Apply(cmd, 5*time.Second)
		}
	} else {
		if h.apNode != nil {
			h.apNode.SyncSeeds()
			return h.apNode.Apply("set_seeds", seeds, false)
		}
	}
	return nil
}

package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
)

// ---------- Cluster Handlers (Membership & Stats) ----------

func (h *Handler) handleNodeInfo(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, h.config)
}

func (h *Handler) handleJoin(w http.ResponseWriter, r *http.Request) {
	mode := h.settings.GetMode()
	if mode == "ap" {
		jsonOK(w, map[string]string{"status": "ignored_in_ap_mode"})
		return
	}
	var req struct {
		NodeID  string `json:"node_id"`
		Address string `json:"address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return
	}
	if err := h.cluster.JoinCluster(req.NodeID, req.Address); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleMembers(w http.ResponseWriter, r *http.Request) {
	mode := h.settings.GetMode()
	env := h.settings.GetEnvironment()

	if env == "standalone" {
		localAddr := h.normalizeAddr(h.config.HTTPAddr)
		members := []map[string]string{
			{"id": h.config.NodeID, "address": localAddr, "role": "Standalone", "status": "Online"},
		}
		seeds := h.settings.GetSeeds()
		for i, seed := range seeds {
			if seed != "" {
				members = append(members, map[string]string{
					"id":      fmt.Sprintf("peer-node-%d", i+1),
					"address": seed,
					"role":    "Peer",
					"status":  "Offline",
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
		seeds := h.settings.GetSeeds()
		for i, seed := range seeds {
			if seed != "" {
				status := "Offline"
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
	
	members, err := h.cluster.GetMembers()
	if err != nil || members == nil {
		localAddr := h.normalizeAddr(h.config.HTTPAddr)
		members = []map[string]string{
			{"id": h.config.NodeID, "address": localAddr, "role": "Local", "status": "Online"},
		}
	}
	jsonOK(w, members)
}

func (h *Handler) handleMember(w http.ResponseWriter, r *http.Request) {
	mode := h.settings.GetMode()
	if r.Method == http.MethodPost {
		var req struct {
			NodeID  string   `json:"node_id"`
			Address string   `json:"address"`
			Seeds   []string `json:"seeds"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if len(req.Seeds) > 0 {
			if err := h.settings.SetSeeds(req.Seeds); err != nil {
				httpError(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonOK(w, map[string]string{"status": "ok"})
			return
		}

		if req.Address == "" {
			httpError(w, http.StatusBadRequest, "address required")
			return
		}

		env := h.settings.GetEnvironment()
		if env == "standalone" || mode == "ap" {
			seeds := h.settings.GetSeeds()
			for _, s := range seeds {
				if s == req.Address {
					jsonOK(w, map[string]string{"status": "ok"})
					return
				}
			}
			seeds = append(seeds, req.Address)
			if err := h.settings.SetSeeds(seeds); err != nil {
				httpError(w, http.StatusInternalServerError, err.Error())
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
			if err := h.cluster.JoinCluster(req.NodeID, req.Address); err != nil {
				h.handleLeaderRedirect(w, err)
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

		env := h.settings.GetEnvironment()
		if env == "standalone" || mode == "ap" {
			seeds := h.settings.GetSeeds()
			newSeeds := []string{}
			for _, s := range seeds {
				if s != addr {
					newSeeds = append(newSeeds, s)
				}
			}
			if err := h.settings.SetSeeds(newSeeds); err != nil {
				httpError(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonOK(w, map[string]string{"status": "ok"})
			return
		}

		if mode == "cp" {
			if nodeID == "" {
				httpError(w, http.StatusBadRequest, "node_id required for CP mode removal")
				return
			}
			if err := h.cluster.RemoveMember(nodeID); err != nil {
				h.handleLeaderRedirect(w, err)
				return
			}
		}
		jsonOK(w, map[string]string{"status": "ok"})
		return
	}

	httpError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.cluster.GetStats()
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	mode := h.settings.GetMode()
	env := h.settings.GetEnvironment()
	isLeader := h.cluster.IsLeader()
	leaderAddr := h.cluster.LeaderAddr()
	if leaderAddr == "" {
		leaderAddr = h.normalizeAddr(h.config.HTTPAddr)
	}
	
	nodeCount := 1
	if env == "cluster" {
		if mode == "cp" {
			members, _ := h.cluster.GetMembers()
			if members != nil {
				if m, ok := members.([]interface{}); ok {
					nodeCount = len(m)
				}
			}
		} else {
			nodeCount = len(h.settings.GetSeeds()) + 1
		}
	}

	if nodeCount < 1 { nodeCount = 1 }

	result := map[string]interface{}{
		"node_count":  nodeCount,
		"is_leader":   isLeader,
		"leader_addr": leaderAddr,
		"mode":        mode,
		"environment": env,
		"stats":       stats,
	}
	jsonOK(w, result)
}

func (h *Handler) handleEvents(w http.ResponseWriter, r *http.Request) {
	events, err := h.cluster.ListEvents()
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, events)
}

// Internal sync handlers
func (h *Handler) handleInternalSyncSeeds(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Seeds []string `json:"seeds"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	h.settings.SetSeeds(req.Seeds)
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleInternalSyncUsers(w http.ResponseWriter, r *http.Request) {
	var u model.User
	json.NewDecoder(r.Body).Decode(&u)
	h.settings.AddUser(&u)
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleInternalDeleteUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	h.settings.DeleteUser(req.Username)
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleInternalSyncAPIKey(w http.ResponseWriter, r *http.Request) {
	var k model.APIKey
	json.NewDecoder(r.Body).Decode(&k)
	h.settings.AddAPIKey(&k)
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleInternalDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key string `json:"key"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	h.settings.DeleteAPIKey(req.Key)
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleInternalSyncSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode        string `json:"mode,omitempty"`
		Environment string `json:"environment,omitempty"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Mode != "" {
		h.settings.SetMode(req.Mode)
	}
	if req.Environment != "" {
		h.settings.SetEnvironment(req.Environment)
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

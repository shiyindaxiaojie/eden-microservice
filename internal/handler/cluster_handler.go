package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
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
	if h.config.Mode == "ap" {
		localAddr := h.normalizeAddr(h.config.HTTPAddr)

		members := []map[string]string{
			{"id": h.config.NodeID, "address": localAddr, "role": "Local"},
		}
		for i, seed := range h.config.Seeds {
			if seed != "" {
				members = append(members, map[string]string{
					"id":      fmt.Sprintf("peer-node-%d", i+1),
					"address": seed,
					"role":    "Peer"},
				)
			}
		}
		jsonOK(w, members)
		return
	}
	members, err := h.cpNode.Members()
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, members)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.registry.Stats()
	mode := h.registry.GetMode()
	isLeader := true
	leaderAddr := h.normalizeAddr(h.config.HTTPAddr)
	nodeCount := 1

	if mode == "cp" {
		isLeader = h.cpNode.IsLeader()
		leaderAddr = h.cpNode.LeaderAddr()
		m, _ := h.cpNode.Members()
		nodeCount = len(m)
	} else {
		nodeCount = len(h.config.Seeds) + 1 // approximate
	}

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

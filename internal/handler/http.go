package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/ap"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/configs"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	raftpkg "github.com/shiyindaxiaojie/eden-go-registry/internal/raft"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/store"
)

// Handler serves the HTTP API for both AP and CP modes.
type Handler struct {
	config  *configs.Config
	cpNode  *raftpkg.Node
	apNode  *ap.Node
	registry *store.Registry
	mux     *http.ServeMux
}

// NewHandler creates a unified HTTP handler.
func NewHandler(cfg *configs.Config, registry *store.Registry, cp *raftpkg.Node, apNode *ap.Node) *Handler {
	h := &Handler{
		config:   cfg,
		registry: registry,
		cpNode:   cp,
		apNode:   apNode,
		mux:      http.NewServeMux(),
	}
	h.registerRoutes()
	return h
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS headers for frontend dev
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) registerRoutes() {
	// Catalog API
	h.mux.HandleFunc("/v1/catalog/register", h.handleRegister)
	h.mux.HandleFunc("/v1/catalog/deregister", h.handleDeregister)
	h.mux.HandleFunc("/v1/catalog/heartbeat", h.handleHeartbeat)
	h.mux.HandleFunc("/v1/catalog/services", h.handleListServices)
	h.mux.HandleFunc("/v1/catalog/service/", h.handleGetService)

	// Cluster API (Mostly meaningful for CP, but we'll expose for both)
	h.mux.HandleFunc("/v1/cluster/join", h.handleJoin)
	h.mux.HandleFunc("/v1/cluster/members", h.handleMembers)
	h.mux.HandleFunc("/v1/cluster/stats", h.handleStats)

	// Events
	h.mux.HandleFunc("/v1/events", h.handleEvents)
}

// ---------- Catalog Handlers ----------

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var inst model.Instance
	if err := json.NewDecoder(r.Body).Decode(&inst); err != nil {
		httpError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	if h.config.Mode == "cp" {
		cmd := raftpkg.Command{Type: raftpkg.CmdRegister, Instance: &inst}
		if err := h.cpNode.Apply(cmd, 5*time.Second); err != nil {
			h.handleLeaderRedirect(w, err)
			return
		}
	} else {
		// AP mode
		isReplicate := r.URL.Query().Get("replicate") == "true"
		h.apNode.Apply("register", &inst, isReplicate)
	}

	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleDeregister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req struct {
		ServiceName string `json:"service_name"`
		InstanceID  string `json:"instance_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return
	}

	if h.config.Mode == "cp" {
		cmd := raftpkg.Command{Type: raftpkg.CmdDeregister, ServiceName: req.ServiceName, InstanceID: req.InstanceID}
		if err := h.cpNode.Apply(cmd, 5*time.Second); err != nil {
			h.handleLeaderRedirect(w, err)
			return
		}
	} else {
		isReplicate := r.URL.Query().Get("replicate") == "true"
		data := map[string]string{"service_name": req.ServiceName, "instance_id": req.InstanceID}
		h.apNode.Apply("deregister", data, isReplicate)
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req struct {
		ServiceName string `json:"service_name"`
		InstanceID  string `json:"instance_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return
	}

	if h.config.Mode == "cp" {
		cmd := raftpkg.Command{Type: raftpkg.CmdHeartbeat, ServiceName: req.ServiceName, InstanceID: req.InstanceID}
		if err := h.cpNode.Apply(cmd, 5*time.Second); err != nil {
			h.handleLeaderRedirect(w, err)
			return
		}
	} else {
		isReplicate := r.URL.Query().Get("replicate") == "true"
		data := map[string]string{"service_name": req.ServiceName, "instance_id": req.InstanceID}
		h.apNode.Apply("heartbeat", data, isReplicate)
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleListServices(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, h.registry.ListServices())
}

func (h *Handler) handleGetService(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[len("/v1/catalog/service/"):]
	healthy := r.URL.Query().Get("passing") == "true"
	var instances []*model.Instance
	if healthy {
		instances = h.registry.GetServiceHealthy(name)
	} else {
		instances = h.registry.GetService(name)
	}
	if instances == nil {
		instances = []*model.Instance{}
	}
	jsonOK(w, instances)
}

// ---------- Cluster Handlers ----------

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
		localAddr := h.config.HTTPAddr
		if len(localAddr) > 0 && localAddr[0] == ':' {
			localAddr = "127.0.0.1" + localAddr
		}
		if !strings.HasPrefix(localAddr, "http") {
			localAddr = "http://" + localAddr
		}

		members := []map[string]string{
			{"id": h.config.NodeID, "address": localAddr, "role": "Peer(Local)"},
		}
		for i, seed := range h.config.Seeds {
			if seed != "" {
				members = append(members, map[string]string{
					"id":      fmt.Sprintf("peer-node-%d", i+1),
					"address": seed,
					"role":    "Peer(Remote)",
				})
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
	isLeader := true
	leaderAddr := h.config.HTTPAddr
	nodeCount := 1

	if h.config.Mode == "cp" {
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
		"is_leader":      isLeader,
		"leader_addr":    leaderAddr,
		"mode":           h.config.Mode,
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

// ---------- Helpers ----------

func (h *Handler) handleLeaderRedirect(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTemporaryRedirect)
	json.NewEncoder(w).Encode(map[string]string{
		"error":  err.Error(),
		"leader": h.cpNode.LeaderAddr(),
	})
}

func httpError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func jsonOK(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

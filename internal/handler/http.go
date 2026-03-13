package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/ap"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/configs"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	raftpkg "github.com/shiyindaxiaojie/eden-go-registry/internal/raft"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// Handler serves the HTTP API for both AP and CP modes.
type Handler struct {
	config   *configs.Config
	cpNode   *raftpkg.Node
	apNode   *ap.Node
	registry *store.Registry
	mux      *http.ServeMux
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
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	h.mux.ServeHTTP(w, r)
}

func (h *Handler) registerRoutes() {
	// --- Catalog API (Service Registration & Discovery) ---
	// Registration and management require API Key
	h.mux.Handle("/v1/catalog/register", h.APIKeyMiddleware(http.HandlerFunc(h.handleRegister)))
	h.mux.Handle("/v1/catalog/deregister", h.ConsolidatedAuthMiddleware("admin", "developer")(http.HandlerFunc(h.handleDeregister)))
	h.mux.Handle("/v1/catalog/heartbeat", h.APIKeyMiddleware(http.HandlerFunc(h.handleHeartbeat)))

	// Discovery is public (or you could protect it too if needed)
	h.mux.HandleFunc("/v1/catalog/services", h.handleListServices)
	h.mux.HandleFunc("/v1/catalog/service/", h.handleGetService)

	// --- Cluster & Management API (Console / Admin / Developer) ---
	// Protected by JWT and RBAC
	adminOnly := h.RBACMiddleware("admin")
	adminOrDev := h.RBACMiddleware("admin", "developer")

	h.mux.Handle("/v1/cluster/join", h.AuthMiddleware(adminOnly(http.HandlerFunc(h.handleJoin))))
	h.mux.Handle("/v1/cluster/members", h.AuthMiddleware(adminOrDev(http.HandlerFunc(h.handleMembers))))
	h.mux.Handle("/v1/cluster/stats", h.AuthMiddleware(adminOrDev(http.HandlerFunc(h.handleStats))))
	h.mux.Handle("/v1/events", h.AuthMiddleware(adminOrDev(http.HandlerFunc(h.handleEvents))))

	// Auth (Public)
	h.mux.HandleFunc("/v1/auth/login", h.handleLogin)

	// RBAC & Settings (Admin only)
	h.mux.Handle("/v1/rbac/users", h.AuthMiddleware(http.HandlerFunc(h.handleListUsers)))
	h.mux.Handle("/v1/rbac/user", h.AuthMiddleware(adminOnly(http.HandlerFunc(h.handleSaveUser))))
	h.mux.Handle("/v1/rbac/user/delete", h.AuthMiddleware(adminOnly(http.HandlerFunc(h.handleDeleteUser))))

	h.mux.Handle("/v1/settings/apikeys", h.AuthMiddleware(adminOnly(http.HandlerFunc(h.handleListAPIKeys))))
	h.mux.Handle("/v1/settings/apikey", h.AuthMiddleware(adminOnly(http.HandlerFunc(h.handleSaveAPIKey))))
	h.mux.Handle("/v1/settings/apikey/delete", h.AuthMiddleware(adminOnly(http.HandlerFunc(h.handleDeleteAPIKey))))
	h.mux.Handle("/v1/settings/mode", h.AuthMiddleware(adminOrDev(http.HandlerFunc(h.handleMode)))) // GET is ok for dev, POST restricted in handler
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

	// Set default datacenter if not provided
	if inst.Datacenter == "" {
		inst.Datacenter = h.config.Datacenter
	}

	mode := h.registry.GetMode()
	if mode == "cp" {
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

	mode := h.registry.GetMode()
	if mode == "cp" {
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

	mode := h.registry.GetMode()
	if mode == "cp" {
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

	// DC filtering
	dc := r.URL.Query().Get("dc")
	if dc != "" {
		filtered := []*model.Instance{}
		for _, inst := range instances {
			if inst.Datacenter == dc {
				filtered = append(filtered, inst)
			}
		}
		instances = filtered
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

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request")
		return
	}

	log.Printf("[Auth] Login attempt: %s", req.Username)

	// Look up user in dynamic registry storage or built-in seeded configs
	if user, ok := h.registry.GetUser(req.Username); ok {
		// The client sends SHA256(password). We compare this hash against our stored hash.
		// Our stored hash is either:
		// 1. bcrypt(SHA256(password)) -> Handled by CompareHashAndPassword
		// 2. SHA256(password)         -> Handled by direct comparison (legacy/startup seed)
		err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
		if err == nil || user.Password == req.Password {
			token, err := h.GenerateToken(user.Username, user.Role)
			if err != nil {
				httpError(w, http.StatusInternalServerError, "failed to generate token")
				return
			}
			nickname := user.Nickname
			if nickname == "" {
				nickname = user.Username
			}
			jsonOK(w, map[string]string{
				"token":    token,
				"role":     user.Role,
				"nickname": nickname,
			})
			return
		}
	}

	httpError(w, http.StatusUnauthorized, "invalid credentials")
}

// ---------- RBAC & Settings Handlers ----------

func (h *Handler) handleListUsers(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, h.registry.ListUsers())
}

func (h *Handler) handleSaveUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var u model.User
	json.NewDecoder(r.Body).Decode(&u)

	// Validate and Hash password
	if u.Username == "" {
		httpError(w, http.StatusBadRequest, "username required")
		return
	}

	existingUser, exists := h.registry.GetUser(u.Username)
	if exists {
		u.IsBuiltIn = existingUser.IsBuiltIn
		// If password is left empty on an edit, keep the old password
		if u.Password == "" {
			u.Password = existingUser.Password
		} else if u.Password != existingUser.Password {
			// Hash new password if changed
			hashed, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
			if err == nil {
				u.Password = string(hashed)
			}
		}
	} else if u.Password != "" {
		// New user, hash password
		hashed, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
		if err == nil {
			u.Password = string(hashed)
		}
	}

	if h.config.Mode == "cp" {
		cmd := raftpkg.Command{Type: raftpkg.CmdAddUser, User: &u}
		h.cpNode.Apply(cmd, 5*time.Second)
	} else {
		h.apNode.Apply("add_user", &u, r.URL.Query().Get("replicate") == "true")
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")

	user, exists := h.registry.GetUser(username)
	if !exists {
		httpError(w, http.StatusNotFound, "user not found")
		return
	}
	if user.IsBuiltIn {
		httpError(w, http.StatusForbidden, "built-in users cannot be deleted")
		return
	}

	if h.config.Mode == "cp" {
		cmd := raftpkg.Command{Type: raftpkg.CmdDeleteUser, Username: username}
		h.cpNode.Apply(cmd, 5*time.Second)
	} else {
		h.apNode.Apply("delete_user", username, r.URL.Query().Get("replicate") == "true")
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, h.registry.ListAPIKeys())
}

func (h *Handler) handleSaveAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var k model.APIKey
	json.NewDecoder(r.Body).Decode(&k)
	k.CreatedAt = time.Now().Unix()

	if h.config.Mode == "cp" {
		cmd := raftpkg.Command{Type: raftpkg.CmdAddAPIKey, APIKey: &k}
		h.cpNode.Apply(cmd, 5*time.Second)
	} else {
		h.apNode.Apply("add_api_key", &k, r.URL.Query().Get("replicate") == "true")
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	mode := h.registry.GetMode()
	if mode == "cp" {
		cmd := raftpkg.Command{Type: raftpkg.CmdDeleteAPIKey, Key: key}
		h.cpNode.Apply(cmd, 5*time.Second)
	} else {
		h.apNode.Apply("delete_api_key", key, r.URL.Query().Get("replicate") == "true")
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleMode(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		jsonOK(w, map[string]string{"mode": h.registry.GetMode()})
		return
	}
	if r.Method == http.MethodPost {
		adminOnly := h.RBACMiddleware("admin")
		adminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mode := r.URL.Query().Get("mode")
			if mode != "ap" && mode != "cp" {
				httpError(w, http.StatusBadRequest, "invalid mode: "+mode)
				return
			}

			if mode == "cp" {
				cmd := raftpkg.Command{Type: raftpkg.CmdSetMode, Mode: mode}
				h.cpNode.Apply(cmd, 5*time.Second)
			} else {
				h.apNode.Apply("set_mode", mode, r.URL.Query().Get("replicate") == "true")
			}
			jsonOK(w, map[string]string{"status": "ok"})
		})).ServeHTTP(w, r)
		return
	}
	httpError(w, http.StatusMethodNotAllowed, "Method not allowed")
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

func (h *Handler) normalizeAddr(addr string) string {
	if addr == "" {
		return ""
	}
	res := addr
	if res[0] == ':' {
		res = "127.0.0.1" + res
	}
	if !strings.HasPrefix(res, "http") {
		res = "http://" + res
	}
	return res
}

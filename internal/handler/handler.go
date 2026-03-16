package handler

import (
	"net/http"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/config"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/store"
)

// Handler serves the HTTP API for both AP and CP modes.
type Handler struct {
	config   *config.Config
	cpNode   CPNode
	apNode   APNode
	registry *store.Registry
	mux      *http.ServeMux
}

// NewHandler creates a unified HTTP handler.
func NewHandler(cfg *config.Config, registry *store.Registry, cpNode CPNode, apNode APNode) *Handler {
	h := &Handler{
		config:   cfg,
		registry: registry,
		cpNode:   cpNode,
		apNode:   apNode,
		mux:      http.NewServeMux(),
	}
	h.registerRoutes()
	return h
}

// ServeHTTP implements http.Handler with CORS support.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	h.mux.ServeHTTP(w, r)
}

// registerRoutes sets up all API route handlers with their respective middleware.
func (h *Handler) registerRoutes() {
	// --- Catalog API (Service Registration & Discovery) ---
	h.mux.Handle("/v1/catalog/register", h.APIKeyMiddleware(http.HandlerFunc(h.handleRegister)))
	h.mux.Handle("/v1/catalog/deregister", h.ConsolidatedAuthMiddleware("admin", "developer")(http.HandlerFunc(h.handleDeregister)))
	h.mux.Handle("/v1/catalog/heartbeat", h.APIKeyMiddleware(http.HandlerFunc(h.handleHeartbeat)))
	h.mux.HandleFunc("/v1/catalog/services", h.handleListServices)
	h.mux.HandleFunc("/v1/catalog/service/", h.handleGetService)

	// --- Cluster & Management API ---
	adminOnly := h.RBACMiddleware("admin")
	adminOrDev := h.RBACMiddleware("admin", "developer")

	h.mux.Handle("/v1/cluster/join", h.AuthMiddleware(adminOnly(http.HandlerFunc(h.handleJoin))))
	h.mux.Handle("/v1/cluster/members", h.AuthMiddleware(adminOrDev(http.HandlerFunc(h.handleMembers))))
	h.mux.Handle("/v1/cluster/member", h.AuthMiddleware(adminOnly(http.HandlerFunc(h.handleMember))))
	h.mux.Handle("/v1/cluster/stats", h.AuthMiddleware(adminOrDev(http.HandlerFunc(h.handleStats))))
	h.mux.Handle("/v1/events", h.AuthMiddleware(adminOrDev(http.HandlerFunc(h.handleEvents))))

	// --- Auth (Public) ---
	h.mux.HandleFunc("/v1/auth/login", h.handleLogin)

	// --- RBAC & Settings (Admin only) ---
	h.mux.Handle("/v1/rbac/users", h.AuthMiddleware(http.HandlerFunc(h.handleListUsers)))
	h.mux.Handle("/v1/rbac/user", h.AuthMiddleware(adminOnly(http.HandlerFunc(h.handleSaveUser))))
	h.mux.Handle("/v1/rbac/user/delete", h.AuthMiddleware(adminOnly(http.HandlerFunc(h.handleDeleteUser))))

	h.mux.Handle("/v1/settings/apikeys", h.AuthMiddleware(adminOnly(http.HandlerFunc(h.handleListAPIKeys))))
	h.mux.Handle("/v1/settings/apikey", h.AuthMiddleware(adminOnly(http.HandlerFunc(h.handleSaveAPIKey))))
	h.mux.Handle("/v1/settings/apikey/delete", h.AuthMiddleware(adminOnly(http.HandlerFunc(h.handleDeleteAPIKey))))
	h.mux.Handle("/v1/settings/mode", h.AuthMiddleware(adminOrDev(http.HandlerFunc(h.handleMode))))

	// --- Health Check ---
	h.mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
}

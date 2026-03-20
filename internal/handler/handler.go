package handler

import (
	"net/http"
	"sync"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/config"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/service"
)

// Handler serves the HTTP API for both AP and CP modes.
type Handler struct {
	config    *config.Config
	catalog   service.CatalogService
	auth      service.AuthService
	settings  service.SettingsService
	cluster   service.ClusterService
	nodeCache sync.Map // map[string]config.Config
	mux       *http.ServeMux
}

// NewHandler creates a unified HTTP handler.
func NewHandler(cfg *config.Config,
	catalog service.CatalogService,
	auth service.AuthService,
	settings service.SettingsService,
	cluster service.ClusterService) *Handler {
	h := &Handler{
		config:    cfg,
		catalog:   catalog,
		auth:      auth,
		settings:  settings,
		cluster:   cluster,
		nodeCache: sync.Map{},
		mux:       http.NewServeMux(),
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
	h.mux.Handle("/v1/catalog/instance/status", h.ConsolidatedAuthMiddleware("admin", "developer")(http.HandlerFunc(h.handleSetInstanceStatus)))
	h.mux.Handle("/v1/catalog/heartbeat", h.APIKeyMiddleware(http.HandlerFunc(h.handleHeartbeat)))
	h.mux.Handle("/v1/catalog/topology/report", h.APIKeyMiddleware(http.HandlerFunc(h.handleReportTopology)))
	h.mux.HandleFunc("/v1/catalog/services", h.handleListServices)
	h.mux.HandleFunc("/v1/catalog/service/", h.handleGetService)
	h.mux.HandleFunc("/v1/catalog/dependency-graph", h.handleDependencyGraph)
	h.mux.HandleFunc("/v1/catalog/topology", h.handleGetTopology)

	// --- Namespace API ---
	adminOrDev := h.RBACMiddleware("admin", "developer")
	h.mux.Handle("/v1/namespaces", h.AuthMiddleware(adminOrDev(http.HandlerFunc(h.handleListNamespaces))))
	h.mux.Handle("/v1/namespace", h.AuthMiddleware(adminOrDev(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			h.handleCreateNamespace(w, r)
		case http.MethodPut:
			h.handleUpdateNamespace(w, r)
		case http.MethodDelete:
			h.handleDeleteNamespace(w, r)
		default:
			httpError(w, http.StatusMethodNotAllowed, "POST, PUT or DELETE required")
		}
	}))))

	// --- Cluster & Management API ---
	h.mux.HandleFunc("/v1/node/info", h.handleNodeInfo)
	adminOnly := h.RBACMiddleware("admin")

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

	// --- Internal Sync (no auth, for inter-node communication) ---
	h.mux.HandleFunc("/internal/sync/seeds", h.handleInternalSyncSeeds)
	h.mux.HandleFunc("/internal/sync/users", h.handleInternalSyncUsers)
	h.mux.HandleFunc("/internal/sync/users/delete", h.handleInternalDeleteUser)
	h.mux.HandleFunc("/internal/sync/apikeys", h.handleInternalSyncAPIKey)
	h.mux.HandleFunc("/internal/sync/apikeys/delete", h.handleInternalDeleteAPIKey)
	h.mux.HandleFunc("/internal/sync/settings", h.handleInternalSyncSettings)

	// --- Health Check ---
	h.mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
}

package httpapi

import (
	"net/http"
	"sync"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/auth"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/catalog"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/cluster"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/config"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/settings"
)

// Handler serves the HTTP API for both AP and CP modes.
type Handler struct {
	config    *config.Config
	catalog   catalog.Registry
	auth      auth.Authenticator
	settings  settings.Controller
	cluster   cluster.Membership
	nodeCache sync.Map // map[string]config.Config
	mux       *http.ServeMux
}

// NewHandler creates a unified HTTP handler.
func NewHandler(cfg *config.Config,
	catalogRegistry catalog.Registry,
	authenticator auth.Authenticator,
	settingsController settings.Controller,
	clusterMembership cluster.Membership) *Handler {
	h := &Handler{
		config:    cfg,
		catalog:   catalogRegistry,
		auth:      authenticator,
		settings:  settingsController,
		cluster:   clusterMembership,
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
	h.mux.Handle("/v1/catalog/register", h.APIKey(http.HandlerFunc(h.registerService)))
	h.mux.Handle("/v1/catalog/instance/status", h.ConsolidatedAuth("admin", "developer")(http.HandlerFunc(h.setInstanceStatus)))
	h.mux.Handle("/v1/catalog/heartbeat", h.APIKey(http.HandlerFunc(h.heartbeat)))
	h.mux.Handle("/v1/catalog/topology/report", h.APIKey(http.HandlerFunc(h.reportTopology)))
	h.mux.HandleFunc("/v1/catalog/services", h.listServices)
	h.mux.HandleFunc("/v1/catalog/service/", h.getService)
	h.mux.HandleFunc("/v1/catalog/dependency-graph", h.getDependencyGraph)
	h.mux.HandleFunc("/v1/catalog/topology", h.getTopology)

	// --- Namespace API ---
	adminOrDev := h.RBAC("admin", "developer")
	h.mux.Handle("/v1/namespaces", h.Auth(adminOrDev(http.HandlerFunc(h.listNamespaces))))
	h.mux.Handle("/v1/namespace", h.Auth(adminOrDev(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			h.createNamespace(w, r)
		case http.MethodPut:
			h.updateNamespace(w, r)
		case http.MethodDelete:
			h.deleteNamespace(w, r)
		default:
			httpError(w, http.StatusMethodNotAllowed, "POST, PUT or DELETE required")
		}
	}))))

	// --- Cluster & Management API ---
	h.mux.HandleFunc("/v1/node/info", h.nodeInfo)
	adminOnly := h.RBAC("admin")

	h.mux.Handle("/v1/cluster/join", h.Auth(adminOnly(http.HandlerFunc(h.joinCluster))))
	h.mux.HandleFunc("/v1/cluster/members", h.listMembers)
	h.mux.Handle("/v1/cluster/member", h.Auth(adminOnly(http.HandlerFunc(h.manageMember))))
	h.mux.Handle("/v1/cluster/stats", h.Auth(adminOrDev(http.HandlerFunc(h.clusterStats))))
	h.mux.Handle("/v1/events", h.Auth(adminOrDev(http.HandlerFunc(h.listEvents))))
	h.mux.Handle("/v1/cluster/logs", h.Auth(adminOrDev(http.HandlerFunc(h.getLogs))))
	h.mux.Handle("/v1/cluster/log-files", h.Auth(adminOrDev(http.HandlerFunc(h.listLogFiles))))

	// --- Auth (Public & Profile) ---
	h.mux.HandleFunc("/v1/auth/login", h.login)
	h.mux.Handle("/v1/auth/profile", h.Auth(http.HandlerFunc(h.profile)))
	h.mux.Handle("/v1/auth/password", h.Auth(http.HandlerFunc(h.updatePassword)))

	// --- RBAC & Settings (Admin only) ---
	h.mux.Handle("/v1/rbac/users", h.Auth(http.HandlerFunc(h.listUsers)))
	h.mux.Handle("/v1/rbac/user", h.Auth(adminOnly(http.HandlerFunc(h.saveUser))))
	h.mux.Handle("/v1/rbac/user/delete", h.Auth(adminOnly(http.HandlerFunc(h.deleteUser))))

	h.mux.Handle("/v1/settings/apikeys", h.Auth(adminOnly(http.HandlerFunc(h.listAPIKeys))))
	h.mux.Handle("/v1/settings/apikey", h.Auth(adminOnly(http.HandlerFunc(h.saveAPIKey))))
	h.mux.Handle("/v1/settings/apikey/delete", h.Auth(adminOnly(http.HandlerFunc(h.deleteAPIKey))))
	h.mux.Handle("/v1/settings/mode", h.Auth(adminOrDev(http.HandlerFunc(h.mode))))
	h.mux.Handle("/v1/settings/system", h.Auth(adminOrDev(http.HandlerFunc(h.systemSettings))))
	h.mux.Handle("/v1/settings/storage", h.Auth(adminOrDev(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.getStorage(w, r)
		} else if r.Method == http.MethodPost {
			h.updateStorage(w, r)
		}
	}))))

	// --- Internal Sync (no auth, for inter-node communication) ---
	h.mux.HandleFunc("/internal/sync/seeds", h.syncSeeds)
	h.mux.HandleFunc("/internal/sync/users", h.syncUser)
	h.mux.HandleFunc("/internal/sync/users/delete", h.deleteSyncedUser)
	h.mux.HandleFunc("/internal/sync/apikeys", h.syncAPIKey)
	h.mux.HandleFunc("/internal/sync/apikeys/delete", h.deleteSyncedAPIKey)
	h.mux.HandleFunc("/internal/sync/settings", h.syncSettings)

	// --- Health Check ---
	h.mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
}

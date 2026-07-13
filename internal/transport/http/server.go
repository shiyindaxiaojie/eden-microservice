package httpapi

import (
	"net/http"
	"sync"

	consuladapter "github.com/shiyindaxiaojie/eden-registry/internal/adapter/consul"
	nacosadapter "github.com/shiyindaxiaojie/eden-registry/internal/adapter/nacos"
	"github.com/shiyindaxiaojie/eden-registry/internal/alert"
	"github.com/shiyindaxiaojie/eden-registry/internal/auth"
	"github.com/shiyindaxiaojie/eden-registry/internal/catalog"
	"github.com/shiyindaxiaojie/eden-registry/internal/cluster"
	"github.com/shiyindaxiaojie/eden-registry/internal/config"
	"github.com/shiyindaxiaojie/eden-registry/internal/configcenter"
	"github.com/shiyindaxiaojie/eden-registry/internal/gateway"
	"github.com/shiyindaxiaojie/eden-registry/internal/notify"
	"github.com/shiyindaxiaojie/eden-registry/internal/settings"
)

// Handler serves the HTTP API for both AP and CP modes.
type Handler struct {
	config         *config.Config
	catalog        catalog.Registry
	configs        configcenter.Service
	gateway        gateway.Service
	gatewayRuntime *gateway.Runtime
	auth           auth.Authenticator
	settings       settings.Controller
	cluster        cluster.Membership
	notify         *notify.Store
	notifyEngine   *notify.Engine
	alerts         *alert.Store
	alertEvaluator *alert.Evaluator
	consul         *consuladapter.HTTPAdapter
	nacos          *nacosadapter.HTTPAdapter
	nacosConfig    *nacosadapter.ConfigHTTPAdapter
	nodeCache      sync.Map // map[string]config.Config
	mux            *http.ServeMux
}

// NewHandler creates a unified HTTP handler.
func NewHandler(cfg *config.Config,
	eventState *catalog.State,
	catalogRegistry catalog.Registry,
	configService configcenter.Service,
	authenticator auth.Authenticator,
	settingsController settings.Controller,
	clusterMembership cluster.Membership) *Handler {
	notifyStore := notify.NewStore(settingsController)
	notifyEngine := notify.NewEngine()
	alertStore := alert.NewStore(settingsController)
	alertEvaluator := alert.NewEvaluator(alertStore, notifyStore, notifyEngine)

	h := &Handler{
		config:         cfg,
		catalog:        catalogRegistry,
		configs:        configService,
		auth:           authenticator,
		settings:       settingsController,
		cluster:        clusterMembership,
		notify:         notifyStore,
		notifyEngine:   notifyEngine,
		alerts:         alertStore,
		alertEvaluator: alertEvaluator,
		consul:         consuladapter.NewHTTPAdapter(cfg, catalogRegistry),
		nacos:          nacosadapter.NewHTTPAdapter(catalogRegistry),
		nacosConfig:    nacosadapter.NewConfigHTTPAdapter(configService),
		nodeCache:      sync.Map{},
		mux:            http.NewServeMux(),
	}
	if eventState != nil {
		eventState.SetOnEventCallback(alertEvaluator.Evaluate)
	}
	h.registerRoutes()
	return h
}

// SetGateway attaches the gateway control service and runtime after bootstrap
// has created the catalog-backed data plane.
func (h *Handler) SetGateway(service gateway.Service, runtime *gateway.Runtime) {
	h.gateway = service
	h.gatewayRuntime = runtime
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
	h.mux.Handle("/v1/catalog/deregister", h.APIKey(http.HandlerFunc(h.consul.DeregisterCatalogService)))
	h.mux.Handle("/v1/catalog/instance/status", h.ConsolidatedAuth("admin", "developer")(http.HandlerFunc(h.setInstanceStatus)))
	h.mux.Handle("/v1/catalog/heartbeat", h.APIKey(http.HandlerFunc(h.heartbeat)))
	h.mux.Handle("/v1/catalog/topology/report", h.APIKey(http.HandlerFunc(h.reportTopology)))
	h.mux.HandleFunc("/v1/catalog/services", h.listServices)
	h.mux.HandleFunc("/v1/catalog/service/", h.getService)
	h.mux.HandleFunc("/v1/catalog/datacenters", h.consul.ListDatacenters)
	h.mux.Handle("/v1/agent/service/register", h.APIKey(http.HandlerFunc(h.registerService)))
	h.mux.Handle("/v1/agent/service/deregister/", h.APIKey(http.HandlerFunc(h.consul.DeregisterAgentService)))
	h.mux.Handle("/v1/agent/check/update/", h.APIKey(http.HandlerFunc(h.consul.UpdateAgentCheck)))
	h.mux.Handle("/v1/agent/check/", h.APIKey(http.HandlerFunc(h.consul.UpdateAgentCheckLegacy)))
	h.mux.HandleFunc("/v1/health/service/", h.consul.GetHealthService)
	h.nacos.RegisterRoutes(h.mux, "", h.APIKey)
	h.nacos.RegisterRoutes(h.mux, "/nacos", h.APIKey)
	h.nacosConfig.RegisterRoutes(h.mux, "", h.APIKey)
	h.nacosConfig.RegisterRoutes(h.mux, "/nacos", h.APIKey)
	h.mux.HandleFunc("/v1/catalog/dependency-graph", h.getDependencyGraph)
	h.mux.HandleFunc("/v1/catalog/topology", h.getTopology)

	// --- Config Center API ---
	adminOrDev := h.RBAC("admin", "developer")
	h.mux.Handle("/v1/config", h.Auth(adminOrDev(http.HandlerFunc(h.configResource))))
	h.mux.Handle("/v1/configs", h.Auth(adminOrDev(http.HandlerFunc(h.listConfigs))))
	h.mux.Handle("/v1/config/history", h.Auth(adminOrDev(http.HandlerFunc(h.configHistory))))
	h.mux.Handle("/v1/config/listener", h.APIKey(http.HandlerFunc(h.configListener)))

	// --- Gateway API ---
	gatewayRead := h.RBAC("admin", "developer", "viewer")
	gatewayWrite := h.RBAC("admin")
	h.mux.Handle("/v1/gateway/routes", h.Auth(gatewayRead(http.HandlerFunc(h.gatewayRoutes))))
	h.mux.Handle("/v1/gateway/route", h.Auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			gatewayRead(http.HandlerFunc(h.gatewayRoute)).ServeHTTP(w, r)
			return
		}
		gatewayWrite(http.HandlerFunc(h.gatewayRoute)).ServeHTTP(w, r)
	})))
	h.mux.Handle("/v1/gateway/route/status", h.Auth(gatewayWrite(http.HandlerFunc(h.gatewayRouteStatus))))
	h.mux.Handle("/v1/gateway/history", h.Auth(gatewayRead(http.HandlerFunc(h.gatewayHistory))))
	h.mux.Handle("/v1/gateway/runtime", h.Auth(gatewayRead(http.HandlerFunc(h.gatewayRuntimeStatus))))

	// --- Namespace API ---
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
	h.mux.Handle("/v1/cluster/stats/history", h.Auth(adminOrDev(http.HandlerFunc(h.statsHistory))))
	h.mux.Handle("/v1/events", h.Auth(adminOrDev(http.HandlerFunc(h.listEvents))))
	h.mux.Handle("/v1/cluster/logs", h.Auth(adminOrDev(http.HandlerFunc(h.getLogs))))
	h.mux.Handle("/v1/cluster/log-files", h.Auth(adminOrDev(http.HandlerFunc(h.listLogFiles))))

	// --- Auth (Public & Profile) ---
	h.mux.HandleFunc("/v1/auth/login", h.login)
	h.mux.Handle("/v1/auth/profile", h.Auth(http.HandlerFunc(h.profile)))
	h.mux.Handle("/v1/auth/password", h.Auth(http.HandlerFunc(h.updatePassword)))
	h.mux.Handle("/v1/auth/guide-status", h.Auth(http.HandlerFunc(h.updateGuideStatus)))

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
	h.mux.Handle("/v1/notify/config", h.Auth(adminOrDev(http.HandlerFunc(h.notificationConfig))))
	h.mux.Handle("/v1/alert/config", h.Auth(adminOrDev(http.HandlerFunc(h.alertConfig))))
	h.mux.Handle("/v1/notify/test", h.Auth(adminOrDev(http.HandlerFunc(h.testNotification))))
	h.mux.Handle("/v1/notify/channel/test", h.Auth(adminOrDev(http.HandlerFunc(h.testChannelNotification))))

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

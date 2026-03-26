package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/catalog"
)

// ---------- Catalog Handlers (Service Registration & Discovery) ----------

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var inst catalog.Instance
	if err := json.NewDecoder(r.Body).Decode(&inst); err != nil {
		httpError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	// Set default datacenter if not provided
	if inst.Datacenter == "" {
		inst.Datacenter = h.config.Datacenter
	}

	if err := h.catalog.Register(&inst); err != nil {
		h.handleLeaderRedirect(w, err)
		return
	}

	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleSetInstanceStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req struct {
		Namespace   string `json:"namespace"`
		ServiceName string `json:"service_name"`
		InstanceID  string `json:"instance_id"`
		Status      string `json:"status"` // "online" or "offline"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if req.ServiceName == "" || req.InstanceID == "" {
		httpError(w, http.StatusBadRequest, "service_name and instance_id required")
		return
	}
	if req.Status != "online" && req.Status != "offline" {
		httpError(w, http.StatusBadRequest, "status must be 'online' or 'offline'")
		return
	}

	if err := h.catalog.SetInstanceStatus(req.Namespace, req.ServiceName, req.InstanceID, req.Status); err != nil {
		httpError(w, http.StatusNotFound, err.Error())
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req struct {
		Namespace   string `json:"namespace"`
		ServiceName string `json:"service_name"`
		InstanceID  string `json:"instance_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return
	}

	if err := h.catalog.Heartbeat(req.Namespace, req.ServiceName, req.InstanceID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			httpError(w, http.StatusNotFound, err.Error())
			return
		}
		h.handleLeaderRedirect(w, err)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleReportTopology(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req struct {
		Namespace       string   `json:"namespace"`
		ConsumerService string   `json:"consumer_service"`
		Providers       []string `json:"providers"`
		Checksum        string   `json:"checksum"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if req.ConsumerService == "" {
		httpError(w, http.StatusBadRequest, "consumer_service required")
		return
	}

	changed := h.catalog.ReportTopology(req.Namespace, req.ConsumerService, req.Providers, req.Checksum)
	jsonOK(w, map[string]interface{}{
		"status":  "ok",
		"changed": changed,
	})
}

func (h *Handler) handleListServices(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	services, err := h.catalog.ListServices(namespace)
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, services)
}

func (h *Handler) handleGetService(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[len("/v1/catalog/service/"):]
	// Strip trailing /subscribers if present
	if strings.HasSuffix(name, "/subscribers") {
		h.handleGetSubscribers(w, r)
		return
	}
	healthy := r.URL.Query().Get("passing") == "true"
	namespace := r.URL.Query().Get("namespace")
	consumerService := r.URL.Query().Get("consumer_service")
	if consumerService != "" {
		h.catalog.RecordDependency(namespace, consumerService, name)
	}

	instances, err := h.catalog.GetService(namespace, name, healthy)
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if instances == nil {
		instances = []*catalog.Instance{}
	}

	// DC filtering
	dc := r.URL.Query().Get("dc")
	if dc != "" {
		filtered := []*catalog.Instance{}
		for _, inst := range instances {
			if inst.Datacenter == dc {
				filtered = append(filtered, inst)
			}
		}
		instances = filtered
	}

	jsonOK(w, instances)
}

func (h *Handler) handleGetSubscribers(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/v1/catalog/service/"):]
	name := strings.TrimSuffix(path, "/subscribers")
	namespace := r.URL.Query().Get("namespace")

	subscribers := h.catalog.GetSubscribers(namespace, name)
	if subscribers == nil {
		subscribers = []string{}
	}
	jsonOK(w, subscribers)
}

func (h *Handler) handleDependencyGraph(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	namespace := r.URL.Query().Get("namespace")
	graph := h.catalog.GetDependencyGraph(namespace)
	jsonOK(w, graph)
}

func (h *Handler) handleGetTopology(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	namespace := r.URL.Query().Get("namespace")
	jsonOK(w, h.catalog.GetTopology(namespace))
}

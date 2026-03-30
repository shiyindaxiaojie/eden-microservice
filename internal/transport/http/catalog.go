package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	consulcompat "github.com/shiyindaxiaojie/eden-go-registry/internal/adapter/consul/compat"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/catalog"
)

// ---------- Catalog Handlers (Service Registration & Discovery) ----------

func (h *Handler) registerService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		httpError(w, http.StatusMethodNotAllowed, "POST or PUT required")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	compatInst, err := consulcompat.DecodeRegisterRequest(body, h.requestDatacenter(r), h.requestNamespace(r))
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	inst := catalog.Instance{
		ID:          compatInst.ID,
		ServiceName: compatInst.ServiceName,
		Namespace:   compatInst.Namespace,
		Host:        compatInst.Address,
		Port:        compatInst.Port,
		Weight:      compatInst.Weight,
		Datacenter:  compatInst.Datacenter,
		Metadata:    compatInst.Metadata,
	}
	if inst.Datacenter == "" {
		inst.Datacenter = h.config.Datacenter
	}
	if inst.Namespace == "" {
		inst.Namespace = h.requestNamespace(r)
	}

	if err := h.catalog.Register(&inst); err != nil {
		h.writeLeaderRedirect(w, err)
		return
	}

	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) setInstanceStatus(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) heartbeat(w http.ResponseWriter, r *http.Request) {
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
		h.writeLeaderRedirect(w, err)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) reportTopology(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) listServices(w http.ResponseWriter, r *http.Request) {
	namespace := h.requestNamespace(r)
	services, err := h.catalog.ListServices(namespace)
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	names := make([]string, 0, len(services))
	for _, item := range services {
		service, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := service["name"].(string)
		if name != "" {
			names = append(names, name)
		}
	}

	consulcompat.ApplyHeaders(w, uint64(len(names)))
	jsonOK(w, consulcompat.BuildServicesMap(names))
}

func (h *Handler) getService(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[len("/v1/catalog/service/"):]
	// Strip trailing /subscribers if present
	if strings.HasSuffix(name, "/subscribers") {
		h.getSubscribers(w, r)
		return
	}
	healthy := r.URL.Query().Get("passing") == "true" || r.URL.Query().Get("passing") == "1"
	namespace := h.requestNamespace(r)
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
	dc := h.requestDatacenter(r)
	if dc == h.config.Datacenter && r.URL.Query().Get("dc") == "" && r.URL.Query().Get("datacenter") == "" {
		dc = ""
	}
	if dc != "" {
		filtered := []*catalog.Instance{}
		for _, inst := range instances {
			if inst.Datacenter == dc {
				filtered = append(filtered, inst)
			}
		}
		instances = filtered
	}

	compatInstances := make([]consulcompat.Instance, 0, len(instances))
	for _, inst := range instances {
		compatInstances = append(compatInstances, consulcompat.Instance{
			ID:          inst.ID,
			ServiceName: inst.ServiceName,
			Namespace:   inst.Namespace,
			Address:     inst.Host,
			Port:        inst.Port,
			Weight:      inst.Weight,
			Metadata:    inst.Metadata,
			Status:      string(inst.Status),
			Datacenter:  inst.Datacenter,
		})
	}

	consulcompat.ApplyHeaders(w, uint64(len(compatInstances)))
	jsonOK(w, consulcompat.BuildCatalogServiceEnvelopes(compatInstances, h.requestTags(r)))
}

func (h *Handler) getSubscribers(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/v1/catalog/service/"):]
	name := strings.TrimSuffix(path, "/subscribers")
	namespace := r.URL.Query().Get("namespace")

	subscribers := h.catalog.GetSubscribers(namespace, name)
	if subscribers == nil {
		subscribers = []string{}
	}
	jsonOK(w, subscribers)
}

func (h *Handler) getDependencyGraph(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	namespace := r.URL.Query().Get("namespace")
	graph := h.catalog.GetDependencyGraph(namespace)
	jsonOK(w, graph)
}

func (h *Handler) getTopology(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	namespace := r.URL.Query().Get("namespace")
	jsonOK(w, h.catalog.GetTopology(namespace))
}

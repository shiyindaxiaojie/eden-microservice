package handler

import (
	"encoding/json"
	"net/http"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
)

// ---------- Catalog Handlers (Service Registration & Discovery) ----------

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

	if err := h.catalog.Register(&inst); err != nil {
		h.handleLeaderRedirect(w, err)
		return
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

	if err := h.catalog.Deregister(req.ServiceName, req.InstanceID); err != nil {
		h.handleLeaderRedirect(w, err)
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
		ServiceName string `json:"service_name"`
		InstanceID  string `json:"instance_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return
	}

	if err := h.catalog.Heartbeat(req.ServiceName, req.InstanceID); err != nil {
		h.handleLeaderRedirect(w, err)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleListServices(w http.ResponseWriter, r *http.Request) {
	services, err := h.catalog.ListServices()
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, services)
}

func (h *Handler) handleGetService(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[len("/v1/catalog/service/"):]
	healthy := r.URL.Query().Get("passing") == "true"
	
	instances, err := h.catalog.GetService(name, healthy)
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
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

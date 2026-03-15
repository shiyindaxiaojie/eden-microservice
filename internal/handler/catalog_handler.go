package handler

import (
	"encoding/json"
	"net/http"
	"time"

	cp "github.com/shiyindaxiaojie/eden-go-registry/internal/cluster/cp"
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

	mode := h.registry.GetMode()
	if mode == "cp" {
		cmd := cp.Command{Type: cp.CmdRegister, Instance: &inst}
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
		cmd := cp.Command{Type: cp.CmdDeregister, ServiceName: req.ServiceName, InstanceID: req.InstanceID}
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
		cmd := cp.Command{Type: cp.CmdHeartbeat, ServiceName: req.ServiceName, InstanceID: req.InstanceID}
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

// handleLeaderRedirect returns a redirect response pointing to the current Raft leader.
func (h *Handler) handleLeaderRedirect(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTemporaryRedirect)
	json.NewEncoder(w).Encode(map[string]string{
		"error":  err.Error(),
		"leader": h.cpNode.LeaderAddr(),
	})
}

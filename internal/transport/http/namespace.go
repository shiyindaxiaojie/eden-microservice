package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/shiyindaxiaojie/eden-registry/internal/catalog"
)

// ---------- Namespace Handlers ----------

func (h *Handler) listNamespaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	namespaces := h.catalog.ListNamespaces()
	jsonOK(w, namespaces)
}

func (h *Handler) createNamespace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var ns catalog.Namespace
	if err := json.NewDecoder(r.Body).Decode(&ns); err != nil {
		httpError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if ns.Name == "" {
		httpError(w, http.StatusBadRequest, "name is required")
		return
	}

	if !h.catalog.CreateNamespace(&ns) {
		httpError(w, http.StatusConflict, "namespace already exists")
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) updateNamespace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		httpError(w, http.StatusMethodNotAllowed, "PUT required")
		return
	}

	var ns catalog.Namespace
	if err := json.NewDecoder(r.Body).Decode(&ns); err != nil {
		httpError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if ns.Name == "" {
		httpError(w, http.StatusBadRequest, "name is required")
		return
	}

	if !h.catalog.UpdateNamespace(&ns) {
		httpError(w, http.StatusNotFound, "namespace not found")
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) deleteNamespace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		httpError(w, http.StatusMethodNotAllowed, "DELETE required")
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		httpError(w, http.StatusBadRequest, "name query parameter required")
		return
	}
	if name == catalog.DefaultNamespace {
		httpError(w, http.StatusForbidden, "cannot delete default namespace")
		return
	}

	if !h.catalog.DeleteNamespace(name) {
		httpError(w, http.StatusNotFound, "namespace not found")
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

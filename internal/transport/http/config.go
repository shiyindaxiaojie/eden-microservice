package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shiyindaxiaojie/eden-registry/internal/configcenter"
)

func configIdentityFromRequest(r *http.Request) configcenter.Identity {
	query := r.URL.Query()
	dataID := query.Get("data_id")
	if dataID == "" {
		dataID = query.Get("dataId")
	}
	return configcenter.Identity{
		Namespace: query.Get("namespace"),
		Group:     query.Get("group"),
		DataID:    dataID,
	}
}

func configOperator(r *http.Request) string {
	if username, ok := r.Context().Value(UserContextKey).(string); ok && strings.TrimSpace(username) != "" {
		return username
	}
	return "system"
}

func (h *Handler) configResource(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getConfig(w, r)
	case http.MethodPost, http.MethodPut:
		h.publishConfig(w, r)
	case http.MethodDelete:
		h.deleteConfig(w, r)
	default:
		httpError(w, http.StatusMethodNotAllowed, "GET, POST, PUT or DELETE required")
	}
}

func (h *Handler) getConfig(w http.ResponseWriter, r *http.Request) {
	resource, err := h.configs.Get(configIdentityFromRequest(r))
	if err != nil {
		writeConfigError(w, err)
		return
	}
	jsonOK(w, resource)
}

func (h *Handler) publishConfig(w http.ResponseWriter, r *http.Request) {
	var request configcenter.PublishRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		httpError(w, http.StatusBadRequest, "invalid config request")
		return
	}
	request.Operator = configOperator(r)
	resource, err := h.configs.Publish(request)
	if err != nil {
		writeConfigError(w, err)
		return
	}
	jsonOK(w, resource)
}

func (h *Handler) deleteConfig(w http.ResponseWriter, r *http.Request) {
	identity := configIdentityFromRequest(r)
	if identity.DataID == "" && r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&identity)
	}
	entry, err := h.configs.Delete(identity, configOperator(r))
	if err != nil {
		writeConfigError(w, err)
		return
	}
	jsonOK(w, entry)
}

func (h *Handler) listConfigs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	query := r.URL.Query()
	page, _ := strconv.Atoi(query.Get("page"))
	pageSize, _ := strconv.Atoi(query.Get("page_size"))
	search := query.Get("query")
	if search == "" {
		search = query.Get("q")
	}
	result, err := h.configs.List(configcenter.ListQuery{
		Namespace: query.Get("namespace"),
		Group:     query.Get("group"),
		Type:      query.Get("type"),
		Query:     search,
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		writeConfigError(w, err)
		return
	}
	jsonOK(w, result)
}

func (h *Handler) configHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	entries, err := h.configs.History(configIdentityFromRequest(r))
	if err != nil {
		writeConfigError(w, err)
		return
	}
	jsonOK(w, entries)
}

func (h *Handler) configListener(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var request struct {
		Targets   []configcenter.WatchTarget `json:"targets"`
		TimeoutMS int64                      `json:"timeout_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		httpError(w, http.StatusBadRequest, "invalid config listener request")
		return
	}
	timeout := time.Duration(request.TimeoutMS) * time.Millisecond
	changes, err := h.configs.Wait(request.Targets, timeout)
	if err != nil {
		writeConfigError(w, err)
		return
	}
	jsonOK(w, changes)
}

func writeConfigError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, configcenter.ErrNotFound):
		httpError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, configcenter.ErrConflict):
		httpError(w, http.StatusConflict, err.Error())
	case errors.Is(err, configcenter.ErrInvalidIdentity), errors.Is(err, configcenter.ErrTooManyTargets):
		httpError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, configcenter.ErrTooManyWaiters):
		httpError(w, http.StatusTooManyRequests, err.Error())
	default:
		httpError(w, http.StatusInternalServerError, err.Error())
	}
}

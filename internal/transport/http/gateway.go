package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/shiyindaxiaojie/eden-registry/internal/gateway"
)

func (h *Handler) gatewayRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	if !h.requireGateway(w) {
		return
	}
	page, err := positiveQueryInt(r, "page", 1)
	if err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}
	pageSize, err := positiveQueryInt(r, "page_size", 50)
	if err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}
	enabled, err := optionalQueryBool(r, "enabled")
	if err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.gateway.List(gateway.ListQuery{
		Namespace: r.URL.Query().Get("namespace"),
		Query:     r.URL.Query().Get("query"),
		Enabled:   enabled,
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		writeGatewayServiceError(w, err)
		return
	}
	jsonOK(w, result)
}

func (h *Handler) gatewayRoute(w http.ResponseWriter, r *http.Request) {
	if !h.requireGateway(w) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		route, err := h.gateway.Get(gateway.Identity{Namespace: r.URL.Query().Get("namespace"), ID: r.URL.Query().Get("id")})
		if err != nil {
			writeGatewayServiceError(w, err)
			return
		}
		jsonOK(w, route)
	case http.MethodPost:
		route, _, err := decodeGatewayRouteMutation(r)
		if err != nil {
			httpError(w, http.StatusBadRequest, "invalid gateway route request: "+err.Error())
			return
		}
		created, err := h.gateway.Create(gateway.CreateRequest{Route: route, Operator: gatewayOperator(r)})
		if err != nil {
			writeGatewayServiceError(w, err)
			return
		}
		jsonCreated(w, created)
	case http.MethodPut:
		route, expectedRevision, err := decodeGatewayRouteMutation(r)
		if err != nil {
			httpError(w, http.StatusBadRequest, "invalid gateway route request: "+err.Error())
			return
		}
		updated, err := h.gateway.Update(gateway.UpdateRequest{Route: route, ExpectedRevision: expectedRevision, Operator: gatewayOperator(r)})
		if err != nil {
			writeGatewayServiceError(w, err)
			return
		}
		jsonOK(w, updated)
	case http.MethodDelete:
		expectedRevision, err := requiredQueryUint(r, "expected_revision")
		if err != nil {
			httpError(w, http.StatusBadRequest, err.Error())
			return
		}
		deleted, err := h.gateway.Delete(gateway.Identity{Namespace: r.URL.Query().Get("namespace"), ID: r.URL.Query().Get("id")}, expectedRevision, gatewayOperator(r))
		if err != nil {
			writeGatewayServiceError(w, err)
			return
		}
		jsonOK(w, deleted)
	default:
		httpError(w, http.StatusMethodNotAllowed, "GET, POST, PUT or DELETE required")
	}
}

func (h *Handler) gatewayRouteStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	if !h.requireGateway(w) {
		return
	}
	var request struct {
		Namespace        string `json:"namespace"`
		ID               string `json:"id"`
		Enabled          bool   `json:"enabled"`
		ExpectedRevision uint64 `json:"expected_revision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		httpError(w, http.StatusBadRequest, "invalid gateway route status request: "+err.Error())
		return
	}
	updated, err := h.gateway.SetEnabled(gateway.Identity{Namespace: request.Namespace, ID: request.ID}, request.Enabled, request.ExpectedRevision, gatewayOperator(r))
	if err != nil {
		writeGatewayServiceError(w, err)
		return
	}
	jsonOK(w, updated)
}

func (h *Handler) gatewayHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	if !h.requireGateway(w) {
		return
	}
	history, err := h.gateway.History(gateway.Identity{Namespace: r.URL.Query().Get("namespace"), ID: r.URL.Query().Get("id")})
	if err != nil {
		writeGatewayServiceError(w, err)
		return
	}
	jsonOK(w, history)
}

func (h *Handler) gatewayRuntimeStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	if h.gatewayRuntime == nil {
		httpError(w, http.StatusServiceUnavailable, "gateway runtime is unavailable")
		return
	}
	statuses := h.gatewayRuntime.Status(r.URL.Query().Get("namespace"))
	dataPlaneEnabled := h.config != nil && h.config.Gateway.Enabled && strings.TrimSpace(h.config.Gateway.HTTP) != ""
	for index := range statuses {
		statuses[index].DataPlaneEnabled = dataPlaneEnabled
	}
	jsonOK(w, statuses)
}

func (h *Handler) requireGateway(w http.ResponseWriter) bool {
	if h.gateway == nil {
		httpError(w, http.StatusServiceUnavailable, "gateway service is unavailable")
		return false
	}
	return true
}

func decodeGatewayRouteMutation(r *http.Request) (gateway.Route, uint64, error) {
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		return gateway.Route{}, 0, err
	}
	var envelope struct {
		Route            json.RawMessage `json:"route"`
		ExpectedRevision uint64          `json:"expected_revision"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return gateway.Route{}, 0, err
	}
	routeBytes := raw
	if len(envelope.Route) != 0 && string(envelope.Route) != "null" {
		routeBytes = envelope.Route
	}
	var route gateway.Route
	if err := json.Unmarshal(routeBytes, &route); err != nil {
		return gateway.Route{}, 0, err
	}
	return route, envelope.ExpectedRevision, nil
}

func gatewayOperator(r *http.Request) string {
	if user, ok := r.Context().Value(UserContextKey).(string); ok && strings.TrimSpace(user) != "" {
		return user
	}
	return "system"
}

func writeGatewayServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, gateway.ErrInvalidRoute):
		httpError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, gateway.ErrNotFound):
		httpError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, gateway.ErrConflict), errors.Is(err, gateway.ErrAlreadyExists):
		httpError(w, http.StatusConflict, err.Error())
	default:
		httpError(w, http.StatusInternalServerError, "gateway operation failed")
	}
}

func jsonCreated(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(data)
}

func positiveQueryInt(r *http.Request, key string, fallback int) (int, error) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, errors.New(key + " must be a positive integer")
	}
	return parsed, nil
}

func requiredQueryUint(r *http.Request, key string) (uint64, error) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return 0, errors.New(key + " is required")
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 {
		return 0, errors.New(key + " must be a positive integer")
	}
	return parsed, nil
}

func optionalQueryBool(r *http.Request, key string) (*bool, error) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil, errors.New(key + " must be true or false")
	}
	return &parsed, nil
}

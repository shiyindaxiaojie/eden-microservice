package consul

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/shiyindaxiaojie/eden-registry/internal/adapter/consul/compat"
	"github.com/shiyindaxiaojie/eden-registry/internal/catalog"
	"github.com/shiyindaxiaojie/eden-registry/internal/config"
)

type HTTPAdapter struct {
	config  *config.Config
	catalog catalog.Registry
}

func NewHTTPAdapter(cfg *config.Config, catalogRegistry catalog.Registry) *HTTPAdapter {
	return &HTTPAdapter{
		config:  cfg,
		catalog: catalogRegistry,
	}
}

func (a *HTTPAdapter) DeregisterCatalogService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST or PUT required")
		return
	}

	var reqBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		httpError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	body, _ := json.Marshal(reqBody)
	req, err := compat.DecodeDeregisterRequest(body, requestNamespace(r))
	if err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := a.catalog.Deregister(req.Namespace, req.ServiceName, req.InstanceID); err != nil {
		httpError(w, http.StatusNotFound, err.Error())
		return
	}

	jsonOK(w, map[string]string{"status": "ok"})
}

func (a *HTTPAdapter) DeregisterAgentService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		httpError(w, http.StatusMethodNotAllowed, "PUT required")
		return
	}

	serviceID := strings.TrimPrefix(r.URL.Path, "/v1/agent/service/deregister/")
	if serviceID == "" {
		httpError(w, http.StatusBadRequest, "service id required")
		return
	}

	namespace := requestNamespace(r)
	if err := a.catalog.Deregister(namespace, "", serviceID); err != nil {
		httpError(w, http.StatusNotFound, err.Error())
		return
	}

	jsonOK(w, map[string]string{"status": "ok"})
}

func (a *HTTPAdapter) UpdateAgentCheckLegacy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		httpError(w, http.StatusMethodNotAllowed, "PUT required")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/agent/check/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		httpError(w, http.StatusBadRequest, "invalid check path")
		return
	}

	status := parts[0]
	switch status {
	case "pass":
		status = consulapi.HealthPassing
	case "warn":
		status = consulapi.HealthWarning
	case "fail":
		status = consulapi.HealthCritical
	default:
		httpError(w, http.StatusBadRequest, "unsupported check update")
		return
	}

	a.applyAgentCheckStatus(w, r, parts[1], status)
}

func (a *HTTPAdapter) UpdateAgentCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		httpError(w, http.StatusMethodNotAllowed, "PUT required")
		return
	}

	checkID := strings.TrimPrefix(r.URL.Path, "/v1/agent/check/update/")
	if checkID == "" {
		httpError(w, http.StatusBadRequest, "check id required")
		return
	}

	var req struct {
		Status string `json:"Status"`
		Output string `json:"Output"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	switch status {
	case "pass":
		status = consulapi.HealthPassing
	case "warn":
		status = consulapi.HealthWarning
	case "fail":
		status = consulapi.HealthCritical
	}
	if status == "" {
		status = consulapi.HealthPassing
	}

	a.applyAgentCheckStatus(w, r, checkID, status)
}

func (a *HTTPAdapter) GetHealthService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	serviceName := strings.TrimPrefix(r.URL.Path, "/v1/health/service/")
	namespace := requestNamespace(r)
	passingOnly := r.URL.Query().Get("passing") == "1" || r.URL.Query().Get("passing") == "true"

	instances, err := a.catalog.GetService(namespace, serviceName, passingOnly)
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if instances == nil {
		instances = []*catalog.Instance{}
	}

	dc := requestDatacenter(r, a.config.Datacenter)
	if dc == a.config.Datacenter && r.URL.Query().Get("dc") == "" && r.URL.Query().Get("datacenter") == "" {
		dc = ""
	}
	if dc != "" {
		filtered := make([]*catalog.Instance, 0, len(instances))
		for _, inst := range instances {
			if inst.Datacenter == dc {
				filtered = append(filtered, inst)
			}
		}
		instances = filtered
	}

	items := make([]compat.Instance, 0, len(instances))
	for _, inst := range instances {
		items = append(items, compat.Instance{
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

	compat.ApplyHeaders(w, uint64(len(items)))
	jsonOK(w, compat.BuildHealthServiceEntries(items, requestTags(r), passingOnly))
}

func (a *HTTPAdapter) ListDatacenters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	seen := make(map[string]struct{})
	if a.config.Datacenter != "" {
		seen[a.config.Datacenter] = struct{}{}
	}

	namespace := requestNamespace(r)
	names, err := a.serviceNames(namespace)
	if err == nil {
		for _, name := range names {
			instances, getErr := a.catalog.GetService(namespace, name, false)
			if getErr != nil {
				continue
			}
			for _, inst := range instances {
				if inst.Datacenter != "" {
					seen[inst.Datacenter] = struct{}{}
				}
			}
		}
	}

	datacenters := make([]string, 0, len(seen))
	for dc := range seen {
		datacenters = append(datacenters, dc)
	}
	sort.Strings(datacenters)

	compat.ApplyHeaders(w, uint64(len(datacenters)))
	jsonOK(w, datacenters)
}

func (a *HTTPAdapter) applyAgentCheckStatus(w http.ResponseWriter, r *http.Request, checkID, status string) {
	namespace := requestNamespace(r)
	if instanceID := serviceInstanceIDFromCheckID(checkID); instanceID != "" {
		switch status {
		case consulapi.HealthPassing:
			if err := a.catalog.SetInstanceStatus(namespace, "", instanceID, "online"); err != nil {
				httpError(w, http.StatusNotFound, err.Error())
				return
			}
			if err := a.catalog.Heartbeat(namespace, "", instanceID); err != nil {
				httpError(w, http.StatusNotFound, err.Error())
				return
			}
		default:
			if err := a.catalog.SetInstanceStatus(namespace, "", instanceID, "offline"); err != nil {
				httpError(w, http.StatusNotFound, err.Error())
				return
			}
		}

		jsonOK(w, map[string]string{"status": "ok"})
		return
	}

	inst, err := a.findInstance(namespace, func(inst *catalog.Instance) bool {
		return compat.StoredCheckID(inst.Metadata) == checkID
	})
	if err != nil {
		httpError(w, http.StatusNotFound, err.Error())
		return
	}

	switch status {
	case consulapi.HealthPassing:
		if err := a.catalog.SetInstanceStatus(namespace, inst.ServiceName, inst.ID, "online"); err != nil {
			httpError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := a.catalog.Heartbeat(namespace, inst.ServiceName, inst.ID); err != nil {
			httpError(w, http.StatusNotFound, err.Error())
			return
		}
	default:
		if err := a.catalog.SetInstanceStatus(namespace, inst.ServiceName, inst.ID, "offline"); err != nil {
			httpError(w, http.StatusNotFound, err.Error())
			return
		}
	}

	jsonOK(w, map[string]string{"status": "ok"})
}

func serviceInstanceIDFromCheckID(checkID string) string {
	if !strings.HasPrefix(checkID, "service:") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(checkID, "service:"))
}

func (a *HTTPAdapter) serviceNames(namespace string) ([]string, error) {
	services, err := a.catalog.ListServices(namespace)
	if err != nil {
		return nil, err
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
	sort.Strings(names)
	return names, nil
}

func (a *HTTPAdapter) findInstance(namespace string, match func(*catalog.Instance) bool) (*catalog.Instance, error) {
	names, err := a.serviceNames(namespace)
	if err != nil {
		return nil, err
	}
	for _, serviceName := range names {
		instances, getErr := a.catalog.GetService(namespace, serviceName, false)
		if getErr != nil {
			continue
		}
		for _, inst := range instances {
			if match(inst) {
				return inst, nil
			}
		}
	}
	return nil, fmt.Errorf("instance not found")
}

func requestNamespace(r *http.Request) string {
	if namespace := strings.TrimSpace(r.URL.Query().Get("namespace")); namespace != "" {
		return namespace
	}
	return strings.TrimSpace(r.URL.Query().Get("ns"))
}

func requestDatacenter(r *http.Request, fallback string) string {
	if dc := strings.TrimSpace(r.URL.Query().Get("dc")); dc != "" {
		return dc
	}
	if dc := strings.TrimSpace(r.URL.Query().Get("datacenter")); dc != "" {
		return dc
	}
	return fallback
}

func requestTags(r *http.Request) []string {
	raw := r.URL.Query()["tag"]
	if len(raw) == 0 {
		return nil
	}
	tags := make([]string, 0, len(raw))
	for _, tag := range raw {
		if trimmed := strings.TrimSpace(tag); trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	return tags
}

func httpError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func jsonOK(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(data)
}

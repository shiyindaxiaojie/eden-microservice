package nacos

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	nacoscompat "github.com/shiyindaxiaojie/eden-registry/internal/adapter/nacos/compat"
	"github.com/shiyindaxiaojie/eden-registry/internal/catalog"
)

type middleware func(http.Handler) http.Handler

type HTTPAdapter struct {
	catalog catalog.Registry
}

func NewHTTPAdapter(catalogRegistry catalog.Registry) *HTTPAdapter {
	return &HTTPAdapter{catalog: catalogRegistry}
}

func (a *HTTPAdapter) RegisterRoutes(mux *http.ServeMux, prefix string, wrap middleware) {
	route := func(path string) string {
		if prefix == "" {
			return path
		}
		return prefix + path
	}

	mux.Handle(route("/v1/ns/instance"), wrap(http.HandlerFunc(a.instance)))
	mux.Handle(route("/v1/ns/instance/beat"), wrap(http.HandlerFunc(a.beat)))
	mux.HandleFunc(route("/v1/ns/instance/list"), a.listInstances)
	mux.HandleFunc(route("/v1/ns/service/list"), a.serviceList)
	mux.HandleFunc(route("/v1/ns/operator/metrics"), a.metrics)
}

func (a *HTTPAdapter) instance(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost, http.MethodPut:
		a.registerInstance(w, r)
	case http.MethodDelete:
		a.deregisterInstance(w, r)
	default:
		httpError(w, http.StatusMethodNotAllowed, "POST, PUT or DELETE required")
	}
}

func (a *HTTPAdapter) registerInstance(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		httpError(w, http.StatusBadRequest, "invalid form: "+err.Error())
		return
	}

	req, err := nacoscompat.DecodeRegisterForm(r.Form)
	if err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	inst := &catalog.Instance{
		ID:          req.InstanceID,
		ServiceName: req.Service.FullName,
		Namespace:   req.Namespace,
		Host:        req.Address,
		Port:        req.Port,
		Weight:      req.Weight,
		Metadata:    req.Metadata,
	}

	if err := a.catalog.Register(inst); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !req.Healthy || !req.Enable {
		if err := a.catalog.SetInstanceStatus(req.Namespace, inst.ServiceName, inst.ID, "offline"); err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	jsonOK(w, true)
}

func (a *HTTPAdapter) deregisterInstance(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		httpError(w, http.StatusBadRequest, "invalid form: "+err.Error())
		return
	}

	req, err := nacoscompat.DecodeDeregisterForm(r.Form)
	if err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	inst, err := a.findInstance(req.Namespace, req.Service, req.ClusterName, req.Address, req.Port)
	if err != nil {
		jsonOK(w, true)
		return
	}

	if err := a.catalog.Deregister(req.Namespace, inst.ServiceName, inst.ID); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, true)
}

func (a *HTTPAdapter) beat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		httpError(w, http.StatusMethodNotAllowed, "PUT required")
		return
	}
	if err := r.ParseForm(); err != nil {
		httpError(w, http.StatusBadRequest, "invalid form: "+err.Error())
		return
	}

	req, err := nacoscompat.DecodeBeatForm(r.Form)
	if err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	inst, findErr := a.findInstance(req.Namespace, req.Service, req.ClusterName, req.Address, req.Port)
	if findErr != nil {
		registerInst := &catalog.Instance{
			ID:          req.InstanceID,
			ServiceName: req.Service.FullName,
			Namespace:   req.Namespace,
			Host:        req.Address,
			Port:        req.Port,
			Weight:      1,
			Metadata:    req.Metadata,
		}
		if err := a.catalog.Register(registerInst); err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonOK(w, map[string]interface{}{"clientBeatInterval": req.ClientBeatInterval})
		return
	}

	if err := a.catalog.Heartbeat(req.Namespace, inst.ServiceName, inst.ID); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, map[string]interface{}{"clientBeatInterval": req.ClientBeatInterval})
}

func (a *HTTPAdapter) listInstances(w http.ResponseWriter, r *http.Request) {
	ref := nacoscompat.ParseService(r.URL.Query().Get("serviceName"), r.URL.Query().Get("groupName"))
	namespace := nacoscompat.NormalizeNamespace(r.URL.Query().Get("namespaceId"))
	healthyOnly := r.URL.Query().Get("healthyOnly") == "true"
	clusters := r.URL.Query().Get("clusters")

	instances, _, err := a.serviceInstances(namespace, ref, healthyOnly)
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	payload := make([]nacoscompat.ServiceInstance, 0, len(instances))
	for _, inst := range instances {
		payload = append(payload, nacoscompat.ServiceInstance{
			ID:          inst.ID,
			ServiceName: inst.ServiceName,
			Address:     inst.Host,
			Port:        inst.Port,
			Weight:      inst.Weight,
			Metadata:    inst.Metadata,
			Healthy:     inst.Status == catalog.HealthPassing,
		})
	}

	jsonOK(w, nacoscompat.ToModelService(ref, clusters, payload))
}

func (a *HTTPAdapter) serviceList(w http.ResponseWriter, r *http.Request) {
	namespace := nacoscompat.NormalizeNamespace(r.URL.Query().Get("namespaceId"))
	groupName := r.URL.Query().Get("groupName")
	pageNo, pageSize := nacoscompat.ParsePagination(r.URL.Query().Get("pageNo"), r.URL.Query().Get("pageSize"))

	names, err := a.serviceNames(namespace)
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	filtered := make([]string, 0, len(names))
	for _, name := range names {
		ref := nacoscompat.ParseService(name, "")
		if groupName != "" && ref.GroupName != groupName {
			continue
		}
		filtered = append(filtered, name)
	}
	sort.Strings(filtered)

	jsonOK(w, nacoscompat.ToServiceList(filtered, pageNo, pageSize))
}

func (a *HTTPAdapter) metrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	jsonOK(w, map[string]string{"status": "UP"})
}

func (a *HTTPAdapter) serviceInstances(namespace string, ref nacoscompat.ServiceRef, healthyOnly bool) ([]*catalog.Instance, string, error) {
	for _, candidate := range nacoscompat.CandidateStoredNames(ref) {
		instances, err := a.catalog.GetService(namespace, candidate, healthyOnly)
		if err != nil {
			return nil, "", err
		}
		if len(instances) > 0 {
			return instances, candidate, nil
		}
	}
	return []*catalog.Instance{}, ref.FullName, nil
}

func (a *HTTPAdapter) findInstance(namespace string, ref nacoscompat.ServiceRef, clusterName, address string, port int) (*catalog.Instance, error) {
	instances, _, err := a.serviceInstances(namespace, ref, false)
	if err != nil {
		return nil, err
	}

	expectedID := nacoscompat.BuildInstanceID(ref, clusterName, address, port)
	targetCluster := clusterName
	if targetCluster == "" {
		targetCluster = nacoscompat.DefaultCluster
	}
	for _, inst := range instances {
		if inst.ID == expectedID {
			return inst, nil
		}
		if inst.Host == address && inst.Port == port && nacoscompat.ClusterName(inst.Metadata) == targetCluster {
			return inst, nil
		}
	}
	return nil, fmt.Errorf("instance not found")
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

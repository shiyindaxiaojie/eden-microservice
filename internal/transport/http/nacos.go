package httpapi

import (
	"fmt"
	"net/http"
	"sort"

	nacoscompat "github.com/shiyindaxiaojie/eden-go-registry/internal/adapter/nacos/compat"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/catalog"
)

func (h *Handler) registerNacosRoutes(prefix string) {
	route := func(path string) string {
		if prefix == "" {
			return path
		}
		return prefix + path
	}

	h.mux.Handle(route("/v1/ns/instance"), h.APIKey(http.HandlerFunc(h.nacosInstance)))
	h.mux.Handle(route("/v1/ns/instance/beat"), h.APIKey(http.HandlerFunc(h.nacosBeat)))
	h.mux.HandleFunc(route("/v1/ns/instance/list"), h.nacosListInstances)
	h.mux.HandleFunc(route("/v1/ns/service/list"), h.nacosServiceList)
	h.mux.HandleFunc(route("/v1/ns/operator/metrics"), h.nacosMetrics)
}

func (h *Handler) nacosInstance(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost, http.MethodPut:
		h.registerNacosInstance(w, r)
	case http.MethodDelete:
		h.deregisterNacosInstance(w, r)
	default:
		httpError(w, http.StatusMethodNotAllowed, "POST, PUT or DELETE required")
	}
}

func (h *Handler) registerNacosInstance(w http.ResponseWriter, r *http.Request) {
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

	if err := h.catalog.Register(inst); err != nil {
		h.writeLeaderRedirect(w, err)
		return
	}
	if !req.Healthy || !req.Enable {
		if err := h.catalog.SetInstanceStatus(req.Namespace, inst.ServiceName, inst.ID, "offline"); err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	jsonOK(w, true)
}

func (h *Handler) deregisterNacosInstance(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		httpError(w, http.StatusBadRequest, "invalid form: "+err.Error())
		return
	}

	req, err := nacoscompat.DecodeDeregisterForm(r.Form)
	if err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	inst, err := h.findNacosInstance(req.Namespace, req.Service, req.ClusterName, req.Address, req.Port)
	if err != nil {
		jsonOK(w, true)
		return
	}

	if err := h.catalog.Deregister(req.Namespace, inst.ServiceName, inst.ID); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, true)
}

func (h *Handler) nacosBeat(w http.ResponseWriter, r *http.Request) {
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

	inst, findErr := h.findNacosInstance(req.Namespace, req.Service, req.ClusterName, req.Address, req.Port)
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
		if err := h.catalog.Register(registerInst); err != nil {
			h.writeLeaderRedirect(w, err)
			return
		}
		jsonOK(w, map[string]interface{}{"clientBeatInterval": req.ClientBeatInterval})
		return
	}

	if err := h.catalog.Heartbeat(req.Namespace, inst.ServiceName, inst.ID); err != nil {
		h.writeLeaderRedirect(w, err)
		return
	}

	jsonOK(w, map[string]interface{}{"clientBeatInterval": req.ClientBeatInterval})
}

func (h *Handler) nacosListInstances(w http.ResponseWriter, r *http.Request) {
	ref := nacoscompat.ParseService(r.URL.Query().Get("serviceName"), r.URL.Query().Get("groupName"))
	namespace := nacoscompat.NormalizeNamespace(r.URL.Query().Get("namespaceId"))
	healthyOnly := r.URL.Query().Get("healthyOnly") == "true"
	clusters := r.URL.Query().Get("clusters")

	instances, _, err := h.nacosServiceInstances(namespace, ref, healthyOnly)
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

func (h *Handler) nacosServiceList(w http.ResponseWriter, r *http.Request) {
	namespace := nacoscompat.NormalizeNamespace(r.URL.Query().Get("namespaceId"))
	groupName := r.URL.Query().Get("groupName")
	pageNo, pageSize := nacoscompat.ParsePagination(r.URL.Query().Get("pageNo"), r.URL.Query().Get("pageSize"))

	names, err := h.serviceNames(namespace)
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

func (h *Handler) nacosMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	jsonOK(w, map[string]string{"status": "UP"})
}

func (h *Handler) nacosServiceInstances(namespace string, ref nacoscompat.ServiceRef, healthyOnly bool) ([]*catalog.Instance, string, error) {
	for _, candidate := range nacoscompat.CandidateStoredNames(ref) {
		instances, err := h.catalog.GetService(namespace, candidate, healthyOnly)
		if err != nil {
			return nil, "", err
		}
		if len(instances) > 0 {
			return instances, candidate, nil
		}
	}
	return []*catalog.Instance{}, ref.FullName, nil
}

func (h *Handler) findNacosInstance(namespace string, ref nacoscompat.ServiceRef, clusterName, address string, port int) (*catalog.Instance, error) {
	instances, _, err := h.nacosServiceInstances(namespace, ref, false)
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

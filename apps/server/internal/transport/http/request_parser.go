package httpapi

import (
	"net/http"
	"sort"
	"strings"
)

func (h *Handler) requestNamespace(r *http.Request) string {
	if namespace := strings.TrimSpace(r.URL.Query().Get("namespace")); namespace != "" {
		return namespace
	}
	return strings.TrimSpace(r.URL.Query().Get("ns"))
}

func (h *Handler) requestDatacenter(r *http.Request) string {
	if dc := strings.TrimSpace(r.URL.Query().Get("dc")); dc != "" {
		return dc
	}
	if dc := strings.TrimSpace(r.URL.Query().Get("datacenter")); dc != "" {
		return dc
	}
	return h.config.Datacenter
}

func (h *Handler) requestTags(r *http.Request) []string {
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

func (h *Handler) serviceNames(namespace string) ([]string, error) {
	services, err := h.catalog.ListServices(namespace)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(services))
	for _, item := range services {
		service, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := service["qualified_name"].(string)
		if name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

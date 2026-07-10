package compat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	consulapi "github.com/hashicorp/consul/api"
)

const (
	metadataTagsKey    = "__eden_consul_tags"
	metadataCheckIDKey = "__eden_consul_check_id"
)

// Instance is the shared shape used by the Consul compatibility layer.
type Instance struct {
	ID          string
	ServiceName string
	Group       string
	Namespace   string
	Node        string
	Address     string
	Port        int
	Weight      int
	Metadata    map[string]string
	Tags        []string
	CheckID     string
	Status        string
	ManualOffline bool
	Datacenter    string
}

// Deregistration captures the minimum data needed to take an instance offline.
type Deregistration struct {
	Namespace   string
	ServiceName string
	Group       string
	InstanceID  string
}

// CatalogServiceEnvelope keeps official Consul fields while preserving legacy registry fields.
type CatalogServiceEnvelope struct {
	*consulapi.CatalogService

	LegacyID          string            `json:"id,omitempty"`
	LegacyServiceName string            `json:"service_name,omitempty"`
	LegacyGroup       string            `json:"group,omitempty"`
	LegacyNamespace   string            `json:"namespace,omitempty"`
	LegacyHost        string            `json:"host,omitempty"`
	LegacyPort        int               `json:"port,omitempty"`
	LegacyWeight      int               `json:"weight,omitempty"`
	LegacyMetadata    map[string]string `json:"metadata,omitempty"`
	LegacyStatus        string            `json:"status,omitempty"`
	LegacyManualOffline bool              `json:"manual_offline,omitempty"`
	LegacyDatacenter    string            `json:"datacenter,omitempty"`
	LegacyDC          string            `json:"dc,omitempty"`
	LegacyTags        []string          `json:"tags,omitempty"`
}

type legacyCatalogInstance struct {
	ID          string            `json:"id"`
	ServiceName string            `json:"service_name"`
	Group       string            `json:"group,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Host        string            `json:"host"`
	Port        int               `json:"port"`
	Weight      int               `json:"weight"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Datacenter  string            `json:"datacenter,omitempty"`
	DC          string            `json:"dc,omitempty"`
	Status        string            `json:"status,omitempty"`
	ManualOffline bool              `json:"manual_offline,omitempty"`
}

type catalogServiceWire struct {
	ID               string            `json:"ID"`
	LegacyID         string            `json:"id"`
	ServiceID        string            `json:"ServiceID"`
	ServiceName      string            `json:"ServiceName"`
	LegacyName       string            `json:"service_name"`
	LegacyGroup      string            `json:"group"`
	Address          string            `json:"Address"`
	ServiceAddress   string            `json:"ServiceAddress"`
	LegacyHost       string            `json:"host"`
	ServicePort      int               `json:"ServicePort"`
	LegacyPort       int               `json:"port"`
	Datacenter       string            `json:"Datacenter"`
	LegacyDatacenter string            `json:"datacenter"`
	LegacyDC         string            `json:"dc"`
	ServiceMeta      map[string]string `json:"ServiceMeta"`
	LegacyMetadata   map[string]string `json:"metadata"`
	ServiceTags      []string          `json:"ServiceTags"`
	LegacyTags       []string          `json:"tags"`
	Namespace        string            `json:"Namespace"`
	LegacyNamespace  string            `json:"namespace"`
	Status           string            `json:"status"`
	ManualOffline    bool              `json:"manual_offline"`
	ServiceWeights   struct {
		Passing int `json:"Passing"`
		Warning int `json:"Warning"`
	} `json:"ServiceWeights"`
	LegacyWeight int `json:"weight"`
}

type legacyServiceSummary struct {
	Name string `json:"name"`
}

// DecodeRegisterRequest accepts either the legacy registry body or official Consul bodies.
func DecodeRegisterRequest(body []byte, defaultDatacenter, defaultNamespace string) (*Instance, error) {
	if inst, ok := decodeLegacyRegister(body, defaultDatacenter, defaultNamespace); ok {
		return inst, nil
	}
	if inst, ok := decodeCatalogRegister(body, defaultDatacenter, defaultNamespace); ok {
		return inst, nil
	}
	if inst, ok := decodeAgentRegister(body, defaultDatacenter, defaultNamespace); ok {
		return inst, nil
	}
	return nil, fmt.Errorf("invalid service registration payload")
}

// DecodeDeregisterRequest accepts either the legacy offline body or Consul catalog deregistration body.
func DecodeDeregisterRequest(body []byte, defaultNamespace string) (*Deregistration, error) {
	var legacy struct {
		Namespace   string `json:"namespace"`
		ServiceName string `json:"service_name"`
		Group       string `json:"group"`
		InstanceID  string `json:"instance_id"`
	}
	if err := json.Unmarshal(body, &legacy); err == nil && (legacy.ServiceName != "" || legacy.InstanceID != "") {
		if legacy.Namespace == "" {
			legacy.Namespace = defaultNamespace
		}
		return &Deregistration{
			Namespace:   legacy.Namespace,
			ServiceName: legacy.ServiceName,
			Group:       legacy.Group,
			InstanceID:  legacy.InstanceID,
		}, nil
	}

	var dereg consulapi.CatalogDeregistration
	if err := json.Unmarshal(body, &dereg); err == nil && dereg.ServiceID != "" {
		namespace := dereg.Namespace
		if namespace == "" {
			namespace = defaultNamespace
		}
		return &Deregistration{
			Namespace:  namespace,
			InstanceID: dereg.ServiceID,
		}, nil
	}

	return nil, fmt.Errorf("invalid service deregistration payload")
}

// DecodeServicesMap parses both the new Consul-compatible services map and the legacy summary list.
func DecodeServicesMap(body []byte) (map[string][]string, error) {
	var services map[string][]string
	if err := json.Unmarshal(body, &services); err == nil && services != nil {
		return services, nil
	}

	var summaries []legacyServiceSummary
	if err := json.Unmarshal(body, &summaries); err == nil {
		services = make(map[string][]string, len(summaries))
		for _, item := range summaries {
			if item.Name != "" {
				services[item.Name] = []string{}
			}
		}
		return services, nil
	}

	return nil, fmt.Errorf("invalid service list payload")
}

// DecodeCatalogInstances parses both legacy registry service rows and new Consul-compatible rows.
func DecodeCatalogInstances(body []byte) ([]Instance, error) {
	var rows []catalogServiceWire
	if err := json.Unmarshal(body, &rows); err == nil {
		result := make([]Instance, 0, len(rows))
		for _, row := range rows {
			name := firstNonEmpty(row.ServiceName, row.LegacyName)
			if name == "" {
				continue
			}
			address := firstNonEmpty(row.ServiceAddress, row.Address, row.LegacyHost)
			meta := copyMap(row.ServiceMeta)
			if len(meta) == 0 {
				meta = copyMap(row.LegacyMetadata)
			}
			tags := append([]string(nil), row.ServiceTags...)
			if len(tags) == 0 {
				tags = append([]string(nil), row.LegacyTags...)
			}
			weight := row.ServiceWeights.Passing
			if weight <= 0 {
				weight = row.LegacyWeight
			}
			result = append(result, normalizeInstance(&Instance{
				ID:          firstNonEmpty(row.ServiceID, row.ID, row.LegacyID),
				ServiceName: name,
				Group:       row.LegacyGroup,
				Namespace:   firstNonEmpty(row.Namespace, row.LegacyNamespace),
				Address:     address,
				Port:        firstNonZero(row.ServicePort, row.LegacyPort),
				Weight:      weight,
				Metadata:    meta,
				Tags:        tags,
				Status:        row.Status,
				ManualOffline: row.ManualOffline,
				Datacenter:  firstNonEmpty(row.Datacenter, row.LegacyDatacenter, row.LegacyDC),
			}))
		}
		return result, nil
	}

	var legacy []legacyCatalogInstance
	if err := json.Unmarshal(body, &legacy); err == nil {
		result := make([]Instance, 0, len(legacy))
		for _, row := range legacy {
			result = append(result, normalizeInstance(&Instance{
				ID:          row.ID,
				ServiceName: row.ServiceName,
				Group:       row.Group,
				Namespace:   row.Namespace,
				Address:     row.Host,
				Port:        row.Port,
				Weight:      row.Weight,
				Metadata:    copyMap(row.Metadata),
				Status:        row.Status,
				ManualOffline: row.ManualOffline,
				Datacenter:  firstNonEmpty(row.Datacenter, row.DC),
			}))
		}
		return result, nil
	}

	return nil, fmt.Errorf("invalid catalog service payload")
}

// BuildCatalogServiceEnvelopes returns Consul-compatible catalog rows plus legacy registry fields.
func BuildCatalogServiceEnvelopes(instances []Instance, requiredTags []string) []CatalogServiceEnvelope {
	result := make([]CatalogServiceEnvelope, 0, len(instances))
	for _, raw := range instances {
		inst := normalizeInstance(&raw)
		if !matchAllTags(inst.Tags, requiredTags) {
			continue
		}
		meta := PublicMetadata(inst.Metadata)
		tags := append([]string(nil), inst.Tags...)
		sort.Strings(tags)
		weight := inst.Weight
		if weight <= 0 {
			weight = 1
		}

		result = append(result, CatalogServiceEnvelope{
			CatalogService: &consulapi.CatalogService{
				ID:             inst.ID,
				Node:           inst.Node,
				Address:        inst.Address,
				Datacenter:     inst.Datacenter,
				ServiceID:      inst.ID,
				ServiceName:    inst.ServiceName,
				ServiceAddress: inst.Address,
				ServiceTags:    tags,
				ServiceMeta:    meta,
				ServicePort:    inst.Port,
				ServiceWeights: consulapi.Weights{
					Passing: weight,
					Warning: 1,
				},
				CreateIndex: 1,
				ModifyIndex: 1,
			},
			LegacyID:          inst.ID,
			LegacyServiceName: inst.ServiceName,
			LegacyGroup:       inst.Group,
			LegacyNamespace:   inst.Namespace,
			LegacyHost:        inst.Address,
			LegacyPort:        inst.Port,
			LegacyWeight:      weight,
			LegacyMetadata:    meta,
			LegacyStatus:      inst.Status,
			LegacyManualOffline: inst.ManualOffline,
			LegacyDatacenter:  inst.Datacenter,
			LegacyDC:          inst.Datacenter,
			LegacyTags:        tags,
		})
	}
	return result
}

// BuildHealthServiceEntries returns Consul-compatible health rows.
func BuildHealthServiceEntries(instances []Instance, requiredTags []string, passingOnly bool) []*consulapi.ServiceEntry {
	result := make([]*consulapi.ServiceEntry, 0, len(instances))
	for _, raw := range instances {
		inst := normalizeInstance(&raw)
		if !matchAllTags(inst.Tags, requiredTags) {
			continue
		}
		if passingOnly && inst.Status != consulapi.HealthPassing {
			continue
		}
		meta := PublicMetadata(inst.Metadata)
		tags := append([]string(nil), inst.Tags...)
		sort.Strings(tags)
		weight := inst.Weight
		if weight <= 0 {
			weight = 1
		}
		checkID := firstNonEmpty(inst.CheckID, "service:"+inst.ID)

		result = append(result, &consulapi.ServiceEntry{
			Node: &consulapi.Node{
				ID:         inst.ID,
				Node:       inst.Node,
				Address:    inst.Address,
				Datacenter: inst.Datacenter,
			},
			Service: &consulapi.AgentService{
				ID:      inst.ID,
				Service: inst.ServiceName,
				Address: inst.Address,
				Port:    inst.Port,
				Tags:    tags,
				Meta:    meta,
				Weights: consulapi.AgentWeights{
					Passing: weight,
					Warning: 1,
				},
				Datacenter: inst.Datacenter,
			},
			Checks: consulapi.HealthChecks{
				{
					Node:        inst.Node,
					CheckID:     checkID,
					Name:        "service:" + inst.ServiceName,
					Status:      inst.Status,
					ServiceID:   inst.ID,
					ServiceName: inst.ServiceName,
					ServiceTags: tags,
				},
			},
		})
	}
	return result
}

// BuildServicesMap returns the official /v1/catalog/services payload shape.
func BuildServicesMap(serviceNames []string) map[string][]string {
	sort.Strings(serviceNames)
	result := make(map[string][]string, len(serviceNames))
	for _, name := range serviceNames {
		if name != "" {
			result[name] = []string{}
		}
	}
	return result
}

// ApplyHeaders writes the minimum Consul metadata headers expected by the official SDK.
func ApplyHeaders(w http.ResponseWriter, index uint64) {
	w.Header().Set("X-Consul-Index", fmt.Sprintf("%d", index))
	w.Header().Set("X-Consul-LastContact", "0")
	w.Header().Set("X-Consul-KnownLeader", "true")
	w.Header().Set("X-Consul-Translate-Addresses", "false")
}

// PublicMetadata strips registry-internal compatibility keys.
func PublicMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	out := make(map[string]string, len(metadata))
	for key, value := range metadata {
		if key == metadataTagsKey || key == metadataCheckIDKey {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// StoredTags restores tags saved inside metadata.
func StoredTags(metadata map[string]string) []string {
	if len(metadata) == 0 {
		return nil
	}
	raw := strings.TrimSpace(metadata[metadataTagsKey])
	if raw == "" {
		return nil
	}
	tags := strings.Split(raw, "\x1f")
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		if trimmed := strings.TrimSpace(tag); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// StoredCheckID restores a check id saved inside metadata.
func StoredCheckID(metadata map[string]string) string {
	if len(metadata) == 0 {
		return ""
	}
	return strings.TrimSpace(metadata[metadataCheckIDKey])
}

func decodeLegacyRegister(body []byte, defaultDatacenter, defaultNamespace string) (*Instance, bool) {
	var legacy legacyCatalogInstance
	if err := json.Unmarshal(body, &legacy); err != nil || legacy.ServiceName == "" {
		return nil, false
	}
	return &Instance{
		ID:          legacy.ID,
		ServiceName: legacy.ServiceName,
		Group:       legacy.Group,
		Namespace:   firstNonEmpty(legacy.Namespace, defaultNamespace),
		Address:     legacy.Host,
		Port:        legacy.Port,
		Weight:      legacy.Weight,
		Metadata:    copyMap(legacy.Metadata),
		Status:      firstNonEmpty(legacy.Status, consulapi.HealthPassing),
		Datacenter:  firstNonEmpty(legacy.Datacenter, legacy.DC, defaultDatacenter),
	}, true
}

func decodeCatalogRegister(body []byte, defaultDatacenter, defaultNamespace string) (*Instance, bool) {
	var reg consulapi.CatalogRegistration
	if err := json.Unmarshal(body, &reg); err != nil || reg.Service == nil || reg.Service.Service == "" {
		return nil, false
	}
	checkID := ""
	switch {
	case reg.Check != nil && reg.Check.CheckID != "":
		checkID = reg.Check.CheckID
	case reg.Check != nil && reg.Check.ServiceID != "":
		checkID = "service:" + reg.Check.ServiceID
	case len(reg.Checks) > 0 && reg.Checks[0] != nil && reg.Checks[0].CheckID != "":
		checkID = reg.Checks[0].CheckID
	}

	return &Instance{
		ID:          firstNonEmpty(reg.Service.ID, reg.ID),
		ServiceName: reg.Service.Service,
		Namespace:   firstNonEmpty(reg.Service.Namespace, defaultNamespace),
		Node:        reg.Node,
		Address:     firstNonEmpty(reg.Service.Address, reg.Address),
		Port:        reg.Service.Port,
		Weight:      reg.Service.Weights.Passing,
		Metadata:    withCompatibilityMetadata(reg.Service.Meta, reg.Service.Tags, checkID),
		Tags:        append([]string(nil), reg.Service.Tags...),
		CheckID:     checkID,
		Status:      consulapi.HealthPassing,
		Datacenter:  firstNonEmpty(reg.Datacenter, reg.Service.Datacenter, defaultDatacenter),
	}, true
}

func decodeAgentRegister(body []byte, defaultDatacenter, defaultNamespace string) (*Instance, bool) {
	var reg consulapi.AgentServiceRegistration
	if err := json.Unmarshal(body, &reg); err != nil || reg.Name == "" {
		return nil, false
	}

	checkID := ""
	switch {
	case reg.Check != nil && reg.Check.CheckID != "":
		checkID = reg.Check.CheckID
	case reg.Check != nil && reg.Check.TTL != "":
		checkID = "service:" + firstNonEmpty(reg.ID, reg.Name)
	case len(reg.Checks) > 0 && reg.Checks[0] != nil && reg.Checks[0].CheckID != "":
		checkID = reg.Checks[0].CheckID
	case len(reg.Checks) > 0 && reg.Checks[0] != nil && reg.Checks[0].TTL != "":
		checkID = "service:" + firstNonEmpty(reg.ID, reg.Name)
	}

	return &Instance{
		ID:          firstNonEmpty(reg.ID, reg.Name),
		ServiceName: reg.Name,
		Namespace:   firstNonEmpty(reg.Namespace, defaultNamespace),
		Address:     reg.Address,
		Port:        reg.Port,
		Weight:      passingWeight(reg.Weights),
		Metadata:    withCompatibilityMetadata(reg.Meta, reg.Tags, checkID),
		Tags:        append([]string(nil), reg.Tags...),
		CheckID:     checkID,
		Status:      consulapi.HealthPassing,
		Datacenter:  defaultDatacenter,
	}, true
}

func normalizeInstance(inst *Instance) Instance {
	if inst == nil {
		return Instance{}
	}
	out := *inst
	if out.Weight <= 0 {
		out.Weight = 1
	}
	if out.Status == "" {
		out.Status = consulapi.HealthPassing
	}
	if out.Status == "offline" {
		out.ManualOffline = true
	}
	if len(out.Tags) == 0 {
		out.Tags = StoredTags(out.Metadata)
	}
	if out.CheckID == "" {
		out.CheckID = StoredCheckID(out.Metadata)
	}
	if out.Node == "" {
		out.Node = firstNonEmpty(out.Address, out.ID, out.ServiceName)
	}
	out.Metadata = copyMap(out.Metadata)
	return out
}

func withCompatibilityMetadata(metadata map[string]string, tags []string, checkID string) map[string]string {
	out := copyMap(metadata)
	if out == nil {
		out = make(map[string]string)
	}
	if len(tags) > 0 {
		out[metadataTagsKey] = strings.Join(tags, "\x1f")
	}
	if checkID != "" {
		out[metadataCheckIDKey] = checkID
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func passingWeight(weights *consulapi.AgentWeights) int {
	if weights == nil || weights.Passing <= 0 {
		return 1
	}
	return weights.Passing
}

func matchAllTags(candidate, required []string) bool {
	if len(required) == 0 {
		return true
	}
	available := make(map[string]struct{}, len(candidate))
	for _, tag := range candidate {
		available[tag] = struct{}{}
	}
	for _, tag := range required {
		if _, ok := available[tag]; !ok {
			return false
		}
	}
	return true
}

func copyMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

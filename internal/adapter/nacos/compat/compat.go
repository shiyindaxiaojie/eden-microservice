package compat

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	nacosconstant "github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	nacosmodel "github.com/nacos-group/nacos-sdk-go/v2/model"
)

const (
	DefaultCluster           = "DEFAULT"
	DefaultBeatIntervalMilli = 5000
	ReservedClusterKey       = "__eden_nacos_cluster"
	ReservedEphemeralKey     = "__eden_nacos_ephemeral"
)

type ServiceRef struct {
	Name      string
	GroupName string
	FullName  string
}

type RegisterInstance struct {
	Namespace   string
	Service     ServiceRef
	ClusterName string
	Address     string
	Port        int
	Weight      int
	Metadata    map[string]string
	Healthy     bool
	Enable      bool
	Ephemeral   bool
	InstanceID  string
}

type DeregisterInstance struct {
	Namespace   string
	Service     ServiceRef
	ClusterName string
	Address     string
	Port        int
	Ephemeral   bool
	InstanceID  string
}

type BeatInfo struct {
	Namespace          string
	Service            ServiceRef
	ClusterName        string
	Address            string
	Port               int
	Metadata           map[string]string
	InstanceID         string
	ClientBeatInterval int64
}

type ServiceInstance struct {
	ID          string
	ServiceName string
	Address     string
	Port        int
	Weight      int
	Metadata    map[string]string
	Healthy     bool
}

func NormalizeNamespace(namespace string) string {
	value := strings.TrimSpace(namespace)
	if value == "" || strings.EqualFold(value, nacosconstant.DEFAULT_NAMESPACE_ID) {
		return ""
	}
	return value
}

func ParseService(serviceName, groupName string) ServiceRef {
	name := strings.TrimSpace(serviceName)
	group := strings.TrimSpace(groupName)

	if left, right, ok := strings.Cut(name, nacosconstant.SERVICE_INFO_SPLITER); ok {
		if group == "" {
			group = left
		}
		name = right
	}

	if group == "" {
		group = nacosconstant.DEFAULT_GROUP
	}

	return ServiceRef{
		Name:      name,
		GroupName: group,
		FullName:  group + nacosconstant.SERVICE_INFO_SPLITER + name,
	}
}

func CandidateStoredNames(ref ServiceRef) []string {
	candidates := []string{ref.FullName}
	if ref.GroupName == nacosconstant.DEFAULT_GROUP && ref.Name != "" {
		candidates = append(candidates, ref.Name)
	}
	return candidates
}

func NormalizeListedServiceName(stored string) string {
	return ParseService(stored, "").FullName
}

func BuildInstanceID(ref ServiceRef, clusterName, address string, port int) string {
	cluster := normalizeCluster(clusterName)
	return fmt.Sprintf("%s#%s#%s#%d", ref.FullName, cluster, address, port)
}

func MetadataWithRuntime(metadata map[string]string, clusterName string, ephemeral bool) map[string]string {
	cloned := UserMetadata(metadata)
	cloned[ReservedClusterKey] = normalizeCluster(clusterName)
	cloned[ReservedEphemeralKey] = strconv.FormatBool(ephemeral)
	return cloned
}

func UserMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return map[string]string{}
	}

	cloned := make(map[string]string, len(metadata))
	for key, value := range metadata {
		switch key {
		case ReservedClusterKey, ReservedEphemeralKey:
			continue
		default:
			cloned[key] = value
		}
	}
	return cloned
}

func ClusterName(metadata map[string]string) string {
	if value := strings.TrimSpace(metadata[ReservedClusterKey]); value != "" {
		return value
	}
	return DefaultCluster
}

func IsEphemeral(metadata map[string]string) bool {
	value := strings.TrimSpace(metadata[ReservedEphemeralKey])
	if value == "" {
		return false
	}
	ok, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return ok
}

func DecodeRegisterForm(values url.Values) (RegisterInstance, error) {
	ref := ParseService(values.Get("serviceName"), values.Get("groupName"))
	if ref.Name == "" {
		return RegisterInstance{}, fmt.Errorf("serviceName required")
	}

	address := strings.TrimSpace(values.Get("ip"))
	if address == "" {
		return RegisterInstance{}, fmt.Errorf("ip required")
	}

	port, err := parseRequiredInt(values.Get("port"), "port")
	if err != nil {
		return RegisterInstance{}, err
	}

	weight := parseWeight(values.Get("weight"))
	healthy := parseBoolDefault(values.Get("healthy"), true)
	enable := parseBoolDefault(values.Get("enable"), parseBoolDefault(values.Get("enabled"), true))
	ephemeral := parseBoolDefault(values.Get("ephemeral"), false)
	cluster := normalizeCluster(values.Get("clusterName"))
	metadata, err := parseMetadata(values.Get("metadata"))
	if err != nil {
		return RegisterInstance{}, err
	}

	return RegisterInstance{
		Namespace:   NormalizeNamespace(values.Get("namespaceId")),
		Service:     ref,
		ClusterName: cluster,
		Address:     address,
		Port:        port,
		Weight:      weight,
		Metadata:    MetadataWithRuntime(metadata, cluster, ephemeral),
		Healthy:     healthy,
		Enable:      enable,
		Ephemeral:   ephemeral,
		InstanceID:  BuildInstanceID(ref, cluster, address, port),
	}, nil
}

func DecodeDeregisterForm(values url.Values) (DeregisterInstance, error) {
	ref := ParseService(values.Get("serviceName"), values.Get("groupName"))
	if ref.Name == "" {
		return DeregisterInstance{}, fmt.Errorf("serviceName required")
	}

	address := strings.TrimSpace(values.Get("ip"))
	if address == "" {
		return DeregisterInstance{}, fmt.Errorf("ip required")
	}

	port, err := parseRequiredInt(values.Get("port"), "port")
	if err != nil {
		return DeregisterInstance{}, err
	}

	cluster := normalizeCluster(firstNonEmpty(values.Get("clusterName"), values.Get("cluster")))
	ephemeral := parseBoolDefault(values.Get("ephemeral"), false)

	return DeregisterInstance{
		Namespace:   NormalizeNamespace(values.Get("namespaceId")),
		Service:     ref,
		ClusterName: cluster,
		Address:     address,
		Port:        port,
		Ephemeral:   ephemeral,
		InstanceID:  BuildInstanceID(ref, cluster, address, port),
	}, nil
}

func DecodeBeatForm(values url.Values) (BeatInfo, error) {
	type rawBeat struct {
		IP          string            `json:"ip"`
		Port        uint64            `json:"port"`
		ServiceName string            `json:"serviceName"`
		Cluster     string            `json:"cluster"`
		Metadata    map[string]string `json:"metadata"`
	}

	var beat rawBeat
	if raw := strings.TrimSpace(values.Get(nacosconstant.KEY_BEAT)); raw != "" {
		if err := json.Unmarshal([]byte(raw), &beat); err != nil {
			return BeatInfo{}, fmt.Errorf("invalid beat: %w", err)
		}
	}

	serviceName := firstNonEmpty(beat.ServiceName, values.Get("serviceName"))
	ref := ParseService(serviceName, values.Get("groupName"))
	if ref.Name == "" {
		return BeatInfo{}, fmt.Errorf("serviceName required")
	}

	address := strings.TrimSpace(firstNonEmpty(beat.IP, values.Get("ip")))
	if address == "" {
		return BeatInfo{}, fmt.Errorf("ip required")
	}

	port := int(beat.Port)
	if port == 0 {
		parsed, err := parseRequiredInt(values.Get("port"), "port")
		if err != nil {
			return BeatInfo{}, err
		}
		port = parsed
	}

	cluster := normalizeCluster(firstNonEmpty(beat.Cluster, values.Get("clusterName"), values.Get("cluster")))
	metadata := MetadataWithRuntime(beat.Metadata, cluster, parseBoolDefault(values.Get("ephemeral"), false))

	return BeatInfo{
		Namespace:          NormalizeNamespace(values.Get("namespaceId")),
		Service:            ref,
		ClusterName:        cluster,
		Address:            address,
		Port:               port,
		Metadata:           metadata,
		InstanceID:         BuildInstanceID(ref, cluster, address, port),
		ClientBeatInterval: DefaultBeatIntervalMilli,
	}, nil
}

func ToModelService(ref ServiceRef, requestedClusters string, instances []ServiceInstance) nacosmodel.Service {
	clusterFilter := splitClusters(requestedClusters)
	hosts := make([]nacosmodel.Instance, 0, len(instances))

	for _, inst := range instances {
		cluster := ClusterName(inst.Metadata)
		if len(clusterFilter) > 0 && !clusterFilter[cluster] {
			continue
		}

		weight := inst.Weight
		if weight <= 0 {
			weight = 1
		}

		hosts = append(hosts, nacosmodel.Instance{
			InstanceId:  inst.ID,
			Ip:          inst.Address,
			Port:        uint64(inst.Port),
			Weight:      float64(weight),
			Healthy:     inst.Healthy,
			Enable:      true,
			Ephemeral:   IsEphemeral(inst.Metadata),
			ClusterName: cluster,
			ServiceName: ref.Name,
			Metadata:    UserMetadata(inst.Metadata),
		})
	}

	sort.Slice(hosts, func(i, j int) bool {
		if hosts[i].Ip == hosts[j].Ip {
			return hosts[i].Port < hosts[j].Port
		}
		return hosts[i].Ip < hosts[j].Ip
	})

	return nacosmodel.Service{
		CacheMillis:              10000,
		Hosts:                    hosts,
		LastRefTime:              uint64(time.Now().UnixMilli()),
		Clusters:                 requestedClusters,
		Name:                     ref.Name,
		GroupName:                ref.GroupName,
		Valid:                    true,
		ReachProtectionThreshold: false,
	}
}

func ToServiceList(names []string, pageNo, pageSize int) nacosmodel.ServiceList {
	if pageNo <= 0 {
		pageNo = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	normalized := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		full := NormalizeListedServiceName(name)
		if _, ok := seen[full]; ok {
			continue
		}
		seen[full] = struct{}{}
		normalized = append(normalized, full)
	}
	sort.Strings(normalized)

	start := (pageNo - 1) * pageSize
	if start > len(normalized) {
		start = len(normalized)
	}
	end := start + pageSize
	if end > len(normalized) {
		end = len(normalized)
	}

	return nacosmodel.ServiceList{
		Count: int64(len(normalized)),
		Doms:  normalized[start:end],
	}
}

func ParsePagination(pageNoRaw, pageSizeRaw string) (int, int) {
	pageNo := 1
	pageSize := 10

	if parsed, err := strconv.Atoi(strings.TrimSpace(pageNoRaw)); err == nil && parsed > 0 {
		pageNo = parsed
	}
	if parsed, err := strconv.Atoi(strings.TrimSpace(pageSizeRaw)); err == nil && parsed > 0 {
		pageSize = parsed
	}

	return pageNo, pageSize
}

func BuildError(message string) map[string]interface{} {
	return map[string]interface{}{
		"timestamp": time.Now().UnixMilli(),
		"status":    500,
		"error":     "Eden Nacos Compatibility Error",
		"message":   message,
	}
}

func normalizeCluster(cluster string) string {
	value := strings.TrimSpace(cluster)
	if value == "" {
		return DefaultCluster
	}
	return value
}

func parseMetadata(raw string) (map[string]string, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]string{}, nil
	}

	metadata := make(map[string]string)
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return nil, fmt.Errorf("invalid metadata: %w", err)
	}
	return metadata, nil
}

func parseRequiredInt(raw, name string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("%s required", name)
	}
	return value, nil
}

func parseBoolDefault(raw string, def bool) bool {
	value := strings.TrimSpace(raw)
	if value == "" {
		return def
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return def
	}
	return parsed
}

func parseWeight(raw string) int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 1
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		return 1
	}
	return int(math.Round(parsed))
}

func splitClusters(raw string) map[string]bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	set := make(map[string]bool)
	for _, item := range strings.Split(raw, ",") {
		cluster := normalizeCluster(item)
		if cluster != "" {
			set[cluster] = true
		}
	}
	return set
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

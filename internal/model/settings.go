package model

import "strings"

const (
	EventTypeServiceRegister  = "service_register"
	EventTypeServiceOnline    = "service_online"
	EventTypeServiceOffline   = "service_offline"
	EventTypeRegistryNodeSync = "registry_node_sync"
	EventTypeServiceHeartbeat = "service_heartbeat"
	EventTypeServiceRemove    = "service_remove"
)

var defaultEventTypes = []string{
	EventTypeServiceRegister,
	EventTypeServiceOnline,
	EventTypeServiceOffline,
	EventTypeRegistryNodeSync,
	EventTypeServiceHeartbeat,
	EventTypeServiceRemove,
}

var eventTypeAliases = map[string][]string{
	EventTypeServiceRegister:  {EventTypeServiceRegister},
	EventTypeServiceOnline:    {EventTypeServiceOnline},
	EventTypeServiceOffline:   {EventTypeServiceOffline},
	EventTypeRegistryNodeSync: {EventTypeRegistryNodeSync},
	EventTypeServiceHeartbeat: {EventTypeServiceHeartbeat},
	EventTypeServiceRemove:    {EventTypeServiceRemove},

	"service register":   {EventTypeServiceRegister},
	"service online":     {EventTypeServiceOnline},
	"service offline":    {EventTypeServiceOffline},
	"registry node sync": {EventTypeRegistryNodeSync},
	"service heartbeat":  {EventTypeServiceHeartbeat},
	"service remove":     {EventTypeServiceRemove},

	"client registration": {
		EventTypeServiceRegister,
		EventTypeServiceOnline,
		EventTypeServiceOffline,
		EventTypeServiceRemove,
	},
	"heartbeat":        {EventTypeServiceHeartbeat},
	"server node sync": {EventTypeRegistryNodeSync},

	"服务注册": {EventTypeServiceRegister},
	"服务上线": {EventTypeServiceOnline},
	"服务下线": {EventTypeServiceOffline},
	"节点同步": {EventTypeRegistryNodeSync},
	"服务心跳": {EventTypeServiceHeartbeat},
	"服务移除": {EventTypeServiceRemove},
}

var validLogLevels = map[string]struct{}{
	"TRACE": {},
	"DEBUG": {},
	"INFO":  {},
	"WARN":  {},
	"ERROR": {},
	"FATAL": {},
	"OFF":   {},
}

type SystemSettings struct {
	Mode                        string   `json:"mode"`
	Environment                 string   `json:"environment"`
	LogLevel                    string   `json:"log_level"`
	EventRetentionDays          int      `json:"event_retention_days"`
	LogRetentionDays            int      `json:"log_retention_days"`
	EventTypes                  []string `json:"event_types"`
	HeartbeatMaxFailures        int      `json:"heartbeat_max_failures"`
	InstanceRemovalDelaySeconds int      `json:"instance_removal_delay_seconds"`
}

func DefaultEventTypes() []string {
	result := make([]string, len(defaultEventTypes))
	copy(result, defaultEventTypes)
	return result
}

func NormalizeEventTypes(values []string) []string {
	if values == nil {
		return nil
	}

	normalized := make([]string, 0, len(defaultEventTypes))
	seen := make(map[string]struct{}, len(defaultEventTypes))

	for _, raw := range values {
		key := strings.TrimSpace(strings.ToLower(raw))
		aliases, ok := eventTypeAliases[key]
		if !ok {
			continue
		}
		for _, alias := range aliases {
			if _, exists := seen[alias]; exists {
				continue
			}
			seen[alias] = struct{}{}
			normalized = append(normalized, alias)
		}
	}

	return normalized
}

func IsValidEventType(value string) bool {
	normalized := NormalizeEventTypes([]string{value})
	return len(normalized) == 1
}

func NormalizeLogLevel(level string) string {
	upper := strings.ToUpper(strings.TrimSpace(level))
	if _, ok := validLogLevels[upper]; ok {
		return upper
	}
	return "INFO"
}

func IsValidLogLevel(level string) bool {
	upper := strings.ToUpper(strings.TrimSpace(level))
	_, ok := validLogLevels[upper]
	return ok
}

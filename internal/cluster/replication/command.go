package replication

import "time"

// CommandType identifies the kind of replicated change.
type CommandType string

const (
	CmdRegister                       CommandType = "register"
	CmdDeregister                     CommandType = "deregister"
	CmdSetInstanceStatus              CommandType = "set_instance_status"
	CmdHeartbeat                      CommandType = "heartbeat"
	CmdAddAPIKey                      CommandType = "add_api_key"
	CmdDeleteAPIKey                   CommandType = "delete_api_key"
	CmdAddUser                        CommandType = "add_user"
	CmdDeleteUser                     CommandType = "delete_user"
	CmdSetMode                        CommandType = "set_mode"
	CmdSetEnv                         CommandType = "set_env"
	CmdSetSeeds                       CommandType = "set_seeds"
	CmdSetLogLevel                    CommandType = "set_log_level"
	CmdSetEventRetentionDays          CommandType = "set_event_retention_days"
	CmdSetLogRetentionDays            CommandType = "set_log_retention_days"
	CmdSetEventTypes                  CommandType = "set_event_types"
	CmdSetHeartbeatMaxFailures        CommandType = "set_heartbeat_max_failures"
	CmdSetInstanceRemovalDelaySeconds CommandType = "set_instance_removal_delay_seconds"
	CmdSetAPIKeyAuthEnabled           CommandType = "set_api_key_auth_enabled"
	CmdSetNotifyAlertNodeID           CommandType = "set_notify_alert_node_id"
	CmdSetRegistryFlushMode           CommandType = "set_registry_flush_mode"
	CmdSetRegistryFlushIntervalMS     CommandType = "set_registry_flush_interval_ms"
	CmdSetEventStorageMode            CommandType = "set_event_storage_mode"
	CmdSetMetricsStorageMode          CommandType = "set_metrics_storage_mode"
	CmdSetMetricsRetentionDays        CommandType = "set_metrics_retention_days"
)

// Instance mirrors the replicated service-instance payload.
type Instance struct {
	ID            string            `json:"id"`
	ServiceName   string            `json:"service_name"`
	Namespace     string            `json:"namespace,omitempty"`
	Host          string            `json:"host"`
	Port          int               `json:"port"`
	Weight        int               `json:"weight"`
	Datacenter    string            `json:"dc"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Status        string            `json:"status"`
	ManualOffline bool              `json:"manual_offline,omitempty"`
	LastHeartbeat time.Time         `json:"last_heartbeat"`
	RegisteredAt  time.Time         `json:"registered_at"`
}

// User mirrors the replicated user payload.
type User struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Nickname  string `json:"nickname"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Remark    string `json:"remark"`
	Role      string `json:"role"`
	IsBuiltIn bool   `json:"is_builtin"`
}

// APIKey mirrors the replicated API key payload.
type APIKey struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	CreatedBy   string `json:"created_by"`
	CreatedAt   int64  `json:"created_at"`
	ExpiresAt   int64  `json:"expires_at"`
	Status      string `json:"status"`
}

// Command is the mutation envelope replicated through Raft and cluster fanout.
type Command struct {
	Type        CommandType `json:"type"`
	Instance    *Instance   `json:"instance,omitempty"`
	Namespace   string      `json:"namespace,omitempty"`
	ServiceName string      `json:"service_name,omitempty"`
	InstanceID  string      `json:"instance_id,omitempty"`
	Status      string      `json:"status,omitempty"`
	APIKey      *APIKey     `json:"api_key,omitempty"`
	User        *User       `json:"user,omitempty"`
	Key         string      `json:"key,omitempty"`
	Username    string      `json:"username,omitempty"`
	Mode        string      `json:"mode,omitempty"`
	Environment string      `json:"environment,omitempty"`
	Seeds       []string    `json:"seeds,omitempty"`
	LogLevel    string      `json:"log_level,omitempty"`
	IntValue    int         `json:"int_value,omitempty"`
	StringList  []string    `json:"string_list,omitempty"`
	BoolValue   bool        `json:"bool_value,omitempty"`
	StringValue string      `json:"string_value,omitempty"`
	NodeID      string      `json:"node_id,omitempty"`
}

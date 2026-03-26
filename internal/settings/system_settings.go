package settings

// SystemSettings defines the mutable runtime settings exposed to clients.
type SystemSettings struct {
	Mode                        string   `json:"mode"`
	Environment                 string   `json:"environment"`
	LogLevel                    string   `json:"log_level"`
	EventRetentionDays          int      `json:"event_retention_days"`
	LogRetentionDays            int      `json:"log_retention_days"`
	EventTypes                  []string `json:"event_types"`
	HeartbeatMaxFailures        int      `json:"heartbeat_max_failures"`
	InstanceRemovalDelaySeconds int      `json:"instance_removal_delay_seconds"`
	APIKeyAuthEnabled           bool     `json:"api_key_auth_enabled"`
}

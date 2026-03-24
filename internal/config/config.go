package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the registry configuration.
type Config struct {
	NodeID   string   `mapstructure:"node_id" json:"node_id"`
	Mode     string   `mapstructure:"mode" json:"mode"` // "ap" or "cp"
	HTTPAddr   string   `mapstructure:"http_addr" json:"http_addr"`
	GRPCAddr   string   `mapstructure:"grpc_addr" json:"grpc_addr"`
	QUICAddr   string   `mapstructure:"quic_addr" json:"quic_addr"`
	RaftAddr   string   `mapstructure:"raft_addr" json:"raft_addr"`
	DataDir    string   `mapstructure:"data_dir" json:"data_dir"`
	Datacenter string   `mapstructure:"datacenter" json:"datacenter"`
	Bootstrap  bool     `mapstructure:"bootstrap" json:"bootstrap"`
	Join       string   `mapstructure:"join" json:"join"`
	Seeds      []string `mapstructure:"seeds" json:"seeds"`
	Auth       Auth        `mapstructure:"auth" json:"auth"`
	Log        LogConfig   `mapstructure:"log" json:"log"`
	Storage    StorageConfig `mapstructure:"storage" json:"storage"`
	Registry   RegistryConfig `mapstructure:"registry" json:"registry"`
}

type RegistryConfig struct {
	HeartbeatIntervalSeconds    int `mapstructure:"heartbeat_interval_seconds" json:"heartbeat_interval_seconds"`
	HeartbeatMaxFailures        int `mapstructure:"heartbeat_max_failures" json:"heartbeat_max_failures"`
	InstanceRemovalDelaySeconds int `mapstructure:"instance_removal_delay_seconds" json:"instance_removal_delay_seconds"`
}

type StorageConfig struct {
	EventRetentionDays int      `mapstructure:"event_retention_days" json:"event_retention_days"`
	LogRetentionDays   int      `mapstructure:"log_retention_days" json:"log_retention_days"`
	EventTypes         []string `mapstructure:"event_types" json:"event_types"`
}

type LogConfig struct {
	Level           string           `mapstructure:"level"`
	Format          string           `mapstructure:"format"`
	Pattern         string           `mapstructure:"pattern"`
	Policies        *PoliciesConfig  `mapstructure:"policies"`
	Rollover        *RolloverConfig  `mapstructure:"rollover"`
	IncludeLocation bool             `mapstructure:"include_location"`
	Appenders       []AppenderConfig `mapstructure:"appenders"`
}

type PoliciesConfig struct {
	CronTriggeringPolicy      *CronPolicyConfig `mapstructure:"cron_triggering_policy"`
	SizeBasedTriggeringPolicy *SizePolicyConfig `mapstructure:"size_based_triggering_policy"`
}

type CronPolicyConfig struct {
	Schedule string `mapstructure:"schedule"`
}

type SizePolicyConfig struct {
	Size string `mapstructure:"size"`
}

type RolloverConfig struct {
	MaxFile   int    `mapstructure:"max_file"`
	Retention string `mapstructure:"retention"`
}

type AppenderConfig struct {
	Name        string                 `mapstructure:"name"`
	Type        string                 `mapstructure:"type"`
	Level       string                 `mapstructure:"level"`
	Pattern     string                 `mapstructure:"pattern"`
	FileName    string                 `mapstructure:"file_name"`
	FilePattern string                 `mapstructure:"file_pattern"`
	Filter      map[string]interface{} `mapstructure:"filter"`
	Async       bool                   `mapstructure:"async"`
	Rollover    *RolloverConfig        `mapstructure:"rollover"`
}

type Auth struct {
	JWT struct {
		Enabled bool   `mapstructure:"enabled"`
		Secret  string `mapstructure:"secret"`
	} `mapstructure:"jwt"`
	APIKey struct {
		Enabled bool     `mapstructure:"enabled"`
		Keys    []string `mapstructure:"keys"`
	} `mapstructure:"api_key"`
	Users []UserConfig `mapstructure:"users"`
}

type UserConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Nickname string `mapstructure:"nickname"`
	Remark   string `mapstructure:"remark"`
	Role     string `mapstructure:"role"` // "admin", "viewer"
}


// LoadConfig loads configuration from yaml file and environment variables.
func LoadConfig(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")
	
	// Environment variables bindings
	// e.g. REGISTRY_NODE_ID overrides node_id in yaml
	viper.SetEnvPrefix("REGISTRY")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Default values
	viper.SetDefault("node_id", "node-1")
	viper.SetDefault("mode", "ap")
	viper.SetDefault("http_addr", ":8500")
	viper.SetDefault("grpc_addr", "")
	viper.SetDefault("quic_addr", "")
	viper.SetDefault("raft_addr", "")
	viper.SetDefault("data_dir", "./data")
	viper.SetDefault("datacenter", "dc1")
	viper.SetDefault("auth.jwt.enabled", false)
	viper.SetDefault("auth.jwt.secret", "eden-jwt-secret")
	viper.SetDefault("auth.api_key.enabled", false)
	viper.SetDefault("auth.api_key.keys", []string{"eden-default-key"})
	// Default user if none provided
	viper.SetDefault("auth.users", []map[string]string{
		{"username": "admin", "password": "admin", "role": "admin"},
	})
	viper.SetDefault("join", "")
	viper.SetDefault("bootstrap", false)

	// Default Storage Configuration
	viper.SetDefault("storage.event_retention_days", 30)
	viper.SetDefault("storage.log_retention_days", 30)
	viper.SetDefault("storage.event_types", []string{"Server Node Sync", "Client Registration", "Heartbeat"})

	viper.SetDefault("registry.heartbeat_interval_seconds", 10)
	viper.SetDefault("registry.heartbeat_max_failures", 3)
	viper.SetDefault("registry.instance_removal_delay_seconds", 3600)

	// Default Log Configuration
	viper.SetDefault("log.level", "INFO")
	viper.SetDefault("log.format", "text")
	viper.SetDefault("log.pattern", "%d [%p] [%T] %m%n")
	viper.SetDefault("log.appenders", []map[string]interface{}{
		{"name": "Console", "type": "Console", "pattern": "%d [%p] [%T] %m%n"},
	})

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config error: %w", err)
		}
		// It's ok if config file is not found, we use defaults or env vars
	}

	var conf Config
	if err := viper.Unmarshal(&conf); err != nil {
		return nil, fmt.Errorf("unmarshal config error: %w", err)
	}

	conf.Mode = strings.ToLower(conf.Mode)
	if conf.Mode != "ap" && conf.Mode != "cp" {
		conf.Mode = "ap"
	}

	return &conf, nil
}

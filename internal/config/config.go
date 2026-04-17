package config

import (
	"fmt"
	"strings"

	logger "github.com/shiyindaxiaojie/eden-go-logger"
	"github.com/shiyindaxiaojie/eden-registry/internal/catalog"
	"github.com/spf13/viper"
)

// Config represents the registry configuration.
type Config struct {
	NodeID      string         `mapstructure:"node_id" json:"node_id"`
	Mode        string         `mapstructure:"mode" json:"mode"` // "standalone" or "cluster"
	Consistency string         `mapstructure:"consistency" json:"consistency"`
	Server      ServerConfig   `mapstructure:"server" json:"server"`
	DataDir     string         `mapstructure:"data_dir" json:"data_dir"`
	Datacenter  string         `mapstructure:"datacenter" json:"datacenter"`
	Bootstrap   bool           `mapstructure:"bootstrap" json:"bootstrap"`
	Auth        Auth           `mapstructure:"auth" json:"auth"`
	Log         LogConfig      `mapstructure:"log" json:"log"`
	Storage     StorageConfig  `mapstructure:"storage" json:"storage"`
	Registry    RegistryConfig `mapstructure:"registry" json:"registry"`
}

type ServerConfig struct {
	HTTP string `mapstructure:"http" json:"http"`
	GRPC string `mapstructure:"grpc" json:"grpc"`
	QUIC string `mapstructure:"quic" json:"quic"`
	Raft string `mapstructure:"raft" json:"raft"`
}

type RegistryConfig struct {
	HeartbeatIntervalSeconds    int `mapstructure:"heartbeat_interval_seconds" json:"heartbeat_interval_seconds"`
	HeartbeatMaxFailures        int `mapstructure:"heartbeat_max_failures" json:"heartbeat_max_failures"`
	InstanceRemovalDelaySeconds int `mapstructure:"instance_removal_delay_seconds" json:"instance_removal_delay_seconds"`
}

type StorageConfig struct {
	EventStorageMode   string   `mapstructure:"event_storage_mode" json:"event_storage_mode"` // "memory" or "persistent"
	EventRetentionDays int      `mapstructure:"event_retention_days" json:"event_retention_days"`
	MetricsStorageMode string   `mapstructure:"metrics_storage_mode" json:"metrics_storage_mode"` // "memory" or "persistent"
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
	viper.SetDefault("mode", "standalone")
	viper.SetDefault("consistency", "ap")
	viper.SetDefault("http_addr", ":8500")
	viper.SetDefault("grpc_addr", ":0")
	viper.SetDefault("quic_addr", "")
	viper.SetDefault("raft_addr", "")
	viper.SetDefault("data_dir", "./data")
	viper.SetDefault("datacenter", "dc1")
	viper.SetDefault("auth.jwt.enabled", false)
	viper.SetDefault("auth.jwt.secret", "registry-jwt-secret")
	viper.SetDefault("auth.api_key.enabled", false)
	viper.SetDefault("auth.api_key.keys", []string{"registry-default-key"})
	// Default user if none provided
	viper.SetDefault("auth.users", []map[string]string{
		{"username": "admin", "password": "admin", "role": "admin"},
	})
	viper.SetDefault("bootstrap", false)
	viper.SetDefault("transport.grpc", "auto")
	viper.SetDefault("transport.quic", "auto")
	viper.SetDefault("transport.raft", "auto")

	// Default Storage Configuration
	viper.SetDefault("storage.event_storage_mode", "memory")
	viper.SetDefault("storage.event_retention_days", 30)
	viper.SetDefault("storage.metrics_storage_mode", "memory")
	viper.SetDefault("storage.log_retention_days", 30)
	viper.SetDefault("storage.event_types", catalog.DefaultEventTypes())

	viper.SetDefault("registry.heartbeat_interval_seconds", 10)
	viper.SetDefault("registry.heartbeat_max_failures", 3)
	viper.SetDefault("registry.instance_removal_delay_seconds", 600)

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

	conf.normalizeRuntime()

	return &conf, nil
}

func normalizeTransportSetting(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "on":
		return "on"
	case "off":
		return "off"
	default:
		return "auto"
	}
}

func normalizeConsistency(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "cp") {
		return "cp"
	}
	return "ap"
}

func normalizeMode(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "cluster") {
		return "cluster"
	}
	return "standalone"
}

func (c *Config) normalizeRuntime() {
	rawMode := strings.ToLower(strings.TrimSpace(c.Mode))
	rawConsistency := strings.ToLower(strings.TrimSpace(c.Consistency))

	// Backward compatibility for legacy configs where mode was "ap" / "cp".
	if rawMode == "ap" || rawMode == "cp" {
		if rawConsistency == "" {
			rawConsistency = rawMode
		}
		if rawMode == "cp" || c.Bootstrap {
			rawMode = "cluster"
		} else {
			rawMode = "standalone"
		}
	}

	c.Mode = normalizeMode(rawMode)
	if c.Mode != "cluster" {
		c.Consistency = "ap"
	} else {
		c.Consistency = normalizeConsistency(rawConsistency)
	}
}

func (c *Config) GRPCEnabled(mode string) bool {
	val := strings.ToLower(strings.TrimSpace(c.Server.GRPC))
	return val != "off"
}

func (c *Config) QUICEnabled(mode string) bool {
	val := strings.ToLower(strings.TrimSpace(c.Server.QUIC))
	return val != "off" && val != ""
}

func (c *Config) RaftEnabled(mode, consistency string) bool {
	val := strings.ToLower(strings.TrimSpace(c.Server.Raft))
	if val == "off" {
		return false
	}
	if val != "auto" && val != "" {
		return true
	}
	return normalizeMode(mode) == "cluster" && normalizeConsistency(consistency) == "cp"
}

func ToLoggerConfiguration(lc LogConfig) logger.Configuration {
	var appenders []logger.AppenderConfig
	for _, a := range lc.Appenders {
		var rollover *logger.RolloverConfig
		if a.Rollover != nil {
			rollover = &logger.RolloverConfig{
				MaxFile:   a.Rollover.MaxFile,
				Retention: a.Rollover.Retention,
			}
		}
		appenders = append(appenders, logger.AppenderConfig{
			Name:        a.Name,
			Type:        a.Type,
			Level:       a.Level,
			Pattern:     a.Pattern,
			FileName:    a.FileName,
			FilePattern: a.FilePattern,
			Filter:      a.Filter,
			Async:       a.Async,
			Rollover:    rollover,
		})
	}

	var policies *logger.PoliciesConfig
	if lc.Policies != nil {
		policies = &logger.PoliciesConfig{}
		if lc.Policies.CronTriggeringPolicy != nil {
			policies.CronTriggeringPolicy = &logger.CronPolicyConfig{
				Schedule: lc.Policies.CronTriggeringPolicy.Schedule,
			}
		}
		if lc.Policies.SizeBasedTriggeringPolicy != nil {
			policies.SizeBasedTriggeringPolicy = &logger.SizePolicyConfig{
				Size: lc.Policies.SizeBasedTriggeringPolicy.Size,
			}
		}
	}

	var rollover *logger.RolloverConfig
	if lc.Rollover != nil {
		rollover = &logger.RolloverConfig{
			MaxFile:   lc.Rollover.MaxFile,
			Retention: lc.Rollover.Retention,
		}
	}

	return logger.Configuration{
		Level:           lc.Level,
		Format:          lc.Format,
		Pattern:         lc.Pattern,
		Policies:        policies,
		Rollover:        rollover,
		IncludeLocation: lc.IncludeLocation,
		Appenders:       appenders,
	}
}

package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the registry configuration.
type Config struct {
	NodeID   string   `mapstructure:"node_id"`
	Mode     string   `mapstructure:"mode"` // "ap" or "cp"
	HTTPAddr string   `mapstructure:"http_addr"`
	RaftAddr string   `mapstructure:"raft_addr"`
	DataDir    string   `mapstructure:"data_dir"`
	Datacenter string   `mapstructure:"datacenter"`
	Join       string   `mapstructure:"join"`  // seed node to join
	Seeds      []string `mapstructure:"seeds"` // seed node list for AP mode
	Auth       Auth     `mapstructure:"auth"`
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
	viper.SetDefault("raft_addr", "127.0.0.1:7000")
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

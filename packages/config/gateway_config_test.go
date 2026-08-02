package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigAppliesGatewayDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "defaults.yaml")
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Gateway.Enabled {
		t.Fatal("Gateway.Enabled = true, want false")
	}
	if cfg.Gateway.HTTP != ":8080" {
		t.Fatalf("Gateway.HTTP = %q, want :8080", cfg.Gateway.HTTP)
	}
	if len(cfg.Gateway.TrustedProxyCIDRs) != 2 || cfg.Gateway.TrustedProxyCIDRs[0] != "127.0.0.1/32" {
		t.Fatalf("Gateway.TrustedProxyCIDRs = %#v", cfg.Gateway.TrustedProxyCIDRs)
	}
}

func TestLoadConfigUsesDefaultsWhenFileDoesNotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.DataDir != "./data" {
		t.Fatalf("DataDir = %q, want ./data", cfg.DataDir)
	}
	if len(cfg.Auth.Users) != 1 {
		t.Fatalf("Auth.Users = %#v, want one default user", cfg.Auth.Users)
	}
	user := cfg.Auth.Users[0]
	if user.Username != "admin" || user.Password != "admin" || user.Role != "admin" {
		t.Fatalf("default user = %#v, want admin/admin with admin role", user)
	}
}

func TestLoadConfigReadsGatewayListenerAndTrustedProxies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.yaml")
	content := []byte("gateway:\n  enabled: true\n  http: ':9080'\n  trusted_proxy_cidrs:\n    - '10.0.0.0/8'\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if !cfg.Gateway.Enabled || cfg.Gateway.HTTP != ":9080" || len(cfg.Gateway.TrustedProxyCIDRs) != 1 || cfg.Gateway.TrustedProxyCIDRs[0] != "10.0.0.0/8" {
		t.Fatalf("Gateway = %#v", cfg.Gateway)
	}
}

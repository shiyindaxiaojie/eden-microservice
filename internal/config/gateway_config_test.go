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

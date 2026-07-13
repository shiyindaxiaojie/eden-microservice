package main

import (
	"testing"

	"github.com/shiyindaxiaojie/eden-registry/internal/config"
)

func TestGatewayListenerEnabledAndRuntimeConfig(t *testing.T) {
	cfg := &config.Config{Gateway: config.GatewayConfig{
		Enabled:           true,
		HTTP:              ":9080",
		TrustedProxyCIDRs: []string{"10.0.0.0/8"},
	}}
	if !gatewayListenerEnabled(cfg) {
		t.Fatal("gatewayListenerEnabled() = false, want true")
	}
	runtimeConfig := gatewayRuntimeConfig(cfg)
	if len(runtimeConfig.TrustedProxyCIDRs) != 1 || runtimeConfig.TrustedProxyCIDRs[0] != "10.0.0.0/8" {
		t.Fatalf("gatewayRuntimeConfig() = %#v", runtimeConfig)
	}

	cfg.Gateway.Enabled = false
	if gatewayListenerEnabled(cfg) {
		t.Fatal("gatewayListenerEnabled() = true for disabled gateway")
	}
}

func TestGatewayListenerAddressesConflictWhenTheyOverlap(t *testing.T) {
	tests := []struct {
		name      string
		control   string
		gateway   string
		conflicts bool
	}{
		{name: "same wildcard", control: ":8080", gateway: ":8080", conflicts: true},
		{name: "wildcard and loopback", control: ":8080", gateway: "127.0.0.1:8080", conflicts: true},
		{name: "ipv4 wildcard and loopback", control: "0.0.0.0:8080", gateway: "127.0.0.1:8080", conflicts: true},
		{name: "different ports", control: ":8080", gateway: ":9080", conflicts: false},
		{name: "different loopbacks", control: "127.0.0.1:8080", gateway: "[::1]:8080", conflicts: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := gatewayListenerAddressesConflict(test.control, test.gateway); got != test.conflicts {
				t.Fatalf("gatewayListenerAddressesConflict(%q, %q) = %v, want %v", test.control, test.gateway, got, test.conflicts)
			}
		})
	}
}

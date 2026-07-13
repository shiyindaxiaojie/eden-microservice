package main

import "testing"

func TestDisplayListenAddrUsesLoopbackForWildcardBind(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{name: "empty", addr: "", want: ""},
		{name: "port only", addr: ":8500", want: "127.0.0.1:8500"},
		{name: "ipv4 wildcard", addr: "0.0.0.0:8500", want: "127.0.0.1:8500"},
		{name: "ipv6 wildcard", addr: "[::]:8500", want: "127.0.0.1:8500"},
		{name: "explicit host", addr: "198.18.0.1:8500", want: "198.18.0.1:8500"},
		{name: "loopback", addr: "127.0.0.1:9000", want: "127.0.0.1:9000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := displayListenAddr(tt.addr); got != tt.want {
				t.Fatalf("displayListenAddr(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}

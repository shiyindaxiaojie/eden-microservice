// Auth Center - 认证中心服务
// 演示使用 Eden SDK 进行服务注册和提供认证 API
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shiyindaxiaojie/eden-go-logger"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/eden"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/registry"
)

var registryAddr = envOr("REGISTRY_ADDR", "127.0.0.1:9000")
var servicePort = envOr("SERVICE_PORT", "9002")

func main() {
	logger.NewBuilder().AddConsole().Init()
	logger.Info("=== Auth Center Service ===")

	client, err := eden.New([]string{registryAddr}, "", "dc1")
	if err != nil {
		logger.Fatal("Failed to create registry client: %v", err)
	}

	instance := &registry.ServiceInstance{
		ID:          "auth-center-1",
		ServiceName: "auth-center",
		Host:        "127.0.0.1",
		Port:        atoi(servicePort),
		Weight:      100,
		Metadata:    map[string]string{"version": "1.0.0"},
	}

	if err := client.Register(instance); err != nil {
		logger.Fatal("Failed to register: %v", err)
	}
	logger.Info("Registered as %s on port %s", instance.ID, servicePort)

	// Heartbeat
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			client.Heartbeat(instance)
		}
	}()

	// Token generation API
	http.HandleFunc("/api/auth/token", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		// Simplified auth logic
		if req.Username != "" && req.Password != "" {
			token := fmt.Sprintf("token-%s-%d", req.Username, time.Now().Unix())
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"token":    token,
				"username": req.Username,
				"expires":  "3600",
			})
			return
		}
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
	})

	http.HandleFunc("/api/auth/verify", func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token != "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"valid": true, "user": "demo-user",
			})
			return
		}
		http.Error(w, `{"valid":false}`, http.StatusUnauthorized)
	})

	go func() {
		logger.Info("Auth Center HTTP API listening on :%s", servicePort)
		http.ListenAndServe(":"+servicePort, nil)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	client.Deregister(instance)
	client.Close()
	logger.Info("Auth Center stopped.")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

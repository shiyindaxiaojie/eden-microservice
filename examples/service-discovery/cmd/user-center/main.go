// User Center - 用户中心服务
// 演示使用 Eden SDK 进行服务注册、心跳和服务发现
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

var registryAddr string

func init() {
	regAddr := os.Getenv("REGISTRY_ADDR")
	if regAddr == "" {
		regAddr = "127.0.0.1:9000"
	}
	registryAddr = regAddr
}

var servicePort = envOr("SERVICE_PORT", "9001")

func main() {
	logger.NewBuilder().AddConsole().Init()
	logger.Info("=== User Center Service ===")

	client, err := eden.New([]string{registryAddr}, "", "dc1")
	if err != nil {
		logger.Fatal("Failed to create registry client: %v", err)
	}

	instance := &registry.ServiceInstance{
		ID:          "user-center-1",
		ServiceName: "user-center",
		Host:        "127.0.0.1",
		Port:        atoi(servicePort),
		Weight:      100,
		Metadata:    map[string]string{"version": "1.0.0"},
	}

	// Register
	if err := client.Register(instance); err != nil {
		logger.Fatal("Failed to register: %v", err)
	}
	logger.Info("Registered as %s on port %s", instance.ID, servicePort)

	// Heartbeat goroutine
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := client.Heartbeat(instance); err != nil {
				logger.Warn("Heartbeat failed: %v", err)
			}
		}
	}()

	// HTTP API
	http.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		users := []map[string]interface{}{
			{"id": 1, "name": "Alice", "email": "alice@example.com"},
			{"id": 2, "name": "Bob", "email": "bob@example.com"},
			{"id": 3, "name": "Charlie", "email": "charlie@example.com"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	})

	http.HandleFunc("/api/user/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/api/user/"):]
		user := map[string]interface{}{
			"id": id, "name": "User-" + id, "email": fmt.Sprintf("user%s@example.com", id),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	})

	go func() {
		logger.Info("User Center HTTP API listening on :%s", servicePort)
		if err := http.ListenAndServe(":"+servicePort, nil); err != nil {
			logger.Fatal("HTTP error: %v", err)
		}
	}()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("Shutting down...")
	client.Deregister(instance)
	client.Close()
	logger.Info("User Center stopped.")
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

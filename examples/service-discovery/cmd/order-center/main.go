// Order Center - 订单中心服务
// 演示使用 Eden SDK 进行服务注册、发现其他服务并跨服务调用
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	logger "github.com/shiyindaxiaojie/eden-go-logger"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/eden"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/registry"
)

var registryAddr string
var servicePort = envOr("SERVICE_PORT", "9003")

func init() {
	regAddr := os.Getenv("REGISTRY_ADDR")
	if regAddr == "" {
		// Client only needs one node to discover the rest of the cluster
		registryAddr = "http://127.0.0.1:8500"
	} else {
		registryAddr = regAddr
	}
}

var client *eden.Client

func main() {
	logger.NewBuilder().AddConsole().Init()
	logger.Info("=== Order Center Service ===")

	var err error
	client, err = eden.New([]string{registryAddr}, "", "dc1")
	if err != nil {
		logger.Fatal("Failed to create registry client: %v", err)
	}

	instance := &registry.ServiceInstance{
		ID:          envOr("SERVICE_ID", "order-center-1"),
		ServiceName: "order-center",
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

	// Subscribe to user-center changes
	client.Subscribe("user-center", func(instances []*registry.ServiceInstance) {
		logger.Info("[Registry Subscribe] user-center updated: %d instances", len(instances))
		for _, inst := range instances {
			logger.Info("  - %s:%d (healthy=%v)", inst.Host, inst.Port, inst.Healthy)
		}
	})

	// Order API - creates order by calling user-center and auth-center
	http.HandleFunc("/api/order/create", handleCreateOrder)
	http.HandleFunc("/api/order/demo", handleDemoFlow)

	go func() {
		logger.Info("Order Center HTTP API listening on :%s", servicePort)
		http.ListenAndServe(":"+servicePort, nil)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	client.Deregister(instance)
	client.Close()
	logger.Info("Order Center stopped.")
}

func handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	// 1. Discover user-center
	userInstances, err := client.Discovery("user-center")
	if err != nil || len(userInstances) == 0 {
		http.Error(w, `{"error":"user-center not available"}`, http.StatusServiceUnavailable)
		return
	}

	// 2. Call user-center to get user info
	userAddr := fmt.Sprintf("http://%s:%d", userInstances[0].Host, userInstances[0].Port)
	userResp, err := http.Get(userAddr + "/api/users")
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to call user-center: %v"}`, err), http.StatusBadGateway)
		return
	}
	defer userResp.Body.Close()
	userData, _ := io.ReadAll(userResp.Body)

	// 3. Discover auth-center
	authInstances, err := client.Discovery("auth-center")
	authAvailable := err == nil && len(authInstances) > 0

	// 4. Build order response
	order := map[string]interface{}{
		"order_id":       fmt.Sprintf("ORD-%d", time.Now().UnixNano()),
		"status":         "created",
		"created_at":     time.Now().Format(time.RFC3339),
		"user_source":    userAddr,
		"auth_available": authAvailable,
	}

	var users interface{}
	json.Unmarshal(userData, &users)
	order["users"] = users

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

// handleDemoFlow runs a full demo: discover → call → respond
func handleDemoFlow(w http.ResponseWriter, r *http.Request) {
	steps := []map[string]interface{}{}

	// Step 1: Discover all services
	for _, svc := range []string{"user-center", "auth-center"} {
		instances, err := client.Discovery(svc)
		step := map[string]interface{}{
			"step":    fmt.Sprintf("Discover %s", svc),
			"service": svc,
		}
		if err != nil {
			step["status"] = "error"
			step["error"] = err.Error()
		} else {
			step["status"] = "ok"
			step["instances"] = len(instances)
			if len(instances) > 0 {
				step["address"] = fmt.Sprintf("%s:%d", instances[0].Host, instances[0].Port)
			}
		}
		steps = append(steps, step)
	}

	// Step 2: Call user-center
	userInstances, _ := client.Discovery("user-center")
	if len(userInstances) > 0 {
		addr := fmt.Sprintf("http://%s:%d/api/users", userInstances[0].Host, userInstances[0].Port)
		resp, err := http.Get(addr)
		step := map[string]interface{}{
			"step": "Call user-center /api/users",
		}
		if err != nil {
			step["status"] = "error"
			step["error"] = err.Error()
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			step["status"] = "ok"
			step["response_size"] = len(body)
			var data interface{}
			json.Unmarshal(body, &data)
			step["data"] = data
		}
		steps = append(steps, step)
	}

	result := map[string]interface{}{
		"demo":  "Service Discovery Flow",
		"steps": steps,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
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

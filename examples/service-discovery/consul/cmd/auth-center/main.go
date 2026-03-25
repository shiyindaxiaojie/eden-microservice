package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	logger "github.com/shiyindaxiaojie/eden-go-logger"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/consul"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/registry"
)

func main() {
	logger.NewBuilder().AddConsole().Init()

	// 初始化基于 Consul API 的注册中心实现，后续通过统一的 registry 接口演示注册、发现、订阅和心跳。
	reg, err := consul.NewRegistry(&registry.Config{
		Addresses:  []string{envOr("CONSUL_ADDR", "127.0.0.1:8500")},
		APIKey:     envOr("CONSUL_API_KEY", ""),
		Datacenter: envOr("CONSUL_DATACENTER", "dc1"),
	})
	if err != nil {
		logger.Fatal("create registry failed: %v", err)
	}

	// 定义当前服务实例元信息，注册和心跳都会复用这份数据。
	instance := &registry.ServiceInstance{
		ID:          envOr("SERVICE_ID", "consul-auth-center-1"),
		ServiceName: "consul-auth-center",
		Host:        envOr("SERVICE_HOST", "127.0.0.1"),
		Port:        atoi(envOr("SERVICE_PORT", "22002")),
		Weight:      100,
	}

	// 将当前实例注册到注册中心，供其他服务通过服务名发现。
	if err := reg.Register(instance); err != nil {
		logger.Fatal("register failed: %v", err)
	}
	logger.Info("registered %s on %s:%d", instance.ID, instance.Host, instance.Port)

	// 订阅依赖服务的实例变化，演示注册中心的变更通知能力。
	if err := reg.Subscribe("consul-user-center", func(items []*registry.ServiceInstance) {
		logger.Info("[subscribe] consul-user-center updated: %d instance(s)", len(items))
	}); err != nil {
		logger.Warn("subscribe consul-user-center failed: %v", err)
	}

	// 定时上报心跳，维持实例在注册中心中的健康状态。
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := reg.Heartbeat(instance); err != nil {
				logger.Warn("heartbeat failed: %v", err)
				continue
			}
			logger.Info("heartbeat ok")
		}
	}()

	server := newHTTPServer(instance.Port, newAuthCenterHandler(reg, instance))
	startHTTPServer("auth-center", server, instance.Port)
	waitForStopSignal()
	shutdownHTTPServer(server)

	_ = reg.Deregister(instance)
	_ = reg.Close()
}

func newAuthCenterHandler(reg registry.Registry, instance *registry.ServiceInstance) http.Handler {
	// HTTP 路由只是为了演示服务间调用链，本身不属于注册中心接入逻辑。
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"status":      "ok",
			"service":     instance.ServiceName,
			"instance_id": instance.ID,
			"host":        instance.Host,
			"port":        instance.Port,
		})
	})
	mux.HandleFunc("/api/auth/token", func(w http.ResponseWriter, r *http.Request) {
		userID := queryOrDefault(r, "user_id", "1")
		user, targetURL, err := callJSON(reg, "consul-user-center", "/api/users/"+neturl.PathEscape(userID))
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "call consul-user-center failed: "+err.Error())
			return
		}

		writeJSON(w, map[string]interface{}{
			"service":       instance.ServiceName,
			"instance_id":   instance.ID,
			"user_id":       userID,
			"token":         fmt.Sprintf("token-%s-%d", userID, time.Now().UnixNano()),
			"user":          user,
			"user_upstream": targetURL,
		})
	})
	mux.HandleFunc("/api/auth/permissions/", func(w http.ResponseWriter, r *http.Request) {
		userID := strings.TrimPrefix(r.URL.Path, "/api/auth/permissions/")
		if userID == "" {
			writeError(w, http.StatusBadRequest, "user id required")
			return
		}

		writeJSON(w, map[string]interface{}{
			"service":      instance.ServiceName,
			"instance_id":  instance.ID,
			"user_id":      userID,
			"permissions":  []string{"order:create", "order:query", "user:read"},
			"checked_at":   time.Now().Format(time.RFC3339),
			"integration":  "pkg/consul",
			"transport":    "http-compat",
			"dependencyOk": true,
		})
	})
	return mux
}

func newHTTPServer(port int, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
	}
}

func startHTTPServer(serviceName string, server *http.Server, port int) {
	go func() {
		logger.Info("%s listening on :%d", serviceName, port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen failed: %v", err)
		}
	}()
}

func waitForStopSignal() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}

func shutdownHTTPServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func callJSON(reg registry.Registry, serviceName, path string) (interface{}, string, error) {
	items, err := reg.Discovery(serviceName)
	if err != nil {
		return nil, "", err
	}
	if len(items) == 0 {
		return nil, "", fmt.Errorf("no instances for %s", serviceName)
	}

	targetURL := fmt.Sprintf("http://%s:%d%s", items[0].Host, items[0].Port, path)
	resp, err := http.Get(targetURL)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var payload interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, "", err
	}
	return payload, targetURL, nil
}

func writeJSON(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func queryOrDefault(r *http.Request, key, def string) string {
	if value := strings.TrimSpace(r.URL.Query().Get(key)); value != "" {
		return value
	}
	return def
}

func envOr(key, def string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return def
}

func atoi(value string) int {
	n := 0
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

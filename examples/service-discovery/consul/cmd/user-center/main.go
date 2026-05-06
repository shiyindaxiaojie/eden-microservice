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

	consulapi "github.com/hashicorp/consul/api"
	logger "github.com/shiyindaxiaojie/eden-go-logger"
	consulhelper "github.com/shiyindaxiaojie/eden-registry/examples/service-discovery/consul/internal/consulapi"
)

func main() {
	logger.NewBuilder().AddConsole().Init()

	// 初始化基于 Consul API 的注册中心实现，后续通过统一的 registry 接口演示注册、发现、订阅和心跳。
	cfg := consulapi.DefaultConfig()
	cfg.Address = envOr("CONSUL_ADDR", "127.0.0.1:8500")
	cfg.Token = envOr("CONSUL_API_KEY", "")
	cfg.Datacenter = envOr("CONSUL_DATACENTER", "dc1")

	rawClient, err := consulapi.NewClient(cfg)
	if err != nil {
		logger.Fatal("create consul client failed: %v", err)
	}
	reg := consulhelper.New(rawClient)

	// 定义当前服务实例元信息，注册和心跳都会复用这份数据。
	instance := &consulhelper.ServiceInstance{
		ID:          envOr("SERVICE_ID", "consul-user-center-1"),
		ServiceName: "consul-user-center",
		Host:        envOr("SERVICE_HOST", "127.0.0.1"),
		Port:        atoi(envOr("SERVICE_PORT", "22001")),
		Weight:      100,
	}

	// 将当前实例注册到注册中心，供其他服务通过服务名发现。
	if err := reg.Register(instance); err != nil {
		logger.Fatal("register failed: %v", err)
	}
	logger.Info("registered %s on %s:%d", instance.ID, instance.Host, instance.Port)

	// 订阅依赖服务的实例变化，演示注册中心的变更通知能力。
	if err := reg.Subscribe("consul-auth-center", func(items []*consulhelper.ServiceInstance) {
		logger.Info("[subscribe] consul-auth-center updated: %d instance(s)", len(items))
	}); err != nil {
		logger.Warn("subscribe consul-auth-center failed: %v", err)
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

	server := newHTTPServer(instance.Port, newUserCenterHandler(reg, instance))
	startHTTPServer("user-center", server, instance.Port)
	waitForStopSignal()
	shutdownHTTPServer(server)

	_ = reg.Deregister(instance)
	_ = reg.Close()
}

func newUserCenterHandler(reg *consulhelper.Client, instance *consulhelper.ServiceInstance) http.Handler {
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
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			userByID("1"),
			userByID("2"),
			userByID("3"),
		})
	})
	mux.HandleFunc("/api/users/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/users/")
		if path == "" {
			writeError(w, http.StatusBadRequest, "user id required")
			return
		}

		if strings.HasSuffix(path, "/profile") {
			userID := strings.TrimSuffix(path, "/profile")
			userID = strings.TrimSuffix(userID, "/")
			permissions, targetURL, err := callJSON(reg, "consul-auth-center", "/api/auth/permissions/"+neturl.PathEscape(userID))
			if err != nil {
				writeError(w, http.StatusServiceUnavailable, "call consul-auth-center failed: "+err.Error())
				return
			}

			writeJSON(w, map[string]interface{}{
				"service":       instance.ServiceName,
				"instance_id":   instance.ID,
				"user":          userByID(userID),
				"permissions":   permissions,
				"auth_upstream": targetURL,
			})
			return
		}

		writeJSON(w, userByID(path))
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

func callJSON(reg *consulhelper.Client, serviceName, path string) (interface{}, string, error) {
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

func userByID(id string) map[string]interface{} {
	switch id {
	case "1":
		return map[string]interface{}{"id": "1", "name": "Alice", "email": "alice@example.com"}
	case "2":
		return map[string]interface{}{"id": "2", "name": "Bob", "email": "bob@example.com"}
	case "3":
		return map[string]interface{}{"id": "3", "name": "Charlie", "email": "charlie@example.com"}
	default:
		return map[string]interface{}{"id": id, "name": "User-" + id, "email": "user-" + id + "@example.com"}
	}
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

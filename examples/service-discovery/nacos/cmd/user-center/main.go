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

	nacosclients "github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	logger "github.com/shiyindaxiaojie/eden-go-logger"
	nacoshelper "github.com/shiyindaxiaojie/eden-go-registry/examples/service-discovery/nacos/internal/nacosapi"
)

func main() {
	logger.NewBuilder().AddConsole().Init()

	// 初始化基于 Nacos 协议的注册中心适配器，后续通过统一的 registry 接口演示注册、发现、订阅和心跳。
	rawClient, err := nacosclients.NewNamingClient(vo.NacosClientParam{
		ServerConfigs: []constant.ServerConfig{
			nacoshelper.ParseServerConfig(envOr("NACOS_ADDR", "127.0.0.1:8500")),
		},
		ClientConfig: nacoshelper.DefaultClientConfig(envOr("NACOS_NAMESPACE", "public")),
	})
	if err != nil {
		logger.Fatal("create nacos client failed: %v", err)
	}
	reg := nacoshelper.New(rawClient)

	// 定义当前服务实例元信息，后续通过 Nacos 协议适配层写入注册中心并用于心跳续约。
	instance := &nacoshelper.ServiceInstance{
		ID:          envOr("SERVICE_ID", "nacos-user-center-1"),
		ServiceName: "nacos-user-center",
		Host:        envOr("SERVICE_HOST", "127.0.0.1"),
		Port:        atoi(envOr("SERVICE_PORT", "23001")),
		Weight:      100,
	}

	// 通过 Nacos 协议适配层将当前实例注册到我们的注册中心，供其他服务按服务名发现。
	if err := reg.Register(instance); err != nil {
		logger.Fatal("register failed: %v", err)
	}
	logger.Info("registered %s on %s:%d", instance.ID, instance.Host, instance.Port)

	// 订阅依赖服务的实例变化，演示适配层如何把注册中心变更事件回调到统一接口。
	if err := reg.Subscribe("nacos-auth-center", func(items []*nacoshelper.ServiceInstance) {
		logger.Info("[subscribe] nacos-auth-center updated: %d instance(s)", len(items))
	}); err != nil {
		logger.Warn("subscribe nacos-auth-center failed: %v", err)
	}

	// 定时通过适配层续约实例，维持当前服务在注册中心中的可发现状态。
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

	// 启动服务，以下演示服务间的调用链，不属于注册中心 SDK 集成逻辑。
	server := newHTTPServer(instance.Port, newUserCenterHandler(reg, instance))
	startHTTPServer("user-center", server, instance.Port)
	waitForStopSignal()
	shutdownHTTPServer(server)

	// 优雅下线
	_ = reg.Deregister(instance)
	_ = reg.Close()
}

func newUserCenterHandler(reg *nacoshelper.Client, instance *nacoshelper.ServiceInstance) http.Handler {
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
			permissions, targetURL, err := callJSON(reg, "nacos-auth-center", "/api/auth/permissions/"+neturl.PathEscape(userID))
			if err != nil {
				writeError(w, http.StatusServiceUnavailable, "call nacos-auth-center failed: "+err.Error())
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

func callJSON(reg *nacoshelper.Client, serviceName, path string) (interface{}, string, error) {
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

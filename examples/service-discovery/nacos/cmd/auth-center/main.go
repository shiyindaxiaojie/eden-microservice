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
	"github.com/shiyindaxiaojie/eden-registry/examples/service-discovery/nacos/internal/instanceguard"
	nacoshelper "github.com/shiyindaxiaojie/eden-registry/examples/service-discovery/nacos/internal/nacosapi"
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
		ID:          envOr("SERVICE_ID", "nacos-auth-center-1"),
		ServiceName: "nacos-auth-center",
		Host:        envOr("SERVICE_HOST", "127.0.0.1"),
		Port:        atoi(envOr("SERVICE_PORT", "23002")),
		Weight:      100,
	}

	// 通过 Nacos 协议适配层将当前实例注册到我们的注册中心，供其他服务按服务名发现。
	if err := reg.Register(instance); err != nil {
		logger.Fatal("register failed: %v", err)
	}
	logger.Info("registered %s on %s:%d", instance.ID, instance.Host, instance.Port)

	// 订阅依赖服务的实例变化，演示适配层如何把注册中心变更事件回调到统一接口。
	if err := reg.Subscribe("nacos-user-center", func(items []*nacoshelper.ServiceInstance) {
		logger.Info("[subscribe] nacos-user-center updated: %d instance(s)", len(items))
	}); err != nil {
		logger.Warn("subscribe nacos-user-center failed: %v", err)
	}

	heartbeatStopCh, err := instanceguard.WatchSelfOffline(reg, instance.ServiceName, instance.ID)
	if err != nil {
		logger.Warn("watch self service failed: %v", err)
	}

	// 定时通过适配层续约实例，维持当前服务在注册中心中的可发现状态。
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatStopCh:
				logger.Info("self instance marked offline, stop heartbeats")
				return
			case <-ticker.C:
				if err := reg.Heartbeat(instance); err != nil {
					logger.Warn("heartbeat failed: %v", err)
					continue
				}
				logger.Info("heartbeat ok")
			}
		}
	}()

	// 启动服务，以下演示服务间的调用链，不属于注册中心 SDK 集成逻辑。
	server := newHTTPServer(instance.Port, newAuthCenterHandler(reg, instance))
	startHTTPServer("auth-center", server, instance.Port)
	waitForStopSignal()
	shutdownHTTPServer(server)

	// 优雅下线
	_ = reg.Deregister(instance)
	_ = reg.Close()
}

func newAuthCenterHandler(reg *nacoshelper.Client, instance *nacoshelper.ServiceInstance) http.Handler {
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
		user, targetURL, err := callJSON(reg, "nacos-user-center", "/api/users/"+neturl.PathEscape(userID))
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "call nacos-user-center failed: "+err.Error())
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
			"integration":  "github.com/nacos-group/nacos-sdk-go/v2",
			"transport":    "nacos-http-grpc-compat",
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

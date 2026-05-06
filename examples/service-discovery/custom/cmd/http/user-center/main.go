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
	httpclient "github.com/shiyindaxiaojie/eden-registry/examples/service-discovery/custom/internal/http"
	"github.com/shiyindaxiaojie/eden-registry/examples/service-discovery/custom/internal/instanceguard"
	"github.com/shiyindaxiaojie/eden-registry/examples/service-discovery/custom/internal/registry"
)

func main() {
	logger.NewBuilder().AddConsole().Init()

	// Initialize the custom HTTP registry client and use the unified registry interface below.
	reg, err := httpclient.NewFromEnv()
	if err != nil {
		logger.Fatal("create registry failed: %v", err)
	}

	// Reuse the same instance metadata for register and heartbeat.
	instance := &registry.ServiceInstance{
		ID:          envOr("SERVICE_ID", "custom-http-user-center-1"),
		ServiceName: "custom-http-user-center",
		Host:        envOr("SERVICE_HOST", "127.0.0.1"),
		Port:        atoi(envOr("SERVICE_PORT", "24101")),
		Weight:      100,
	}

	// Register the current instance so other services can discover it by service name.
	if err := reg.Register(instance); err != nil {
		logger.Fatal("register failed: %v", err)
	}
	logger.Info("registered %s on %s:%d", instance.ID, instance.Host, instance.Port)

	// Subscribe to dependency changes to show registry callbacks.
	if err := reg.Subscribe("custom-http-auth-center", func(items []*registry.ServiceInstance) {
		logger.Info("[subscribe] custom-http-auth-center updated: %d instance(s)", len(items))
	}); err != nil {
		logger.Warn("subscribe custom-http-auth-center failed: %v", err)
	}

	heartbeatStopCh, err := instanceguard.WatchSelfOffline(reg, instance.ServiceName, instance.ID)
	if err != nil {
		logger.Warn("watch self service failed: %v", err)
	}

	// Report heartbeats periodically to keep the instance discoverable.
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

	server := newHTTPServer(instance.Port, newUserCenterHandler(reg, instance))
	startHTTPServer("user-center", server, instance.Port)
	waitForStopSignal()
	shutdownHTTPServer(server)

	_ = reg.Deregister(instance)
	_ = reg.Close()
}

func newUserCenterHandler(reg registry.Registry, instance *registry.ServiceInstance) http.Handler {
	// HTTP routes only demonstrate the dependency chain; they are not part of registry integration.
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
			permissions, targetURL, err := callJSON(reg, "custom-http-auth-center", "/api/auth/permissions/"+neturl.PathEscape(userID))
			if err != nil {
				writeError(w, http.StatusServiceUnavailable, "call custom-http-auth-center failed: "+err.Error())
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

package app

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
	"sync/atomic"
	"syscall"
	"time"

	logger "github.com/shiyindaxiaojie/eden-go-logger"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/registry"
)

type ServiceConfig struct {
	Title           string
	Integration     string
	Transport       string
	ServiceName     string
	ServiceID       string
	Host            string
	Port            int
	Registry        registry.Registry
	UserServiceName string
	AuthServiceName string
}

var pickCounter uint64

func RunAuthCenter(cfg ServiceConfig) error {
	return runHTTPService(cfg, []string{cfg.UserServiceName}, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/auth/token", func(w http.ResponseWriter, r *http.Request) {
			userID := queryOrDefault(r, "user_id", "1")
			user, upstream, err := callJSONDependency(cfg.Registry, cfg.UserServiceName, "/api/users/"+neturl.PathEscape(userID))
			if err != nil {
				writeError(w, http.StatusServiceUnavailable, "failed to call user-center: "+err.Error())
				return
			}

			writeJSON(w, map[string]interface{}{
				"service":        cfg.ServiceName,
				"instance_id":    cfg.ServiceID,
				"transport":      cfg.Transport,
				"integration":    cfg.Integration,
				"user_id":        userID,
				"token":          fmt.Sprintf("token-%s-%d", userID, time.Now().UnixNano()),
				"user":           user,
				"user_upstream":  upstream,
				"generated_at":   time.Now().Format(time.RFC3339),
				"dependency_ok":  true,
				"dependency_set": []string{cfg.UserServiceName},
			})
		})

		mux.HandleFunc("/api/auth/permissions/", func(w http.ResponseWriter, r *http.Request) {
			userID := strings.TrimPrefix(r.URL.Path, "/api/auth/permissions/")
			if userID == "" {
				writeError(w, http.StatusBadRequest, "user id required")
				return
			}

			writeJSON(w, map[string]interface{}{
				"service":      cfg.ServiceName,
				"instance_id":  cfg.ServiceID,
				"user_id":      userID,
				"permissions":  []string{"order:create", "order:query", "user:read"},
				"checked_at":   time.Now().Format(time.RFC3339),
				"integration":  cfg.Integration,
				"transport":    cfg.Transport,
				"dependencyOk": true,
			})
		})
	})
}

func RunUserCenter(cfg ServiceConfig) error {
	return runHTTPService(cfg, []string{cfg.AuthServiceName}, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]interface{}{
				"service":     cfg.ServiceName,
				"instance_id": cfg.ServiceID,
				"items": []map[string]interface{}{
					userByID("1"),
					userByID("2"),
					userByID("3"),
				},
			})
		})

		mux.HandleFunc("/api/users/", func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/api/users/")
			if path == "" {
				writeError(w, http.StatusBadRequest, "user path required")
				return
			}

			if strings.HasSuffix(path, "/profile") {
				userID := strings.TrimSuffix(path, "/profile")
				userID = strings.TrimSuffix(userID, "/")
				permissions, upstream, err := callJSONDependency(cfg.Registry, cfg.AuthServiceName, "/api/auth/permissions/"+neturl.PathEscape(userID))
				if err != nil {
					writeError(w, http.StatusServiceUnavailable, "failed to call auth-center: "+err.Error())
					return
				}

				writeJSON(w, map[string]interface{}{
					"service":         cfg.ServiceName,
					"instance_id":     cfg.ServiceID,
					"user":            userByID(userID),
					"permissions":     permissions,
					"auth_upstream":   upstream,
					"integration":     cfg.Integration,
					"transport":       cfg.Transport,
					"dependency_set":  []string{cfg.AuthServiceName},
					"dependency_call": true,
				})
				return
			}

			writeJSON(w, map[string]interface{}{
				"service":     cfg.ServiceName,
				"instance_id": cfg.ServiceID,
				"user":        userByID(path),
			})
		})
	})
}

func RunOrderCenter(cfg ServiceConfig) error {
	return runHTTPService(cfg, []string{cfg.UserServiceName, cfg.AuthServiceName}, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/orders/create", func(w http.ResponseWriter, r *http.Request) {
			userID := queryOrDefault(r, "user_id", "1")

			user, userUpstream, err := callJSONDependency(cfg.Registry, cfg.UserServiceName, "/api/users/"+neturl.PathEscape(userID))
			if err != nil {
				writeError(w, http.StatusServiceUnavailable, "failed to call user-center: "+err.Error())
				return
			}

			token, authUpstream, err := callJSONDependency(cfg.Registry, cfg.AuthServiceName, "/api/auth/token?user_id="+neturl.QueryEscape(userID))
			if err != nil {
				writeError(w, http.StatusServiceUnavailable, "failed to call auth-center: "+err.Error())
				return
			}

			writeJSON(w, map[string]interface{}{
				"service":        cfg.ServiceName,
				"instance_id":    cfg.ServiceID,
				"order_id":       fmt.Sprintf("order-%d", time.Now().UnixNano()),
				"user_id":        userID,
				"user":           user,
				"token":          token,
				"user_upstream":  userUpstream,
				"auth_upstream":  authUpstream,
				"integration":    cfg.Integration,
				"transport":      cfg.Transport,
				"dependency_set": []string{cfg.UserServiceName, cfg.AuthServiceName},
			})
		})

		mux.HandleFunc("/api/orders/demo", func(w http.ResponseWriter, r *http.Request) {
			userID := queryOrDefault(r, "user_id", "1")

			profile, userUpstream, err := callJSONDependency(cfg.Registry, cfg.UserServiceName, "/api/users/"+neturl.PathEscape(userID)+"/profile")
			if err != nil {
				writeError(w, http.StatusServiceUnavailable, "failed to call user-center profile: "+err.Error())
				return
			}

			permissions, authUpstream, err := callJSONDependency(cfg.Registry, cfg.AuthServiceName, "/api/auth/permissions/"+neturl.PathEscape(userID))
			if err != nil {
				writeError(w, http.StatusServiceUnavailable, "failed to call auth-center permissions: "+err.Error())
				return
			}

			writeJSON(w, map[string]interface{}{
				"service":           cfg.ServiceName,
				"instance_id":       cfg.ServiceID,
				"user_profile":      profile,
				"permissions":       permissions,
				"user_upstream":     userUpstream,
				"auth_upstream":     authUpstream,
				"integration":       cfg.Integration,
				"transport":         cfg.Transport,
				"demo_relationship": "order-center -> user-center + auth-center",
			})
		})
	})
}

func runHTTPService(cfg ServiceConfig, subscriptions []string, registerRoutes func(*http.ServeMux)) error {
	logger.NewBuilder().AddConsole().Init()

	instance := &registry.ServiceInstance{
		ID:          cfg.ServiceID,
		ServiceName: cfg.ServiceName,
		Host:        cfg.Host,
		Port:        cfg.Port,
		Weight:      100,
		Metadata: map[string]string{
			"integration": cfg.Integration,
			"transport":   cfg.Transport,
		},
	}

	if err := cfg.Registry.Register(instance); err != nil {
		return err
	}
	logger.Info("[%s] registered: %s at %s:%d", cfg.Title, instance.ID, instance.Host, instance.Port)

	for _, serviceName := range subscriptions {
		if serviceName == "" {
			continue
		}
		if err := cfg.Registry.Subscribe(serviceName, func(items []*registry.ServiceInstance) {
			logger.Info("[%s] dependency updated: %s -> %d instance(s)", cfg.Title, serviceName, len(items))
		}); err != nil {
			logger.Warn("[%s] subscribe failed for %s: %v", cfg.Title, serviceName, err)
		}
	}

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := cfg.Registry.Heartbeat(instance); err != nil {
					logger.Warn("[%s] heartbeat failed: %v", cfg.Title, err)
					continue
				}
				logger.Info("[%s] heartbeat ok", cfg.Title)
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"status":      "ok",
			"service":     cfg.ServiceName,
			"instance_id": cfg.ServiceID,
			"host":        cfg.Host,
			"port":        cfg.Port,
			"integration": cfg.Integration,
			"transport":   cfg.Transport,
		})
	})
	registerRoutes(mux)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}

	go func() {
		logger.Info("[%s] http listening on :%d", cfg.Title, cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("[%s] listen failed: %v", cfg.Title, err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	close(done)
	logger.Info("[%s] shutting down", cfg.Title)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	_ = cfg.Registry.Deregister(instance)
	return cfg.Registry.Close()
}

func callJSONDependency(reg registry.Registry, serviceName, path string) (interface{}, map[string]interface{}, error) {
	instances, err := reg.Discovery(serviceName)
	if err != nil {
		return nil, nil, err
	}
	if len(instances) == 0 {
		return nil, nil, fmt.Errorf("no instances for %s", serviceName)
	}

	instance := pickInstance(instances)
	targetURL := fmt.Sprintf("http://%s:%d%s", instance.Host, instance.Port, path)

	resp, err := http.Get(targetURL)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var payload interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, nil, err
	}

	return payload, map[string]interface{}{
		"service":     serviceName,
		"instance_id": instance.ID,
		"target_url":  targetURL,
		"host":        instance.Host,
		"port":        instance.Port,
	}, nil
}

func pickInstance(items []*registry.ServiceInstance) *registry.ServiceInstance {
	index := atomic.AddUint64(&pickCounter, 1)
	return items[int((index-1)%uint64(len(items)))]
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
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": message,
	})
}

func queryOrDefault(r *http.Request, key, def string) string {
	if value := strings.TrimSpace(r.URL.Query().Get(key)); value != "" {
		return value
	}
	return def
}

func EnvOr(key, def string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return def
}

func Atoi(value string) int {
	n := 0
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

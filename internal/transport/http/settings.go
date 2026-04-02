package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shiyindaxiaojie/eden-go-logger"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/alert"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/auth"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/catalog"
	clusterpkg "github.com/shiyindaxiaojie/eden-go-registry/internal/cluster"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/notify"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/settings"
	"golang.org/x/crypto/bcrypt"
)

// ---------- RBAC (User Management) Handlers ----------

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.settings.ListUsers()
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, users)
}

func (h *Handler) saveUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var u auth.User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		httpError(w, http.StatusBadRequest, "invalid body")
		return
	}

	if u.Username == "" {
		httpError(w, http.StatusBadRequest, "username required")
		return
	}

	existingUser, exists := h.settings.GetUser(u.Username)
	if exists {
		u.IsBuiltIn = existingUser.IsBuiltIn
		if u.Password == "" {
			u.Password = existingUser.Password
		} else if u.Password != existingUser.Password {
			hashed, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
			if err == nil {
				u.Password = string(hashed)
			}
		}
	} else if u.Password != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
		if err == nil {
			u.Password = string(hashed)
		}
	}

	if err := h.settings.AddUser(&u); err != nil {
		h.writeLeaderRedirect(w, err)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")

	user, exists := h.settings.GetUser(username)
	if !exists {
		httpError(w, http.StatusNotFound, "user not found")
		return
	}
	if user.IsBuiltIn {
		httpError(w, http.StatusForbidden, "built-in users cannot be deleted")
		return
	}

	if err := h.settings.DeleteUser(username); err != nil {
		h.writeLeaderRedirect(w, err)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

// ---------- API Key Management Handlers ----------

func (h *Handler) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.settings.ListAPIKeys()
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, keys)
}

func (h *Handler) saveAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var k auth.APIKey
	if err := json.NewDecoder(r.Body).Decode(&k); err != nil {
		httpError(w, http.StatusBadRequest, "invalid body")
		return
	}
	k.CreatedAt = time.Now().Unix()

	if err := h.settings.AddAPIKey(&k); err != nil {
		h.writeLeaderRedirect(w, err)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) deleteAPIKey(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if err := h.settings.DeleteAPIKey(key); err != nil {
		h.writeLeaderRedirect(w, err)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

// ---------- Runtime Mode Handler ----------

func (h *Handler) mode(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		systemSettings := h.settings.GetSystemSettings()
		jsonOK(w, map[string]string{
			"mode":        systemSettings.Mode,
			"consistency": systemSettings.Consistency,
			"log_level":   h.settings.GetLogLevel(),
		})
		return
	}
	if r.Method == http.MethodPost {
		mode := r.URL.Query().Get("mode")
		consistency := r.URL.Query().Get("consistency")
		env := r.URL.Query().Get("env")
		logLevel := r.URL.Query().Get("log_level")

		if mode != "" {
			logger.Info("[Settings] Changing topology to: %s", mode)
			if err := h.settings.SetEnvironment(mode); err != nil {
				h.writeLeaderRedirect(w, err)
				return
			}
		}

		if env != "" {
			logger.Info("[Settings] Changing topology to: %s", env)
			if err := h.settings.SetEnvironment(env); err != nil {
				h.writeLeaderRedirect(w, err)
				return
			}
		}

		if consistency != "" {
			logger.Info("[Settings] Changing consistency to: %s", consistency)
			if err := h.settings.SetMode(consistency); err != nil {
				h.writeLeaderRedirect(w, err)
				return
			}
		}

		if mode == "ap" || mode == "cp" {
			logger.Info("[Settings] Changing consistency to: %s", mode)
			if err := h.settings.SetMode(mode); err != nil {
				h.writeLeaderRedirect(w, err)
				return
			}
		}

		if logLevel != "" {
			logger.Info("[Settings] Changing log level to: %s", logLevel)
			if err := h.settings.SetLogLevel(logLevel); err != nil {
				h.writeLeaderRedirect(w, err)
				return
			}
		}

		jsonOK(w, map[string]string{"status": "ok"})
		return
	}
}

func (h *Handler) systemSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		jsonOK(w, h.settings.GetSystemSettings())
	case http.MethodPost:
		var req settings.SystemSettings
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpError(w, http.StatusBadRequest, "invalid body")
			return
		}
		result, err := h.settings.ApplySystemSettings(&req)
		if err != nil {
			h.writeLeaderRedirect(w, err)
			return
		}
		jsonOK(w, result)
	default:
		httpError(w, http.StatusMethodNotAllowed, "GET or POST required")
	}
}

func (h *Handler) notificationConfig(w http.ResponseWriter, r *http.Request) {
	if h.forwardedToNotifyAlertNode(w, r) {
		return
	}
	namespace, ok := h.validatedNamespace(r)
	if !ok {
		httpError(w, http.StatusBadRequest, "namespace not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		cfg, err := h.notify.Load(namespace)
		if err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonOK(w, cfg)
	case http.MethodPost:
		var cfg notify.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			httpError(w, http.StatusBadRequest, "invalid request")
			return
		}
		if err := h.notify.Save(namespace, &cfg); err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
		latest, err := h.notify.Load(namespace)
		if err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonOK(w, latest)
	default:
		httpError(w, http.StatusMethodNotAllowed, "GET or POST required")
	}
}

func (h *Handler) alertConfig(w http.ResponseWriter, r *http.Request) {
	if h.forwardedToNotifyAlertNode(w, r) {
		return
	}
	namespace, ok := h.validatedNamespace(r)
	if !ok {
		httpError(w, http.StatusBadRequest, "namespace not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		cfg, err := h.alerts.LoadConfig(namespace)
		if err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonOK(w, cfg)
	case http.MethodPost:
		var cfg alert.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			httpError(w, http.StatusBadRequest, "invalid request")
			return
		}
		if err := h.alerts.SaveConfig(namespace, &cfg); err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
		latest, err := h.alerts.LoadConfig(namespace)
		if err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonOK(w, latest)
	default:
		httpError(w, http.StatusMethodNotAllowed, "GET or POST required")
	}
}

func (h *Handler) validatedNamespace(r *http.Request) (string, bool) {
	namespace := strings.TrimSpace(r.URL.Query().Get("namespace"))
	if namespace == "" {
		namespace = catalog.DefaultNamespace
	}

	for _, item := range h.catalog.ListNamespaces() {
		if item != nil && item.Name == namespace {
			return namespace, true
		}
	}
	return "", false
}

func (h *Handler) forwardedToNotifyAlertNode(w http.ResponseWriter, r *http.Request) bool {
	systemSettings := h.settings.GetSystemSettings()
	if systemSettings == nil {
		return false
	}

	targetNodeID := strings.TrimSpace(systemSettings.NotifyAlertNodeID)
	if targetNodeID == "" || targetNodeID == h.config.NodeID {
		return false
	}

	targetBase, err := h.notifyAlertNodeBaseURL(targetNodeID)
	if err != nil {
		httpError(w, http.StatusBadGateway, err.Error())
		return true
	}

	if err := h.proxyRequest(w, r, targetBase); err != nil {
		httpError(w, http.StatusBadGateway, err.Error())
	}
	return true
}

func (h *Handler) notifyAlertNodeBaseURL(nodeID string) (string, error) {
	members, err := clusterpkg.BuildClusterMemberViews(h.config, h.settings, h.cluster, &h.nodeCache)
	if err != nil {
		return "", err
	}

	for _, member := range members {
		if member == nil || member.ID != nodeID {
			continue
		}
		if candidate := strings.TrimSpace(member.Address); candidate != "" {
			return h.normalizeAddr(candidate), nil
		}
		if candidate := strings.TrimSpace(member.HTTPAddr); candidate != "" {
			return h.normalizeAddr(candidate), nil
		}
	}
	return "", fmt.Errorf("notify/alert node %s not found", nodeID)
}

func (h *Handler) proxyRequest(w http.ResponseWriter, r *http.Request, targetBase string) error {
	var body []byte
	if r.Body != nil {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			return err
		}
		body = data
	}

	targetURL := strings.TrimRight(targetBase, "/") + r.URL.RequestURI()
	req, err := http.NewRequest(r.Method, targetURL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header = r.Header.Clone()
	req.Header.Set("X-Eden-Proxied-By", h.config.NodeID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	return err
}

func (h *Handler) testNotification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req struct {
		Rule alert.Rule `json:"rule"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	namespace := "default"
	notifyCfg, err := h.notify.Load(namespace)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to load notification config")
		return
	}

	// Filter channels requested in the test
	var targetChannels []notify.Channel
	for _, id := range req.Rule.ChannelIDs {
		for _, ch := range notifyCfg.Channels {
			if ch.ID == id {
				targetChannels = append(targetChannels, ch)
				break
			}
		}
	}

	if len(targetChannels) == 0 {
		httpError(w, http.StatusBadRequest, "no valid notification channels selected")
		return
	}

	msg := notify.Message{
		Title: req.Rule.TitleTemplate,
		Body:  req.Rule.BodyTemplate,
	}

	var errors []string
	for _, ch := range targetChannels {
		if err := h.notifyEngine.Send(ch, msg); err != nil {
			logger.Error("[Notify] Failed to send test notification to channel %s: %v", ch.Name, err)
			errors = append(errors, fmt.Sprintf("%s: %v", ch.Name, err))
		}
	}

	if len(errors) > 0 {
		httpError(w, http.StatusInternalServerError, strings.Join(errors, "; "))
		return
	}

	jsonOK(w, map[string]string{"status": "ok", "message": "测试通知已成功发送"})
}

func (h *Handler) testChannelNotification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req struct {
		Channel notify.Channel `json:"channel"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	msg := notify.Message{
		Title: "测试通知",
		Body:  "这是一条来自注册中心的测试通知，用于验证您的配置是否正确。",
	}

	if err := h.notifyEngine.Send(req.Channel, msg); err != nil {
		logger.Error("[Notify] Failed to send test notification to channel %s: %v", req.Channel.Name, err)
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, map[string]string{"status": "ok", "message": "测试通知已成功发送"})
}


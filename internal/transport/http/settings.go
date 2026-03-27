package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/shiyindaxiaojie/eden-go-logger"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/auth"
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

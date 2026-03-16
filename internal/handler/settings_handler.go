package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/shiyindaxiaojie/eden-go-logger"
	cp "github.com/shiyindaxiaojie/eden-go-registry/internal/cluster/cp"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	"golang.org/x/crypto/bcrypt"
	"strings"
)

// ---------- RBAC (User Management) Handlers ----------

func (h *Handler) handleListUsers(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, h.registry.ListUsers())
}

func (h *Handler) handleSaveUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var u model.User
	json.NewDecoder(r.Body).Decode(&u)

	// Validate and Hash password
	if u.Username == "" {
		httpError(w, http.StatusBadRequest, "username required")
		return
	}

	existingUser, exists := h.registry.GetUser(u.Username)
	if exists {
		u.IsBuiltIn = existingUser.IsBuiltIn
		// If password is left empty on an edit, keep the old password
		if u.Password == "" {
			u.Password = existingUser.Password
		} else if u.Password != existingUser.Password {
			// Hash new password if changed
			hashed, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
			if err == nil {
				u.Password = string(hashed)
			}
		}
	} else if u.Password != "" {
		// New user, hash password
		hashed, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
		if err == nil {
			u.Password = string(hashed)
		}
	}

	if h.config.Mode == "cp" {
		cmd := cp.Command{Type: cp.CmdAddUser, User: &u}
		h.cpNode.Apply(cmd, 5*time.Second)
	} else {
		h.apNode.Apply("add_user", &u, r.URL.Query().Get("replicate") == "true")
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")

	user, exists := h.registry.GetUser(username)
	if !exists {
		httpError(w, http.StatusNotFound, "user not found")
		return
	}
	if user.IsBuiltIn {
		httpError(w, http.StatusForbidden, "built-in users cannot be deleted")
		return
	}

	if h.config.Mode == "cp" {
		cmd := cp.Command{Type: cp.CmdDeleteUser, Username: username}
		h.cpNode.Apply(cmd, 5*time.Second)
	} else {
		h.apNode.Apply("delete_user", username, r.URL.Query().Get("replicate") == "true")
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

// ---------- API Key Management Handlers ----------

func (h *Handler) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, h.registry.ListAPIKeys())
}

func (h *Handler) handleSaveAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var k model.APIKey
	json.NewDecoder(r.Body).Decode(&k)
	k.CreatedAt = time.Now().Unix()

	if h.config.Mode == "cp" {
		cmd := cp.Command{Type: cp.CmdAddAPIKey, APIKey: &k}
		h.cpNode.Apply(cmd, 5*time.Second)
	} else {
		h.apNode.Apply("add_api_key", &k, r.URL.Query().Get("replicate") == "true")
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	mode := h.registry.GetMode()
	if mode == "cp" {
		cmd := cp.Command{Type: cp.CmdDeleteAPIKey, Key: key}
		h.cpNode.Apply(cmd, 5*time.Second)
	} else {
		h.apNode.Apply("delete_api_key", key, r.URL.Query().Get("replicate") == "true")
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

// ---------- Mode (Consistency) Handler ----------

func (h *Handler) handleMode(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		jsonOK(w, map[string]string{
			"mode": h.registry.GetMode(),
			"env":  h.registry.GetEnvironment(),
		})
		return
	}
	if r.Method == http.MethodPost {
		mode := r.URL.Query().Get("mode")
		env := r.URL.Query().Get("env")

		if mode != "" && mode != "ap" && mode != "cp" {
				httpError(w, http.StatusBadRequest, "invalid mode: "+mode)
				return
			}
			if env != "" && env != "standalone" && env != "cluster" {
				httpError(w, http.StatusBadRequest, "invalid env: "+env)
				return
			}

			if env != "" {
				logger.Info("[Settings] Switching environment to: %s", env)
				currentMode := h.registry.GetMode()
				currentEnv := h.registry.GetEnvironment()

				if currentEnv == "standalone" {
					h.registry.SetEnvironment(env)
				} else if env == "standalone" {
					// Switching TO standalone should always be allowed locally 
					// to recover from broken cluster states.
					logger.Warn("[Settings] Emergency switch to standalone mode")
					h.registry.SetEnvironment(env)
				} else if currentMode == "cp" {
					cmd := cp.Command{Type: cp.CmdSetEnv, Environment: env}
					if err := h.cpNode.Apply(cmd, 5*time.Second); err != nil {
						errStr := err.Error()
						if strings.Contains(errStr, "no leader") || strings.Contains(errStr, "leadership lost") || strings.Contains(errStr, "not leader") {
							logger.Warn("[Settings] Consistency cluster has no leader, performing local maintenance switch")
							h.registry.SetEnvironment(env)
						} else {
							httpError(w, http.StatusInternalServerError, "failed to apply env change: "+errStr)
							return
						}
					}
				} else {
					h.registry.SetEnvironment(env)
				}
			}

			if mode != "" {
				logger.Info("[Settings] Switching consistency mode to: %s", mode)
				currentMode := h.registry.GetMode()
				currentEnv := h.registry.GetEnvironment()

				if currentEnv == "standalone" {
					h.registry.SetMode(mode)
				} else if currentMode == "cp" {
					cmd := cp.Command{Type: cp.CmdSetMode, Mode: mode}
					if err := h.cpNode.Apply(cmd, 5*time.Second); err != nil {
						errStr := err.Error()
						if strings.Contains(errStr, "no leader") || strings.Contains(errStr, "leadership lost") {
							logger.Warn("[Settings] Consistency cluster has no leader, performing local mode switch")
							h.registry.SetMode(mode)
						} else if strings.Contains(errStr, "not leader") || strings.Contains(errStr, "redirect to") {
							h.handleLeaderRedirect(w, err)
							return
						} else {
							logger.Error("[Settings] Failed to switch CP mode: %v", err)
							httpError(w, http.StatusInternalServerError, "failed to apply mode change: "+errStr)
							return
						}
					}
				} else {
					if err := h.apNode.Apply("set_mode", mode, r.URL.Query().Get("replicate") == "true"); err != nil {
						logger.Error("[Settings] Failed to switch AP mode: %v", err)
						httpError(w, http.StatusInternalServerError, "failed to apply mode change: "+err.Error())
						return
					}
				}
			}

		jsonOK(w, map[string]string{"status": "ok"})
		return
	}
	httpError(w, http.StatusMethodNotAllowed, "Method not allowed")
}

package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	cp "github.com/shiyindaxiaojie/eden-go-registry/internal/cluster/cp"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	"golang.org/x/crypto/bcrypt"
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
		jsonOK(w, map[string]string{"mode": h.registry.GetMode()})
		return
	}
	if r.Method == http.MethodPost {
		// Allow both admin and developer to switch modes
		authMiddleware := h.RBACMiddleware("admin", "developer")
		authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mode := r.URL.Query().Get("mode")
			if mode != "ap" && mode != "cp" {
				httpError(w, http.StatusBadRequest, "invalid mode: "+mode)
				return
			}

			log.Printf("[Settings] Switching consistency mode to: %s", mode)

			currentMode := h.registry.GetMode()
			var err error
			if currentMode == "cp" {
				// We are currently in CP mode, must use Raft to change anything
				cmd := cp.Command{Type: cp.CmdSetMode, Mode: mode}
				err = h.cpNode.Apply(cmd, 5*time.Second)
			} else {
				// We are currently in AP mode, use Gossip/Broadcast to change mode
				err = h.apNode.Apply("set_mode", mode, r.URL.Query().Get("replicate") == "true")
			}

			if err != nil {
				log.Printf("[Settings] Failed to switch mode: %v", err)
				httpError(w, http.StatusInternalServerError, "failed to apply mode change: "+err.Error())
				return
			}

			jsonOK(w, map[string]string{"status": "ok", "mode": mode})
		})).ServeHTTP(w, r)
		return
	}
	httpError(w, http.StatusMethodNotAllowed, "Method not allowed")
}

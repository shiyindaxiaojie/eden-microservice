package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	logger "github.com/shiyindaxiaojie/eden-go-logger"
)

type contextKey string

const (
	UserContextKey contextKey = "user"
	RoleContextKey contextKey = "role"
)

// Auth handles JWT authentication for the console.
func (h *Handler) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.config.Auth.JWT.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Skip auth for login
		if r.URL.Path == "/v1/auth/login" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			httpError(w, http.StatusUnauthorized, "Missing Authorization header")
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			httpError(w, http.StatusUnauthorized, "Invalid Authorization header format")
			return
		}

		tokenString := parts[1]
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(h.config.Auth.JWT.Secret), nil
		})

		if err != nil || !token.Valid {
			httpError(w, http.StatusUnauthorized, "Invalid token: "+err.Error())
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			httpError(w, http.StatusUnauthorized, "Invalid token claims")
			return
		}

		username := claims["user_id"].(string)

		user, ok := h.auth.GetUser(username)
		if !ok {
			httpError(w, http.StatusUnauthorized, "user not found")
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user.Username)
		ctx = context.WithValue(ctx, RoleContextKey, user.Role)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RBAC checks if the user has the required roles.
func (h *Handler) RBAC(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !h.config.Auth.JWT.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			userRole, ok := r.Context().Value(RoleContextKey).(string)
			if !ok {
				httpError(w, http.StatusForbidden, "Role not found in context")
				return
			}

			allowed := false
			for _, role := range roles {
				if userRole == role {
					allowed = true
					break
				}
			}

			if !allowed {
				httpError(w, http.StatusForbidden, "Permission denied")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// APIKey handles API Key authentication for service registration.
func (h *Handler) APIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKeyEnabled := h.config.Auth.APIKey.Enabled
		if h.settings != nil {
			if systemSettings := h.settings.GetSystemSettings(); systemSettings != nil {
				apiKeyEnabled = systemSettings.APIKeyAuthEnabled
			}
		}
		if !apiKeyEnabled {
			next.ServeHTTP(w, r)
			return
		}

		key := r.Header.Get("X-API-Key")
		if key == "" {
			key = r.Header.Get("X-Consul-Token")
		}
		if key == "" {
			key = r.URL.Query().Get("api_key")
		}

		if key == "" {
			httpError(w, http.StatusUnauthorized, "Missing API Key")
			return
		}

		valid := false
		for _, k := range h.config.Auth.APIKey.Keys {
			if k == key {
				valid = true
				break
			}
		}

		if !valid && h.auth != nil {
			if _, ok := h.auth.VerifyAPIKey(key); ok {
				valid = true
			}
		}

		if !valid {
			httpError(w, http.StatusUnauthorized, "Invalid API Key")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ConsolidatedAuth allows EITHER a valid API Key OR a valid JWT with required roles.
func (h *Handler) ConsolidatedAuth(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			if key == "" {
				key = r.Header.Get("X-Consul-Token")
			}
			if key == "" {
				key = r.URL.Query().Get("api_key")
			}

			if key != "" {
				valid := false
				for _, k := range h.config.Auth.APIKey.Keys {
					if k == key {
						valid = true
						break
					}
				}
				if !valid && h.auth != nil {
					if _, ok := h.auth.VerifyAPIKey(key); ok {
						valid = true
					}
				}
				if valid {
					next.ServeHTTP(w, r)
					return
				}
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.Split(authHeader, " ")
				if len(parts) == 2 && parts[0] == "Bearer" {
					tokenString := parts[1]
					token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
						return []byte(h.config.Auth.JWT.Secret), nil
					})

					if err == nil && token.Valid {
						claims, ok := token.Claims.(jwt.MapClaims)
						if ok {
							userRole := claims["role"].(string)
							allowed := false
							for _, r := range roles {
								if userRole == r {
									allowed = true
									break
								}
							}
							if allowed {
								next.ServeHTTP(w, r)
								return
							}
						}
					}
				}
			}

			httpError(w, http.StatusUnauthorized, "Authentication required (Valid API Key or Authorized User)")
		})
	}
}

// GenerateToken creates a new JWT token for login.
func (h *Handler) GenerateToken(userID, role string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    role,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.config.Auth.JWT.Secret))
}

// login authenticates a user and returns a JWT token.
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request")
		return
	}

	logger.Info("[Auth] Login attempt: %s", req.Username)

	if _, err := h.auth.Login(req.Username, req.Password); err == nil {
		user, ok := h.auth.GetUser(req.Username)
		if !ok {
			httpError(w, http.StatusInternalServerError, "user not found after successful login")
			return
		}
		token, err := h.GenerateToken(user.Username, user.Role)
		if err != nil {
			httpError(w, http.StatusInternalServerError, "failed to generate token")
			return
		}
		nickname := user.Nickname
		if nickname == "" {
			nickname = user.Username
		}
		jsonOK(w, map[string]string{
			"token":    token,
			"role":     user.Role,
			"nickname": nickname,
		})
		return
	}

	httpError(w, http.StatusUnauthorized, "invalid credentials")
}

// profile handles retrieving and updating the user's basic profile.
func (h *Handler) profile(w http.ResponseWriter, r *http.Request) {
	username, _ := r.Context().Value(UserContextKey).(string)
	if username == "" {
		httpError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if r.Method == http.MethodGet {
		user, ok := h.auth.GetUser(username)
		if !ok {
			httpError(w, http.StatusNotFound, "user not found")
			return
		}
		jsonOK(w, map[string]interface{}{
			"username": user.Username,
			"nickname": user.Nickname,
			"phone":    user.Phone,
			"email":    user.Email,
			"role":     user.Role,
		})
		return
	}

	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "GET or POST required")
		return
	}

	var req struct {
		Nickname string `json:"nickname"`
		Phone    string `json:"phone"`
		Email    string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if err := h.auth.UpdateProfile(username, req.Nickname, req.Phone, req.Email); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, "Profile updated")
}

// updatePassword updates the user's password.
func (h *Handler) updatePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	username, _ := r.Context().Value(UserContextKey).(string)
	if username == "" {
		httpError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		OldPassword string `json:"old"`
		NewPassword string `json:"new"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if err := h.auth.UpdatePassword(username, req.OldPassword, req.NewPassword); err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonOK(w, "Password updated")
}

// updateGuideStatus updates the user's guide completion status.
func (h *Handler) updateGuideStatus(w http.ResponseWriter, r *http.Request) {
	username, _ := r.Context().Value(UserContextKey).(string)
	if username == "" {
		httpError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if r.Method == http.MethodGet {
		user, ok := h.auth.GetUser(username)
		if !ok {
			httpError(w, http.StatusNotFound, "user not found")
			return
		}
		jsonOK(w, map[string]interface{}{
			"guide_completed": user.GuideCompleted,
		})
		return
	}

	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "GET or POST required")
		return
	}

	var req struct {
		Completed bool `json:"completed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if err := h.auth.UpdateGuideStatus(username, req.Completed); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, "Guide status updated")
}

package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	UserContextKey contextKey = "user"
	RoleContextKey contextKey = "role"
)

// AuthMiddleware handles JWT authentication for the console.
func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
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
		role := claims["role"].(string)

		ctx := context.WithValue(r.Context(), UserContextKey, username)
		ctx = context.WithValue(ctx, RoleContextKey, role)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RBACMiddleware checks if the user has the required roles.
func (h *Handler) RBACMiddleware(roles ...string) func(http.Handler) http.Handler {
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

// APIKeyMiddleware handles API Key authentication for service registration.
func (h *Handler) APIKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.config.Auth.APIKey.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		key := r.Header.Get("X-API-Key")
		if key == "" {
			// Also check query param
			key = r.URL.Query().Get("api_key")
		}

		if key == "" {
			httpError(w, http.StatusUnauthorized, "Missing API Key")
			return
		}

		valid := false
		// Check static config keys
		for _, k := range h.config.Auth.APIKey.Keys {
			if k == key {
				valid = true
				break
			}
		}

		// Check dynamic registry keys
		if !valid {
			if k, ok := h.registry.GetAPIKey(key); ok {
				// Check expiration
				now := time.Now().Unix()
				if k.ExpiresAt == 0 || now <= k.ExpiresAt {
					valid = true
				}
			}
		}

		if !valid {
			httpError(w, http.StatusUnauthorized, "Invalid API Key")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ConsolidatedAuthMiddleware allows EITHER a valid API Key OR a valid JWT with required roles.
func (h *Handler) ConsolidatedAuthMiddleware(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Try API Key first
			key := r.Header.Get("X-API-Key")
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
				if !valid {
					if k, ok := h.registry.GetAPIKey(key); ok {
						now := time.Now().Unix()
						if k.ExpiresAt == 0 || now <= k.ExpiresAt {
							valid = true
						}
					}
				}
				if valid {
					next.ServeHTTP(w, r)
					return
				}
			}

			// 2. Try JWT
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

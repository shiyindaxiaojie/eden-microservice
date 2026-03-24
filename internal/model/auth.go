package model

// User represents a console user.
type User struct {
	Username string `json:"username"`
	Password string `json:"password"` // In memory/config it might be plain, in production should be hashed
	Nickname string `json:"nickname"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Remark   string `json:"remark"`
	Role     string `json:"role"` // "admin", "viewer"
	IsBuiltIn bool  `json:"is_builtin"`
}

// APIKey represents a key for service registration.
type APIKey struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	CreatedBy   string `json:"created_by"`
	CreatedAt   int64  `json:"created_at"`   // Unix timestamp
	ExpiresAt   int64  `json:"expires_at"`   // Unix timestamp, 0 means never
	Status      string `json:"status"`       // "active" or "expired"
}

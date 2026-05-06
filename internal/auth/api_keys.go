package auth

// APIKey represents a key for service registration.
type APIKey struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	CreatedBy   string `json:"created_by"`
	CreatedAt   int64  `json:"created_at"`
	ExpiresAt   int64  `json:"expires_at"`
	Status      string `json:"status"`
}

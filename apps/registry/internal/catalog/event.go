package catalog

import "time"

// Event records a registry change for auditing and dashboards.
type Event struct {
	ID        uint64    `json:"id"`
	Type      string    `json:"type"`
	Service   string    `json:"service"`
	Instance  string    `json:"instance"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

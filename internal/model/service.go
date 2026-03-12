package model

import "time"

// HealthStatus represents the health state of a service instance.
type HealthStatus string

const (
	HealthPassing  HealthStatus = "passing"
	HealthCritical HealthStatus = "critical"
)

// Instance represents a single instance of a service.
type Instance struct {
	ID          string            `json:"id"`
	ServiceName string            `json:"service_name"`
	Host        string            `json:"host"`
	Port        int               `json:"port"`
	Weight      int               `json:"weight"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Status      HealthStatus      `json:"status"`
	LastHeartbeat time.Time       `json:"last_heartbeat"`
	RegisteredAt  time.Time       `json:"registered_at"`
}

// Service holds a named collection of instances.
type Service struct {
	Name      string      `json:"name"`
	Instances []*Instance `json:"instances"`
}

// Event records a registry change for auditing / dashboard.
type Event struct {
	ID        uint64    `json:"id"`
	Type      string    `json:"type"`       // "register", "deregister", "health_change"
	Service   string    `json:"service"`
	Instance  string    `json:"instance"`   // host:port
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

package gateway

import (
	"errors"
	"time"
)

const DefaultNamespace = "default"

var (
	ErrNotFound      = errors.New("gateway route not found")
	ErrAlreadyExists = errors.New("gateway route already exists")
	ErrConflict      = errors.New("gateway route revision conflict")
	ErrInvalidRoute  = errors.New("invalid gateway route")
)

// Identity is the stable control-plane identity of a route.
type Identity struct {
	Namespace string `json:"namespace"`
	ID        string `json:"id"`
}

// Route is a persisted HTTP gateway route.
type Route struct {
	Identity
	Name      string        `json:"name"`
	Enabled   bool          `json:"enabled"`
	Priority  int           `json:"priority"`
	Match     RouteMatch    `json:"match"`
	Targets   []Target      `json:"targets"`
	Traffic   TrafficPolicy `json:"traffic"`
	Filters   []Filter      `json:"filters,omitempty"`
	TimeoutMS int           `json:"timeout_ms"`
	Revision  uint64        `json:"revision"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	CreatedBy string        `json:"created_by"`
	UpdatedBy string        `json:"updated_by"`
}

// RouteMatch contains conditions that all need to match a request.
type RouteMatch struct {
	Hosts      []string          `json:"hosts,omitempty"`
	PathPrefix string            `json:"path_prefix,omitempty"`
	Path       string            `json:"path,omitempty"`
	Methods    []string          `json:"methods,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
}

type TargetType string

const (
	TargetService TargetType = "service"
	TargetStatic  TargetType = "static"
)

type LoadBalance string

const (
	LoadBalanceRoundRobin LoadBalance = "round_robin"
	LoadBalanceRandom     LoadBalance = "random"
	LoadBalanceWeighted   LoadBalance = "weighted"
)

// Target is a version or release destination bound to a route.
type Target struct {
	ID          string            `json:"id"`
	Name        string            `json:"name,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Type        TargetType        `json:"type"`
	Service     *ServiceTarget    `json:"service,omitempty"`
	Static      *StaticTarget     `json:"static,omitempty"`
	LoadBalance LoadBalance       `json:"load_balance"`
	HealthyOnly bool              `json:"healthy_only"`
}

// ServiceTarget resolves endpoints through the catalog domain.
type ServiceTarget struct {
	Namespace   string `json:"namespace"`
	Group       string `json:"group"`
	ServiceName string `json:"service_name"`
}

// StaticTarget owns a list of HTTP endpoints and does not register them in catalog.
type StaticTarget struct {
	Endpoints []StaticEndpoint `json:"endpoints"`
}

type StaticEndpoint struct {
	URL    string `json:"url"`
	Weight int    `json:"weight"`
}

type TrafficMode string

const (
	TrafficWeighted  TrafficMode = "weighted"
	TrafficCanary    TrafficMode = "canary"
	TrafficBlueGreen TrafficMode = "blue_green"
)

// TrafficPolicy selects a release target after a route matches.
type TrafficPolicy struct {
	Mode            TrafficMode      `json:"mode"`
	DefaultTargetID string           `json:"default_target_id,omitempty"`
	WeightedTargets []WeightedTarget `json:"weighted_targets,omitempty"`
	BetaTargets     []BetaTarget     `json:"beta_targets,omitempty"`
	ActiveTargetID  string           `json:"active_target_id,omitempty"`
}

type WeightedTarget struct {
	TargetID string `json:"target_id"`
	Weight   int    `json:"weight"`
}

// BetaTarget pins listed users and tenants to a target before normal release selection.
type BetaTarget struct {
	TargetID string   `json:"target_id"`
	Users    []string `json:"users,omitempty"`
	Tenants  []string `json:"tenants,omitempty"`
}

type FilterType string

const (
	FilterStripPrefix       FilterType = "strip_prefix"
	FilterAddRequestHeader  FilterType = "add_request_header"
	FilterSetResponseHeader FilterType = "set_response_header"
)

type Filter struct {
	Type  FilterType `json:"type"`
	Name  string     `json:"name,omitempty"`
	Value string     `json:"value,omitempty"`
	Parts int        `json:"parts,omitempty"`
}

type HistoryAction string

const (
	HistoryCreate  HistoryAction = "create"
	HistoryUpdate  HistoryAction = "update"
	HistoryDelete  HistoryAction = "delete"
	HistoryEnable  HistoryAction = "enable"
	HistoryDisable HistoryAction = "disable"
)

type HistoryEntry struct {
	Identity
	Route     *Route        `json:"route,omitempty"`
	Revision  uint64        `json:"revision"`
	Action    HistoryAction `json:"action"`
	Operator  string        `json:"operator"`
	Summary   string        `json:"summary"`
	CreatedAt time.Time     `json:"created_at"`
}

type ListQuery struct {
	Namespace string
	Query     string
	Enabled   *bool
	Page      int
	PageSize  int
}

type ListResult struct {
	Total int     `json:"total"`
	Data  []Route `json:"data"`
}

type CreateRequest struct {
	Route    Route  `json:"route"`
	Operator string `json:"-"`
}

type UpdateRequest struct {
	Route            Route  `json:"route"`
	ExpectedRevision uint64 `json:"expected_revision"`
	Operator         string `json:"-"`
}

type RuntimeStatus struct {
	Identity
	Enabled          bool      `json:"enabled"`
	DataPlaneEnabled bool      `json:"data_plane_enabled"`
	Requests         uint64    `json:"requests"`
	Errors           uint64    `json:"errors"`
	LastStatus       int       `json:"last_status"`
	LastError        string    `json:"last_error,omitempty"`
	LastRequestAt    time.Time `json:"last_request_at,omitempty"`
	SnapshotRevision uint64    `json:"snapshot_revision"`
}

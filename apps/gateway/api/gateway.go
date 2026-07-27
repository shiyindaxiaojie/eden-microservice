// Package api defines the public gateway control-plane and runtime contract.
package gateway

import internal "eden-microservice/apps/gateway/internal/gateway"

type (
	BetaTarget     = internal.BetaTarget
	CreateRequest  = internal.CreateRequest
	Discovery      = internal.Discovery
	Filter         = internal.Filter
	FilterType     = internal.FilterType
	HistoryAction  = internal.HistoryAction
	HistoryEntry   = internal.HistoryEntry
	Identity       = internal.Identity
	ListQuery      = internal.ListQuery
	ListResult     = internal.ListResult
	LoadBalance    = internal.LoadBalance
	Route          = internal.Route
	RouteMatch     = internal.RouteMatch
	Runtime        = internal.Runtime
	RuntimeConfig  = internal.RuntimeConfig
	RuntimeStatus  = internal.RuntimeStatus
	Service        = internal.Service
	ServiceTarget  = internal.ServiceTarget
	StaticEndpoint = internal.StaticEndpoint
	StaticTarget   = internal.StaticTarget
	Target         = internal.Target
	TargetType     = internal.TargetType
	TrafficMode    = internal.TrafficMode
	TrafficPolicy  = internal.TrafficPolicy
	UpdateRequest  = internal.UpdateRequest
	WeightedTarget = internal.WeightedTarget
)

const (
	TargetService           = internal.TargetService
	TargetStatic            = internal.TargetStatic
	LoadBalanceRoundRobin   = internal.LoadBalanceRoundRobin
	LoadBalanceRandom       = internal.LoadBalanceRandom
	LoadBalanceWeighted     = internal.LoadBalanceWeighted
	TrafficWeighted         = internal.TrafficWeighted
	TrafficCanary           = internal.TrafficCanary
	TrafficBlueGreen        = internal.TrafficBlueGreen
	FilterStripPrefix       = internal.FilterStripPrefix
	FilterAddRequestHeader  = internal.FilterAddRequestHeader
	FilterSetResponseHeader = internal.FilterSetResponseHeader
	HistoryCreate           = internal.HistoryCreate
	HistoryUpdate           = internal.HistoryUpdate
	HistoryDelete           = internal.HistoryDelete
	HistoryEnable           = internal.HistoryEnable
	HistoryDisable          = internal.HistoryDisable
)

var (
	ErrAlreadyExists = internal.ErrAlreadyExists
	ErrConflict      = internal.ErrConflict
	ErrInvalidRoute  = internal.ErrInvalidRoute
	ErrNotFound      = internal.ErrNotFound
	NewRuntime       = internal.NewRuntime
	Open             = internal.Open
)

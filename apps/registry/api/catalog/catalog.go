// Package catalog defines the public registry contract used by other modules.
package catalog

import internal "eden-microservice/apps/registry/internal/catalog"

type (
	APNode         = internal.APNode
	CPNode         = internal.CPNode
	Checker        = internal.Checker
	Event          = internal.Event
	HealthSettings = internal.HealthSettings
	HealthStatus   = internal.HealthStatus
	Instance       = internal.Instance
	MetricsStore   = internal.MetricsStore
	ModeReader     = internal.ModeReader
	Namespace      = internal.Namespace
	Registry       = internal.Registry
	ServiceSummary = internal.ServiceSummary
	State          = internal.State
	Stats          = internal.Stats
	TopologyGraph  = internal.TopologyGraph
	TopologyReport = internal.TopologyReport
	WatchEvent     = internal.WatchEvent
)

const (
	DefaultNamespace          = internal.DefaultNamespace
	DefaultServiceGroup       = internal.DefaultServiceGroup
	ServiceGroupSeparator     = internal.ServiceGroupSeparator
	HealthPassing             = internal.HealthPassing
	HealthCritical            = internal.HealthCritical
	HealthOffline             = internal.HealthOffline
	EventTypeServiceRegister  = internal.EventTypeServiceRegister
	EventTypeServiceOnline    = internal.EventTypeServiceOnline
	EventTypeServiceOffline   = internal.EventTypeServiceOffline
	EventTypeRegistryNodeSync = internal.EventTypeRegistryNodeSync
	EventTypeServiceHeartbeat = internal.EventTypeServiceHeartbeat
	EventTypeServiceRemove    = internal.EventTypeServiceRemove
)

var (
	DefaultEventTypes        = internal.DefaultEventTypes
	IsValidEventType         = internal.IsValidEventType
	NewChecker               = internal.NewChecker
	NewRegistry              = internal.NewRegistry
	NewState                 = internal.NewState
	NormalizeEventTypes      = internal.NormalizeEventTypes
	NormalizeServiceIdentity = internal.NormalizeServiceIdentity
	QualifiedServiceName     = internal.QualifiedServiceName
)

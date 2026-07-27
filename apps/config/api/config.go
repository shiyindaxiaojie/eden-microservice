// Package api defines the public configuration-center contract.
package configcenter

import internal "eden-microservice/apps/config/internal/configcenter"

type (
	Change         = internal.Change
	HistoryEntry   = internal.HistoryEntry
	Identity       = internal.Identity
	ListQuery      = internal.ListQuery
	ListResult     = internal.ListResult
	PublishRequest = internal.PublishRequest
	Resource       = internal.Resource
	Service        = internal.Service
	WatchTarget    = internal.WatchTarget
)

const (
	DefaultNamespace = internal.DefaultNamespace
	DefaultGroup     = internal.DefaultGroup
	HistoryPublish   = internal.HistoryPublish
	HistoryDelete    = internal.HistoryDelete
)

var (
	ErrConflict        = internal.ErrConflict
	ErrInvalidIdentity = internal.ErrInvalidIdentity
	ErrNotFound        = internal.ErrNotFound
	ErrTooManyTargets  = internal.ErrTooManyTargets
	ErrTooManyWaiters  = internal.ErrTooManyWaiters
	NormalizeIdentity  = internal.NormalizeIdentity
	Open               = internal.Open
)

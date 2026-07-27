// Package settings exposes cluster runtime settings to the aggregate server.
package settings

import internal "eden-microservice/apps/cluster/internal/settings"

type (
	APNode                    = internal.APNode
	ApplySystemSettingsResult = internal.ApplySystemSettingsResult
	Controller                = internal.Controller
	CPNode                    = internal.CPNode
	EventCleaner              = internal.EventCleaner
	MetricsCleaner            = internal.MetricsCleaner
	Profile                   = internal.Profile
	RuntimeStorage            = internal.RuntimeStorage
	StartupState              = internal.StartupState
	SystemSettings            = internal.SystemSettings
)

var (
	NewController = internal.NewController
	NewProfile    = internal.NewProfile
)

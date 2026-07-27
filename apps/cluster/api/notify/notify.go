// Package notify exposes cluster notification configuration to the aggregate server.
package notify

import internal "eden-microservice/apps/cluster/internal/notify"

type (
	Channel        = internal.Channel
	Config         = internal.Config
	ConfigProvider = internal.ConfigProvider
	Engine         = internal.Engine
	Message        = internal.Message
	Store          = internal.Store
)

var (
	NewEngine = internal.NewEngine
	NewStore  = internal.NewStore
)

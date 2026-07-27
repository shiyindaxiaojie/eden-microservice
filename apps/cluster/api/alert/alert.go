// Package alert exposes cluster alert configuration to the aggregate server.
package alert

import internal "eden-microservice/apps/cluster/internal/alert"

type (
	Config         = internal.Config
	ConfigProvider = internal.ConfigProvider
	Evaluator      = internal.Evaluator
	Rule           = internal.Rule
	Store          = internal.Store
)

var (
	NewEvaluator = internal.NewEvaluator
	NewStore     = internal.NewStore
)

// Package module is the public composition surface for cluster management.
package module

import (
	api "eden-microservice/apps/cluster/api"
	"eden-microservice/apps/cluster/api/ap"
	"eden-microservice/apps/cluster/api/cp"
)

type (
	APNode   = ap.Node
	CPConfig = cp.Config
	CPNode   = cp.Node
)

var (
	NewAPNode       = ap.NewNode
	NewCPNode       = cp.NewNode
	NewMembership   = api.NewMembership
	NewRuntimeState = api.NewRuntimeState
)

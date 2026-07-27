// Package cp exposes the CP/Raft cluster node without exposing implementation paths.
package cp

import internal "eden-microservice/apps/cluster/internal/cluster/cp"

type (
	Config   = internal.Config
	Node     = internal.Node
	ServerID = internal.ServerID
)

var NewNode = internal.NewNode

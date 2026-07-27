// Package ap exposes the AP cluster node without exposing implementation paths.
package ap

import internal "eden-microservice/apps/cluster/internal/cluster/ap"

type Node = internal.Node

var NewNode = internal.NewNode

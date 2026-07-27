// Package api defines the public cluster-management contract.
package cluster

import internal "eden-microservice/apps/cluster/internal/cluster"

type (
	ClusterMember     = internal.ClusterMember
	ClusterMemberView = internal.ClusterMemberView
	ConsensusNode     = internal.ConsensusNode
	Membership        = internal.Membership
	Peer              = internal.Peer
	PeerManager       = internal.PeerManager
	RuntimeState      = internal.RuntimeState
	SettingsReader    = internal.SettingsReader
	SnapshotData      = internal.SnapshotData
)

var (
	BuildClusterMemberViews = internal.BuildClusterMemberViews
	NewMembership           = internal.NewMembership
	NewPeerManager          = internal.NewPeerManager
	NewRuntimeState         = internal.NewRuntimeState
)

package handler

// CPNode represents the CP mode node.
type CPNode interface {
	Apply(cmd interface{}, timeout interface{}) error
	Join(nodeID, addr string) error
	Members() (interface{}, error)
	IsLeader() bool
	LeaderAddr() string
	RemoveServer(nodeID string) error
}

// APNode represents the AP mode node.
type APNode interface {
	Apply(cmdType string, data interface{}, isReplicate bool) error
	SyncSeeds()
}

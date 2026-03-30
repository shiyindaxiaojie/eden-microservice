// Package model is a drop-in replacement for
// github.com/nacos-group/nacos-sdk-go/v2/model.
// It provides the same Instance type.
package model

// Instance represents a Nacos service instance.
type Instance struct {
	InstanceId  string
	Ip          string
	Port        uint64
	Weight      float64
	Healthy     bool
	Enable      bool
	Ephemeral   bool
	ClusterName string
	ServiceName string
	Metadata    map[string]string
}

// ServiceInfo holds service information.
type ServiceInfo struct {
	Name     string
	Clusters string
	Hosts    []Instance
}

// ServiceList is a paginated list of service names.
type ServiceList struct {
	Count int64
	Doms  []string
}

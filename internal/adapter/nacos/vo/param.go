// Package vo is a drop-in replacement for
// github.com/nacos-group/nacos-sdk-go/v2/vo.
// It provides the same parameter types for Nacos API calls.
package vo

import (
	"github.com/shiyindaxiaojie/eden-go-registry/internal/adapter/nacos/common/constant"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/adapter/nacos/model"
)

// NacosClientParam holds the params for creating a Nacos client.
type NacosClientParam struct {
	ClientConfig  *constant.ClientConfig
	ServerConfigs []constant.ServerConfig
}

// RegisterInstanceParam defines the parameters for registering an instance.
type RegisterInstanceParam struct {
	Ip          string
	Port        uint64
	Weight      float64
	Enable      bool
	Healthy     bool
	Metadata    map[string]string
	ClusterName string
	ServiceName string
	GroupName   string
	Ephemeral   bool
}

// DeregisterInstanceParam defines the parameters for deregistering an instance.
type DeregisterInstanceParam struct {
	Ip          string
	Port        uint64
	ServiceName string
	Cluster     string
	GroupName   string
	Ephemeral   bool
}

// SelectInstancesParam defines the parameters for selecting instances.
type SelectInstancesParam struct {
	ServiceName string
	GroupName   string
	Clusters    []string
	HealthyOnly bool
}

// SelectOneHealthyInstanceParam defines the parameters for selecting one healthy instance.
type SelectOneHealthyInstanceParam struct {
	ServiceName string
	GroupName   string
	Clusters    []string
}

// GetAllServiceInfoParam defines the parameters for getting all services.
type GetAllServiceInfoParam struct {
	NameSpace string
	GroupName string
	PageNo    uint32
	PageSize  uint32
}

// SubscribeParam defines the parameters for subscribing to a service.
type SubscribeParam struct {
	ServiceName       string
	GroupName         string
	Clusters          []string
	SubscribeCallback func(services []model.Instance, err error)
}

// GetServiceParam defines the parameters for getting a service.
type GetServiceParam struct {
	ServiceName string
	GroupName   string
	Clusters    []string
}

// SelectAllInstancesParam defines the parameters for selecting all instances.
type SelectAllInstancesParam struct {
	ServiceName string
	GroupName   string
	Clusters    []string
}

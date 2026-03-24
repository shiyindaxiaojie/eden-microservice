package service

import (
	"github.com/shiyindaxiaojie/eden-go-registry/internal/config"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/store"
	"time"
)

// WatchEvent represents a service watch notification.
type WatchEvent struct {
	Action    string
	Instances []*model.Instance
}

// CatalogService handles service registration and discovery.
type CatalogService interface {
	Register(inst *model.Instance) error
	SetInstanceStatus(namespace, serviceName, instanceID string, status string) error
	Heartbeat(namespace, serviceName, instanceID string) error
	ListServices(namespace string) ([]interface{}, error)
	GetService(namespace, name string, healthyOnly bool) ([]*model.Instance, error)
	Subscribe(namespace, serviceName, consumerService string, ch chan WatchEvent)
	Unsubscribe(namespace, serviceName string, ch chan WatchEvent)
	GetSubscribers(namespace, serviceName string) []string
	RecordDependency(namespace, consumerService, providerService string)
	GetDependencyGraph(namespace string) map[string]interface{}
	ReportTopology(namespace, consumerService string, providers []string, checksum string) bool
	GetTopology(namespace string) *model.TopologyGraph

	// Namespace management
	ListNamespaces() []*model.Namespace
	CreateNamespace(ns *model.Namespace) bool
	UpdateNamespace(ns *model.Namespace) bool
	DeleteNamespace(name string) bool
}

// AuthService handles authentication and user lookup.
type AuthService interface {
	Login(username, password string) (string, error)
	VerifyAPIKey(key string) (*model.APIKey, bool)
	GetUser(username string) (*model.User, bool)
	UpdateProfile(username, nickname, phone, email string) error
	UpdatePassword(username, oldPassword, newPassword string) error
}

// SettingsService handles system settings and user management.
type SettingsService interface {
	AddUser(u *model.User) error
	GetUser(username string) (*model.User, bool)
	DeleteUser(username string) error
	ListUsers() ([]*model.User, error)
	AddAPIKey(key *model.APIKey) error
	DeleteAPIKey(key string) error
	ListAPIKeys() ([]*model.APIKey, error)
	SetMode(mode string) error
	SetEnvironment(env string) error
	SetLogLevel(level string) error
	GetMode() string
	GetEnvironment() string
	GetSeeds() []string
	SetSeeds(seeds []string) error
	SaveSeedsLocal(seeds []string)      // save locally only, no broadcast
	SaveSettingLocal(key, value string) // save setting locally, no broadcast
}

// ClusterService handles cluster membership and monitoring.
type ClusterService interface {
	JoinCluster(nodeID, addr string) error
	GetMembers() (interface{}, error)
	RemoveMember(nodeID string) error
	ListEvents() ([]*model.Event, error)
	GetStats() (*store.Stats, error)
	IsLeader() bool
	LeaderAddr() string
	ReplicateData(cmdType string, payload []byte)
}

// CPNode and APNode interfaces for service layer to interact with cluster nodes.
type CPNode interface {
	Apply(cmd interface{}, timeout time.Duration) error
	IsLeader() bool
	LeaderAddr() string
	LeaderID() string
	Join(nodeID, addr string) error
	Members() (interface{}, error)
	RemoveServer(nodeID string) error
}

type APNode interface {
	Apply(cmdType string, data interface{}, isReplicate bool) error
	SyncSeeds()
	GetPM() interface{}
	GetConfig() *config.Config
}

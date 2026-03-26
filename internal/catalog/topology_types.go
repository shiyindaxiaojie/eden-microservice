package catalog

// TopologyReport stores the latest subscription set reported by a consumer service.
type TopologyReport struct {
	ConsumerService string   `json:"consumer_service"`
	Providers       []string `json:"providers"`
	Checksum        string   `json:"checksum"`
	UpdatedAt       string   `json:"updated_at"`
}

// TopologyInstance describes a runtime service instance attached to a topology node.
type TopologyInstance struct {
	ID         string       `json:"id"`
	Host       string       `json:"host"`
	Port       int          `json:"port"`
	Status     HealthStatus `json:"status"`
	Datacenter string       `json:"datacenter,omitempty"`
}

// TopologyNode describes a service node in the topology graph.
type TopologyNode struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Namespace     string             `json:"namespace"`
	InstanceCount int                `json:"instance_count"`
	HealthyCount  int                `json:"healthy_count"`
	Instances     []TopologyInstance `json:"instances"`
}

// TopologyEdge describes a service dependency edge in the topology graph.
type TopologyEdge struct {
	Source    string `json:"source"`
	Target    string `json:"target"`
	Checksum  string `json:"checksum,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// TopologyGraph is the namespace-scoped runtime topology graph.
type TopologyGraph struct {
	Namespace string         `json:"namespace"`
	Nodes     []TopologyNode `json:"nodes"`
	Edges     []TopologyEdge `json:"edges"`
}

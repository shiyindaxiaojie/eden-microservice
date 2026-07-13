package catalog

import "testing"

func TestRegistryListAndTopologyExposeSeparatedServiceGroup(t *testing.T) {
	registry := NewRegistry(NewState(""), nil, nil, nil)
	if err := registry.Register(&Instance{
		ID:          "auth-1",
		ServiceName: "auth-service",
		Group:       "DEFAULT_GROUP",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	services, err := registry.ListServices(DefaultNamespace)
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected one service, got %d", len(services))
	}
	service, ok := services[0].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected service payload: %#v", services[0])
	}
	if service["name"] != "auth-service" || service["group"] != "DEFAULT_GROUP" {
		t.Fatalf("expected separated service identity, got %#v", service)
	}
	if service["qualified_name"] != "DEFAULT_GROUP@@auth-service" {
		t.Fatalf("unexpected qualified name: %#v", service["qualified_name"])
	}

	topology := registry.GetTopology(DefaultNamespace)
	if len(topology.Nodes) != 1 {
		t.Fatalf("expected one topology node, got %d", len(topology.Nodes))
	}
	node := topology.Nodes[0]
	if node.ID != "DEFAULT_GROUP@@auth-service" || node.Name != "auth-service" || node.Group != "DEFAULT_GROUP" {
		t.Fatalf("unexpected topology identity: %#v", node)
	}
}

func TestReportTopologyResolvesUnqualifiedNamesToGroupedServices(t *testing.T) {
	registry := NewRegistry(NewState(""), nil, nil, nil)
	for index, name := range []string{"nacos-order-center", "nacos-user-center", "nacos-auth-center"} {
		if err := registry.Register(&Instance{
			ID:          name + "-" + string(rune('1'+index)),
			ServiceName: name,
			Group:       "DEFAULT_GROUP",
		}); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}

	if !registry.ReportTopology(
		DefaultNamespace,
		"nacos-order-center",
		[]string{"nacos-user-center", "nacos-auth-center"},
		"example-checksum",
	) {
		t.Fatal("expected topology report to change stored state")
	}

	topology := registry.GetTopology(DefaultNamespace)
	if len(topology.Edges) != 2 {
		t.Fatalf("expected two grouped dependency edges, got %#v", topology.Edges)
	}
	want := map[string]bool{
		"DEFAULT_GROUP@@nacos-order-center->DEFAULT_GROUP@@nacos-auth-center": true,
		"DEFAULT_GROUP@@nacos-order-center->DEFAULT_GROUP@@nacos-user-center": true,
	}
	for _, edge := range topology.Edges {
		key := edge.Source + "->" + edge.Target
		if !want[key] {
			t.Fatalf("unexpected dependency edge %q", key)
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Fatalf("missing dependency edges: %#v", want)
	}
}

func TestGetTopologyMigratesStoredUnqualifiedGroupReports(t *testing.T) {
	state := NewState("")
	registry := NewRegistry(state, nil, nil, nil)
	for _, name := range []string{"nacos-order-center", "nacos-auth-center"} {
		if err := registry.Register(&Instance{
			ID:          name + "-1",
			ServiceName: name,
			Group:       "DEFAULT_GROUP",
		}); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}
	state.Topology.Report(
		DefaultNamespace,
		"nacos-order-center",
		[]string{"nacos-auth-center"},
		"legacy-checksum",
	)

	topology := registry.GetTopology(DefaultNamespace)
	if len(topology.Edges) != 1 {
		t.Fatalf("expected migrated dependency edge, got %#v", topology.Edges)
	}
	edge := topology.Edges[0]
	if edge.Source != "DEFAULT_GROUP@@nacos-order-center" || edge.Target != "DEFAULT_GROUP@@nacos-auth-center" {
		t.Fatalf("unexpected migrated edge: %#v", edge)
	}
}

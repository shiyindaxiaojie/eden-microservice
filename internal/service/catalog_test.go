package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/store"
)

func TestRecordDependencyPersistsTopologyReport(t *testing.T) {
	dataDir := t.TempDir()
	registry := store.NewRegistry(dataDir)
	catalog := NewCatalogService(registry, nil, nil)

	catalog.RecordDependency("default", "order-center", "user-center")

	reports := readTopologyReports(t, dataDir)
	report := reports["default"]["order-center"]
	if report == nil {
		t.Fatalf("expected persisted report for order-center")
	}
	if !sameStringSlices(report.Providers, []string{"user-center"}) {
		t.Fatalf("unexpected providers: %#v", report.Providers)
	}
}

func TestRecordDependencyKeepsChecksumUntilProvidersChange(t *testing.T) {
	dataDir := t.TempDir()
	registry := store.NewRegistry(dataDir)
	catalog := NewCatalogService(registry, nil, nil)

	changed := catalog.ReportTopology("default", "order-center", []string{"user-center"}, "checksum-1")
	if !changed {
		t.Fatalf("expected initial topology report to be stored")
	}

	catalog.RecordDependency("default", "order-center", "user-center")

	reports := readTopologyReports(t, dataDir)
	report := reports["default"]["order-center"]
	if report == nil {
		t.Fatalf("expected persisted report for order-center")
	}
	if report.Checksum != "checksum-1" {
		t.Fatalf("expected checksum to be preserved, got %q", report.Checksum)
	}

	catalog.RecordDependency("default", "order-center", "auth-center")

	reports = readTopologyReports(t, dataDir)
	report = reports["default"]["order-center"]
	if report == nil {
		t.Fatalf("expected updated report for order-center")
	}
	if !sameStringSlices(report.Providers, []string{"auth-center", "user-center"}) {
		t.Fatalf("unexpected providers after merge: %#v", report.Providers)
	}
	if report.Checksum != "" {
		t.Fatalf("expected checksum to be cleared after dependency merge, got %q", report.Checksum)
	}
}

func TestGetTopologyBackfillsPersistedReportsFromInMemoryDependencies(t *testing.T) {
	dataDir := t.TempDir()
	registry := store.NewRegistry(dataDir)
	catalog := NewCatalogService(registry, nil, nil).(*catalogService)

	registry.Register(&model.Instance{ID: "order-1", ServiceName: "order-center", Host: "127.0.0.1", Port: 8080})
	registry.Register(&model.Instance{ID: "user-1", ServiceName: "user-center", Host: "127.0.0.1", Port: 8081})

	catalog.deps["default"] = map[string]map[string]bool{
		"order-center": {
			"user-center": true,
		},
	}

	if _, err := os.Stat(filepath.Join(dataDir, "topology.json")); !os.IsNotExist(err) {
		t.Fatalf("expected topology.json to be absent before backfill")
	}

	graph := catalog.GetTopology("default")
	if len(graph.Edges) != 1 {
		t.Fatalf("expected one topology edge after backfill, got %d", len(graph.Edges))
	}

	reports := readTopologyReports(t, dataDir)
	report := reports["default"]["order-center"]
	if report == nil {
		t.Fatalf("expected persisted report after topology read")
	}
	if !sameStringSlices(report.Providers, []string{"user-center"}) {
		t.Fatalf("unexpected providers after backfill: %#v", report.Providers)
	}
}

func readTopologyReports(t *testing.T, dataDir string) map[string]map[string]*model.TopologyReport {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dataDir, "topology.json"))
	if err != nil {
		t.Fatalf("failed to read topology.json: %v", err)
	}

	var reports map[string]map[string]*model.TopologyReport
	if err := json.Unmarshal(data, &reports); err != nil {
		t.Fatalf("failed to decode topology.json: %v", err)
	}
	return reports
}

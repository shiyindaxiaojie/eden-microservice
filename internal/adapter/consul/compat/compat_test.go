package compat

import (
	"encoding/json"
	"testing"
)

func TestBuildCatalogServiceEnvelopesPreservesManualOffline(t *testing.T) {
	rows := BuildCatalogServiceEnvelopes([]Instance{
		{
			ID:            "instance-1",
			ServiceName:   "auth-service",
			Status:        "offline",
			ManualOffline: true,
		},
	}, nil)
	if len(rows) != 1 {
		t.Fatalf("expected one catalog row, got %d", len(rows))
	}

	payload, err := json.Marshal(rows)
	if err != nil {
		t.Fatalf("marshal catalog rows: %v", err)
	}

	var decoded []map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode catalog rows: %v", err)
	}
	if manualOffline, ok := decoded[0]["manual_offline"].(bool); !ok || !manualOffline {
		t.Fatalf("expected manual_offline=true in catalog row, got %#v", decoded[0]["manual_offline"])
	}
}

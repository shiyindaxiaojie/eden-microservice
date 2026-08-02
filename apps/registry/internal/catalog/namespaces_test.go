package catalog

import "testing"

func TestNamespaceTimestamps(t *testing.T) {
	registry := NewNamespaceRegistry("")
	if !registry.Create(&Namespace{Name: "payments", Description: "Payments"}) {
		t.Fatal("Create() = false, want true")
	}

	created, ok := registry.Get("payments")
	if !ok {
		t.Fatal("created namespace not found")
	}
	if created.CreatedAt == "" || created.UpdatedAt == "" {
		t.Fatalf("timestamps = (%q, %q), want both populated", created.CreatedAt, created.UpdatedAt)
	}

	registry.namespaces["payments"].UpdatedAt = "2000-01-01T00:00:00Z"
	if !registry.Update(&Namespace{Name: "payments", Description: "Updated payments"}) {
		t.Fatal("Update() = false, want true")
	}

	updated, ok := registry.Get("payments")
	if !ok {
		t.Fatal("updated namespace not found")
	}
	if updated.CreatedAt != created.CreatedAt {
		t.Fatalf("CreatedAt = %q after update, want %q", updated.CreatedAt, created.CreatedAt)
	}
	if updated.UpdatedAt == "2000-01-01T00:00:00Z" {
		t.Fatal("UpdatedAt was not refreshed")
	}
}

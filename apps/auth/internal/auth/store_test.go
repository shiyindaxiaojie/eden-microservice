package auth

import "testing"

func TestAddUserAssignsCreatedAtAndUpdatedAt(t *testing.T) {
	store := NewStore("")
	store.AddUser(&User{Username: "alice", Role: "developer"})

	created, ok := store.GetUser("alice")
	if !ok {
		t.Fatal("created user not found")
	}
	if created.CreatedAt <= 0 {
		t.Fatalf("CreatedAt = %d, want a positive Unix timestamp", created.CreatedAt)
	}
	if created.UpdatedAt <= 0 {
		t.Fatalf("UpdatedAt = %d, want a positive Unix timestamp", created.UpdatedAt)
	}

	createdAt := created.CreatedAt
	store.users["alice"].UpdatedAt = 1
	store.AddUser(&User{Username: "alice", Role: "admin"})

	updated, ok := store.GetUser("alice")
	if !ok {
		t.Fatal("updated user not found")
	}
	if updated.CreatedAt != createdAt {
		t.Fatalf("CreatedAt = %d after update, want %d", updated.CreatedAt, createdAt)
	}
	if updated.UpdatedAt <= 1 {
		t.Fatalf("UpdatedAt = %d after update, want a refreshed timestamp", updated.UpdatedAt)
	}
}

func TestAddUserDoesNotInventCreatedAtForLegacyUser(t *testing.T) {
	store := NewStore("")
	store.users["legacy"] = &User{Username: "legacy", Role: "developer"}

	store.AddUser(&User{Username: "legacy", Role: "admin"})
	updated, ok := store.GetUser("legacy")
	if !ok {
		t.Fatal("legacy user not found after update")
	}
	if updated.CreatedAt != 0 {
		t.Fatalf("CreatedAt = %d, want 0 for an unknown historical creation time", updated.CreatedAt)
	}
	if updated.UpdatedAt <= 0 {
		t.Fatalf("UpdatedAt = %d, want a positive timestamp after editing a legacy user", updated.UpdatedAt)
	}
}

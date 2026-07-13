package gateway

import (
	"errors"
	"math"
	"testing"
	"time"
)

func openTestStore(t *testing.T) Service {
	t.Helper()
	service, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return service
}

func TestStoreUpdatesRouteAndRejectsStaleRevision(t *testing.T) {
	service := openTestStore(t)
	created, err := service.Create(CreateRequest{Route: validRoute(), Operator: "admin"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Revision == 0 || created.CreatedBy != "admin" || created.UpdatedBy != "admin" {
		t.Fatalf("created = %#v", created)
	}

	updatedRoute := *created
	updatedRoute.Name = "Orders API"
	updated, err := service.Update(UpdateRequest{Route: updatedRoute, ExpectedRevision: created.Revision, Operator: "developer"})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Revision <= created.Revision || updated.Name != "Orders API" || updated.UpdatedBy != "developer" {
		t.Fatalf("updated = %#v", updated)
	}

	_, err = service.Update(UpdateRequest{Route: updatedRoute, ExpectedRevision: created.Revision, Operator: "developer"})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("stale Update() error = %v, want ErrConflict", err)
	}

	history, err := service.History(created.Identity)
	if err != nil {
		t.Fatalf("History() error = %v", err)
	}
	if len(history) != 2 || history[0].Action != HistoryUpdate || history[1].Action != HistoryCreate {
		t.Fatalf("history = %#v", history)
	}
}

func TestStorePersistsAndNotifiesAfterMutation(t *testing.T) {
	dir := t.TempDir()
	service, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	notified := make(chan struct{}, 1)
	cancel := service.Subscribe(func() { notified <- struct{}{} })
	created, err := service.Create(CreateRequest{Route: validRoute(), Operator: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-notified:
	case <-time.After(time.Second):
		t.Fatal("subscriber was not notified")
	}
	cancel()
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.Get(created.Identity)
	if err != nil {
		t.Fatalf("Get() after reopen error = %v", err)
	}
	if got.Revision != created.Revision || got.Name != created.Name {
		t.Fatalf("reopened route = %#v, want %#v", got, created)
	}
}

func TestStoreDeleteRequiresCurrentRevision(t *testing.T) {
	service := openTestStore(t)
	created, err := service.Create(CreateRequest{Route: validRoute(), Operator: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Delete(created.Identity, created.Revision+1, "admin"); !errors.Is(err, ErrConflict) {
		t.Fatalf("Delete(stale) error = %v, want ErrConflict", err)
	}
	deleted, err := service.Delete(created.Identity, created.Revision, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if deleted.Action != HistoryDelete || deleted.Revision <= created.Revision {
		t.Fatalf("deleted = %#v", deleted)
	}
	if _, err := service.Get(created.Identity); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(deleted) error = %v, want ErrNotFound", err)
	}
}

func TestStoreListFiltersEnabledRoutes(t *testing.T) {
	service := openTestStore(t)
	enabledRoute := validRoute()
	if _, err := service.Create(CreateRequest{Route: enabledRoute, Operator: "admin"}); err != nil {
		t.Fatal(err)
	}
	disabledRoute := validRoute()
	disabledRoute.ID = "orders-disabled"
	disabledRoute.Enabled = false
	if _, err := service.Create(CreateRequest{Route: disabledRoute, Operator: "admin"}); err != nil {
		t.Fatal(err)
	}
	enabled := true
	result, err := service.List(ListQuery{Enabled: &enabled})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Data) != 1 || result.Data[0].ID != enabledRoute.ID {
		t.Fatalf("enabled route list = %#v", result)
	}
}

func TestStoreListHandlesAnOversizedPageWithoutOverflow(t *testing.T) {
	service := openTestStore(t)
	if _, err := service.Create(CreateRequest{Route: validRoute(), Operator: "admin"}); err != nil {
		t.Fatal(err)
	}
	result, err := service.List(ListQuery{Page: math.MaxInt, PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Data) != 0 {
		t.Fatalf("oversized page result = %#v", result)
	}
}

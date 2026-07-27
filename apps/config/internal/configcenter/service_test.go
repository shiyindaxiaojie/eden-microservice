package configcenter

import (
	"errors"
	"testing"
	"time"
)

func openTestService(t *testing.T, dir string) Service {
	t.Helper()
	service, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	return service
}

func TestPublishPersistsAndDoesNotAdvanceRevisionForUnchangedContent(t *testing.T) {
	dir := t.TempDir()
	service := openTestService(t, dir)

	first, err := service.Publish(PublishRequest{
		Identity: Identity{DataID: "order.yaml"},
		Content:  "port: 8080",
		Type:     "yaml",
		Operator: "admin",
	})
	if err != nil {
		t.Fatalf("Publish(first) error = %v", err)
	}
	if first.Namespace != DefaultNamespace || first.Group != DefaultGroup {
		t.Fatalf("normalized identity = %#v", first.Identity)
	}

	again, err := service.Publish(PublishRequest{
		Identity:    first.Identity,
		Content:     first.Content,
		Type:        first.Type,
		Description: "display-only change",
		Operator:    "developer",
	})
	if err != nil {
		t.Fatalf("Publish(unchanged) error = %v", err)
	}
	if again.Revision != first.Revision {
		t.Fatalf("unchanged revision = %d, want %d", again.Revision, first.Revision)
	}
	if again.Description != "display-only change" {
		t.Fatalf("description = %q", again.Description)
	}

	if err := service.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened := openTestService(t, dir)
	defer reopened.Close()
	got, err := reopened.Get(first.Identity)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.MD5 != first.MD5 || got.Content != first.Content || got.Revision != first.Revision {
		t.Fatalf("reopened resource = %#v, want md5=%q content=%q revision=%d", got, first.MD5, first.Content, first.Revision)
	}
}

func TestPublishRecordsHistoryAndRejectsStaleMD5(t *testing.T) {
	service := openTestService(t, t.TempDir())
	defer service.Close()

	first, err := service.Publish(PublishRequest{Identity: Identity{DataID: "app.properties"}, Content: "feature=false", Operator: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Publish(PublishRequest{Identity: first.Identity, Content: "feature=true", ExpectedMD5: first.MD5, Operator: "admin"})
	if err != nil {
		t.Fatalf("Publish(second) error = %v", err)
	}
	if second.Revision <= first.Revision {
		t.Fatalf("second revision = %d, want greater than %d", second.Revision, first.Revision)
	}

	_, err = service.Publish(PublishRequest{Identity: first.Identity, Content: "feature=stale", ExpectedMD5: first.MD5})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("stale Publish() error = %v, want ErrConflict", err)
	}

	history, err := service.History(first.Identity)
	if err != nil {
		t.Fatalf("History() error = %v", err)
	}
	if len(history) != 1 || history[0].Revision != first.Revision || history[0].Action != HistoryPublish {
		t.Fatalf("history = %#v, want previous publish revision %d", history, first.Revision)
	}
}

func TestDeleteHidesCurrentResourceAndKeepsDeleteHistory(t *testing.T) {
	service := openTestService(t, t.TempDir())
	defer service.Close()

	resource, err := service.Publish(PublishRequest{Identity: Identity{DataID: "deleted.txt"}, Content: "present", Operator: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := service.Delete(resource.Identity, "admin")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deleted.Action != HistoryDelete || deleted.Revision <= resource.Revision {
		t.Fatalf("delete history = %#v", deleted)
	}
	if _, err := service.Get(resource.Identity); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(deleted) error = %v, want ErrNotFound", err)
	}

	history, err := service.History(resource.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Action != HistoryDelete {
		t.Fatalf("history = %#v, want one delete entry", history)
	}
}

func TestWaitReturnsChangedIdentityAndTimeoutsCleanly(t *testing.T) {
	service := openTestService(t, t.TempDir())
	defer service.Close()

	resource, err := service.Publish(PublishRequest{Identity: Identity{DataID: "watched.yaml"}, Content: "value: 1"})
	if err != nil {
		t.Fatal(err)
	}

	result := make(chan []Change, 1)
	errCh := make(chan error, 1)
	go func() {
		changes, waitErr := service.Wait([]WatchTarget{{Identity: resource.Identity, MD5: resource.MD5}}, time.Second)
		if waitErr != nil {
			errCh <- waitErr
			return
		}
		result <- changes
	}()

	time.Sleep(25 * time.Millisecond)
	updated, err := service.Publish(PublishRequest{Identity: resource.Identity, Content: "value: 2"})
	if err != nil {
		t.Fatal(err)
	}

	select {
	case waitErr := <-errCh:
		t.Fatalf("Wait() error = %v", waitErr)
	case changes := <-result:
		if len(changes) != 1 || changes[0].MD5 != updated.MD5 || changes[0].Identity != updated.Identity {
			t.Fatalf("changes = %#v, want updated resource", changes)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Wait() did not return after publish")
	}

	changes, err := service.Wait([]WatchTarget{{Identity: updated.Identity, MD5: updated.MD5}}, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("Wait(timeout) error = %v", err)
	}
	if len(changes) != 0 {
		t.Fatalf("timeout changes = %#v, want empty", changes)
	}
}

func TestListFiltersAndPaginatesCurrentResources(t *testing.T) {
	service := openTestService(t, t.TempDir())
	defer service.Close()

	for _, request := range []PublishRequest{
		{Identity: Identity{Namespace: "prod", Group: "PAY", DataID: "payment.yaml"}, Content: "a", Type: "yaml", Tags: []string{"critical"}},
		{Identity: Identity{Namespace: "dev", Group: "ORDER", DataID: "order.json"}, Content: "b", Type: "json"},
	} {
		if _, err := service.Publish(request); err != nil {
			t.Fatal(err)
		}
	}

	listed, err := service.List(ListQuery{Namespace: "prod", Query: "payment", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if listed.Total != 1 || len(listed.Data) != 1 || listed.Data[0].DataID != "payment.yaml" {
		t.Fatalf("List() = %#v", listed)
	}
}

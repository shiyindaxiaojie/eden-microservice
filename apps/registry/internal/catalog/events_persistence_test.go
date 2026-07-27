package catalog

import (
	"testing"
	"time"
)

func TestEventLogPersistentRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	log := NewEventLog(10, dir)
	log.SetStorageMode("persistent")

	ts := time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC)
	log.Append(&Event{
		Type:      EventTypeServiceRegister,
		Service:   "auth-center",
		Instance:  "127.0.0.1:9000",
		Message:   "registered",
		Timestamp: ts,
	})
	log.Close()

	reloaded := NewEventLog(10, dir)
	reloaded.SetStorageMode("persistent")
	defer reloaded.Close()

	events := reloaded.List()
	if len(events) != 1 {
		t.Fatalf("len(List()) = %d, want 1", len(events))
	}
	if events[0].Service != "auth-center" {
		t.Fatalf("event service = %q, want auth-center", events[0].Service)
	}
	if !events[0].Timestamp.Equal(ts) {
		t.Fatalf("event timestamp = %s, want %s", events[0].Timestamp, ts)
	}
}

func TestEventLogCleanupRemovesExpiredPersistentAndInMemoryEvents(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	log := NewEventLog(10, dir)
	log.SetStorageMode("persistent")
	defer log.Close()

	oldTS := time.Now().UTC().Add(-48 * time.Hour)
	newTS := time.Now().UTC().Add(-2 * time.Hour)

	log.Append(&Event{
		Type:      EventTypeServiceOffline,
		Service:   "old-service",
		Instance:  "old-instance",
		Message:   "old",
		Timestamp: oldTS,
	})
	log.Append(&Event{
		Type:      EventTypeServiceOnline,
		Service:   "new-service",
		Instance:  "new-instance",
		Message:   "new",
		Timestamp: newTS,
	})

	log.Cleanup(1)

	events := log.List()
	if len(events) != 1 {
		t.Fatalf("len(List()) after cleanup = %d, want 1", len(events))
	}
	if events[0].Service != "new-service" {
		t.Fatalf("remaining service = %q, want new-service", events[0].Service)
	}

	all, total := log.QueryEvents(10, 0, "", "", "", "", "", "")
	if total != 1 || len(all) != 1 {
		t.Fatalf("QueryEvents() = (%d items, total=%d), want (1,1)", len(all), total)
	}
	if all[0].Service != "new-service" {
		t.Fatalf("QueryEvents remaining service = %q, want new-service", all[0].Service)
	}

	oldFiltered, oldTotal := log.QueryEvents(10, 0, "", "", "", "", "", "old-service")
	if oldTotal != 0 || len(oldFiltered) != 0 {
		t.Fatalf("old service query = (%d items, total=%d), want (0,0)", len(oldFiltered), oldTotal)
	}
}

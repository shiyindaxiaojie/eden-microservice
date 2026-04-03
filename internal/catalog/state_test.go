package catalog

import (
	"testing"
	"time"
)

func TestStateAppendEventInvokesCallback(t *testing.T) {
	t.Parallel()

	state := NewState("")
	state.SetEventTypesProvider(func() []string {
		return []string{EventTypeServiceOffline}
	})

	events := make(chan *Event, 1)
	state.SetOnEventCallback(func(event *Event) {
		events <- event
	})

	state.AppendEvent(EventTypeServiceOffline, "order-service", "10.0.0.8:8080", "Instance deregistered")

	select {
	case event := <-events:
		if event == nil {
			t.Fatalf("expected callback event")
		}
		if event.Type != EventTypeServiceOffline {
			t.Fatalf("unexpected event type: %s", event.Type)
		}
		if event.Service != "order-service" {
			t.Fatalf("unexpected service: %s", event.Service)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for callback")
	}
}

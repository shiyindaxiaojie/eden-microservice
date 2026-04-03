package alert

import (
	"errors"
	"testing"
	"time"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/catalog"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/notify"
)

type stubRuleLoader struct {
	cfg *Config
	err error
}

func (s *stubRuleLoader) LoadConfig(namespace string) (*Config, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.cfg, nil
}

type stubChannelLoader struct {
	cfg *notify.Config
	err error
}

func (s *stubChannelLoader) Load(namespace string) (*notify.Config, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.cfg, nil
}

type sentMessage struct {
	channel notify.Channel
	msg     notify.Message
}

type recordingNotifier struct {
	sent []sentMessage
	err  error
}

func (r *recordingNotifier) Send(channel notify.Channel, msg notify.Message) error {
	r.sent = append(r.sent, sentMessage{channel: channel, msg: msg})
	return r.err
}

func TestEvaluatorTriggersAtThreshold(t *testing.T) {
	t.Parallel()

	notifier := &recordingNotifier{}
	evaluator := NewEvaluator(
		&stubRuleLoader{cfg: &Config{Rules: []Rule{{
			ID:            "offline-burst",
			Name:          "Offline Burst",
			EventCode:     catalog.EventTypeServiceOffline,
			Threshold:     2,
			WindowSec:     300,
			ChannelIDs:    []string{"chan-1"},
			TitleTemplate: "Alarm - {{ event_name }}",
			BodyTemplate:  "{{ service }} {{ instance }} {{ count }} {{ threshold }} {{ window_sec }} {{ timestamp }}",
			Enabled:       true,
		}}}},
		&stubChannelLoader{cfg: &notify.Config{Channels: []notify.Channel{{
			ID:      "chan-1",
			Name:    "Primary",
			Enabled: true,
		}}}},
		notifier,
	)

	ts := time.Date(2026, 4, 3, 9, 0, 0, 0, time.UTC)
	evaluator.Evaluate(&catalog.Event{
		Type:      catalog.EventTypeServiceOffline,
		Service:   "order-service",
		Instance:  "10.0.0.8:8080",
		Message:   "Instance deregistered",
		Timestamp: ts,
	})
	if len(notifier.sent) != 0 {
		t.Fatalf("expected no notification before threshold, got %d", len(notifier.sent))
	}

	evaluator.Evaluate(&catalog.Event{
		Type:      catalog.EventTypeServiceOffline,
		Service:   "order-service",
		Instance:  "10.0.0.8:8080",
		Message:   "Instance deregistered",
		Timestamp: ts.Add(30 * time.Second),
	})
	if len(notifier.sent) != 1 {
		t.Fatalf("expected one notification at threshold, got %d", len(notifier.sent))
	}

	got := notifier.sent[0].msg
	if got.Title != "Alarm - Service Offline" {
		t.Fatalf("unexpected title: %q", got.Title)
	}
	expectedBody := "order-service 10.0.0.8:8080 2 2 300 2026-04-03T09:00:30Z"
	if got.Body != expectedBody {
		t.Fatalf("unexpected body: %q", got.Body)
	}
}

func TestEvaluatorCooldownSuppressesDuplicates(t *testing.T) {
	t.Parallel()

	notifier := &recordingNotifier{}
	evaluator := NewEvaluator(
		&stubRuleLoader{cfg: &Config{Rules: []Rule{{
			ID:         "offline-once",
			Name:       "Offline Once",
			EventCode:  catalog.EventTypeServiceOffline,
			Threshold:  1,
			WindowSec:  60,
			ChannelIDs: []string{"chan-1"},
			Enabled:    true,
		}}}},
		&stubChannelLoader{cfg: &notify.Config{Channels: []notify.Channel{{
			ID:      "chan-1",
			Name:    "Primary",
			Enabled: true,
		}}}},
		notifier,
	)

	start := time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC)
	evaluator.Evaluate(&catalog.Event{Type: catalog.EventTypeServiceOffline, Timestamp: start})
	evaluator.Evaluate(&catalog.Event{Type: catalog.EventTypeServiceOffline, Timestamp: start.Add(10 * time.Second)})
	evaluator.Evaluate(&catalog.Event{Type: catalog.EventTypeServiceOffline, Timestamp: start.Add(61 * time.Second)})

	if len(notifier.sent) != 2 {
		t.Fatalf("expected two notifications across cooldown boundary, got %d", len(notifier.sent))
	}
}

func TestEvaluatorUsesDefaultTemplates(t *testing.T) {
	t.Parallel()

	notifier := &recordingNotifier{}
	evaluator := NewEvaluator(
		&stubRuleLoader{cfg: &Config{Rules: []Rule{{
			ID:         "offline-default-template",
			Name:       "Offline Default Template",
			EventCode:  catalog.EventTypeServiceOffline,
			Threshold:  1,
			WindowSec:  60,
			ChannelIDs: []string{"chan-1"},
			Enabled:    true,
		}}}},
		&stubChannelLoader{cfg: &notify.Config{Channels: []notify.Channel{{
			ID:      "chan-1",
			Name:    "Primary",
			Enabled: true,
		}}}},
		notifier,
	)

	evaluator.Evaluate(&catalog.Event{
		Type:      catalog.EventTypeServiceOffline,
		Service:   "billing-service",
		Instance:  "10.0.0.9:8080",
		Message:   "node unhealthy",
		Timestamp: time.Date(2026, 4, 3, 11, 0, 0, 0, time.UTC),
	})

	if len(notifier.sent) != 1 {
		t.Fatalf("expected notification, got %d", len(notifier.sent))
	}
	if notifier.sent[0].msg.Title != "Registry Alarm - Service Offline" {
		t.Fatalf("unexpected default title: %q", notifier.sent[0].msg.Title)
	}
	if notifier.sent[0].msg.Body == "" {
		t.Fatalf("expected default body to be rendered")
	}
}

func TestEvaluatorSkipsWhenConfigLoadFails(t *testing.T) {
	t.Parallel()

	notifier := &recordingNotifier{}
	evaluator := NewEvaluator(
		&stubRuleLoader{err: errors.New("boom")},
		&stubChannelLoader{},
		notifier,
	)

	evaluator.Evaluate(&catalog.Event{Type: catalog.EventTypeServiceOffline, Timestamp: time.Now()})
	if len(notifier.sent) != 0 {
		t.Fatalf("expected no notifications when config load fails, got %d", len(notifier.sent))
	}
}

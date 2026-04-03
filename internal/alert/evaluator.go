package alert

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	logger "github.com/shiyindaxiaojie/eden-go-logger"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/catalog"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/notify"
)

const (
	defaultThreshold = 1
	defaultWindowSec = 300
)

type ruleLoader interface {
	LoadConfig(namespace string) (*Config, error)
}

type channelLoader interface {
	Load(namespace string) (*notify.Config, error)
}

type notifier interface {
	Send(channel notify.Channel, msg notify.Message) error
}

type dispatch struct {
	rule     Rule
	channels []notify.Channel
	message  notify.Message
}

// Evaluator matches events against alert rules and dispatches notifications.
type Evaluator struct {
	rules         ruleLoader
	channels      channelLoader
	notifier      notifier
	now           func() time.Time
	mu            sync.Mutex
	windows       map[string][]time.Time
	cooldownUntil map[string]time.Time
}

func NewEvaluator(rules ruleLoader, channels channelLoader, notifier notifier) *Evaluator {
	return &Evaluator{
		rules:         rules,
		channels:      channels,
		notifier:      notifier,
		now:           time.Now,
		windows:       make(map[string][]time.Time),
		cooldownUntil: make(map[string]time.Time),
	}
}

func (e *Evaluator) Evaluate(event *catalog.Event) {
	if e == nil || event == nil {
		return
	}

	alertCfg, err := e.rules.LoadConfig(defaultNamespace)
	if err != nil {
		logger.Error("[Alert] Failed to load alert config: %v", err)
		return
	}
	if alertCfg == nil || len(alertCfg.Rules) == 0 {
		return
	}

	notifyCfg, err := e.channels.Load(defaultNamespace)
	if err != nil {
		logger.Error("[Alert] Failed to load notify config: %v", err)
		return
	}

	channelIndex := make(map[string]notify.Channel)
	if notifyCfg != nil {
		for _, channel := range notifyCfg.Channels {
			channelIndex[channel.ID] = channel
		}
	}

	evaluatedAt := event.Timestamp
	if evaluatedAt.IsZero() {
		evaluatedAt = e.now()
	}

	pending := e.collectDispatches(alertCfg.Rules, channelIndex, event, evaluatedAt)
	for _, item := range pending {
		for _, channel := range item.channels {
			if err := e.notifier.Send(channel, item.message); err != nil {
				logger.Error("[Alert] Failed to send rule %s via channel %s: %v", item.rule.Name, channel.Name, err)
			}
		}
	}
}

func (e *Evaluator) collectDispatches(rules []Rule, channelIndex map[string]notify.Channel, event *catalog.Event, evaluatedAt time.Time) []dispatch {
	e.mu.Lock()
	defer e.mu.Unlock()

	pending := make([]dispatch, 0)
	for idx, rule := range rules {
		if !rule.Enabled || strings.TrimSpace(rule.EventCode) != event.Type {
			continue
		}

		threshold := normalizeThreshold(rule.Threshold)
		windowSec := normalizeWindow(rule.WindowSec)
		stateKey := ruleStateKey(rule, idx)

		trimmed := append(e.windows[stateKey], evaluatedAt)
		trimmed = trimWindow(trimmed, evaluatedAt, windowSec)
		e.windows[stateKey] = trimmed

		if len(trimmed) < threshold {
			continue
		}
		if until := e.cooldownUntil[stateKey]; until.After(evaluatedAt) {
			continue
		}

		channels := collectChannels(rule.ChannelIDs, channelIndex)
		if len(channels) == 0 {
			logger.Warn("[Alert] Rule %s matched event %s but no configured channels were found", rule.Name, event.Type)
		}

		count := len(trimmed)
		pending = append(pending, dispatch{
			rule:     rule,
			channels: channels,
			message: notify.Message{
				Title: e.renderTitle(rule, event, count, windowSec),
				Body:  e.renderBody(rule, event, count, windowSec),
			},
		})
		delete(e.windows, stateKey)
		e.cooldownUntil[stateKey] = evaluatedAt.Add(time.Duration(windowSec) * time.Second)
	}
	return pending
}

func collectChannels(ids []string, channelIndex map[string]notify.Channel) []notify.Channel {
	if len(ids) == 0 {
		return nil
	}

	channels := make([]notify.Channel, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		channel, ok := channelIndex[id]
		if !ok {
			continue
		}
		seen[id] = struct{}{}
		channels = append(channels, channel)
	}
	return channels
}

func normalizeThreshold(value int) int {
	if value <= 0 {
		return defaultThreshold
	}
	return value
}

func normalizeWindow(value int) int {
	if value <= 0 {
		return defaultWindowSec
	}
	return value
}

func trimWindow(values []time.Time, evaluatedAt time.Time, windowSec int) []time.Time {
	cutoff := evaluatedAt.Add(-time.Duration(windowSec) * time.Second)
	trimmed := make([]time.Time, 0, len(values))
	for _, ts := range values {
		if !ts.Before(cutoff) {
			trimmed = append(trimmed, ts)
		}
	}
	return trimmed
}

func ruleStateKey(rule Rule, idx int) string {
	if trimmed := strings.TrimSpace(rule.ID); trimmed != "" {
		return trimmed
	}
	return fmt.Sprintf("%s|%s|%d|%d|%d", rule.EventCode, rule.Name, rule.Threshold, rule.WindowSec, idx)
}

func (e *Evaluator) renderTitle(rule Rule, event *catalog.Event, count, windowSec int) string {
	template := strings.TrimSpace(rule.TitleTemplate)
	if template == "" {
		template = defaultTitle(rule, event)
	}
	return renderTemplate(template, templateVariables(rule, event, count, windowSec))
}

func (e *Evaluator) renderBody(rule Rule, event *catalog.Event, count, windowSec int) string {
	template := strings.TrimSpace(rule.BodyTemplate)
	if template == "" {
		template = defaultBody(rule, event)
	}
	return renderTemplate(template, templateVariables(rule, event, count, windowSec))
}

func renderTemplate(template string, vars map[string]string) string {
	if template == "" {
		return ""
	}

	pairs := make([]string, 0, len(vars)*4)
	for key, value := range vars {
		pairs = append(pairs,
			"{{ "+key+" }}", value,
			"{{"+key+"}}", value,
		)
	}
	return strings.NewReplacer(pairs...).Replace(template)
}

func templateVariables(rule Rule, event *catalog.Event, count, windowSec int) map[string]string {
	evaluatedAt := event.Timestamp
	if evaluatedAt.IsZero() {
		evaluatedAt = time.Now()
	}

	return map[string]string{
		"count":      fmt.Sprintf("%d", count),
		"event_code": event.Type,
		"event_name": humanizeEventName(event.Type),
		"instance":   event.Instance,
		"message":    event.Message,
		"rule_name":  rule.Name,
		"service":    event.Service,
		"threshold":  fmt.Sprintf("%d", normalizeThreshold(rule.Threshold)),
		"timestamp":  evaluatedAt.Format(time.RFC3339),
		"window_min": fmt.Sprintf("%d", int(math.Round(float64(windowSec)/60))),
		"window_sec": fmt.Sprintf("%d", windowSec),
	}
}

func defaultTitle(rule Rule, event *catalog.Event) string {
	return fmt.Sprintf("Registry Alarm - %s", humanizeEventName(event.Type))
}

func defaultBody(rule Rule, event *catalog.Event) string {
	return fmt.Sprintf(
		"Service: {{ service }}\nInstance: {{ instance }}\nEvent: {{ event_name }} ({{ event_code }})\nCondition: reached {{ threshold }} occurrences within {{ window_sec }}s\nRecorded At: {{ timestamp }}\nMessage: {{ message }}",
	)
}

func humanizeEventName(code string) string {
	switch strings.TrimSpace(code) {
	case catalog.EventTypeServiceRegister:
		return "Service Register"
	case catalog.EventTypeServiceOnline:
		return "Service Online"
	case catalog.EventTypeServiceOffline:
		return "Service Offline"
	case catalog.EventTypeRegistryNodeSync:
		return "Registry Node Sync"
	case catalog.EventTypeServiceHeartbeat:
		return "Service Heartbeat"
	case catalog.EventTypeServiceRemove:
		return "Service Remove"
	default:
		return strings.ReplaceAll(strings.TrimSpace(code), "_", " ")
	}
}

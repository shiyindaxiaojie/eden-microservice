package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
)

// EventStore handles auditing and events.
type EventStore struct {
	mu                    sync.RWMutex
	events                []*model.Event
	eventSeq              uint64
	maxEvents             int
	dataPath              string
	retentionDaysProvider func() int
}

func NewEventStore(maxEvents int, dataPath string) *EventStore {
	s := &EventStore{
		events:    make([]*model.Event, 0, maxEvents),
		maxEvents: maxEvents,
		dataPath:  dataPath,
	}
	s.init()
	return s
}

func (s *EventStore) init() {
	if s.dataPath == "" {
		return
	}
	eventDir := filepath.Join(s.dataPath, "events")
	os.MkdirAll(eventDir, 0755)
	s.Load()

	// Start retention cleaner
	go s.retentionCleaner()
}

func (s *EventStore) Append(e *model.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.eventSeq++
	e.ID = s.eventSeq
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}

	// 1. Add to in-memory ring buffer
	s.events = append(s.events, e)
	if len(s.events) > s.maxEvents {
		s.events = s.events[len(s.events)-s.maxEvents:]
	}

	// 2. Append to file (JSON Lines)
	s.appendToFile(e)
}

func (s *EventStore) appendToFile(e *model.Event) {
	if s.dataPath == "" {
		return
	}
	fileName := fmt.Sprintf("events-%s.log", time.Now().Format("2006-01-02"))
	file := filepath.Join(s.dataPath, "events", fileName)

	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	data, _ := json.Marshal(e)
	f.Write(data)
	f.WriteString("\n")
}

func (s *EventStore) List() []*model.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*model.Event, len(s.events))
	copy(result, s.events)
	return result
}

func (s *EventStore) Snapshot() []*model.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.events
}

func (s *EventStore) Restore(events []*model.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = events
	s.Save()
}

func (s *EventStore) Load() {
	if s.dataPath == "" {
		return
	}
	// Load the most recent event file to populate the in-memory buffer
	eventDir := filepath.Join(s.dataPath, "events")
	files, err := os.ReadDir(eventDir)
	if err != nil || len(files) == 0 {
		return
	}

	// Sort files by name (date) descending
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() > files[j].Name()
	})

	// Read from the latest files until we fill the buffer
	var allEvents []*model.Event
	for _, f := range files {
		if !strings.HasPrefix(f.Name(), "events-") {
			continue
		}
		path := filepath.Join(eventDir, f.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := strings.Split(string(content), "\n")
		var fileEvents []*model.Event
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var e model.Event
			if err := json.Unmarshal([]byte(line), &e); err == nil {
				fileEvents = append(fileEvents, &e)
			}
		}
		// prepend file events since we are reading newest files first
		allEvents = append(fileEvents, allEvents...)
		if len(allEvents) >= s.maxEvents {
			break
		}
	}

	if len(allEvents) > s.maxEvents {
		allEvents = allEvents[len(allEvents)-s.maxEvents:]
	}
	s.events = allEvents
	if len(allEvents) > 0 {
		s.eventSeq = allEvents[len(allEvents)-1].ID
	}
}

func (s *EventStore) Save() {
	// No-op for sequential writing as Append handles it per event
}

func (s *EventStore) SetRetentionDaysProvider(fn func() int) {
	s.retentionDaysProvider = fn
}

func (s *EventStore) retentionCleaner() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		days := 30
		if s.retentionDaysProvider != nil {
			if provided := s.retentionDaysProvider(); provided > 0 {
				days = provided
			}
		}
		s.Cleanup(days)
	}
}

func (s *EventStore) Cleanup(days int) {
	if s.dataPath == "" || days <= 0 {
		return
	}
	eventDir := filepath.Join(s.dataPath, "events")
	files, err := os.ReadDir(eventDir)
	if err != nil {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	for _, f := range files {
		if !strings.HasPrefix(f.Name(), "events-") {
			continue
		}
		// Extract date from events-YYYY-MM-DD.log
		dateStr := strings.TrimPrefix(f.Name(), "events-")
		dateStr = strings.TrimSuffix(dateStr, ".log")
		fileDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		if fileDate.Before(cutoff) {
			_ = os.Remove(filepath.Join(eventDir, f.Name()))
		}
	}
}

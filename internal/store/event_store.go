package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
)

// EventStore handles auditing and events.
type EventStore struct {
	mu        sync.RWMutex
	events    []*model.Event
	eventSeq  uint64
	maxEvents int
	dataPath  string
}

func NewEventStore(maxEvents int, dataPath string) *EventStore {
	s := &EventStore{
		events:    make([]*model.Event, 0, maxEvents),
		maxEvents: maxEvents,
		dataPath:  dataPath,
	}
	s.Load()
	return s
}

func (s *EventStore) Append(e *model.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.eventSeq++
	e.ID = s.eventSeq
	s.events = append(s.events, e)

	if len(s.events) > s.maxEvents {
		s.events = s.events[len(s.events)-s.maxEvents:]
	}
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
	file := filepath.Join(s.dataPath, "events.json")
	if data, err := os.ReadFile(file); err == nil {
		var events []*model.Event
		if err := json.Unmarshal(data, &events); err == nil {
			s.events = events
		}
	}
}

func (s *EventStore) Save() {
	if s.dataPath == "" {
		return
	}
	os.MkdirAll(s.dataPath, 0755)
	file := filepath.Join(s.dataPath, "events.json")
	data, _ := json.MarshalIndent(s.Snapshot(), "", "  ")
	_ = os.WriteFile(file, data, 0644)
}

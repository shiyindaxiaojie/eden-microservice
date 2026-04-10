package catalog

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	bucketEvents     = "events"
	bucketIdxTime    = "idx_time"
	bucketIdxType    = "idx_type"
	bucketIdxService = "idx_service"
)

// EventLog handles auditing and event retention.
type EventLog struct {
	mu                    sync.RWMutex
	events                []*Event
	eventSeq              uint64
	maxEvents             int
	dataPath              string
	db                    *bolt.DB
	retentionDaysProvider func() int
}

func NewEventLog(maxEvents int, dataPath string) *EventLog {
	s := &EventLog{
		events:    make([]*Event, 0, maxEvents),
		maxEvents: maxEvents,
		dataPath:  dataPath,
	}
	s.init()
	return s
}

func (s *EventLog) init() {
	if s.dataPath == "" {
		return
	}
	eventDir := filepath.Join(s.dataPath, "events")
	os.MkdirAll(eventDir, 0755)

	dbPath := filepath.Join(eventDir, "events.db")
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		fmt.Printf("Failed to open events db: %v\n", err)
		return
	}
	s.db = db

	s.db.Update(func(tx *bolt.Tx) error {
		tx.CreateBucketIfNotExists([]byte(bucketEvents))
		tx.CreateBucketIfNotExists([]byte(bucketIdxTime))
		tx.CreateBucketIfNotExists([]byte(bucketIdxType))
		tx.CreateBucketIfNotExists([]byte(bucketIdxService))
		return nil
	})

	// Load last seq from DB
	s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketEvents))
		if b != nil {
			c := b.Cursor()
			k, _ := c.Last()
			if k != nil {
				s.eventSeq = binary.BigEndian.Uint64(k)
			}
		}
		return nil
	})

	// Load recent events into ring buffer
	s.LoadRecent()

	// Start retention cleaner
	go s.retentionCleaner()
}

func (s *EventLog) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		s.db.Close()
		s.db = nil
	}
}

func (s *EventLog) LoadRecent() {
	if s.db == nil {
		return
	}
	s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketEvents))
		c := b.Cursor()
		count := 0
		for k, v := c.Last(); k != nil && count < s.maxEvents; k, v = c.Prev() {
			var e Event
			if err := json.Unmarshal(v, &e); err == nil {
				s.events = append([]*Event{&e}, s.events...)
				count++
			}
		}
		return nil
	})
}

func (s *EventLog) Append(e *Event) {
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

	// 2. Persist to BoltDB
	if s.db != nil {
		s.db.Update(func(tx *bolt.Tx) error {
			idBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(idBytes, e.ID)
			
			data, _ := json.Marshal(e)
			tx.Bucket([]byte(bucketEvents)).Put(idBytes, data)

			// Use UTC for indexing to avoid timezone confusion with frontend UTC strings
			tsUTC := e.Timestamp.UTC().Format("2006-01-02T15:04:05.000Z")

			// Index by Time: Time(UTC) | ID
			timeKey := fmt.Sprintf("%s|%020d", tsUTC, e.ID)
			tx.Bucket([]byte(bucketIdxTime)).Put([]byte(timeKey), idBytes)

			// Index by Type: Type | Time(UTC) | ID
			typeKey := fmt.Sprintf("%s|%s|%020d", e.Type, tsUTC, e.ID)
			tx.Bucket([]byte(bucketIdxType)).Put([]byte(typeKey), idBytes)

			// Index by Service: Service | Time(UTC) | ID
			serviceKey := fmt.Sprintf("%s|%s|%020d", e.Service, tsUTC, e.ID)
			tx.Bucket([]byte(bucketIdxService)).Put([]byte(serviceKey), idBytes)

			return nil
		})
	}
}

func (s *EventLog) appendToFile(e *Event) {
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

func (s *EventLog) List() []*Event {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Event, len(s.events))
	copy(result, s.events)
	return result
}

func (s *EventLog) Snapshot() []*Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.events
}

func (s *EventLog) Restore(events []*Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = events
	s.Save()
}

func (s *EventLog) Load() {
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
	var allEvents []*Event
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
		var fileEvents []*Event
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var e Event
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

func (s *EventLog) Save() {
	// No-op for sequential writing as Append handles it per event
}

func (s *EventLog) SetRetentionDaysProvider(fn func() int) {
	s.retentionDaysProvider = fn
}

func (s *EventLog) retentionCleaner() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		if s.db == nil {
			continue
		}
		
		days := 7
		if s.retentionDaysProvider != nil {
			days = s.retentionDaysProvider()
		}
		if days <= 0 {
			continue
		}

		cutoff := time.Now().AddDate(0, 0, -days)
		cutoffKey := cutoff.Format(time.RFC3339Nano)

		s.db.Update(func(tx *bolt.Tx) error {
			bTime := tx.Bucket([]byte(bucketIdxTime))
			bEvents := tx.Bucket([]byte(bucketEvents))

			c := bTime.Cursor()
			for k, v := c.First(); k != nil; k, v = c.Next() {
				if string(k) < cutoffKey {
					bEvents.Delete(v)
					bTime.Delete(k)
				} else {
					break
				}
			}
			return nil
		})
	}
}

func (s *EventLog) Cleanup(days int) {
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

func (s *EventLog) QueryEvents(count, offset int, query, date, startTime, endTime, eventType, service string) ([]*Event, int) {
	if s.db == nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		var matched []*Event
		for _, e := range s.events {
			if eventType != "" && e.Type != eventType {
				continue
			}
			if service != "" && e.Service != service {
				continue
			}
			if query != "" {
				b, _ := json.Marshal(e)
				if !strings.Contains(strings.ToLower(string(b)), strings.ToLower(query)) {
					continue
				}
			}
			matched = append(matched, e)
		}
		
		total := len(matched)
		start := total - offset - count
		end := total - offset
		if start < 0 { start = 0 }
		if end < 0 { end = 0 }
		if start > end { start = end }
		return matched[start:end], total
	}

	var result []*Event
	var total int
	s.db.View(func(tx *bolt.Tx) error {
		var bIndex *bolt.Bucket
		var prefix string

		// Choose index bucket
		if service != "" {
			bIndex = tx.Bucket([]byte(bucketIdxService))
			prefix = service
		} else if eventType != "" {
			bIndex = tx.Bucket([]byte(bucketIdxType))
			prefix = eventType
		} else {
			bIndex = tx.Bucket([]byte(bucketIdxTime))
		}

		bEvents := tx.Bucket([]byte(bucketEvents))
		c := bIndex.Cursor()

		// Optimized Fast-Path: If no conditions are provided, use the master events bucket (sorted by sequential ID)
		// This guarantees we always get the absolute latest events regardless of time format in indices.
		if query == "" && date == "" && startTime == "" && endTime == "" && eventType == "" && service == "" {
			total = bEvents.Stats().KeyN
			c := bEvents.Cursor()
			skipped := 0
			found := 0
			for k, v := c.Last(); k != nil && found < count; k, v = c.Prev() {
				if skipped < offset {
					skipped++
					continue
				}
				var e Event
				if err := json.Unmarshal(v, &e); err == nil {
					result = append(result, &e)
					found++
				}
			}
			return nil
		}

		// Calculate Total for filtered queries
		if startTime != "" || endTime != "" {
			var seekKey string
			if prefix != "" {
				seekKey = prefix + "|" + startTime
			} else {
				seekKey = startTime
			}
			
			for k, _ := c.Seek([]byte(seekKey)); k != nil; k, _ = c.Next() {
				if prefix != "" && !strings.HasPrefix(string(k), prefix) {
					break
				}
				
				var timePart string
				if prefix != "" {
					parts := strings.Split(string(k), "|")
					if len(parts) > 1 {
						timePart = parts[1]
					}
				} else {
					timePart = strings.Split(string(k), "|")[0]
				}

				if endTime != "" && timePart > endTime {
					break
				}
				total++
			}
		} else if date != "" {
			// Count events for a specific day
			var seekKey string
			if prefix != "" {
				seekKey = prefix + "|" + date
			} else {
				seekKey = date
			}
			for k, _ := c.Seek([]byte(seekKey)); k != nil && strings.HasPrefix(string(k), seekKey); k, _ = c.Next() {
				total++
			}
		} else {
			if prefix != "" {
				// Count all keys for this prefix
				for k, _ := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, _ = c.Next() {
					total++
				}
			} else {
				total = bIndex.Stats().KeyN
			}
		}

		// Determine start position for retrieval
		var k, v []byte
		if date != "" || endTime != "" {
			// Seek to the end of the range and go backwards
			var seekKey string
			if endTime != "" {
				if prefix != "" {
					seekKey = prefix + "|" + endTime + "\xff"
				} else {
					seekKey = endTime + "\xff"
				}
			} else {
				if prefix != "" {
					seekKey = prefix + "|" + date + "T23:59:59Z"
				} else {
					seekKey = date + "T23:59:59Z"
				}
			}

			k, v = c.Seek([]byte(seekKey))
			if k == nil {
				if prefix != "" {
					k, v = c.Seek([]byte(prefix + "\xff"))
					if k == nil || !strings.HasPrefix(string(k), prefix) {
						k, v = c.Prev()
					}
				} else {
					k, v = c.Last()
				}
			} else {
				// c.Seek returns >= seekKey. We want <= the end of our range.
				// If the returned key is exactly outside our range, go back one.
				if prefix != "" {
					if !strings.HasPrefix(string(k), prefix) {
						k, v = c.Prev()
					} else {
						parts := strings.Split(string(k), "|")
						if len(parts) > 1 {
							t := parts[1]
							limit := endTime
							if limit == "" { limit = date + "T23:59:59" }
							if t > limit {
								k, v = c.Prev()
							}
						}
					}
				} else {
					t := strings.Split(string(k), "|")[0]
					limit := endTime
					if limit == "" { limit = date + "T23:59:59" }
					if t > limit {
						k, v = c.Prev()
					}
				}
			}
		} else {
			// Get latest
			if prefix != "" {
				k, v = c.Seek([]byte(prefix + "\xff"))
				if k == nil || !strings.HasPrefix(string(k), prefix) {
					k, v = c.Prev()
				}
			} else {
				k, v = c.Last()
			}
		}

		skipped := 0
		found := 0
		for ; k != nil && found < count; k, v = c.Prev() {
			if prefix != "" && !strings.HasPrefix(string(k), prefix) {
				break
			}
			
			var timePart string
			if prefix != "" {
				parts := strings.Split(string(k), "|")
				if len(parts) > 1 {
					timePart = parts[1]
				}
			} else {
				timePart = strings.Split(string(k), "|")[0]
			}

			if date != "" && !strings.HasPrefix(timePart, date) {
				break
			}
			if startTime != "" && timePart < startTime {
				break
			}
			if endTime != "" && timePart > endTime {
				continue
			}

			if skipped < offset {
				skipped++
				continue
			}

			eventData := bEvents.Get(v)
			if eventData == nil {
				continue
			}

			var e Event
			if err := json.Unmarshal(eventData, &e); err == nil {
				if query != "" {
					if !strings.Contains(strings.ToLower(string(eventData)), strings.ToLower(query)) {
						continue
					}
				}
				result = append(result, &e)
				found++
			}
		}
		return nil
	})

	return result, total
}

package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	bucketMetricsMemory = "metrics_memory"
)

type MemoryMetric struct {
	Timestamp int64   `json:"timestamp"`
	Value     uint64  `json:"value"`
}

type MetricsStore struct {
	dbPath   string
	db                    *bolt.DB
	retentionDaysProvider func() int
	storageMode           string // "memory" or "persistent"
}

func NewMetricsStore(dataPath string, retentionDaysProvider func() int) *MetricsStore {
	ms := &MetricsStore{
		dbPath:                filepath.Join(dataPath, "metrics"),
		retentionDaysProvider: retentionDaysProvider,
		storageMode:           "memory", // Default to memory
	}
	return ms
}

func (s *MetricsStore) SetStorageMode(mode string) {
	s.storageMode = mode
	if mode == "persistent" && s.db == nil {
		s.init()
	}
}

func (s *MetricsStore) SetRetentionDaysProvider(fn func() int) {
	s.retentionDaysProvider = fn
}

func (s *MetricsStore) init() {
	if s.dbPath == "" {
		return
	}
	os.MkdirAll(s.dbPath, 0755)

	dbPath := filepath.Join(s.dbPath, "metrics.db")
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		fmt.Printf("Failed to open metrics db: %v\n", err)
		return
	}
	s.db = db

	s.db.Update(func(tx *bolt.Tx) error {
		tx.CreateBucketIfNotExists([]byte(bucketMetricsMemory))
		return nil
	})

	// Start retention cleaner
	go s.retentionCleaner()
}

func (s *MetricsStore) Close() {
	if s.db != nil {
		s.db.Close()
	}
}

func (s *MetricsStore) RecordMemory(usage uint64) {
	if s.storageMode != "persistent" || s.db == nil {
		return
	}

	now := time.Now().UTC()
	metric := MemoryMetric{
		Timestamp: now.UnixMilli(),
		Value:     usage,
	}

	data, _ := json.Marshal(metric)
	key := now.Format(time.RFC3339)

	s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketMetricsMemory))
		return b.Put([]byte(key), data)
	})
}

func (s *MetricsStore) QueryMemory(start, end time.Time) ([]MemoryMetric, error) {
	if s.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}

	var result []MemoryMetric
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketMetricsMemory))
		c := b.Cursor()

		startKey := start.UTC().Format(time.RFC3339)
		endKey := end.UTC().Format(time.RFC3339)

		for k, v := c.Seek([]byte(startKey)); k != nil && string(k) <= endKey; k, v = c.Next() {
			var m MemoryMetric
			if err := json.Unmarshal(v, &m); err == nil {
				result = append(result, m)
			}
		}
		return nil
	})

	return result, err
}

func (s *MetricsStore) retentionCleaner() {
	ticker := time.NewTicker(12 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		s.Cleanup()
	}
}

func (s *MetricsStore) Cleanup() {
	if s.db == nil || s.retentionDaysProvider == nil {
		return
	}

	days := s.retentionDaysProvider()
	if days <= 0 {
		return
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)

	s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketMetricsMemory))
		c := b.Cursor()

		for k, _ := c.First(); k != nil && string(k) < cutoff; k, _ = c.Next() {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

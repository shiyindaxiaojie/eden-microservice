package configcenter

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	bucketConfigs = []byte("configs")
	bucketHistory = []byte("config_history")
	bucketIndex   = []byte("config_index")
	keyRevision   = []byte("revision")
)

const (
	maxWatchTargets = 64
	maxWaiters      = 2048
	maxWaitDuration = 30 * time.Second
)

type waiter struct {
	targets map[Identity]string
	result  chan Change
}

type store struct {
	db *bolt.DB

	waitMu    sync.Mutex
	waiters   map[uint64]*waiter
	waiterSeq uint64
}

func Open(dataDir string) (Service, error) {
	path := filepath.Join(dataDir, "config", "configs.db")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create config data directory: %w", err)
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, fmt.Errorf("open config database: %w", err)
	}
	s := &store{db: db, waiters: make(map[uint64]*waiter)}
	if err := db.Update(func(tx *bolt.Tx) error {
		for _, name := range [][]byte{bucketConfigs, bucketHistory, bucketIndex} {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize config database: %w", err)
	}
	return s, nil
}

func (s *store) Close() error {
	return s.db.Close()
}

func identityKey(identity Identity) []byte {
	return []byte(identity.Namespace + "\x00" + identity.Group + "\x00" + identity.DataID)
}

func historyKey(identity Identity, revision uint64) []byte {
	key := append(identityKey(identity), 0)
	encoded := make([]byte, 8)
	binary.BigEndian.PutUint64(encoded, revision)
	return append(key, encoded...)
}

func nextRevision(tx *bolt.Tx) (uint64, error) {
	bucket := tx.Bucket(bucketIndex)
	revision := uint64(1)
	if raw := bucket.Get(keyRevision); len(raw) == 8 {
		revision = binary.BigEndian.Uint64(raw) + 1
	}
	encoded := make([]byte, 8)
	binary.BigEndian.PutUint64(encoded, revision)
	if err := bucket.Put(keyRevision, encoded); err != nil {
		return 0, err
	}
	return revision, nil
}

func contentMD5(content string) string {
	sum := md5.Sum([]byte(content))
	return hex.EncodeToString(sum[:])
}

func normalizeOperator(operator string) string {
	if value := strings.TrimSpace(operator); value != "" {
		return value
	}
	return "system"
}

func normalizeType(value string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return "text"
}

func normalizeTags(tags []string) []string {
	result := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, tag)
	}
	return result
}

func decodeResource(raw []byte) (*Resource, error) {
	if raw == nil {
		return nil, ErrNotFound
	}
	var resource Resource
	if err := json.Unmarshal(raw, &resource); err != nil {
		return nil, err
	}
	return &resource, nil
}

func (s *store) Get(identity Identity) (*Resource, error) {
	identity, err := NormalizeIdentity(identity)
	if err != nil {
		return nil, err
	}
	var resource *Resource
	err = s.db.View(func(tx *bolt.Tx) error {
		var decodeErr error
		resource, decodeErr = decodeResource(tx.Bucket(bucketConfigs).Get(identityKey(identity)))
		return decodeErr
	})
	if err != nil {
		return nil, err
	}
	return resource, nil
}

func (s *store) Publish(request PublishRequest) (*Resource, error) {
	identity, err := NormalizeIdentity(request.Identity)
	if err != nil {
		return nil, err
	}
	request.Identity = identity
	request.Type = normalizeType(request.Type)
	request.Description = strings.TrimSpace(request.Description)
	request.Tags = normalizeTags(request.Tags)
	request.Operator = normalizeOperator(request.Operator)
	request.ExpectedMD5 = strings.TrimSpace(request.ExpectedMD5)

	var published *Resource
	changed := false
	err = s.db.Update(func(tx *bolt.Tx) error {
		configs := tx.Bucket(bucketConfigs)
		existing, getErr := decodeResource(configs.Get(identityKey(identity)))
		if getErr != nil && !errors.Is(getErr, ErrNotFound) {
			return getErr
		}
		if request.ExpectedMD5 != "" && (existing == nil || existing.MD5 != request.ExpectedMD5) {
			return ErrConflict
		}

		now := time.Now().UTC()
		if existing != nil && existing.Content == request.Content {
			existing.Type = request.Type
			existing.Description = request.Description
			existing.Tags = request.Tags
			existing.UpdatedAt = now
			existing.UpdatedBy = request.Operator
			encoded, marshalErr := json.Marshal(existing)
			if marshalErr != nil {
				return marshalErr
			}
			if putErr := configs.Put(identityKey(identity), encoded); putErr != nil {
				return putErr
			}
			published = existing
			return nil
		}

		if existing != nil {
			history := HistoryEntry{
				Identity:  existing.Identity,
				Content:   existing.Content,
				Type:      existing.Type,
				MD5:       existing.MD5,
				Revision:  existing.Revision,
				Action:    HistoryPublish,
				Operator:  request.Operator,
				Summary:   "发布新版本前的配置",
				CreatedAt: now,
			}
			encoded, marshalErr := json.Marshal(history)
			if marshalErr != nil {
				return marshalErr
			}
			if putErr := tx.Bucket(bucketHistory).Put(historyKey(identity, history.Revision), encoded); putErr != nil {
				return putErr
			}
		}

		revision, revisionErr := nextRevision(tx)
		if revisionErr != nil {
			return revisionErr
		}
		resource := &Resource{
			Identity:    identity,
			Content:     request.Content,
			Type:        request.Type,
			MD5:         contentMD5(request.Content),
			Revision:    revision,
			Description: request.Description,
			Tags:        request.Tags,
			CreatedAt:   now,
			UpdatedAt:   now,
			CreatedBy:   request.Operator,
			UpdatedBy:   request.Operator,
		}
		if existing != nil {
			resource.CreatedAt = existing.CreatedAt
			resource.CreatedBy = existing.CreatedBy
		}
		encoded, marshalErr := json.Marshal(resource)
		if marshalErr != nil {
			return marshalErr
		}
		if putErr := configs.Put(identityKey(identity), encoded); putErr != nil {
			return putErr
		}
		published = resource
		changed = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if changed {
		s.notify(Change{Identity: published.Identity, MD5: published.MD5, Revision: published.Revision})
	}
	return published, nil
}

func (s *store) Delete(identity Identity, operator string) (*HistoryEntry, error) {
	identity, err := NormalizeIdentity(identity)
	if err != nil {
		return nil, err
	}
	operator = normalizeOperator(operator)
	var deleted *HistoryEntry
	err = s.db.Update(func(tx *bolt.Tx) error {
		configs := tx.Bucket(bucketConfigs)
		existing, getErr := decodeResource(configs.Get(identityKey(identity)))
		if getErr != nil {
			return getErr
		}
		revision, revisionErr := nextRevision(tx)
		if revisionErr != nil {
			return revisionErr
		}
		entry := &HistoryEntry{
			Identity:  identity,
			Content:   existing.Content,
			Type:      existing.Type,
			MD5:       existing.MD5,
			Revision:  revision,
			Action:    HistoryDelete,
			Operator:  operator,
			Summary:   "删除配置",
			CreatedAt: time.Now().UTC(),
		}
		encoded, marshalErr := json.Marshal(entry)
		if marshalErr != nil {
			return marshalErr
		}
		if putErr := tx.Bucket(bucketHistory).Put(historyKey(identity, revision), encoded); putErr != nil {
			return putErr
		}
		if deleteErr := configs.Delete(identityKey(identity)); deleteErr != nil {
			return deleteErr
		}
		deleted = entry
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.notify(Change{Identity: identity, Revision: deleted.Revision})
	return deleted, nil
}

func (s *store) History(identity Identity) ([]HistoryEntry, error) {
	identity, err := NormalizeIdentity(identity)
	if err != nil {
		return nil, err
	}
	prefix := append(identityKey(identity), 0)
	entries := make([]HistoryEntry, 0)
	err = s.db.View(func(tx *bolt.Tx) error {
		cursor := tx.Bucket(bucketHistory).Cursor()
		for key, value := cursor.Seek(prefix); key != nil && bytes.HasPrefix(key, prefix); key, value = cursor.Next() {
			var entry HistoryEntry
			if unmarshalErr := json.Unmarshal(value, &entry); unmarshalErr != nil {
				return unmarshalErr
			}
			entries = append(entries, entry)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Revision > entries[j].Revision })
	return entries, nil
}

func (s *store) List(query ListQuery) (ListResult, error) {
	query.Namespace = strings.TrimSpace(query.Namespace)
	query.Group = strings.TrimSpace(query.Group)
	query.Type = strings.TrimSpace(query.Type)
	term := strings.ToLower(strings.TrimSpace(query.Query))
	resources := make([]Resource, 0)
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketConfigs).ForEach(func(_, value []byte) error {
			resource, decodeErr := decodeResource(value)
			if decodeErr != nil {
				return decodeErr
			}
			if query.Namespace != "" && resource.Namespace != query.Namespace {
				return nil
			}
			if query.Group != "" && resource.Group != query.Group {
				return nil
			}
			if query.Type != "" && resource.Type != query.Type {
				return nil
			}
			if term != "" {
				haystack := strings.ToLower(strings.Join(append([]string{resource.DataID, resource.Group, resource.Description}, resource.Tags...), " "))
				if !strings.Contains(haystack, term) {
					return nil
				}
			}
			resources = append(resources, *resource)
			return nil
		})
	})
	if err != nil {
		return ListResult{}, err
	}
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].UpdatedAt.Equal(resources[j].UpdatedAt) {
			return string(identityKey(resources[i].Identity)) < string(identityKey(resources[j].Identity))
		}
		return resources[i].UpdatedAt.After(resources[j].UpdatedAt)
	})
	total := len(resources)
	page := query.Page
	if page <= 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 500 {
		pageSize = 500
	}
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return ListResult{Total: total, Data: resources[start:end]}, nil
}

func (s *store) currentChanges(targets []WatchTarget) ([]Change, error) {
	changes := make([]Change, 0)
	for _, target := range targets {
		identity, err := NormalizeIdentity(target.Identity)
		if err != nil {
			return nil, err
		}
		resource, err := s.Get(identity)
		switch {
		case err == nil && resource.MD5 != target.MD5:
			changes = append(changes, Change{Identity: identity, MD5: resource.MD5, Revision: resource.Revision})
		case errors.Is(err, ErrNotFound) && target.MD5 != "":
			changes = append(changes, Change{Identity: identity})
		case err != nil && !errors.Is(err, ErrNotFound):
			return nil, err
		}
	}
	return changes, nil
}

func (s *store) Wait(targets []WatchTarget, timeout time.Duration) ([]Change, error) {
	if len(targets) == 0 || len(targets) > maxWatchTargets {
		return nil, ErrTooManyTargets
	}
	if timeout <= 0 || timeout > maxWaitDuration {
		timeout = maxWaitDuration
	}
	for index := range targets {
		identity, err := NormalizeIdentity(targets[index].Identity)
		if err != nil {
			return nil, err
		}
		targets[index].Identity = identity
	}
	if changes, err := s.currentChanges(targets); err != nil || len(changes) > 0 {
		return changes, err
	}

	w := &waiter{targets: make(map[Identity]string, len(targets)), result: make(chan Change, 1)}
	for _, target := range targets {
		w.targets[target.Identity] = target.MD5
	}
	s.waitMu.Lock()
	if len(s.waiters) >= maxWaiters {
		s.waitMu.Unlock()
		return nil, ErrTooManyWaiters
	}
	s.waiterSeq++
	id := s.waiterSeq
	s.waiters[id] = w
	s.waitMu.Unlock()
	defer func() {
		s.waitMu.Lock()
		delete(s.waiters, id)
		s.waitMu.Unlock()
	}()

	if changes, err := s.currentChanges(targets); err != nil || len(changes) > 0 {
		return changes, err
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case change := <-w.result:
		return []Change{change}, nil
	case <-timer.C:
		return []Change{}, nil
	}
}

func (s *store) notify(change Change) {
	s.waitMu.Lock()
	defer s.waitMu.Unlock()
	for _, w := range s.waiters {
		md5Value, ok := w.targets[change.Identity]
		if !ok || md5Value == change.MD5 {
			continue
		}
		select {
		case w.result <- change:
		default:
		}
	}
}

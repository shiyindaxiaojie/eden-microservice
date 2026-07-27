package gateway

import (
	"bytes"
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

var (
	bucketRoutes  = []byte("routes")
	bucketHistory = []byte("route_history")
	bucketIndex   = []byte("route_index")
	keyRevision   = []byte("revision")
)

type store struct {
	db *bolt.DB

	subMu     sync.Mutex
	subs      map[uint64]func()
	nextSubID uint64
}

// Open creates or opens the local gateway route store.
func Open(dataDir string) (Service, error) {
	path := filepath.Join(dataDir, "gateway", "routes.db")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create gateway data directory: %w", err)
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, fmt.Errorf("open gateway database: %w", err)
	}
	s := &store{db: db, subs: make(map[uint64]func())}
	if err := db.Update(func(tx *bolt.Tx) error {
		for _, name := range [][]byte{bucketRoutes, bucketHistory, bucketIndex} {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize gateway database: %w", err)
	}
	return s, nil
}

func (s *store) Close() error {
	return s.db.Close()
}

func routeKey(identity Identity) []byte {
	return []byte(identity.Namespace + "\x00" + identity.ID)
}

func historyKey(identity Identity, revision uint64) []byte {
	prefix := routeKey(identity)
	key := make([]byte, len(prefix)+1+8)
	copy(key, prefix)
	encoded := key[len(prefix)+1:]
	binary.BigEndian.PutUint64(encoded, revision)
	return key
}

func historyPrefix(identity Identity) []byte {
	prefix := routeKey(identity)
	return append(prefix, 0)
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

func decodeRoute(raw []byte) (*Route, error) {
	if raw == nil {
		return nil, ErrNotFound
	}
	var route Route
	if err := json.Unmarshal(raw, &route); err != nil {
		return nil, fmt.Errorf("decode gateway route: %w", err)
	}
	return &route, nil
}

func decodeHistory(raw []byte) (*HistoryEntry, error) {
	var entry HistoryEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nil, fmt.Errorf("decode gateway route history: %w", err)
	}
	return &entry, nil
}

func putRoute(bucket *bolt.Bucket, route *Route) error {
	encoded, err := json.Marshal(route)
	if err != nil {
		return err
	}
	return bucket.Put(routeKey(route.Identity), encoded)
}

func putHistory(tx *bolt.Tx, entry *HistoryEntry) error {
	encoded, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return tx.Bucket(bucketHistory).Put(historyKey(entry.Identity, entry.Revision), encoded)
}

func (s *store) Get(identity Identity) (*Route, error) {
	identity, err := NormalizeIdentity(identity)
	if err != nil {
		return nil, err
	}
	var route *Route
	err = s.db.View(func(tx *bolt.Tx) error {
		var decodeErr error
		route, decodeErr = decodeRoute(tx.Bucket(bucketRoutes).Get(routeKey(identity)))
		return decodeErr
	})
	if err != nil {
		return nil, err
	}
	return route, nil
}

func (s *store) List(query ListQuery) (ListResult, error) {
	query.Namespace = strings.TrimSpace(query.Namespace)
	term := strings.ToLower(strings.TrimSpace(query.Query))
	routes := make([]Route, 0)
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketRoutes).ForEach(func(_, value []byte) error {
			route, decodeErr := decodeRoute(value)
			if decodeErr != nil {
				return decodeErr
			}
			if query.Namespace != "" && route.Namespace != query.Namespace {
				return nil
			}
			if query.Enabled != nil && route.Enabled != *query.Enabled {
				return nil
			}
			if term != "" && !routeMatchesQuery(*route, term) {
				return nil
			}
			routes = append(routes, *route)
			return nil
		})
	})
	if err != nil {
		return ListResult{}, err
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Priority != routes[j].Priority {
			return routes[i].Priority < routes[j].Priority
		}
		leftPath := matchPathLength(routes[i].Match)
		rightPath := matchPathLength(routes[j].Match)
		if leftPath != rightPath {
			return leftPath > rightPath
		}
		if !routes[i].CreatedAt.Equal(routes[j].CreatedAt) {
			return routes[i].CreatedAt.Before(routes[j].CreatedAt)
		}
		if routes[i].Namespace != routes[j].Namespace {
			return routes[i].Namespace < routes[j].Namespace
		}
		return routes[i].ID < routes[j].ID
	})
	total := len(routes)
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 500 {
		pageSize = 500
	}
	start := total
	if page <= (total/pageSize)+1 {
		start = (page - 1) * pageSize
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return ListResult{Total: total, Data: routes[start:end]}, nil
}

func routeMatchesQuery(route Route, term string) bool {
	values := []string{route.ID, route.Name, route.Match.Path, route.Match.PathPrefix, route.Traffic.Mode.String()}
	for _, target := range route.Targets {
		values = append(values, target.ID, target.Name)
		if target.Service != nil {
			values = append(values, target.Service.ServiceName, target.Service.Group)
		}
		if target.Static != nil {
			for _, endpoint := range target.Static.Endpoints {
				values = append(values, endpoint.URL)
			}
		}
	}
	return strings.Contains(strings.ToLower(strings.Join(values, " ")), term)
}

func (m TrafficMode) String() string {
	return string(m)
}

func matchPathLength(match RouteMatch) int {
	if match.Path != "" {
		return len(match.Path)
	}
	return len(match.PathPrefix)
}

func (s *store) Create(request CreateRequest) (*Route, error) {
	route, err := NormalizeRoute(request.Route)
	if err != nil {
		return nil, err
	}
	operator := normalizeOperator(request.Operator)
	var created *Route
	err = s.db.Update(func(tx *bolt.Tx) error {
		routes := tx.Bucket(bucketRoutes)
		if existing, getErr := decodeRoute(routes.Get(routeKey(route.Identity))); getErr == nil && existing != nil {
			return ErrAlreadyExists
		} else if getErr != nil && getErr != ErrNotFound {
			return getErr
		}
		revision, revisionErr := nextRevision(tx)
		if revisionErr != nil {
			return revisionErr
		}
		now := time.Now().UTC()
		route.Revision = revision
		route.CreatedAt = now
		route.UpdatedAt = now
		route.CreatedBy = operator
		route.UpdatedBy = operator
		if err := putRoute(routes, &route); err != nil {
			return err
		}
		entry := &HistoryEntry{Identity: route.Identity, Route: cloneRoute(&route), Revision: revision, Action: HistoryCreate, Operator: operator, Summary: "创建路由", CreatedAt: now}
		if err := putHistory(tx, entry); err != nil {
			return err
		}
		created = cloneRoute(&route)
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.notify()
	return created, nil
}

func (s *store) Update(request UpdateRequest) (*Route, error) {
	route, err := NormalizeRoute(request.Route)
	if err != nil {
		return nil, err
	}
	operator := normalizeOperator(request.Operator)
	var updated *Route
	err = s.db.Update(func(tx *bolt.Tx) error {
		routes := tx.Bucket(bucketRoutes)
		existing, getErr := decodeRoute(routes.Get(routeKey(route.Identity)))
		if getErr != nil {
			return getErr
		}
		if request.ExpectedRevision == 0 || request.ExpectedRevision != existing.Revision {
			return ErrConflict
		}
		revision, revisionErr := nextRevision(tx)
		if revisionErr != nil {
			return revisionErr
		}
		route.Revision = revision
		route.CreatedAt = existing.CreatedAt
		route.CreatedBy = existing.CreatedBy
		route.UpdatedAt = time.Now().UTC()
		route.UpdatedBy = operator
		if err := putRoute(routes, &route); err != nil {
			return err
		}
		entry := &HistoryEntry{Identity: route.Identity, Route: cloneRoute(&route), Revision: revision, Action: HistoryUpdate, Operator: operator, Summary: "更新路由", CreatedAt: route.UpdatedAt}
		if err := putHistory(tx, entry); err != nil {
			return err
		}
		updated = cloneRoute(&route)
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.notify()
	return updated, nil
}

func (s *store) Delete(identity Identity, expectedRevision uint64, operator string) (*HistoryEntry, error) {
	identity, err := NormalizeIdentity(identity)
	if err != nil {
		return nil, err
	}
	operator = normalizeOperator(operator)
	var deleted *HistoryEntry
	err = s.db.Update(func(tx *bolt.Tx) error {
		routes := tx.Bucket(bucketRoutes)
		existing, getErr := decodeRoute(routes.Get(routeKey(identity)))
		if getErr != nil {
			return getErr
		}
		if expectedRevision == 0 || expectedRevision != existing.Revision {
			return ErrConflict
		}
		revision, revisionErr := nextRevision(tx)
		if revisionErr != nil {
			return revisionErr
		}
		now := time.Now().UTC()
		entry := &HistoryEntry{Identity: identity, Route: cloneRoute(existing), Revision: revision, Action: HistoryDelete, Operator: operator, Summary: "删除路由", CreatedAt: now}
		if err := putHistory(tx, entry); err != nil {
			return err
		}
		if err := routes.Delete(routeKey(identity)); err != nil {
			return err
		}
		deleted = entry
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.notify()
	return deleted, nil
}

func (s *store) SetEnabled(identity Identity, enabled bool, expectedRevision uint64, operator string) (*Route, error) {
	identity, err := NormalizeIdentity(identity)
	if err != nil {
		return nil, err
	}
	operator = normalizeOperator(operator)
	var updated *Route
	changed := false
	err = s.db.Update(func(tx *bolt.Tx) error {
		routes := tx.Bucket(bucketRoutes)
		existing, getErr := decodeRoute(routes.Get(routeKey(identity)))
		if getErr != nil {
			return getErr
		}
		if expectedRevision == 0 || expectedRevision != existing.Revision {
			return ErrConflict
		}
		if existing.Enabled == enabled {
			updated = cloneRoute(existing)
			return nil
		}
		revision, revisionErr := nextRevision(tx)
		if revisionErr != nil {
			return revisionErr
		}
		now := time.Now().UTC()
		existing.Enabled = enabled
		existing.Revision = revision
		existing.UpdatedAt = now
		existing.UpdatedBy = operator
		if err := putRoute(routes, existing); err != nil {
			return err
		}
		action := HistoryDisable
		summary := "停用路由"
		if enabled {
			action = HistoryEnable
			summary = "启用路由"
		}
		entry := &HistoryEntry{Identity: identity, Route: cloneRoute(existing), Revision: revision, Action: action, Operator: operator, Summary: summary, CreatedAt: now}
		if err := putHistory(tx, entry); err != nil {
			return err
		}
		updated = cloneRoute(existing)
		changed = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if changed {
		s.notify()
	}
	return updated, nil
}

func (s *store) History(identity Identity) ([]HistoryEntry, error) {
	identity, err := NormalizeIdentity(identity)
	if err != nil {
		return nil, err
	}
	prefix := historyPrefix(identity)
	entries := make([]HistoryEntry, 0)
	err = s.db.View(func(tx *bolt.Tx) error {
		cursor := tx.Bucket(bucketHistory).Cursor()
		for key, value := cursor.Seek(prefix); key != nil && bytes.HasPrefix(key, prefix); key, value = cursor.Next() {
			entry, decodeErr := decodeHistory(value)
			if decodeErr != nil {
				return decodeErr
			}
			entries = append(entries, *entry)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Revision > entries[j].Revision })
	return entries, nil
}

func (s *store) Subscribe(callback func()) func() {
	if callback == nil {
		return func() {}
	}
	s.subMu.Lock()
	s.nextSubID++
	id := s.nextSubID
	s.subs[id] = callback
	s.subMu.Unlock()
	return func() {
		s.subMu.Lock()
		delete(s.subs, id)
		s.subMu.Unlock()
	}
}

func (s *store) notify() {
	s.subMu.Lock()
	callbacks := make([]func(), 0, len(s.subs))
	for _, callback := range s.subs {
		callbacks = append(callbacks, callback)
	}
	s.subMu.Unlock()
	for _, callback := range callbacks {
		callback()
	}
}

func cloneRoute(route *Route) *Route {
	if route == nil {
		return nil
	}
	encoded, err := json.Marshal(route)
	if err != nil {
		return nil
	}
	var cloned Route
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		return nil
	}
	return &cloned
}

func normalizeOperator(operator string) string {
	if operator = strings.TrimSpace(operator); operator != "" {
		return operator
	}
	return "system"
}

package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
)

// AuthStore handles users and API keys.
type AuthStore struct {
	mu       sync.RWMutex
	apiKeys  map[string]*model.APIKey
	users    map[string]*model.User
	dataPath string
}

func NewAuthStore(dataPath string) *AuthStore {
	s := &AuthStore{
		apiKeys:  make(map[string]*model.APIKey),
		users:    make(map[string]*model.User),
		dataPath: dataPath,
	}
	s.Load()
	return s
}

// API Key Management
func (s *AuthStore) AddAPIKey(key *model.APIKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.apiKeys[key.Key] = key
}

func (s *AuthStore) DeleteAPIKey(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.apiKeys, key)
}

func (s *AuthStore) ListAPIKeys() []*model.APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now().Unix()
	keys := make([]*model.APIKey, 0, len(s.apiKeys))
	for _, k := range s.apiKeys {
		cp := *k
		if cp.ExpiresAt > 0 && now > cp.ExpiresAt {
			cp.Status = "expired"
		} else {
			cp.Status = "active"
		}
		keys = append(keys, &cp)
	}
	return keys
}

func (s *AuthStore) GetAPIKey(key string) (*model.APIKey, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k, ok := s.apiKeys[key]
	return k, ok
}

// User Management
func (s *AuthStore) AddUser(u *model.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[u.Username] = u
}

func (s *AuthStore) DeleteUser(username string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.users, username)
}

func (s *AuthStore) ListUsers() []*model.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	users := make([]*model.User, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, u)
	}
	return users
}

func (s *AuthStore) GetUser(username string) (*model.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[username]
	return u, ok
}

func (s *AuthStore) GetAllUsers() map[string]*model.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copy := make(map[string]*model.User, len(s.users))
	for k, v := range s.users {
		copy[k] = v
	}
	return copy
}

func (s *AuthStore) GetAllAPIKeys() map[string]*model.APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copy := make(map[string]*model.APIKey, len(s.apiKeys))
	for k, v := range s.apiKeys {
		copy[k] = v
	}
	return copy
}

func (s *AuthStore) Restore(users map[string]*model.User, apiKeys map[string]*model.APIKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users = users
	s.apiKeys = apiKeys
	s.saveLocked()
}

func (s *AuthStore) Load() {
	if s.dataPath == "" {
		return
	}
	file := filepath.Join(s.dataPath, "users.json")
	data, err := os.ReadFile(file)
	if err == nil {
		var meta struct {
			Users map[string]*model.User `json:"users"`
		}
		if err := json.Unmarshal(data, &meta); err == nil && meta.Users != nil {
			s.users = meta.Users
		}
	}
}

func (s *AuthStore) Save() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.saveLocked()
}

// saveLocked persists user data to disk. Caller must hold s.mu.
func (s *AuthStore) saveLocked() {
	if s.dataPath == "" {
		return
	}
	os.MkdirAll(s.dataPath, 0755)
	file := filepath.Join(s.dataPath, "users.json")
	// Copy users map while we already hold the lock
	usersCopy := make(map[string]*model.User, len(s.users))
	for k, v := range s.users {
		usersCopy[k] = v
	}
	meta := struct {
		Users map[string]*model.User `json:"users"`
	}{
		Users: usersCopy,
	}
	data, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(file, data, 0644)
}

func (s *AuthStore) SeedBuiltInUsers(builtin []model.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for _, u := range builtin {
		u := u
		u.IsBuiltIn = true
		if existing, ok := s.users[u.Username]; !ok {
			s.users[u.Username] = &u
			changed = true
		} else {
			if existing.Role != u.Role {
				existing.Role = u.Role
				existing.IsBuiltIn = true
				changed = true
			}
		}
	}
	if changed {
		s.saveLocked()
	}
}

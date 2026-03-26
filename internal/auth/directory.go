package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Directory stores users and API keys.
type Directory struct {
	mu       sync.RWMutex
	apiKeys  map[string]*APIKey
	users    map[string]*User
	dataPath string
}

func NewDirectory(dataPath string) *Directory {
	s := &Directory{
		apiKeys:  make(map[string]*APIKey),
		users:    make(map[string]*User),
		dataPath: dataPath,
	}
	s.Load()
	return s
}

// API Key Management
func (s *Directory) AddAPIKey(key *APIKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.apiKeys[key.Key] = key
}

func (s *Directory) DeleteAPIKey(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.apiKeys, key)
}

func (s *Directory) ListAPIKeys() []*APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now().Unix()
	keys := make([]*APIKey, 0, len(s.apiKeys))
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

func (s *Directory) GetAPIKey(key string) (*APIKey, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k, ok := s.apiKeys[key]
	return k, ok
}

// User Management
func (s *Directory) AddUser(u *User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[u.Username] = u
}

func (s *Directory) DeleteUser(username string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.users, username)
}

func (s *Directory) ListUsers() []*User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	users := make([]*User, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, u)
	}
	return users
}

func (s *Directory) GetUser(username string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[username]
	return u, ok
}

func (s *Directory) GetAllUsers() map[string]*User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copy := make(map[string]*User, len(s.users))
	for k, v := range s.users {
		copy[k] = v
	}
	return copy
}

func (s *Directory) GetAllAPIKeys() map[string]*APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copy := make(map[string]*APIKey, len(s.apiKeys))
	for k, v := range s.apiKeys {
		copy[k] = v
	}
	return copy
}

func (s *Directory) Restore(users map[string]*User, apiKeys map[string]*APIKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users = users
	s.apiKeys = apiKeys
	s.saveLocked()
}

func (s *Directory) Load() {
	if s.dataPath == "" {
		return
	}
	file := filepath.Join(s.dataPath, "users.json")
	data, err := os.ReadFile(file)
	if err == nil {
		var meta struct {
			Users map[string]*User `json:"users"`
		}
		if err := json.Unmarshal(data, &meta); err == nil && meta.Users != nil {
			s.users = meta.Users
		}
	}
}

func (s *Directory) Save() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.saveLocked()
}

// saveLocked persists user data to disk. Caller must hold s.mu.
func (s *Directory) saveLocked() {
	if s.dataPath == "" {
		return
	}
	os.MkdirAll(s.dataPath, 0755)
	file := filepath.Join(s.dataPath, "users.json")
	// Copy users map while we already hold the lock
	usersCopy := make(map[string]*User, len(s.users))
	for k, v := range s.users {
		usersCopy[k] = v
	}
	meta := struct {
		Users map[string]*User `json:"users"`
	}{
		Users: usersCopy,
	}
	data, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(file, data, 0644)
}

func (s *Directory) SeedBuiltInUsers(builtin []User) {
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

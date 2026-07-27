package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// NamespaceRegistry manages namespace CRUD and persistence.
type NamespaceRegistry struct {
	mu         sync.RWMutex
	namespaces map[string]*Namespace
	dataPath   string
}

// NewNamespaceRegistry creates a new NamespaceRegistry with persistence.
func NewNamespaceRegistry(dataPath string) *NamespaceRegistry {
	s := &NamespaceRegistry{
		namespaces: make(map[string]*Namespace),
		dataPath:   dataPath,
	}
	s.load()
	// Ensure default namespace always exists
	if _, ok := s.namespaces[DefaultNamespace]; !ok {
		s.namespaces[DefaultNamespace] = &Namespace{
			Name:        DefaultNamespace,
			Description: "Default namespace",
			CreatedAt:   time.Now().Format(time.RFC3339),
		}
		s.saveNoLock()
	}
	return s
}

// List returns all namespaces.
func (s *NamespaceRegistry) List() []*Namespace {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Namespace, 0, len(s.namespaces))
	for _, ns := range s.namespaces {
		cp := *ns
		result = append(result, &cp)
	}
	return result
}

// Get returns a namespace by name.
func (s *NamespaceRegistry) Get(name string) (*Namespace, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ns, ok := s.namespaces[name]
	if !ok {
		return nil, false
	}
	cp := *ns
	return &cp, true
}

// Create adds a new namespace.
func (s *NamespaceRegistry) Create(ns *Namespace) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.namespaces[ns.Name]; exists {
		return false
	}
	ns.CreatedAt = time.Now().Format(time.RFC3339)
	s.namespaces[ns.Name] = ns
	s.saveNoLock()
	return true
}

// Update modifies an existing namespace.
func (s *NamespaceRegistry) Update(ns *Namespace) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.namespaces[ns.Name]
	if !ok {
		return false
	}
	existing.Description = ns.Description
	existing.UpdatedAt = time.Now().Format(time.RFC3339)
	s.saveNoLock()
	return true
}

// Delete removes a namespace. The default namespace cannot be deleted.
func (s *NamespaceRegistry) Delete(name string) bool {
	if name == DefaultNamespace {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.namespaces[name]; !ok {
		return false
	}
	delete(s.namespaces, name)
	s.saveNoLock()
	return true
}

// Exists checks if a namespace exists.
func (s *NamespaceRegistry) Exists(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if name == "" {
		return true // empty maps to default
	}
	_, ok := s.namespaces[name]
	return ok
}

// Restore replaces namespaces from snapshot data.
func (s *NamespaceRegistry) Restore(namespaces []*Namespace) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.namespaces = make(map[string]*Namespace, len(namespaces)+1)
	for _, ns := range namespaces {
		cp := *ns
		s.namespaces[cp.Name] = &cp
	}
	if _, ok := s.namespaces[DefaultNamespace]; !ok {
		s.namespaces[DefaultNamespace] = &Namespace{
			Name:        DefaultNamespace,
			Description: "Default namespace",
			CreatedAt:   time.Now().Format(time.RFC3339),
		}
	}
	s.saveNoLock()
}

func (s *NamespaceRegistry) load() {
	if s.dataPath == "" {
		return
	}
	file := filepath.Join(s.dataPath, "namespace.json")
	data, err := os.ReadFile(file)
	if err != nil {
		return
	}
	var namespaces []*Namespace
	if err := json.Unmarshal(data, &namespaces); err == nil {
		for _, ns := range namespaces {
			s.namespaces[ns.Name] = ns
		}
	}
}

func (s *NamespaceRegistry) saveNoLock() {
	if s.dataPath == "" {
		return
	}
	os.MkdirAll(s.dataPath, 0755)
	file := filepath.Join(s.dataPath, "namespace.json")
	list := make([]*Namespace, 0, len(s.namespaces))
	for _, ns := range s.namespaces {
		cp := *ns
		list = append(list, &cp)
	}
	data, _ := json.MarshalIndent(list, "", "  ")
	_ = os.WriteFile(file, data, 0644)
}

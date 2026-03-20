package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
)

// TopologyStore persists namespace-scoped service dependency reports.
type TopologyStore struct {
	mu       sync.RWMutex
	reports  map[string]map[string]*model.TopologyReport
	dataPath string
}

func NewTopologyStore(dataPath string) *TopologyStore {
	s := &TopologyStore{
		reports:  make(map[string]map[string]*model.TopologyReport),
		dataPath: dataPath,
	}
	s.load()
	return s
}

func normalizeTopologyNS(namespace string) string {
	if namespace == "" {
		return model.DefaultNamespace
	}
	return namespace
}

func uniqueSortedServices(services []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(services))
	for _, service := range services {
		if service == "" || seen[service] {
			continue
		}
		seen[service] = true
		result = append(result, service)
	}
	sort.Strings(result)
	return result
}

// Report stores the latest provider set for a consumer service.
// Returns true if the stored topology changed.
func (s *TopologyStore) Report(namespace, consumerService string, providers []string, checksum string) bool {
	if consumerService == "" {
		return false
	}

	ns := normalizeTopologyNS(namespace)
	providers = uniqueSortedServices(providers)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.reports[ns] == nil {
		s.reports[ns] = make(map[string]*model.TopologyReport)
	}

	if len(providers) == 0 {
		if _, ok := s.reports[ns][consumerService]; ok {
			delete(s.reports[ns], consumerService)
			if len(s.reports[ns]) == 0 {
				delete(s.reports, ns)
			}
			s.saveNoLock()
			return true
		}
		return false
	}

	if existing, ok := s.reports[ns][consumerService]; ok {
		if existing.Checksum == checksum && equalStrings(existing.Providers, providers) {
			return false
		}
	}

	s.reports[ns][consumerService] = &model.TopologyReport{
		ConsumerService: consumerService,
		Providers:       providers,
		Checksum:        checksum,
		UpdatedAt:       time.Now().Format(time.RFC3339),
	}
	s.saveNoLock()
	return true
}

// Reports returns a copy of the namespace reports keyed by consumer service.
func (s *TopologyStore) Reports(namespace string) map[string]*model.TopologyReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ns := normalizeTopologyNS(namespace)
	result := make(map[string]*model.TopologyReport, len(s.reports[ns]))
	for service, report := range s.reports[ns] {
		cp := *report
		cp.Providers = append([]string(nil), report.Providers...)
		result[service] = &cp
	}
	return result
}

// Snapshot returns a deep copy of all namespace reports.
func (s *TopologyStore) Snapshot() map[string]map[string]*model.TopologyReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]map[string]*model.TopologyReport, len(s.reports))
	for ns, services := range s.reports {
		result[ns] = make(map[string]*model.TopologyReport, len(services))
		for service, report := range services {
			cp := *report
			cp.Providers = append([]string(nil), report.Providers...)
			result[ns][service] = &cp
		}
	}
	return result
}

// Restore replaces topology reports from snapshot data.
func (s *TopologyStore) Restore(reports map[string]map[string]*model.TopologyReport) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.reports = make(map[string]map[string]*model.TopologyReport, len(reports))
	for ns, services := range reports {
		s.reports[ns] = make(map[string]*model.TopologyReport, len(services))
		for service, report := range services {
			cp := *report
			cp.Providers = append([]string(nil), report.Providers...)
			s.reports[ns][service] = &cp
		}
	}
	s.saveNoLock()
}

func (s *TopologyStore) load() {
	if s.dataPath == "" {
		return
	}
	file := filepath.Join(s.dataPath, "topology.json")
	data, err := os.ReadFile(file)
	if err != nil {
		return
	}
	var reports map[string]map[string]*model.TopologyReport
	if err := json.Unmarshal(data, &reports); err == nil {
		s.reports = reports
	}
}

func (s *TopologyStore) saveNoLock() {
	if s.dataPath == "" {
		return
	}
	_ = os.MkdirAll(s.dataPath, 0755)
	file := filepath.Join(s.dataPath, "topology.json")
	data, _ := json.MarshalIndent(s.reports, "", "  ")
	_ = os.WriteFile(file, data, 0644)
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

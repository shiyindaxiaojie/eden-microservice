package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// TopologyIndex persists namespace-scoped service dependency reports.
type TopologyIndex struct {
	mu       sync.RWMutex
	reports  map[string]map[string]*TopologyReport
	dataPath string
}

func NewTopologyIndex(dataPath string) *TopologyIndex {
	s := &TopologyIndex{
		reports:  make(map[string]map[string]*TopologyReport),
		dataPath: dataPath,
	}
	s.load()
	return s
}

func normalizeTopologyNS(namespace string) string {
	if namespace == "" {
		return DefaultNamespace
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
func (s *TopologyIndex) Report(namespace, consumerService string, providers []string, checksum string) bool {
	if consumerService == "" {
		return false
	}

	ns := normalizeTopologyNS(namespace)
	providers = uniqueSortedServices(providers)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.reports[ns] == nil {
		s.reports[ns] = make(map[string]*TopologyReport)
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

	s.reports[ns][consumerService] = &TopologyReport{
		ConsumerService: consumerService,
		Providers:       providers,
		Checksum:        checksum,
		UpdatedAt:       time.Now().Format(time.RFC3339),
	}
	s.saveNoLock()
	return true
}

// Reports returns a copy of the namespace reports keyed by consumer service.
func (s *TopologyIndex) Reports(namespace string) map[string]*TopologyReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ns := normalizeTopologyNS(namespace)
	result := make(map[string]*TopologyReport, len(s.reports[ns]))
	for service, report := range s.reports[ns] {
		cp := *report
		cp.Providers = append([]string(nil), report.Providers...)
		result[service] = &cp
	}
	return result
}

// Prune removes stale topology reports that point to non-existent services.
// It keeps only active consumer/provider relationships for the given namespace.
func (s *TopologyIndex) Prune(namespace string, activeServices map[string]bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns := normalizeTopologyNS(namespace)
	reports, ok := s.reports[ns]
	if !ok {
		return false
	}

	changed := false
	for consumer, report := range reports {
		if !activeServices[consumer] {
			delete(reports, consumer)
			changed = true
			continue
		}

		filteredProviders := make([]string, 0, len(report.Providers))
		for _, provider := range report.Providers {
			if provider == "" || provider == consumer || !activeServices[provider] {
				continue
			}
			filteredProviders = append(filteredProviders, provider)
		}
		filteredProviders = uniqueSortedServices(filteredProviders)

		if len(filteredProviders) == 0 {
			delete(reports, consumer)
			changed = true
			continue
		}

		if !equalStrings(report.Providers, filteredProviders) {
			cp := *report
			cp.Providers = filteredProviders
			cp.UpdatedAt = time.Now().Format(time.RFC3339)
			reports[consumer] = &cp
			changed = true
		}
	}

	if len(reports) == 0 {
		delete(s.reports, ns)
		changed = true
	}

	if changed {
		s.saveNoLock()
	}

	return changed
}

// Snapshot returns a deep copy of all namespace reports.
func (s *TopologyIndex) Snapshot() map[string]map[string]*TopologyReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]map[string]*TopologyReport, len(s.reports))
	for ns, services := range s.reports {
		result[ns] = make(map[string]*TopologyReport, len(services))
		for service, report := range services {
			cp := *report
			cp.Providers = append([]string(nil), report.Providers...)
			result[ns][service] = &cp
		}
	}
	return result
}

// Restore replaces topology reports from snapshot data.
func (s *TopologyIndex) Restore(reports map[string]map[string]*TopologyReport) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.reports = make(map[string]map[string]*TopologyReport, len(reports))
	for ns, services := range reports {
		s.reports[ns] = make(map[string]*TopologyReport, len(services))
		for service, report := range services {
			cp := *report
			cp.Providers = append([]string(nil), report.Providers...)
			s.reports[ns][service] = &cp
		}
	}
	s.saveNoLock()
}

func (s *TopologyIndex) load() {
	if s.dataPath == "" {
		return
	}
	file := filepath.Join(s.dataPath, "topology.json")
	data, err := os.ReadFile(file)
	if err != nil {
		return
	}
	var reports map[string]map[string]*TopologyReport
	if err := json.Unmarshal(data, &reports); err == nil {
		s.reports = reports
	}
}

func (s *TopologyIndex) saveNoLock() {
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

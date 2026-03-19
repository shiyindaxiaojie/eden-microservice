package eden

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/shiyindaxiaojie/eden-go-logger"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/registry"
)

// LocalCache manages a file-backed cache of service discovery results.
type LocalCache struct {
	mu        sync.RWMutex
	dir       string
	cacheFile string
	services  map[string][]*registry.ServiceInstance
}

// NewLocalCache creates a new local cache for the SDK.
func NewLocalCache(cacheDir string) *LocalCache {
	if cacheDir == "" {
		return nil
	}
	
	err := os.MkdirAll(cacheDir, 0755)
	if err != nil {
		logger.Warn("[Registry SDK] Failed to create cache directory %s: %v", cacheDir, err)
	}

	c := &LocalCache{
		dir:       cacheDir,
		cacheFile: filepath.Join(cacheDir, "services.json"),
		services:  make(map[string][]*registry.ServiceInstance),
	}
	c.Load()
	return c
}

// Load loads cached service instances from disk.
func (c *LocalCache) Load() {
	if c == nil || c.cacheFile == "" {
		return
	}
	
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(c.cacheFile)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("[Registry SDK] Failed to read cache file: %v", err)
		}
		return
	}

	var services map[string][]*registry.ServiceInstance
	if err := json.Unmarshal(data, &services); err != nil {
		logger.Warn("[Registry SDK] Failed to parse cache file: %v", err)
		return
	}

	c.services = services
	logger.Info("[Registry SDK] Loaded local cache with %d services from %s", len(services), c.cacheFile)
}

// Save persists the current cache state to disk.
func (c *LocalCache) Save() {
	if c == nil || c.cacheFile == "" {
		return
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := json.MarshalIndent(c.services, "", "  ")
	if err != nil {
		logger.Warn("[Registry SDK] Failed to serialize cache: %v", err)
		return
	}

	err = os.WriteFile(c.cacheFile, data, 0644)
	if err != nil {
		logger.Warn("[Registry SDK] Failed to write cache file: %v", err)
	}
}

// Get retrieves cached instances for a service.
func (c *LocalCache) Get(serviceName string) ([]*registry.ServiceInstance, bool) {
	if c == nil {
		return nil, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	instances, ok := c.services[serviceName]
	if !ok {
		return nil, false
	}

	// Make a copy to prevent external mutation
	result := make([]*registry.ServiceInstance, 0, len(instances))
	for _, inst := range instances {
		cp := *inst
		result = append(result, &cp)
	}
	return result, true
}

// Update caches new instances for a service and saves to disk.
func (c *LocalCache) Update(serviceName string, instances []*registry.ServiceInstance) {
	if c == nil {
		return
	}

	c.mu.Lock()
	// Need to copy instances
	copied := make([]*registry.ServiceInstance, 0, len(instances))
	for _, inst := range instances {
		cp := *inst
		copied = append(copied, &cp)
	}
	c.services[serviceName] = copied
	c.mu.Unlock()

	// Asynchronously save to avoid blocking the caller
	go c.Save()
}

// UpdateAll caches instances for multiple services and saves to disk.
func (c *LocalCache) UpdateAll(services map[string][]*registry.ServiceInstance) {
	if c == nil {
		return
	}

	c.mu.Lock()
	changed := false
	for svcName, instances := range services {
		copied := make([]*registry.ServiceInstance, 0, len(instances))
		for _, inst := range instances {
			cp := *inst
			copied = append(copied, &cp)
		}
		c.services[svcName] = copied
		changed = true
	}
	c.mu.Unlock()

	if changed {
		go c.Save()
	}
}

// StartBackgroundSyncer periodically triggers a full sync of discovered services (optional utility).
func (c *LocalCache) StartBackgroundSyncer(interval time.Duration, syncFn func() map[string][]*registry.ServiceInstance, stopCh <-chan struct{}) {
	if c == nil {
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				services := syncFn()
				if len(services) > 0 {
					c.UpdateAll(services)
				}
			}
		}
	}()
}

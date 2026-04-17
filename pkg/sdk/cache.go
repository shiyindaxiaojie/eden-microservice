package sdk

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/shiyindaxiaojie/eden-go-logger"
)

type localCache struct {
	mu        sync.RWMutex
	dir       string
	cacheFile string
	services  map[string][]*ServiceInstance
}

func newLocalCache(cacheDir string) *localCache {
	if cacheDir == "" {
		return nil
	}

	err := os.MkdirAll(cacheDir, 0755)
	if err != nil {
		logger.Warn("[Registry SDK] Failed to create cache directory %s: %v", cacheDir, err)
	}

	c := &localCache{
		dir:       cacheDir,
		cacheFile: filepath.Join(cacheDir, "services.json"),
		services:  make(map[string][]*ServiceInstance),
	}
	c.Load()
	return c
}

// Load loads cached service instances from disk.
func (c *localCache) Load() {
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

	var services map[string][]*ServiceInstance
	if err := json.Unmarshal(data, &services); err != nil {
		logger.Warn("[Registry SDK] Failed to parse cache file: %v", err)
		return
	}

	c.services = services
	logger.Info("[Registry SDK] Loaded local cache with %d services from %s", len(services), c.cacheFile)
}

// Save persists the current cache state to disk.
func (c *localCache) Save() {
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
func (c *localCache) Get(serviceName string) ([]*ServiceInstance, bool) {
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
	result := make([]*ServiceInstance, 0, len(instances))
	for _, inst := range instances {
		cp := *inst
		result = append(result, &cp)
	}
	return result, true
}

// ServiceNames returns the cached service names.
func (c *localCache) ServiceNames() []string {
	if c == nil {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.services))
	for name := range c.services {
		names = append(names, name)
	}
	return names
}

// Update caches new instances for a service and saves to disk.
func (c *localCache) Update(serviceName string, instances []*ServiceInstance) {
	if c == nil {
		return
	}

	c.mu.Lock()
	// Need to copy instances
	copied := make([]*ServiceInstance, 0, len(instances))
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
func (c *localCache) UpdateAll(services map[string][]*ServiceInstance) {
	if c == nil {
		return
	}

	c.mu.Lock()
	changed := false
	for svcName, instances := range services {
		copied := make([]*ServiceInstance, 0, len(instances))
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
func (c *localCache) StartBackgroundSyncer(interval time.Duration, syncFn func() map[string][]*ServiceInstance, stopCh <-chan struct{}) {
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

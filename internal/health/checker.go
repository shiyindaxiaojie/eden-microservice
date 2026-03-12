package health

import (
	"log"
	"time"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/store"
)

// Checker periodically scans the registry for expired instances.
type Checker struct {
	registry *store.Registry
	ttl      time.Duration
	interval time.Duration
	stopCh   chan struct{}
}

// NewChecker creates a health checker.
// ttl: how long an instance can go without heartbeat before being marked critical.
// interval: how often to run the check.
func NewChecker(registry *store.Registry, ttl, interval time.Duration) *Checker {
	return &Checker{
		registry: registry,
		ttl:      ttl,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the background health check loop.
func (c *Checker) Start() {
	go func() {
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		log.Printf("[HealthChecker] started, ttl=%v interval=%v", c.ttl, c.interval)
		for {
			select {
			case <-ticker.C:
				removed := c.registry.MarkCritical(c.ttl)
				if len(removed) > 0 {
					log.Printf("[HealthChecker] removed %d expired instances", len(removed))
				}
			case <-c.stopCh:
				log.Println("[HealthChecker] stopped")
				return
			}
		}
	}()
}

// Stop terminates the health checker.
func (c *Checker) Stop() {
	close(c.stopCh)
}

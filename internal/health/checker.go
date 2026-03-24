package health

import (
	"fmt"
	"time"

	"github.com/shiyindaxiaojie/eden-go-logger"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
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
		logger.Info("[HealthChecker] started, ttl=%v interval=%v", c.ttl, c.interval)
		for {
			select {
			case <-ticker.C:
				marked, removed := c.registry.MarkCritical(c.ttl)
				if len(marked) > 0 {
					logger.Debug("[HealthChecker] marked %d instances as critical", len(marked))
					for _, inst := range marked {
						c.registry.AppendEvent(
							model.EventTypeServiceOffline,
							inst.ServiceName,
							fmt.Sprintf("%s:%d", inst.Host, inst.Port),
							"Heartbeat timeout, instance marked offline",
						)
					}
				}
				if len(removed) > 0 {
					logger.Info("[HealthChecker] removed %d expired instances", len(removed))
					for _, inst := range removed {
						c.registry.AppendEvent(
							model.EventTypeServiceRemove,
							inst.ServiceName,
							fmt.Sprintf("%s:%d", inst.Host, inst.Port),
							"Instance removed after retention window",
						)
					}
				}
			case <-c.stopCh:
				logger.Info("[HealthChecker] stopped")
				return
			}
		}
	}()
}

// Stop terminates the health checker.
func (c *Checker) Stop() {
	close(c.stopCh)
}

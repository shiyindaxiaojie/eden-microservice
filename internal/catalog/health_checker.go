package catalog

import (
	"fmt"
	"time"

	"github.com/shiyindaxiaojie/eden-go-logger"
)

type HealthSettings interface {
	GetHeartbeatMaxFailures() int
	GetInstanceRemovalDelaySeconds() int
}

// Checker periodically scans the catalog for expired instances.
type Checker struct {
	catalog  *State
	settings HealthSettings
	ttl      time.Duration
	interval time.Duration
	stopCh   chan struct{}
}

func NewChecker(catalog *State, settings HealthSettings, ttl, interval time.Duration) *Checker {
	return &Checker{
		catalog:  catalog,
		settings: settings,
		ttl:      ttl,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

func (c *Checker) Start() {
	go func() {
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		logger.Info("[HealthChecker] started, ttl=%v interval=%v", c.ttl, c.interval)
		for {
			select {
			case <-ticker.C:
				maxFailures := 3
				if c.settings != nil && c.settings.GetHeartbeatMaxFailures() > 0 {
					maxFailures = c.settings.GetHeartbeatMaxFailures()
				}
				removalDelay := 600 * time.Second
				if c.settings != nil && c.settings.GetInstanceRemovalDelaySeconds() > 0 {
					removalDelay = time.Duration(c.settings.GetInstanceRemovalDelaySeconds()) * time.Second
				}
				marked, removed := c.catalog.Instances.MarkCritical(c.ttl, maxFailures, removalDelay)
				if len(marked) > 0 {
					logger.Debug("[HealthChecker] marked %d instances as critical", len(marked))
					for _, inst := range marked {
						c.catalog.AppendEvent(
							EventTypeServiceOffline,
							inst.ServiceName,
							fmt.Sprintf("%s:%d", inst.Host, inst.Port),
							"Heartbeat timeout, instance marked offline",
						)
					}
				}
				if len(removed) > 0 {
					logger.Info("[HealthChecker] removed %d expired instances", len(removed))
					for _, inst := range removed {
						c.catalog.AppendEvent(
							EventTypeServiceRemove,
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

func (c *Checker) Stop() {
	close(c.stopCh)
}

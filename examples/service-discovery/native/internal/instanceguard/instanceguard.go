package instanceguard

import (
	"sync"

	"github.com/shiyindaxiaojie/eden-registry/pkg/sdk"
)

// WatchSelfOffline closes the returned channel when the instance becomes unhealthy
// or disappears from its own service watch stream.
func WatchSelfOffline(reg sdk.Registry, serviceName, instanceID string) (<-chan struct{}, error) {
	if serviceName == "" || instanceID == "" {
		return nil, nil
	}

	stopCh := make(chan struct{})
	var once sync.Once
	stop := func() {
		once.Do(func() {
			close(stopCh)
		})
	}

	if err := reg.Subscribe(serviceName, func(items []*sdk.ServiceInstance) {
		if shouldStopHeartbeat(items, instanceID) {
			stop()
		}
	}); err != nil {
		return nil, err
	}

	return stopCh, nil
}

func shouldStopHeartbeat(items []*sdk.ServiceInstance, instanceID string) bool {
	for _, item := range items {
		if item == nil || item.ID != instanceID {
			continue
		}
		return !item.Healthy
	}
	return true
}

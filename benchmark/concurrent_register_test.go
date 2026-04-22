package benchmark

import (
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/shiyindaxiaojie/eden-registry/internal/catalog"
)

func BenchmarkConcurrentRegister(b *testing.B) {
	b.Run("same-service", func(b *testing.B) {
		benchmarkConcurrentRegister(b, func(seq uint64) *catalog.Instance {
			return &catalog.Instance{
				ID:          "instance-" + strconv.FormatUint(seq, 10),
				ServiceName: "order-service",
				Namespace:   catalog.DefaultNamespace,
				Host:        "127.0.0.1",
				Port:        9000 + int(seq%1000),
				Weight:      1,
				Metadata: map[string]string{
					"zone": "benchmark-a",
				},
			}
		})
	})

	b.Run("multi-service", func(b *testing.B) {
		benchmarkConcurrentRegister(b, func(seq uint64) *catalog.Instance {
			return &catalog.Instance{
				ID:          "instance-" + strconv.FormatUint(seq, 10),
				ServiceName: "svc-" + strconv.FormatUint(seq%32, 10),
				Namespace:   catalog.DefaultNamespace,
				Host:        "127.0.0.1",
				Port:        10000 + int(seq%1000),
				Weight:      1,
				Metadata: map[string]string{
					"zone": "benchmark-b",
				},
			}
		})
	})
}

func benchmarkConcurrentRegister(b *testing.B, newInstance func(seq uint64) *catalog.Instance) {
	b.Helper()
	b.ReportAllocs()

	state := catalog.NewState("")
	registry := catalog.NewRegistry(state, nil, nil, nil)

	var counter uint64

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			seq := atomic.AddUint64(&counter, 1) - 1
			if err := registry.Register(newInstance(seq)); err != nil {
				b.Fatalf("register failed: %v", err)
			}
		}
	})
	b.StopTimer()

	stats := state.Stats()
	if stats.InstanceCount != b.N {
		b.Fatalf("instance count = %d, want %d", stats.InstanceCount, b.N)
	}
	if stats.HealthyCount != b.N {
		b.Fatalf("healthy count = %d, want %d", stats.HealthyCount, b.N)
	}
}

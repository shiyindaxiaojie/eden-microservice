package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInstanceRegistryRegisterSyncFlushPersistsToDisk(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	reg := NewInstanceRegistry(dir)
	reg.SetFlushMode("sync")

	reg.Register(&Instance{
		ID:          "order-1",
		ServiceName: "order-service",
		Namespace:   DefaultNamespace,
		Host:        "127.0.0.1",
		Port:        8080,
	})

	services := readServicesFile(t, filepath.Join(dir, "services.json"))
	if got := len(services["order-service"]); got != 1 {
		t.Fatalf("persisted instance count = %d, want 1", got)
	}
}

func TestInstanceRegistryRegisterAsyncFlushPersistsToDisk(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	reg := NewInstanceRegistry(dir)
	reg.SetFlushMode("async")
	reg.SetFlushInterval(20 * time.Millisecond)

	reg.Register(&Instance{
		ID:          "order-2",
		ServiceName: "order-service",
		Namespace:   DefaultNamespace,
		Host:        "127.0.0.1",
		Port:        8081,
	})

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		services := readServicesFileIfExists(t, filepath.Join(dir, "services.json"))
		if len(services["order-service"]) == 1 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("async flush did not persist services.json within 1s")
}

func readServicesFile(t *testing.T, file string) map[string][]*Instance {
	t.Helper()

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}

	var services map[string][]*Instance
	if err := json.Unmarshal(data, &services); err != nil {
		t.Fatalf("unmarshal %s: %v", file, err)
	}
	return services
}

func readServicesFileIfExists(t *testing.T, file string) map[string][]*Instance {
	t.Helper()

	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string][]*Instance{}
		}
		t.Fatalf("read %s: %v", file, err)
	}

	var services map[string][]*Instance
	if err := json.Unmarshal(data, &services); err != nil {
		t.Fatalf("unmarshal %s: %v", file, err)
	}
	return services
}

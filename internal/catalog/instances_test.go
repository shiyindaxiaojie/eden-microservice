package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInstanceRegistryDefaultsServiceGroup(t *testing.T) {
	reg := NewInstanceRegistry("")
	inst := &Instance{
		ID:          "auth-1",
		ServiceName: "auth-service",
	}

	reg.Register(inst)

	if inst.Group != DefaultServiceGroup {
		t.Fatalf("expected default group %q, got %q", DefaultServiceGroup, inst.Group)
	}
	services := reg.ListServicesNS(DefaultNamespace)
	if len(services) != 1 {
		t.Fatalf("expected one service, got %d", len(services))
	}
	if services[0].Name != "auth-service" || services[0].Group != DefaultServiceGroup {
		t.Fatalf("unexpected service identity: %#v", services[0])
	}
}

func TestInstanceRegistrySeparatesServicesByGroup(t *testing.T) {
	reg := NewInstanceRegistry("")
	reg.Register(&Instance{ID: "auth-default", ServiceName: "auth-service", Group: "DEFAULT_GROUP"})
	reg.Register(&Instance{ID: "auth-internal", ServiceName: "auth-service", Group: "INTERNAL"})

	defaultInstances := reg.GetServiceNS(DefaultNamespace, QualifiedServiceName("DEFAULT_GROUP", "auth-service"))
	internalInstances := reg.GetServiceNS(DefaultNamespace, QualifiedServiceName("INTERNAL", "auth-service"))
	if len(defaultInstances) != 1 || defaultInstances[0].ID != "auth-default" {
		t.Fatalf("unexpected DEFAULT_GROUP instances: %#v", defaultInstances)
	}
	if len(internalInstances) != 1 || internalInstances[0].ID != "auth-internal" {
		t.Fatalf("unexpected INTERNAL instances: %#v", internalInstances)
	}
	if services := reg.ListServicesNS(DefaultNamespace); len(services) != 2 {
		t.Fatalf("expected two grouped services, got %d", len(services))
	}
}

func TestInstanceRegistryMigratesQualifiedServiceName(t *testing.T) {
	reg := NewInstanceRegistry("")
	inst := &Instance{ID: "auth-1", ServiceName: "DEFAULT_GROUP@@auth-service"}

	reg.Register(inst)

	if inst.ServiceName != "auth-service" || inst.Group != "DEFAULT_GROUP" {
		t.Fatalf("expected separated legacy identity, got service=%q group=%q", inst.ServiceName, inst.Group)
	}
	if got := reg.GetServiceNS(DefaultNamespace, "DEFAULT_GROUP@@auth-service"); len(got) != 1 {
		t.Fatalf("expected legacy qualified lookup to resolve, got %d instances", len(got))
	}
}

func TestInstanceRegistryPersistsSeparatedGroup(t *testing.T) {
	dir := t.TempDir()
	reg := NewInstanceRegistry(dir)
	reg.SetFlushMode("sync")
	reg.Register(&Instance{ID: "auth-1", ServiceName: "auth-service", Group: "DEFAULT_GROUP"})

	raw, err := os.ReadFile(filepath.Join(dir, "services.json"))
	if err != nil {
		t.Fatalf("read services: %v", err)
	}
	var stored map[string][]Instance
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatalf("decode services: %v", err)
	}
	instances := stored["DEFAULT_GROUP@@auth-service"]
	if len(instances) != 1 {
		t.Fatalf("expected qualified storage key, got keys %#v", stored)
	}
	if instances[0].ServiceName != "auth-service" || instances[0].Group != "DEFAULT_GROUP" {
		t.Fatalf("expected separated persisted identity, got %#v", instances[0])
	}
}

func TestInstanceRegistryHeartbeatResolvesInstanceIDWithoutServiceName(t *testing.T) {
	t.Parallel()

	reg := NewInstanceRegistry("")
	inst := &Instance{
		ID:          "auth-1",
		ServiceName: "auth-service",
		Namespace:   DefaultNamespace,
		Host:        "127.0.0.1",
		Port:        8080,
	}
	reg.Register(inst)

	inst.Status = HealthCritical
	inst.ManualOffline = false
	inst.LastHeartbeat = time.Now().Add(-time.Minute)

	got, recovered := reg.HeartbeatNS(DefaultNamespace, "", inst.ID)
	if got == nil {
		t.Fatalf("expected heartbeat to resolve instance by id")
	}
	if !recovered {
		t.Fatalf("expected heartbeat to recover critical instance")
	}
	if got.ServiceName != inst.ServiceName {
		t.Fatalf("expected resolved service %q, got %q", inst.ServiceName, got.ServiceName)
	}
	if got.Status != HealthPassing {
		t.Fatalf("expected status %q, got %q", HealthPassing, got.Status)
	}
}

func TestInstanceRegistrySetStatusFallsBackToInstanceIDLookup(t *testing.T) {
	t.Parallel()

	reg := NewInstanceRegistry("")
	inst := &Instance{
		ID:          "auth-2",
		ServiceName: "auth-service",
		Namespace:   DefaultNamespace,
		Host:        "127.0.0.1",
		Port:        8081,
	}
	reg.Register(inst)

	got, ok := reg.SetInstanceStatus(DefaultNamespace, "missing-service", inst.ID, HealthCritical)
	if !ok || got == nil {
		t.Fatalf("expected status update to resolve instance by id")
	}
	if got.Status != HealthCritical {
		t.Fatalf("expected status %q, got %q", HealthCritical, got.Status)
	}
}

func TestRegistrySetInstanceStatusKeepsManualOfflineDistinctFromCritical(t *testing.T) {
	registry := NewRegistry(NewState(""), nil, nil, nil)
	instance := &Instance{
		ID:          "auth-offline-1",
		ServiceName: "auth-service",
		Namespace:   DefaultNamespace,
		Host:        "127.0.0.1",
		Port:        8083,
	}
	if err := registry.Register(instance); err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := registry.SetInstanceStatus(DefaultNamespace, instance.QualifiedServiceName(), instance.ID, "offline"); err != nil {
		t.Fatalf("take offline: %v", err)
	}

	instances, err := registry.GetService(DefaultNamespace, instance.QualifiedServiceName(), false)
	if err != nil {
		t.Fatalf("get service: %v", err)
	}
	if len(instances) != 1 {
		t.Fatalf("expected one instance, got %d", len(instances))
	}
	if instances[0].Status != HealthStatus("offline") {
		t.Fatalf("expected manual offline status, got %q", instances[0].Status)
	}
	if !instances[0].ManualOffline {
		t.Fatal("expected instance to retain its manual-offline marker")
	}

	if err := registry.SetInstanceStatus(DefaultNamespace, instance.QualifiedServiceName(), instance.ID, "online"); err != nil {
		t.Fatalf("restore online: %v", err)
	}
	instances, err = registry.GetService(DefaultNamespace, instance.QualifiedServiceName(), false)
	if err != nil {
		t.Fatalf("get restored service: %v", err)
	}
	if instances[0].Status != HealthPassing || instances[0].ManualOffline {
		t.Fatalf("expected restored passing instance, got status=%q manual_offline=%v", instances[0].Status, instances[0].ManualOffline)
	}
}

func TestInstanceRegistryDeregisterFallsBackToInstanceIDLookup(t *testing.T) {
	t.Parallel()

	reg := NewInstanceRegistry("")
	inst := &Instance{
		ID:          "auth-3",
		ServiceName: "auth-service",
		Namespace:   DefaultNamespace,
		Host:        "127.0.0.1",
		Port:        8082,
	}
	reg.Register(inst)

	got, ok := reg.DeregisterNS(DefaultNamespace, "", inst.ID)
	if !ok || got == nil {
		t.Fatalf("expected deregister to resolve instance by id")
	}
	if got.ID != inst.ID {
		t.Fatalf("expected instance %q, got %q", inst.ID, got.ID)
	}
	if items := reg.GetServiceNS(DefaultNamespace, inst.ServiceName); len(items) != 0 {
		t.Fatalf("expected service to be empty after deregister, got %d instance(s)", len(items))
	}
}

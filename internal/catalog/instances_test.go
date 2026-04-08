package catalog

import (
	"testing"
	"time"
)

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

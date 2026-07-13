package gateway

import (
	"errors"
	"testing"
)

func validRoute() Route {
	return Route{
		Identity: Identity{Namespace: "default", ID: "orders"},
		Name:     "Orders",
		Enabled:  true,
		Priority: 10,
		Match: RouteMatch{
			PathPrefix: "/orders",
			Methods:    []string{"GET"},
		},
		Targets: []Target{
			{
				ID:          "stable",
				Type:        TargetService,
				Service:     &ServiceTarget{Namespace: "default", Group: "default", ServiceName: "order-service"},
				LoadBalance: LoadBalanceRoundRobin,
				HealthyOnly: true,
			},
			{
				ID:          "canary",
				Type:        TargetService,
				Service:     &ServiceTarget{Namespace: "default", Group: "default", ServiceName: "order-service-v2"},
				LoadBalance: LoadBalanceRoundRobin,
				HealthyOnly: true,
			},
		},
		Traffic: TrafficPolicy{
			Mode:            TrafficCanary,
			DefaultTargetID: "stable",
			WeightedTargets: []WeightedTarget{
				{TargetID: "stable", Weight: 95},
				{TargetID: "canary", Weight: 5},
			},
		},
		TimeoutMS: 30_000,
	}
}

func TestValidateRouteAcceptsCanaryWithBetaTarget(t *testing.T) {
	route := validRoute()
	route.Traffic.BetaTargets = []BetaTarget{{TargetID: "canary", Users: []string{"u-1"}, Tenants: []string{"tenant-a"}}}

	if err := ValidateRoute(route); err != nil {
		t.Fatalf("ValidateRoute() error = %v", err)
	}
}

func TestValidateRouteRejectsOverlappingBetaUsers(t *testing.T) {
	route := validRoute()
	route.Traffic.BetaTargets = []BetaTarget{
		{TargetID: "stable", Users: []string{"u-1"}},
		{TargetID: "canary", Users: []string{"u-1"}},
	}

	if err := ValidateRoute(route); !errors.Is(err, ErrInvalidRoute) {
		t.Fatalf("ValidateRoute() error = %v, want ErrInvalidRoute", err)
	}
}

func TestValidateRouteRejectsUnsafeStaticEndpoint(t *testing.T) {
	route := validRoute()
	route.Targets[1] = Target{
		ID:          "legacy",
		Type:        TargetStatic,
		Static:      &StaticTarget{Endpoints: []StaticEndpoint{{URL: "https://user:secret@legacy.internal"}}},
		LoadBalance: LoadBalanceRoundRobin,
	}
	route.Traffic = TrafficPolicy{
		Mode:            TrafficWeighted,
		DefaultTargetID: "stable",
		WeightedTargets: []WeightedTarget{{TargetID: "stable", Weight: 50}, {TargetID: "legacy", Weight: 50}},
	}

	if err := ValidateRoute(route); !errors.Is(err, ErrInvalidRoute) {
		t.Fatalf("ValidateRoute() error = %v, want ErrInvalidRoute", err)
	}
}

func TestValidateRouteRequiresActiveBlueGreenTarget(t *testing.T) {
	route := validRoute()
	route.Traffic = TrafficPolicy{Mode: TrafficBlueGreen, ActiveTargetID: "missing"}

	if err := ValidateRoute(route); !errors.Is(err, ErrInvalidRoute) {
		t.Fatalf("ValidateRoute() error = %v, want ErrInvalidRoute", err)
	}
}

func TestValidateRouteRejectsGatewayIdentityRequestHeaderFilter(t *testing.T) {
	route := validRoute()
	route.Filters = []Filter{{Type: FilterAddRequestHeader, Name: "X-Eden-User-ID", Value: "spoofed"}}

	if err := ValidateRoute(route); !errors.Is(err, ErrInvalidRoute) {
		t.Fatalf("ValidateRoute() error = %v, want ErrInvalidRoute", err)
	}
}

package gateway

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func NormalizeIdentity(identity Identity) (Identity, error) {
	identity.Namespace = strings.TrimSpace(identity.Namespace)
	identity.ID = strings.TrimSpace(identity.ID)
	if identity.Namespace == "" {
		identity.Namespace = DefaultNamespace
	}
	if identity.ID == "" || !identifierPattern.MatchString(identity.ID) {
		return Identity{}, fmt.Errorf("%w: route id must use letters, numbers, dots, hyphens, or underscores", ErrInvalidRoute)
	}
	return identity, nil
}

// NormalizeRoute returns a safe canonical representation suitable for persistence.
func NormalizeRoute(route Route) (Route, error) {
	identity, err := NormalizeIdentity(route.Identity)
	if err != nil {
		return Route{}, err
	}
	route.Identity = identity
	route.Name = strings.TrimSpace(route.Name)
	if route.Name == "" {
		return Route{}, invalid("route name is required")
	}
	route.Match, err = normalizeMatch(route.Match)
	if err != nil {
		return Route{}, err
	}
	if len(route.Targets) == 0 {
		return Route{}, invalid("at least one target is required")
	}
	targets := make(map[string]Target, len(route.Targets))
	for i := range route.Targets {
		target, normalizeErr := normalizeTarget(route.Targets[i])
		if normalizeErr != nil {
			return Route{}, normalizeErr
		}
		if _, exists := targets[target.ID]; exists {
			return Route{}, invalid("target ids must be unique")
		}
		targets[target.ID] = target
		route.Targets[i] = target
	}
	route.Traffic, err = normalizeTraffic(route.Traffic, targets)
	if err != nil {
		return Route{}, err
	}
	route.Filters, err = normalizeFilters(route.Filters)
	if err != nil {
		return Route{}, err
	}
	if route.TimeoutMS <= 0 {
		route.TimeoutMS = 30_000
	}
	if route.TimeoutMS > 300_000 {
		return Route{}, invalid("timeout_ms must not exceed 300000")
	}
	return route, nil
}

func ValidateRoute(route Route) error {
	_, err := NormalizeRoute(route)
	return err
}

func normalizeMatch(match RouteMatch) (RouteMatch, error) {
	match.PathPrefix = strings.TrimSpace(match.PathPrefix)
	match.Path = strings.TrimSpace(match.Path)
	if (match.PathPrefix == "" && match.Path == "") || (match.PathPrefix != "" && match.Path != "") {
		return RouteMatch{}, invalid("exact path or path_prefix is required")
	}
	if match.PathPrefix != "" && !strings.HasPrefix(match.PathPrefix, "/") {
		return RouteMatch{}, invalid("path_prefix must start with /")
	}
	if match.Path != "" && !strings.HasPrefix(match.Path, "/") {
		return RouteMatch{}, invalid("path must start with /")
	}
	match.Hosts = normalizedStrings(match.Hosts)
	match.Methods = normalizedStrings(match.Methods)
	for i := range match.Methods {
		match.Methods[i] = strings.ToUpper(match.Methods[i])
		if !identifierPattern.MatchString(match.Methods[i]) {
			return RouteMatch{}, invalid("invalid method")
		}
	}
	if len(match.Headers) == 0 {
		match.Headers = nil
		return match, nil
	}
	headers := make(map[string]string, len(match.Headers))
	for name, value := range match.Headers {
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if name == "" || value == "" || strings.ContainsAny(name, "\r\n") || strings.ContainsAny(value, "\r\n") {
			return RouteMatch{}, invalid("invalid route header matcher")
		}
		headers[strings.ToLower(name)] = value
	}
	match.Headers = headers
	return match, nil
}

func normalizeTarget(target Target) (Target, error) {
	target.ID = strings.TrimSpace(target.ID)
	if target.ID == "" || !identifierPattern.MatchString(target.ID) {
		return Target{}, invalid("invalid target id")
	}
	target.Name = strings.TrimSpace(target.Name)
	if target.LoadBalance == "" {
		target.LoadBalance = LoadBalanceRoundRobin
	}
	if target.LoadBalance != LoadBalanceRoundRobin && target.LoadBalance != LoadBalanceRandom && target.LoadBalance != LoadBalanceWeighted {
		return Target{}, invalid("invalid target load_balance")
	}
	switch target.Type {
	case TargetService:
		if target.Service == nil || target.Static != nil {
			return Target{}, invalid("service target must contain only service")
		}
		target.Service.Namespace = strings.TrimSpace(target.Service.Namespace)
		target.Service.Group = strings.TrimSpace(target.Service.Group)
		target.Service.ServiceName = strings.TrimSpace(target.Service.ServiceName)
		if target.Service.Namespace == "" {
			target.Service.Namespace = DefaultNamespace
		}
		if target.Service.Group == "" {
			target.Service.Group = "default"
		}
		if target.Service.ServiceName == "" {
			return Target{}, invalid("service target service_name is required")
		}
		// Service traffic must never include unhealthy catalog instances by default.
		target.HealthyOnly = true
	case TargetStatic:
		if target.Static == nil || target.Service != nil {
			return Target{}, invalid("static target must contain only static")
		}
		if len(target.Static.Endpoints) == 0 {
			return Target{}, invalid("static target needs an endpoint")
		}
		seen := make(map[string]struct{}, len(target.Static.Endpoints))
		for i := range target.Static.Endpoints {
			endpoint, err := normalizeStaticEndpoint(target.Static.Endpoints[i])
			if err != nil {
				return Target{}, err
			}
			if _, exists := seen[endpoint.URL]; exists {
				return Target{}, invalid("static endpoint URLs must be unique")
			}
			seen[endpoint.URL] = struct{}{}
			target.Static.Endpoints[i] = endpoint
		}
	default:
		return Target{}, invalid("target type must be service or static")
	}
	return target, nil
}

func normalizeStaticEndpoint(endpoint StaticEndpoint) (StaticEndpoint, error) {
	endpoint.URL = strings.TrimSpace(endpoint.URL)
	parsed, err := url.Parse(endpoint.URL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return StaticEndpoint{}, invalid("static endpoint must be a safe http(s) URL")
	}
	if endpoint.Weight == 0 {
		endpoint.Weight = 1
	}
	if endpoint.Weight < 1 {
		return StaticEndpoint{}, invalid("static endpoint weight must be positive")
	}
	return endpoint, nil
}

func normalizeTraffic(policy TrafficPolicy, targets map[string]Target) (TrafficPolicy, error) {
	if policy.Mode == "" {
		policy.Mode = TrafficWeighted
	}
	policy.DefaultTargetID = strings.TrimSpace(policy.DefaultTargetID)
	policy.ActiveTargetID = strings.TrimSpace(policy.ActiveTargetID)
	if policy.DefaultTargetID == "" && len(targets) == 1 {
		for id := range targets {
			policy.DefaultTargetID = id
		}
	}
	if policy.DefaultTargetID != "" {
		if _, exists := targets[policy.DefaultTargetID]; !exists {
			return TrafficPolicy{}, invalid("default_target_id must reference a target")
		}
	}
	if err := normalizeBetaTargets(&policy, targets); err != nil {
		return TrafficPolicy{}, err
	}
	switch policy.Mode {
	case TrafficWeighted, TrafficCanary:
		if policy.DefaultTargetID == "" {
			return TrafficPolicy{}, invalid("default_target_id is required")
		}
		if len(policy.WeightedTargets) == 0 && len(targets) == 1 {
			policy.WeightedTargets = []WeightedTarget{{TargetID: policy.DefaultTargetID, Weight: 100}}
		}
		if len(policy.WeightedTargets) == 0 {
			return TrafficPolicy{}, invalid("weighted_targets are required")
		}
		seen := make(map[string]struct{}, len(policy.WeightedTargets))
		total := 0
		for i := range policy.WeightedTargets {
			item := &policy.WeightedTargets[i]
			item.TargetID = strings.TrimSpace(item.TargetID)
			if _, exists := targets[item.TargetID]; !exists || item.Weight <= 0 || item.Weight > 100 {
				return TrafficPolicy{}, invalid("invalid weighted target")
			}
			if _, exists := seen[item.TargetID]; exists {
				return TrafficPolicy{}, invalid("weighted target ids must be unique")
			}
			seen[item.TargetID] = struct{}{}
			total += item.Weight
		}
		if total != 100 {
			return TrafficPolicy{}, invalid("weighted target weights must total 100")
		}
	case TrafficBlueGreen:
		if policy.ActiveTargetID == "" {
			return TrafficPolicy{}, invalid("active_target_id is required for blue_green")
		}
		if _, exists := targets[policy.ActiveTargetID]; !exists {
			return TrafficPolicy{}, invalid("active_target_id must reference a target")
		}
		if len(policy.WeightedTargets) != 0 {
			return TrafficPolicy{}, invalid("blue_green does not allow weighted_targets")
		}
		policy.DefaultTargetID = policy.ActiveTargetID
	default:
		return TrafficPolicy{}, invalid("invalid traffic mode")
	}
	return policy, nil
}

func normalizeBetaTargets(policy *TrafficPolicy, targets map[string]Target) error {
	users := make(map[string]string)
	tenants := make(map[string]string)
	seenTargets := make(map[string]struct{}, len(policy.BetaTargets))
	for i := range policy.BetaTargets {
		beta := &policy.BetaTargets[i]
		beta.TargetID = strings.TrimSpace(beta.TargetID)
		if _, exists := targets[beta.TargetID]; !exists {
			return invalid("beta target must reference a target")
		}
		if _, exists := seenTargets[beta.TargetID]; exists {
			return invalid("beta target ids must be unique")
		}
		seenTargets[beta.TargetID] = struct{}{}
		beta.Users = normalizedStrings(beta.Users)
		beta.Tenants = normalizedStrings(beta.Tenants)
		if len(beta.Users) == 0 && len(beta.Tenants) == 0 {
			return invalid("beta target needs users or tenants")
		}
		for _, user := range beta.Users {
			if existing, exists := users[user]; exists && existing != beta.TargetID {
				return invalid("a beta user cannot map to multiple targets")
			}
			users[user] = beta.TargetID
		}
		for _, tenant := range beta.Tenants {
			if existing, exists := tenants[tenant]; exists && existing != beta.TargetID {
				return invalid("a beta tenant cannot map to multiple targets")
			}
			tenants[tenant] = beta.TargetID
		}
	}
	return nil
}

func normalizeFilters(filters []Filter) ([]Filter, error) {
	if len(filters) == 0 {
		return nil, nil
	}
	result := make([]Filter, len(filters))
	for i, filter := range filters {
		filter.Name = strings.TrimSpace(filter.Name)
		filter.Value = strings.TrimSpace(filter.Value)
		switch filter.Type {
		case FilterStripPrefix:
			if filter.Parts < 1 {
				return nil, invalid("strip_prefix parts must be positive")
			}
		case FilterAddRequestHeader, FilterSetResponseHeader:
			if filter.Name == "" || strings.ContainsAny(filter.Name, "\r\n") || strings.ContainsAny(filter.Value, "\r\n") {
				return nil, invalid("invalid header filter")
			}
			if filter.Type == FilterAddRequestHeader && protectedRequestHeader(filter.Name) {
				return nil, invalid("request filter may not change protected header")
			}
		default:
			return nil, invalid("unknown filter type")
		}
		result[i] = filter
	}
	return result, nil
}

func protectedRequestHeader(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return name == "authorization" ||
		name == "cookie" ||
		name == "x-api-key" ||
		name == "x-eden-user-id" ||
		name == "x-eden-tenant-id" ||
		strings.HasPrefix(name, "x-forwarded-") ||
		name == "x-real-ip"
}

func normalizedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func invalid(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidRoute, message)
}

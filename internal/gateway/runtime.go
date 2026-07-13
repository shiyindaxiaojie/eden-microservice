package gateway

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	logger "github.com/shiyindaxiaojie/eden-go-logger"
	"github.com/shiyindaxiaojie/eden-registry/internal/catalog"
)

var errNoAvailableEndpoint = errors.New("no available gateway endpoint")

// Discovery is the narrow catalog dependency required by the gateway runtime.
type Discovery interface {
	GetService(namespace, name string, healthyOnly bool) ([]*catalog.Instance, error)
}

// RuntimeConfig controls data-plane identity trust.
type RuntimeConfig struct {
	TrustedProxyCIDRs []string
	UserHeader        string
	TenantHeader      string
}

type routeSnapshot struct {
	matched []Route
	all     []Route
}

type routeMetrics struct {
	requests atomic.Uint64
	errors   atomic.Uint64
	lastCode atomic.Int64

	mu            sync.RWMutex
	lastError     string
	lastRequestAt time.Time
}

// Runtime is the independent HTTP data-plane handler backed by immutable route snapshots.
type Runtime struct {
	service      Service
	discovery    Discovery
	trusted      []*net.IPNet
	userHeader   string
	tenantHeader string

	snapshot atomic.Value // *routeSnapshot
	cancel   func()

	metrics      sync.Map // map[string]*routeMetrics
	loadCounters sync.Map // map[string]*atomic.Uint64
}

// NewRuntime loads the first route snapshot and subscribes to committed route changes.
func NewRuntime(service Service, discovery Discovery, config RuntimeConfig) (*Runtime, error) {
	if service == nil {
		return nil, fmt.Errorf("gateway service is required")
	}
	if discovery == nil {
		return nil, fmt.Errorf("gateway discovery is required")
	}
	trusted, err := parseTrustedCIDRs(config.TrustedProxyCIDRs)
	if err != nil {
		return nil, err
	}
	if len(trusted) == 0 {
		trusted, err = parseTrustedCIDRs([]string{"127.0.0.1/32", "::1/128"})
		if err != nil {
			return nil, err
		}
	}
	runtime := &Runtime{
		service:      service,
		discovery:    discovery,
		trusted:      trusted,
		userHeader:   headerOrDefault(config.UserHeader, "X-Eden-User-ID"),
		tenantHeader: headerOrDefault(config.TenantHeader, "X-Eden-Tenant-ID"),
	}
	if err := runtime.reload(); err != nil {
		return nil, err
	}
	runtime.cancel = service.Subscribe(func() { _ = runtime.reload() })
	return runtime, nil
}

func (r *Runtime) Close() {
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
}

func (r *Runtime) reload() error {
	all, err := listAllRoutes(r.service)
	if err != nil {
		return err
	}
	matched := make([]Route, 0, len(all))
	for _, route := range all {
		if route.Enabled {
			matched = append(matched, route)
		}
	}
	r.snapshot.Store(&routeSnapshot{matched: matched, all: all})
	return nil
}

func listAllRoutes(service Service) ([]Route, error) {
	const pageSize = 500
	routes := make([]Route, 0)
	for page := 1; ; page++ {
		result, err := service.List(ListQuery{Page: page, PageSize: pageSize})
		if err != nil {
			return nil, err
		}
		routes = append(routes, result.Data...)
		if len(routes) >= result.Total || len(result.Data) == 0 {
			return routes, nil
		}
	}
}

func (r *Runtime) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	started := time.Now()
	snapshot, _ := r.snapshot.Load().(*routeSnapshot)
	if snapshot == nil {
		writeRuntimeError(w, http.StatusServiceUnavailable, "gateway route snapshot unavailable")
		logGatewayAccess(nil, request, http.StatusServiceUnavailable, started, "", "route snapshot unavailable")
		return
	}
	route, ok := matchRoute(snapshot.matched, request)
	if !ok {
		writeRuntimeError(w, http.StatusNotFound, "gateway route not found")
		logGatewayAccess(nil, request, http.StatusNotFound, started, "", "route not found")
		return
	}
	target, ok := r.selectTarget(request, route)
	if !ok {
		r.record(route, http.StatusBadGateway, "route target is unavailable")
		writeRuntimeError(w, http.StatusBadGateway, "gateway route target unavailable")
		logGatewayAccess(&route, request, http.StatusBadGateway, started, "", "route target is unavailable")
		return
	}
	baseURL, endpointKey, err := r.resolveEndpoint(route, target)
	if err != nil {
		if errors.Is(err, errNoAvailableEndpoint) {
			r.record(route, http.StatusServiceUnavailable, "no healthy upstream endpoint")
			writeRuntimeError(w, http.StatusServiceUnavailable, "gateway upstream unavailable")
			logGatewayAccess(&route, request, http.StatusServiceUnavailable, started, "", "no healthy upstream endpoint")
			return
		}
		r.record(route, http.StatusBadGateway, "upstream resolution failed")
		writeRuntimeError(w, http.StatusBadGateway, "gateway upstream unavailable")
		logGatewayAccess(&route, request, http.StatusBadGateway, started, "", "upstream resolution failed")
		return
	}
	r.proxy(w, request, route, baseURL, endpointKey, started)
}

func (r *Runtime) proxy(w http.ResponseWriter, request *http.Request, route Route, baseURL *url.URL, endpointKey string, started time.Time) {
	timeout := time.Duration(route.TimeoutMS) * time.Millisecond
	ctx, cancel := context.WithTimeout(request.Context(), timeout)
	defer cancel()
	outgoing := request.Clone(ctx)
	outgoing.Header = request.Header.Clone()
	outgoing.RequestURI = ""
	applyRequestFilters(outgoing, route.Filters)
	r.stripUntrustedIdentityHeaders(outgoing, request)
	responseWriter := &statusWriter{ResponseWriter: w}
	var proxyError error
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = baseURL.Scheme
			req.URL.Host = baseURL.Host
			req.URL.Path = joinURLPath(baseURL.Path, req.URL.Path)
			req.URL.RawPath = ""
			req.Host = baseURL.Host
		},
		ModifyResponse: func(response *http.Response) error {
			applyResponseFilters(response.Header, route.Filters)
			return nil
		},
		ErrorHandler: func(writer http.ResponseWriter, req *http.Request, err error) {
			proxyError = err
			if errors.Is(err, context.DeadlineExceeded) || ctx.Err() == context.DeadlineExceeded {
				writeRuntimeError(writer, http.StatusGatewayTimeout, "gateway upstream timeout")
				return
			}
			writeRuntimeError(writer, http.StatusBadGateway, "gateway upstream error")
		},
	}
	proxy.ServeHTTP(responseWriter, outgoing)
	status := responseWriter.Status()
	if proxyError != nil && status == 0 {
		status = http.StatusBadGateway
	}
	if status == 0 {
		status = http.StatusOK
	}
	message := ""
	if proxyError != nil {
		message = "upstream proxy error"
	}
	r.record(route, status, message)
	logGatewayAccess(&route, request, status, started, endpointKey, message)
}

func (r *Runtime) resolveEndpoint(route Route, target Target) (*url.URL, string, error) {
	switch target.Type {
	case TargetService:
		name := catalog.QualifiedServiceName(target.Service.Group, target.Service.ServiceName)
		instances, err := r.discovery.GetService(target.Service.Namespace, name, target.HealthyOnly)
		if err != nil || len(instances) == 0 {
			return nil, "", errNoAvailableEndpoint
		}
		instance := r.selectInstance(route, target, instances)
		if instance == nil || instance.Host == "" || instance.Port <= 0 {
			return nil, "", errNoAvailableEndpoint
		}
		return &url.URL{Scheme: "http", Host: net.JoinHostPort(instance.Host, strconv.Itoa(instance.Port))}, routeCounterKey(route, target) + "/" + instance.ID, nil
	case TargetStatic:
		if target.Static == nil || len(target.Static.Endpoints) == 0 {
			return nil, "", errNoAvailableEndpoint
		}
		endpoint := r.selectStaticEndpoint(route, target)
		if endpoint == nil {
			return nil, "", errNoAvailableEndpoint
		}
		parsed, err := url.Parse(endpoint.URL)
		if err != nil {
			return nil, "", errNoAvailableEndpoint
		}
		return parsed, routeCounterKey(route, target) + "/" + endpoint.URL, nil
	default:
		return nil, "", errNoAvailableEndpoint
	}
}

func (r *Runtime) selectInstance(route Route, target Target, instances []*catalog.Instance) *catalog.Instance {
	if len(instances) == 0 {
		return nil
	}
	key := targetLoadCounterKey(route, target)
	switch target.LoadBalance {
	case LoadBalanceRandom:
		return instances[rand.Intn(len(instances))]
	case LoadBalanceWeighted:
		weights := make([]int, len(instances))
		for i, instance := range instances {
			weights[i] = instance.Weight
		}
		return instances[weightedIndex(weights, r.nextCounter(key))]
	default:
		return instances[int(r.nextCounter(key)%uint64(len(instances)))]
	}
}

func (r *Runtime) selectStaticEndpoint(route Route, target Target) *StaticEndpoint {
	if target.Static == nil || len(target.Static.Endpoints) == 0 {
		return nil
	}
	key := targetLoadCounterKey(route, target)
	index := 0
	switch target.LoadBalance {
	case LoadBalanceRandom:
		index = rand.Intn(len(target.Static.Endpoints))
	case LoadBalanceWeighted:
		weights := make([]int, len(target.Static.Endpoints))
		for i, endpoint := range target.Static.Endpoints {
			weights[i] = endpoint.Weight
		}
		index = weightedIndex(weights, r.nextCounter(key))
	default:
		index = int(r.nextCounter(key) % uint64(len(target.Static.Endpoints)))
	}
	endpoint := target.Static.Endpoints[index]
	return &endpoint
}

func weightedIndex(weights []int, counter uint64) int {
	total := 0
	for _, weight := range weights {
		if weight > 0 {
			total += weight
		}
	}
	if total == 0 {
		return int(counter % uint64(len(weights)))
	}
	selected := int(counter % uint64(total))
	for index, weight := range weights {
		if weight <= 0 {
			continue
		}
		if selected < weight {
			return index
		}
		selected -= weight
	}
	return len(weights) - 1
}

func (r *Runtime) nextCounter(key string) uint64 {
	value, _ := r.loadCounters.LoadOrStore(key, &atomic.Uint64{})
	return value.(*atomic.Uint64).Add(1) - 1
}

func routeCounterKey(route Route, target Target) string {
	return route.Namespace + "/" + route.ID + "/" + target.ID
}

func targetLoadCounterKey(route Route, target Target) string {
	return "target\x00" + route.Namespace + "\x00" + route.ID + "\x00" + target.ID
}

func releaseCounterKey(route Route) string {
	return "release\x00" + route.Namespace + "\x00" + route.ID
}

func (r *Runtime) selectTarget(request *http.Request, route Route) (Target, bool) {
	targets := make(map[string]Target, len(route.Targets))
	for _, target := range route.Targets {
		targets[target.ID] = target
	}
	userID, tenantID := r.trustedIdentities(request)
	if userID != "" {
		for _, beta := range route.Traffic.BetaTargets {
			if contains(beta.Users, userID) {
				target, ok := targets[beta.TargetID]
				return target, ok
			}
		}
	}
	if tenantID != "" {
		for _, beta := range route.Traffic.BetaTargets {
			if contains(beta.Tenants, tenantID) {
				target, ok := targets[beta.TargetID]
				return target, ok
			}
		}
	}
	if route.Traffic.Mode == TrafficBlueGreen {
		target, ok := targets[route.Traffic.ActiveTargetID]
		return target, ok
	}
	bucket := r.releaseBucket(route, request, userID, tenantID)
	progress := 0
	for _, weighted := range route.Traffic.WeightedTargets {
		progress += weighted.Weight
		if bucket < progress {
			target, ok := targets[weighted.TargetID]
			return target, ok
		}
	}
	target, ok := targets[route.Traffic.DefaultTargetID]
	return target, ok
}

func (r *Runtime) trustedIdentities(request *http.Request) (string, string) {
	if !r.isTrustedProxy(request.RemoteAddr) {
		return "", ""
	}
	return strings.TrimSpace(request.Header.Get(r.userHeader)), strings.TrimSpace(request.Header.Get(r.tenantHeader))
}

// stripUntrustedIdentityHeaders prevents a client from spoofing BETA identity
// to an upstream service when it did not arrive through a trusted identity proxy.
func (r *Runtime) stripUntrustedIdentityHeaders(outgoing, incoming *http.Request) {
	if r.isTrustedProxy(incoming.RemoteAddr) {
		return
	}
	outgoing.Header.Del(r.userHeader)
	outgoing.Header.Del(r.tenantHeader)
}

func (r *Runtime) isTrustedProxy(remoteAddress string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddress))
	if err != nil {
		host = strings.TrimSpace(remoteAddress)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, network := range r.trusted {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func (r *Runtime) selectionKey(request *http.Request, userID, tenantID string) string {
	if userID != "" {
		return "user:" + userID
	}
	if tenantID != "" {
		return "tenant:" + tenantID
	}
	if requestID := strings.TrimSpace(request.Header.Get("X-Request-ID")); requestID != "" {
		return "request:" + requestID
	}
	return ""
}

func (r *Runtime) releaseBucket(route Route, request *http.Request, userID, tenantID string) int {
	if key := r.selectionKey(request, userID, tenantID); key != "" {
		return selectionBucket(route, key)
	}
	// A shared ingress address is not a stable identity. Use a route-local
	// counter so a missing identity cannot collapse a configured release split
	// to 0/100 or 100/0 for all traffic through that ingress.
	return int(r.nextCounter(releaseCounterKey(route)) % 100)
}

func selectionBucket(route Route, key string) int {
	hash := fnv.New32a()
	_, _ = io.WriteString(hash, route.Namespace)
	_, _ = io.WriteString(hash, "\x00")
	_, _ = io.WriteString(hash, route.ID)
	_, _ = io.WriteString(hash, "\x00")
	_, _ = io.WriteString(hash, key)
	return int(hash.Sum32() % 100)
}

func matchRoute(routes []Route, request *http.Request) (Route, bool) {
	for _, route := range routes {
		if routeMatchesRequest(route, request) {
			return route, true
		}
	}
	return Route{}, false
}

func routeMatchesRequest(route Route, request *http.Request) bool {
	if !matchHosts(route.Match.Hosts, request.Host) {
		return false
	}
	if route.Match.Path != "" && request.URL.Path != route.Match.Path {
		return false
	}
	if route.Match.PathPrefix != "" && !matchesPathPrefix(request.URL.Path, route.Match.PathPrefix) {
		return false
	}
	if len(route.Match.Methods) > 0 && !contains(route.Match.Methods, strings.ToUpper(request.Method)) {
		return false
	}
	for name, value := range route.Match.Headers {
		if request.Header.Get(name) != value {
			return false
		}
	}
	return true
}

func matchHosts(patterns []string, requestHost string) bool {
	if len(patterns) == 0 {
		return true
	}
	host := strings.ToLower(strings.TrimSpace(requestHost))
	if split, _, err := net.SplitHostPort(host); err == nil {
		host = split
	}
	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == host {
			return true
		}
		if strings.HasPrefix(pattern, "*.") {
			suffix := strings.TrimPrefix(pattern, "*")
			if strings.HasSuffix(host, suffix) && len(host) > len(suffix) {
				return true
			}
		}
	}
	return false
}

func matchesPathPrefix(path, prefix string) bool {
	if prefix == "/" {
		return true
	}
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func applyRequestFilters(request *http.Request, filters []Filter) {
	for _, filter := range filters {
		switch filter.Type {
		case FilterStripPrefix:
			request.URL.Path = stripPrefix(request.URL.Path, filter.Parts)
			request.URL.RawPath = ""
		case FilterAddRequestHeader:
			request.Header.Add(filter.Name, filter.Value)
		}
	}
}

func applyResponseFilters(header http.Header, filters []Filter) {
	for _, filter := range filters {
		if filter.Type == FilterSetResponseHeader {
			header.Set(filter.Name, filter.Value)
		}
	}
}

func stripPrefix(path string, parts int) string {
	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(segments) <= parts {
		return "/"
	}
	remaining := strings.Join(segments[parts:], "/")
	if remaining == "" {
		return "/"
	}
	return "/" + remaining
}

func joinURLPath(basePath, requestPath string) string {
	if basePath == "" || basePath == "/" {
		return requestPath
	}
	if requestPath == "" || requestPath == "/" {
		return basePath
	}
	return strings.TrimRight(basePath, "/") + "/" + strings.TrimLeft(requestPath, "/")
}

func (r *Runtime) record(route Route, status int, message string) {
	key := route.Namespace + "\x00" + route.ID
	value, _ := r.metrics.LoadOrStore(key, &routeMetrics{})
	metrics := value.(*routeMetrics)
	metrics.requests.Add(1)
	if status >= http.StatusInternalServerError {
		metrics.errors.Add(1)
	}
	metrics.lastCode.Store(int64(status))
	metrics.mu.Lock()
	metrics.lastRequestAt = time.Now().UTC()
	metrics.lastError = message
	metrics.mu.Unlock()
}

func logGatewayAccess(route *Route, request *http.Request, status int, started time.Time, upstream, message string) {
	namespace := "-"
	routeID := "-"
	if route != nil {
		namespace = route.Namespace
		routeID = route.ID
	}
	path := "/"
	method := "-"
	if request != nil {
		method = request.Method
		if request.URL != nil && request.URL.EscapedPath() != "" {
			path = request.URL.EscapedPath()
		}
	}
	logger.Info("[Gateway] route=%s namespace=%s method=%s path=%q status=%d duration_ms=%d upstream=%q error=%q",
		routeID,
		namespace,
		method,
		path,
		status,
		time.Since(started).Milliseconds(),
		upstream,
		message,
	)
}

// Status reports local runtime state independently from the persisted route state.
func (r *Runtime) Status(namespace string) []RuntimeStatus {
	snapshot, _ := r.snapshot.Load().(*routeSnapshot)
	if snapshot == nil {
		return nil
	}
	namespace = strings.TrimSpace(namespace)
	statuses := make([]RuntimeStatus, 0, len(snapshot.all))
	for _, route := range snapshot.all {
		if namespace != "" && route.Namespace != namespace {
			continue
		}
		status := RuntimeStatus{Identity: route.Identity, Enabled: route.Enabled, SnapshotRevision: route.Revision}
		if value, ok := r.metrics.Load(route.Namespace + "\x00" + route.ID); ok {
			metrics := value.(*routeMetrics)
			status.Requests = metrics.requests.Load()
			status.Errors = metrics.errors.Load()
			status.LastStatus = int(metrics.lastCode.Load())
			metrics.mu.RLock()
			status.LastError = metrics.lastError
			status.LastRequestAt = metrics.lastRequestAt
			metrics.mu.RUnlock()
		}
		statuses = append(statuses, status)
	}
	return statuses
}

func parseTrustedCIDRs(values []string) ([]*net.IPNet, error) {
	result := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			return nil, fmt.Errorf("invalid gateway trusted_proxy_cidrs entry %q: %w", value, err)
		}
		result = append(result, network)
	}
	return result, nil
}

func headerOrDefault(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}

func writeRuntimeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(message + "\n"))
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (w *statusWriter) Status() int {
	return w.status
}

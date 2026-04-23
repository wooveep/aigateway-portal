package portal

import (
	"context"
	"net/url"
	"strings"

	clientK8s "higress-portal-backend/internal/client/k8s"
)

type gatewayAddressResolver struct {
	service     *Service
	ctx         context.Context
	routeLoaded bool
	routes      []clientK8s.GatewayIngressRoute
	routeErr    error
}

type resolvedGatewayTarget struct {
	URL        string
	HostHeader string
}

func (s *Service) newGatewayAddressResolver(ctx context.Context) *gatewayAddressResolver {
	if ctx == nil {
		ctx = context.Background()
	}
	return &gatewayAddressResolver{
		service: s,
		ctx:     ctx,
	}
}

func (r *gatewayAddressResolver) resolveURL(endpoint string, fallbackPath string, internal bool) string {
	return r.resolveTarget(endpoint, fallbackPath, internal).URL
}

func (r *gatewayAddressResolver) resolveTarget(endpoint string, fallbackPath string, internal bool) resolvedGatewayTarget {
	normalizedEndpoint := strings.TrimSpace(endpoint)
	if normalizedEndpoint == "" || normalizedEndpoint == "-" {
		normalizedEndpoint = strings.TrimSpace(fallbackPath)
	}
	if normalizedEndpoint == "" {
		return resolvedGatewayTarget{}
	}
	if isAbsoluteGatewayURL(normalizedEndpoint) {
		return resolvedGatewayTarget{URL: normalizedEndpoint}
	}
	normalizedPath := normalizeGatewayPath(normalizedEndpoint)
	if normalizedPath == "" {
		return resolvedGatewayTarget{}
	}

	var baseURL string
	hostHeader := ""
	if internal {
		baseURL = r.resolveInternalBaseURL()
		if route, ok := r.findBestRoute(normalizedPath); ok {
			hostHeader = strings.TrimSpace(route.Host)
		}
	} else {
		baseURL = r.resolvePublicBaseURL(normalizedPath)
	}
	if baseURL == "" {
		return resolvedGatewayTarget{
			URL:        normalizedPath,
			HostHeader: hostHeader,
		}
	}
	return resolvedGatewayTarget{
		URL:        joinGatewayBaseAndPath(baseURL, normalizedPath),
		HostHeader: hostHeader,
	}
}

func (r *gatewayAddressResolver) resolvePublicBaseURL(path string) string {
	if baseURL := strings.TrimRight(strings.TrimSpace(r.service.cfg.GatewayPublicBaseURL), "/"); baseURL != "" {
		return baseURL
	}

	if route, ok := r.findBestRoute(path); ok {
		if baseURL := routeToPublicBaseURL(route); baseURL != "" {
			return baseURL
		}
	}

	for _, route := range r.listGatewayRoutes() {
		if baseURL := routeToPublicBaseURL(route); baseURL != "" {
			return baseURL
		}
	}

	return normalizePublicBaseFallback(r.service.cfg.GatewayPublicHostFallback)
}

func (r *gatewayAddressResolver) resolveInternalBaseURL() string {
	if baseURL := strings.TrimRight(strings.TrimSpace(r.service.cfg.GatewayInternalBaseURL), "/"); baseURL != "" {
		return baseURL
	}
	serviceName := strings.TrimSpace(r.service.cfg.GatewayServiceName)
	namespace := strings.TrimSpace(r.service.cfg.K8sNamespace)
	if serviceName == "" || namespace == "" {
		return ""
	}
	return "http://" + serviceName + "." + namespace + ".svc.cluster.local"
}

func (r *gatewayAddressResolver) listGatewayRoutes() []clientK8s.GatewayIngressRoute {
	if r.routeLoaded {
		return r.routes
	}
	r.routeLoaded = true
	r.routes, r.routeErr = r.service.modelK8s.ListGatewayIngressRoutes(r.ctx)
	return r.routes
}

func (r *gatewayAddressResolver) findBestRoute(path string) (clientK8s.GatewayIngressRoute, bool) {
	best := clientK8s.GatewayIngressRoute{}
	bestScore := -1
	for _, route := range r.listGatewayRoutes() {
		if strings.TrimSpace(route.Host) == "" {
			continue
		}
		score := routeMatchScore(path, route.Path)
		if score > bestScore {
			best = route
			bestScore = score
		}
	}
	return best, bestScore >= 0
}

func routeMatchScore(targetPath string, routePath string) int {
	targetPath = normalizeGatewayPath(targetPath)
	routePath = normalizeGatewayPath(routePath)
	if routePath == "" {
		return 0
	}
	if targetPath == routePath {
		return len(routePath) + 10_000
	}
	if routePath == "/" {
		return 1
	}
	if strings.HasPrefix(targetPath, routePath+"/") {
		return len(routePath)
	}
	return -1
}

func routeToPublicBaseURL(route clientK8s.GatewayIngressRoute) string {
	host := strings.TrimSpace(route.Host)
	if host == "" {
		return ""
	}
	scheme := "http"
	if route.HasTLS {
		scheme = "https"
	}
	return scheme + "://" + host
}

func normalizePublicBaseFallback(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if isAbsoluteGatewayURL(value) {
		return strings.TrimRight(value, "/")
	}
	return strings.TrimRight("https://"+strings.TrimLeft(value, "/"), "/")
}

func normalizeGatewayPath(raw string) string {
	path := strings.TrimSpace(raw)
	if path == "" {
		return ""
	}
	if isAbsoluteGatewayURL(path) {
		parsed, err := url.Parse(path)
		if err != nil {
			return ""
		}
		return normalizeGatewayPath(parsed.Path)
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	return path
}

func joinGatewayBaseAndPath(baseURL string, path string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	path = normalizeGatewayPath(path)
	if baseURL == "" {
		return path
	}
	if path == "" || path == "/" {
		return baseURL
	}
	return baseURL + path
}

func isAbsoluteGatewayURL(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

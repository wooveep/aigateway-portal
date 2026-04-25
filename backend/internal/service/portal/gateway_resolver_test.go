package portal

import (
	"testing"

	clientK8s "higress-portal-backend/internal/client/k8s"
)

func TestRouteMatchScorePrefersLongestPrefix(t *testing.T) {
	target := "/v1/qwen/chat/completions"
	shortScore := routeMatchScore(target, "/v1")
	longScore := routeMatchScore(target, "/v1/qwen")
	missScore := routeMatchScore(target, "/other")

	if longScore <= shortScore {
		t.Fatalf("expected longer prefix to win, got short=%d long=%d", shortScore, longScore)
	}
	if missScore >= 0 {
		t.Fatalf("expected non-matching route to be negative, got %d", missScore)
	}
}

func TestRouteToPublicBaseURL(t *testing.T) {
	httpsRoute := clientK8s.GatewayIngressRoute{
		Host:   "gateway.example.com",
		Path:   "/v1/chat/completions",
		HasTLS: true,
	}
	httpRoute := clientK8s.GatewayIngressRoute{
		Host:   "gateway.internal.example.com",
		Path:   "/v1/chat/completions",
		HasTLS: false,
	}

	if got := routeToPublicBaseURL(httpsRoute); got != "https://gateway.example.com" {
		t.Fatalf("unexpected https base url: %s", got)
	}
	if got := routeToPublicBaseURL(httpRoute); got != "http://gateway.internal.example.com" {
		t.Fatalf("unexpected http base url: %s", got)
	}
}

func TestNormalizePublicBaseFallback(t *testing.T) {
	if got := normalizePublicBaseFallback("portal.example.com"); got != "https://portal.example.com" {
		t.Fatalf("unexpected fallback host normalization: %s", got)
	}
	if got := normalizePublicBaseFallback("https://portal.example.com/"); got != "https://portal.example.com" {
		t.Fatalf("unexpected fallback base normalization: %s", got)
	}
}

func TestResolveTargetInternalDoesNotReusePublicHostHeader(t *testing.T) {
	resolver := &gatewayAddressResolver{
		service: &Service{},
		routes: []clientK8s.GatewayIngressRoute{
			{Host: "console.aigateway.io", Path: "/"},
			{Host: "api.aigateway.io", Path: "/qwen"},
			{Host: "", Path: "/internal/ai-routes/qwen"},
		},
		routeLoaded: true,
	}
	resolver.service.cfg.K8sNamespace = "aigateway-system"
	resolver.service.cfg.GatewayServiceName = "aigateway-gateway"

	target := resolver.resolveTarget("/internal/ai-routes/qwen", "/v1/chat/completions", true)
	if got := target.URL; got != "http://aigateway-gateway.aigateway-system.svc.cluster.local/internal/ai-routes/qwen" {
		t.Fatalf("unexpected internal target url: %s", got)
	}
	if target.HostHeader != "" {
		t.Fatalf("expected empty internal host header, got %q", target.HostHeader)
	}
}

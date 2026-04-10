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

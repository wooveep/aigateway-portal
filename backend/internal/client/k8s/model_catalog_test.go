package k8s

import "testing"

func TestBuildGatewayEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		route    string
		protocol string
		fallback string
		want     string
	}{
		{
			name:     "route prefix with openai protocol uses completions suffix",
			route:    "/doubao",
			protocol: "openai/v1",
			want:     "/doubao/v1/chat/completions",
		},
		{
			name:     "absolute fallback url reuses path component",
			route:    "/doubao",
			protocol: "openai/v1",
			fallback: "https://example.com/custom/v1/chat/completions",
			want:     "/doubao/custom/v1/chat/completions",
		},
		{
			name:     "anthropic protocol uses messages suffix",
			route:    "/claude",
			protocol: "anthropic/v1/messages",
			want:     "/claude/v1/messages",
		},
		{
			name:     "non-openai route keeps prefix when no suffix",
			route:    "/rerank",
			protocol: "original",
			want:     "/rerank",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := buildGatewayEndpoint(tt.route, tt.protocol, tt.fallback); got != tt.want {
				t.Fatalf("buildGatewayEndpoint() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildInternalGatewayEndpoint(t *testing.T) {
	t.Parallel()

	got := buildInternalGatewayEndpoint(
		"/internal/ai-routes/doubao",
		"/doubao",
		"openai/v1",
		"/doubao/v1/chat/completions",
	)
	want := "/internal/ai-routes/doubao/v1/chat/completions"
	if got != want {
		t.Fatalf("buildInternalGatewayEndpoint() = %q, want %q", got, want)
	}
}

func TestNormalizeProtocol(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"":                    "openai/v1",
		"auto":                "openai/v1",
		"openai":              "openai/v1",
		"responses":           "openai/v1",
		"anthropic":           "anthropic/v1/messages",
		"claude":              "anthropic/v1/messages",
		"/v1/messages":        "anthropic/v1/messages",
		"original":            "original",
		"custom/protocol":     "custom/protocol",
	}

	for input, want := range tests {
		if got := normalizeProtocol(input); got != want {
			t.Fatalf("normalizeProtocol(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeDestinationRegistryName(t *testing.T) {
	t.Parallel()

	got := normalizeDestinationRegistryName("llm-doubao.internal.dns:443")
	if got != "llm-doubao.internal" {
		t.Fatalf("normalizeDestinationRegistryName() = %q, want %q", got, "llm-doubao.internal")
	}
}

func TestExtractRouteModelHeader(t *testing.T) {
	t.Parallel()

	exactObject := map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				"higress.io/exact-match-header-x-higress-llm-model": "doubao-seed-2-0-pro-260215",
			},
		},
	}
	if got := extractRouteModelHeader(exactObject); got != "doubao-seed-2-0-pro-260215" {
		t.Fatalf("extractRouteModelHeader() exact = %q, want %q", got, "doubao-seed-2-0-pro-260215")
	}

	prefixObject := map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				"higress.io/prefix-match-header-x-higress-llm-model": "doubao-seed-2-0-pro",
			},
		},
	}
	if got := extractRouteModelHeader(prefixObject); got != "doubao-seed-2-0-pro" {
		t.Fatalf("extractRouteModelHeader() prefix = %q, want %q", got, "doubao-seed-2-0-pro")
	}
}

func TestInferMappedModel(t *testing.T) {
	t.Parallel()

	if got := inferMappedModel(map[string]any{"*": "qwen3.6-plus"}); got != "qwen3.6-plus" {
		t.Fatalf("inferMappedModel() wildcard = %q, want %q", got, "qwen3.6-plus")
	}
	if got := inferMappedModel(map[string]any{"qwen": "qwen3.6-plus"}); got != "qwen3.6-plus" {
		t.Fatalf("inferMappedModel() single entry = %q, want %q", got, "qwen3.6-plus")
	}
	if got := inferMappedModel(map[string]any{"qwen": "", "other": "x"}); got != "" {
		t.Fatalf("inferMappedModel() multi entry = %q, want empty", got)
	}
}

func TestRouteBindingName(t *testing.T) {
	t.Parallel()

	if got := routeBindingName("ai-route-qwen.internal-internal"); got != "ai-route-qwen.internal" {
		t.Fatalf("routeBindingName() = %q, want %q", got, "ai-route-qwen.internal")
	}
	if got := routeBindingName("ai-route-qwen.internal"); got != "ai-route-qwen.internal" {
		t.Fatalf("routeBindingName() public = %q, want %q", got, "ai-route-qwen.internal")
	}
}

func TestIsInternalAIRoutePath(t *testing.T) {
	t.Parallel()

	if !isInternalAIRoutePath("/internal/ai-routes/ai-route-doubao.internal") {
		t.Fatalf("expected internal ai route path to be detected")
	}
	if isInternalAIRoutePath("/doubao") {
		t.Fatalf("expected public route path not to be treated as internal")
	}
}

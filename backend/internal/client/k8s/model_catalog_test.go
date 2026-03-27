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
			protocol: "openai",
			want:     "/doubao/v1/chat/completions",
		},
		{
			name:     "absolute fallback url reuses path component",
			route:    "/doubao",
			protocol: "openai",
			fallback: "https://example.com/custom/v1/chat/completions",
			want:     "/doubao/custom/v1/chat/completions",
		},
		{
			name:     "non-openai route keeps prefix when no suffix",
			route:    "/rerank",
			protocol: "rest",
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

func TestNormalizeDestinationRegistryName(t *testing.T) {
	t.Parallel()

	got := normalizeDestinationRegistryName("llm-doubao.internal.dns:443")
	if got != "llm-doubao.internal" {
		t.Fatalf("normalizeDestinationRegistryName() = %q, want %q", got, "llm-doubao.internal")
	}
}

func TestExtractExactModelHeader(t *testing.T) {
	t.Parallel()

	object := map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				"higress.io/exact-match-header-x-higress-llm-model": "doubao-seed-2-0-pro-260215",
			},
		},
	}
	got := extractExactModelHeader(object)
	if got != "doubao-seed-2-0-pro-260215" {
		t.Fatalf("extractExactModelHeader() = %q, want %q", got, "doubao-seed-2-0-pro-260215")
	}
}

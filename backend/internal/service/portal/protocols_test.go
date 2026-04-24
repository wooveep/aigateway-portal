package portal

import "testing"

func TestNormalizePublishedBindingProtocol(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"":                publishedBindingProtocolOpenAI,
		"auto":            publishedBindingProtocolOpenAI,
		"openai":          publishedBindingProtocolOpenAI,
		"responses":       publishedBindingProtocolOpenAI,
		"anthropic":       publishedBindingProtocolAnthropic,
		"claude":          publishedBindingProtocolAnthropic,
		"/v1/messages":    publishedBindingProtocolAnthropic,
		"original":        publishedBindingProtocolOriginal,
		"custom/protocol": "custom/protocol",
	}

	for input, want := range tests {
		if got := normalizePublishedBindingProtocol(input); got != want {
			t.Fatalf("normalizePublishedBindingProtocol(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizePublishedBindingEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		endpoint string
		protocol string
		want     string
	}{
		{name: "default openai", endpoint: "", protocol: "", want: publishedBindingOpenAIEndpoint},
		{name: "default anthropic", endpoint: "", protocol: "anthropic", want: publishedBindingAnthropicEndpoint},
		{name: "responses keeps responses endpoint", endpoint: "", protocol: "responses", want: publishedBindingResponsesEndpoint},
		{name: "historical chat path is normalized", endpoint: "/v1/chatcompletions", protocol: "openai", want: publishedBindingOpenAIEndpoint},
		{name: "original keeps placeholder", endpoint: "", protocol: "original", want: "-"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizePublishedBindingEndpoint(tt.endpoint, tt.protocol); got != tt.want {
				t.Fatalf("normalizePublishedBindingEndpoint(%q, %q) = %q, want %q", tt.endpoint, tt.protocol, got, tt.want)
			}
		})
	}
}


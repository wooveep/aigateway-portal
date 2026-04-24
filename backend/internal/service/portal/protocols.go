package portal

import (
	"net/url"
	"strings"
)

const (
	publishedBindingProtocolOpenAI    = "openai/v1"
	publishedBindingProtocolAnthropic = "anthropic/v1/messages"
	publishedBindingProtocolOriginal  = "original"

	publishedBindingOpenAIEndpoint    = "/v1/chat/completions"
	publishedBindingAnthropicEndpoint = "/v1/messages"
	publishedBindingResponsesEndpoint = "/v1/responses"
)

func normalizePublishedBindingProtocol(protocol string) string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "", "auto", "openai", "openai/v1", "openai/v1/chatcompletions", "openai/v1/chat/completions", "responses", "openai/v1/responses":
		return publishedBindingProtocolOpenAI
	case "anthropic", "claude", "anthropic/v1/messages", "/v1/messages":
		return publishedBindingProtocolAnthropic
	case "original":
		return publishedBindingProtocolOriginal
	default:
		return strings.TrimSpace(protocol)
	}
}

func normalizePublishedBindingEndpoint(endpoint, protocol string) string {
	rawProtocol := strings.ToLower(strings.TrimSpace(protocol))
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "-" {
		endpoint = ""
	}
	if endpoint != "" {
		if parsed, err := url.Parse(endpoint); err == nil && parsed.Path != "" {
			endpoint = parsed.Path
		}
		if !strings.HasPrefix(endpoint, "/") {
			endpoint = "/" + endpoint
		}
		if endpoint == "/v1/chatcompletions" {
			return publishedBindingOpenAIEndpoint
		}
		return endpoint
	}
	if strings.Contains(rawProtocol, "responses") {
		return publishedBindingResponsesEndpoint
	}
	switch normalizePublishedBindingProtocol(protocol) {
	case publishedBindingProtocolOpenAI:
		return publishedBindingOpenAIEndpoint
	case publishedBindingProtocolAnthropic:
		return publishedBindingAnthropicEndpoint
	default:
		return "-"
	}
}


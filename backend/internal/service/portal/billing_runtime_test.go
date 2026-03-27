package portal

import (
	"testing"
	"time"
)

func TestParseBillingUsageEventPayload(t *testing.T) {
	occurredAt := time.Date(2026, time.March, 25, 11, 12, 13, 0, time.UTC)
	payload, err := parseBillingUsageEventPayload("1-0", map[string]any{
		"event_id":         "evt-1",
		"request_id":       "req-1",
		"consumer_name":    "consumer-a",
		"route_name":       "route-a",
		"api_key_id":       "KEY123",
		"model_id":         "qwen-plus",
		"input_tokens":     "120",
		"output_tokens":    "80",
		"cost_micro_yuan":  "308",
		"price_version_id": "9",
		"occurred_at":      occurredAt.Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatalf("parse billing usage event payload failed: %v", err)
	}
	if payload.APIKeyID != "KEY123" {
		t.Fatalf("unexpected api key id: %s", payload.APIKeyID)
	}
	if payload.TotalTokens != 200 {
		t.Fatalf("unexpected total tokens: %d", payload.TotalTokens)
	}
	if !payload.OccurredAt.Equal(occurredAt) {
		t.Fatalf("unexpected occurred at: %s", payload.OccurredAt)
	}
}

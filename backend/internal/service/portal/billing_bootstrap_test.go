package portal

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidateBillingBackfillSummary(t *testing.T) {
	t.Parallel()

	valid := billingBackfillSummary{
		LegacyModelCount:              2,
		MatchedBillingModelCount:      2,
		LegacyPricedModelCount:        2,
		MatchedBillingPriceModelCount: 2,
		LegacyRechargeCount:           3,
		LedgerRechargeCount:           3,
		LegacyRechargeTotalMicroYuan:  300000000,
		LedgerRechargeTotalMicroYuan:  300000000,
		LegacyUsageCount:              4,
		LedgerUsageCount:              4,
		LegacyUsageTotalMicroYuan:     120000000,
		LedgerUsageTotalMicroYuan:     120000000,
		LedgerNetTotalMicroYuan:       180000000,
		WalletNetTotalMicroYuan:       180000000,
	}

	tests := []struct {
		name    string
		summary billingBackfillSummary
		wantErr bool
	}{
		{
			name:    "valid summary",
			summary: valid,
			wantErr: false,
		},
		{
			name: "usage total mismatch",
			summary: func() billingBackfillSummary {
				item := valid
				item.LedgerUsageTotalMicroYuan = 119000000
				return item
			}(),
			wantErr: true,
		},
		{
			name: "wallet total mismatch",
			summary: func() billingBackfillSummary {
				item := valid
				item.WalletNetTotalMicroYuan = 179000000
				return item
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateBillingBackfillSummary(tt.summary)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestProcessBillingUsageEventSkipsAdministrator(t *testing.T) {
	t.Parallel()

	svc := &Service{}
	err := svc.processBillingUsageEvent(context.Background(), "1743050000-0", map[string]any{
		"event_id":        "evt-admin",
		"consumer_name":   "administrator",
		"model_id":        "qwen-plus",
		"request_status":  "success",
		"usage_status":    "parsed",
		"http_status":     "200",
		"cost_micro_yuan": "1000",
		"occurred_at":     time.Now().UTC().Format(time.RFC3339Nano),
	})
	require.NoError(t, err)
}

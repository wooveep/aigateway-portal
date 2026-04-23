package portal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/stretchr/testify/require"

	"higress-portal-backend/internal/config"
)

func TestBillingUsageEventPersistsWithoutPrometheus(t *testing.T) {
	dsn := os.Getenv("PORTAL_BILLING_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("PORTAL_BILLING_POSTGRES_DSN is not set")
	}

	cfg := config.Config{
		DBDriver:            "postgres",
		DBDSN:               dsn,
		SessionSecret:       "test-secret",
		SessionCookieName:   "portal-test-session",
		UsageSyncEnabled:    true,
		KeyAuthSyncEnabled:  false,
		BillingSyncEnabled:  false,
		CorePrometheusURL:   "",
		BillingSyncInterval: 15 * time.Second,
	}
	service, err := New(cfg)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, service.Close(context.Background()))
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.StartUsageSync(ctx)

	eventID := "evt-no-prometheus"
	occurredAt := time.Date(2026, time.March, 27, 10, 11, 12, 0, time.UTC)
	require.NoError(t, service.processBillingUsageEvent(ctx, "1-0", map[string]any{
		"event_id":         eventID,
		"request_id":       "req-no-prometheus",
		"consumer_name":    "integration-user",
		"route_name":       "ai-route",
		"request_path":     "/v1/chat/completions",
		"request_kind":     "chat.completions",
		"model_id":         "qwen-plus",
		"request_status":   "success",
		"usage_status":     "parsed",
		"http_status":      200,
		"input_tokens":     120,
		"output_tokens":    80,
		"total_tokens":     200,
		"cost_micro_yuan":  308,
		"price_version_id": 9,
		"occurred_at":      occurredAt.Format(time.RFC3339Nano),
	}))

	usageCount, err := service.db.GetValue(ctx, "SELECT COUNT(1) FROM billing_usage_event WHERE event_id = ?", eventID)
	require.NoError(t, err)
	require.EqualValues(t, 1, usageCount.Int64())

	txCount, err := service.db.GetValue(ctx, `
		SELECT COUNT(1)
		FROM billing_transaction
		WHERE source_type = 'billing_usage_event' AND source_id = ?`, eventID)
	require.NoError(t, err)
	require.EqualValues(t, 1, txCount.Int64())

	walletValue, err := service.db.GetValue(ctx, `
		SELECT available_micro_yuan
		FROM billing_wallet
		WHERE consumer_name = ?`, "integration-user")
	require.NoError(t, err)
	require.EqualValues(t, -308, walletValue.Int64())
}

func TestBootstrapLegacyBillingTransactionsRefreshesExistingUsageRows(t *testing.T) {
	dsn := os.Getenv("PORTAL_BILLING_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("PORTAL_BILLING_POSTGRES_DSN is not set")
	}

	db, err := gdb.New(gdb.ConfigNode{
		Type: "pgsql",
		Link: dsn,
	})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close(context.Background()))
	}()

	svc := &Service{db: db}
	ctx := context.Background()
	consumerName := fmt.Sprintf("backfill-refresh-%d", time.Now().UnixNano())
	billingDate := time.Now().UTC().Format("2006-01-02")
	occurredAt := time.Now().UTC().Truncate(time.Second)
	modelID := "backfill-refresh-model"

	defer func() {
		_, delErr := db.Exec(ctx, `DELETE FROM billing_transaction WHERE consumer_name = ?`, consumerName)
		require.NoError(t, delErr)
		_, delErr = db.Exec(ctx, `DELETE FROM portal_usage_daily WHERE consumer_name = ?`, consumerName)
		require.NoError(t, delErr)
	}()

	_, err = db.Exec(ctx, `
		INSERT INTO portal_usage_daily
		(billing_date, consumer_name, model_name, request_count, input_tokens, output_tokens, total_tokens, cost_amount, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		billingDate, consumerName, modelID, 1, 120, 80, 200, 1.811000, occurredAt, occurredAt,
	)
	require.NoError(t, err)

	usageIDValue, err := db.GetValue(ctx, `
		SELECT id
		FROM portal_usage_daily
		WHERE consumer_name = ?
		ORDER BY id DESC
		LIMIT 1`, consumerName)
	require.NoError(t, err)
	usageID := usageIDValue.Int64()
	sourceID := strconv.FormatInt(usageID, 10)

	_, err = db.Exec(ctx, `
		INSERT INTO billing_transaction
		(tx_id, consumer_name, tx_type, amount_micro_yuan, currency, source_type, source_id, model_id, input_tokens, output_tokens, occurred_at, created_at)
		VALUES (?, ?, 'consume', ?, 'CNY', 'portal_usage_daily', ?, ?, ?, ?, ?, ?)`,
		legacyUsageTransactionID(sourceID), consumerName, -837000, sourceID, modelID, 40, 30, occurredAt, occurredAt,
	)
	require.NoError(t, err)

	require.NoError(t, svc.bootstrapLegacyBillingTransactions(ctx))

	record, err := db.GetOne(ctx, `
		SELECT amount_micro_yuan, input_tokens, output_tokens
		FROM billing_transaction
		WHERE source_type = 'portal_usage_daily' AND source_id = ?`, sourceID)
	require.NoError(t, err)
	require.EqualValues(t, -1811000, record["amount_micro_yuan"].Int64())
	require.EqualValues(t, 120, record["input_tokens"].Int64())
	require.EqualValues(t, 80, record["output_tokens"].Int64())
}

func legacyUsageTransactionID(sourceID string) string {
	sum := sha256.Sum256([]byte("portal_usage_daily:" + sourceID))
	return "u" + hex.EncodeToString(sum[:])[:32]
}

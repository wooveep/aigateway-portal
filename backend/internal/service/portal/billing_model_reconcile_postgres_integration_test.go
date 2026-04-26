//go:build integration
// +build integration

package portal

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	clientK8s "higress-portal-backend/internal/client/k8s"
	"higress-portal-backend/internal/config"
	"higress-portal-backend/schema/shared"
)

func TestSyncBillingModelCatalogCanonicalizesPublishedBindingLegacyPricingOnPostgres(t *testing.T) {
	ctx := context.Background()
	dsn := startPortalCompatPostgres(t, ctx, "portal_billing_reconcile_pg_it")

	rawDB, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	defer rawDB.Close()
	require.Eventually(t, func() bool {
		return rawDB.PingContext(ctx) == nil
	}, 60*time.Second, 500*time.Millisecond)
	require.NoError(t, shared.ApplyToSQLWithDriver(ctx, rawDB, "postgres"))

	db, err := gdb.New(gdb.ConfigNode{Type: "pgsql", Link: gfPostgresLink(dsn)})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close(ctx))
	}()

	svc := &Service{
		cfg:      config.Config{DBDriver: "postgres"},
		db:       db,
		modelK8s: clientK8s.New(config.Config{}),
	}
	require.NoError(t, svc.runMigrations(ctx))

	var priceVersionTableCount int
	require.NoError(t, rawDB.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM information_schema.tables
		WHERE table_schema = current_schema()
		  AND table_name = 'portal_model_binding_price_version'`,
	).Scan(&priceVersionTableCount))
	require.Equal(t, 1, priceVersionTableCount)

	_, err = rawDB.ExecContext(ctx, `
		INSERT INTO portal_model_asset (
			asset_id, canonical_name, display_name, intro, tags_json, modalities_json, features_json, request_kinds_json
		) VALUES (
			'asset-qwen36', 'qwen3.6-plus', 'Qwen 3.6 Plus', 'legacy priced model',
			'["chat"]', '["text"]', '["reasoning"]', '["chat_completions"]'
		)`)
	require.NoError(t, err)
	_, err = rawDB.ExecContext(ctx, `
		INSERT INTO portal_model_binding (
			binding_id, asset_id, model_id, provider_name, target_model, protocol, endpoint, pricing_json, rpm, tpm, context_window, status, published_at
		) VALUES (
			'binding-qwen36', 'asset-qwen36', 'qwen3.6-plus', 'aliyun', 'qwen3.6-plus', 'openai/v1', '/v1/chat/completions',
			'{"currency":"CNY","inputCostPerToken":1,"outputCostPerToken":2}', 60, 120000, 8192, 'published', CURRENT_TIMESTAMP
		)`)
	require.NoError(t, err)
	_, err = rawDB.ExecContext(ctx, `
		INSERT INTO portal_model_binding_price_version (
			asset_id, binding_id, status, active, effective_from, pricing_json
		) VALUES (
			'asset-qwen36', 'binding-qwen36', 'active', TRUE, CURRENT_TIMESTAMP,
			'{"currency":"CNY","inputCostPerToken":1,"outputCostPerToken":2}'
		)`)
	require.NoError(t, err)
	_, err = rawDB.ExecContext(ctx, `
		INSERT INTO billing_model_catalog (model_id, name, vendor, capability, endpoint, sdk, summary, status)
		VALUES ('stale-model', 'stale-model', 'legacy', 'legacy', '-', 'openai/v1', '-', 'active')`)
	require.NoError(t, err)
	_, err = rawDB.ExecContext(ctx, `
		INSERT INTO billing_model_price_version (
			model_id, currency,
			input_price_micro_yuan_per_token, output_price_micro_yuan_per_token,
			input_price_per_1k_micro_yuan, output_price_per_1k_micro_yuan,
			effective_from, status
		) VALUES (
			'stale-model', 'CNY',
			123, 456,
			123000, 456000,
			CURRENT_TIMESTAMP, 'active'
		)`)
	require.NoError(t, err)

	require.NoError(t, svc.syncBillingModelCatalog(ctx))

	var bindingPricingJSON string
	require.NoError(t, rawDB.QueryRowContext(ctx, `
		SELECT pricing_json
		FROM portal_model_binding
		WHERE asset_id = 'asset-qwen36' AND binding_id = 'binding-qwen36'`,
	).Scan(&bindingPricingJSON))
	require.JSONEq(t, `{"currency":"CNY","inputCostPerMillionTokens":1,"outputCostPerMillionTokens":2}`, bindingPricingJSON)

	var versionPricingJSON string
	require.NoError(t, rawDB.QueryRowContext(ctx, `
		SELECT pricing_json
		FROM portal_model_binding_price_version
		WHERE asset_id = 'asset-qwen36' AND binding_id = 'binding-qwen36' AND active = TRUE`,
	).Scan(&versionPricingJSON))
	require.JSONEq(t, `{"currency":"CNY","inputCostPerMillionTokens":1,"outputCostPerMillionTokens":2}`, versionPricingJSON)

	var inputMicro, outputMicro, inputPer1K, outputPer1K int64
	require.NoError(t, rawDB.QueryRowContext(ctx, `
		SELECT
			input_price_micro_yuan_per_token,
			output_price_micro_yuan_per_token,
			input_price_per_1k_micro_yuan,
			output_price_per_1k_micro_yuan
		FROM billing_model_price_version
		WHERE model_id = 'qwen3.6-plus'
		  AND status = 'active'
		  AND effective_to IS NULL
		ORDER BY id DESC
		LIMIT 1`,
	).Scan(&inputMicro, &outputMicro, &inputPer1K, &outputPer1K))
	require.EqualValues(t, 1, inputMicro)
	require.EqualValues(t, 2, outputMicro)
	require.EqualValues(t, 1000, inputPer1K)
	require.EqualValues(t, 2000, outputPer1K)

	var staleCatalogStatus string
	require.NoError(t, rawDB.QueryRowContext(ctx, `
		SELECT status
		FROM billing_model_catalog
		WHERE model_id = 'stale-model'`,
	).Scan(&staleCatalogStatus))
	require.Equal(t, "disabled", staleCatalogStatus)

	var staleActiveCount int
	require.NoError(t, rawDB.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM billing_model_price_version
		WHERE model_id = 'stale-model'
		  AND status = 'active'
		  AND effective_to IS NULL`,
	).Scan(&staleActiveCount))
	require.Equal(t, 0, staleActiveCount)
}

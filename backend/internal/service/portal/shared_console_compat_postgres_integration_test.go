package portal

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	"higress-portal-backend/internal/config"
	"higress-portal-backend/schema/shared"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPortalReadsConsoleWrittenSharedSchemaRowsOnPostgres(t *testing.T) {
	ctx := context.Background()
	dsn := startPortalCompatPostgres(t, ctx, "portal_console_compat_pg_it")

	rawDB, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	defer rawDB.Close()
	require.Eventually(t, func() bool {
		return rawDB.PingContext(ctx) == nil
	}, 60*time.Second, 500*time.Millisecond)

	require.NoError(t, shared.ApplyToSQLWithDriver(ctx, rawDB, "postgres"))
	_, err = rawDB.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS portal_model_binding_price_version (
			version_id BIGSERIAL PRIMARY KEY,
			asset_id VARCHAR(255) NOT NULL,
			binding_id VARCHAR(255) NOT NULL,
			status VARCHAR(32) NOT NULL DEFAULT 'active',
			active BOOLEAN NOT NULL DEFAULT FALSE,
			effective_from TIMESTAMP NULL,
			effective_to TIMESTAMP NULL,
			pricing_json TEXT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`)
	require.NoError(t, err)

	_, err = rawDB.ExecContext(ctx, `
		INSERT INTO org_department (department_id, name, parent_department_id, admin_consumer_name, path, level, sort_order, status)
		VALUES
			('root', 'Root', NULL, NULL, 'Root', 0, 0, 'active'),
			('dept-eng', 'Engineering', 'root', 'demo', 'Root / Engineering', 1, 0, 'active')`)
	require.NoError(t, err)
	_, err = rawDB.ExecContext(ctx, `
		INSERT INTO portal_user (
			consumer_name, display_name, email, password_hash, status, source, user_level, is_deleted
		) VALUES
			('demo', 'Demo', 'demo@example.com', 'hash', 'active', 'console', 'pro', FALSE)`)
	require.NoError(t, err)
	_, err = rawDB.ExecContext(ctx, `
		INSERT INTO org_account_membership (consumer_name, department_id, parent_consumer_name)
		VALUES ('demo', 'dept-eng', NULL)`)
	require.NoError(t, err)
	_, err = rawDB.ExecContext(ctx, `
		INSERT INTO quota_policy_user (
			consumer_name, limit_total_micro_yuan, limit_5h_micro_yuan, limit_daily_micro_yuan,
			daily_reset_mode, daily_reset_time, limit_weekly_micro_yuan, limit_monthly_micro_yuan
		) VALUES ('demo', 1000, 200, 300, 'fixed', '08:00', 400, 500)`)
	require.NoError(t, err)
	_, err = rawDB.ExecContext(ctx, `
		INSERT INTO portal_model_asset (
			asset_id, canonical_name, display_name, intro, tags_json, modalities_json, features_json, request_kinds_json
		) VALUES (
			'qwen-plus', 'qwen-plus', 'Qwen Plus', 'test model', '["chat"]', '["text"]', '["reasoning"]', '["chat_completions"]'
		)`)
	require.NoError(t, err)
	_, err = rawDB.ExecContext(ctx, `
		INSERT INTO portal_model_binding (
			binding_id, asset_id, model_id, provider_name, target_model, protocol, endpoint, pricing_json, rpm, tpm, context_window, status, published_at
		) VALUES (
			'binding-qwen-plus', 'qwen-plus', 'qwen-plus-model', 'aliyun', 'qwen-plus', 'openai/v1', '/v1/chat/completions',
			'{"currency":"CNY"}', 60, 120000, 8192, 'published', CURRENT_TIMESTAMP
		)`)
	require.NoError(t, err)
	_, err = rawDB.ExecContext(ctx, `
		INSERT INTO portal_model_binding_price_version (
			asset_id, binding_id, status, active, effective_from, pricing_json
		) VALUES (
			'qwen-plus', 'binding-qwen-plus', 'active', TRUE, CURRENT_TIMESTAMP,
			'{"currency":"CNY","inputCostPerToken":100,"outputCostPerToken":200}'
		)`)
	require.NoError(t, err)
	_, err = rawDB.ExecContext(ctx, `
		INSERT INTO asset_grant (asset_type, asset_id, subject_type, subject_id)
		VALUES ('model_binding', 'binding-qwen-plus', 'consumer', 'demo')`)
	require.NoError(t, err)

	db, err := gdb.New(gdb.ConfigNode{Type: "pgsql", Link: gfPostgresLink(dsn)})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close(ctx))
	}()

	svc := &Service{
		cfg: config.Config{
			DBDriver:             "postgres",
			GatewayPublicBaseURL: "http://gateway.example.com",
		},
		db: db,
	}

	user, err := svc.ResolveScopeUser(ctx, "demo")
	require.NoError(t, err)
	require.Equal(t, "dept-eng", user.DepartmentID)
	require.Equal(t, "Engineering", user.DepartmentName)

	models, err := svc.ListModels(ctx, user)
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "qwen-plus-model", models[0].ID)
	require.Equal(t, int64(60), models[0].Limits.RPM)
}

func startPortalCompatPostgres(t *testing.T, ctx context.Context, databaseName string) string {
	t.Helper()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:16-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_PASSWORD": "postgres",
				"POSTGRES_USER":     "postgres",
				"POSTGRES_DB":       databaseName,
			},
			WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(90 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)
	return fmt.Sprintf("host=%s port=%s user=postgres password=postgres dbname=%s sslmode=disable", host, port.Port(), databaseName)
}

func gfPostgresLink(dsn string) string {
	parts := map[string]string{}
	for _, field := range strings.Fields(dsn) {
		kv := strings.SplitN(field, "=", 2)
		if len(kv) == 2 {
			parts[kv[0]] = kv[1]
		}
	}
	return fmt.Sprintf(
		"pgsql:%s:%s@tcp(%s:%s)/%s?sslmode=%s",
		parts["user"],
		parts["password"],
		parts["host"],
		parts["port"],
		parts["dbname"],
		defaultString(parts["sslmode"], "disable"),
	)
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

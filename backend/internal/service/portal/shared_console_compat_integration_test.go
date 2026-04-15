package portal

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/stretchr/testify/require"

	"higress-portal-backend/internal/config"
	"higress-portal-backend/schema/shared"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPortalReadsConsoleWrittenSharedSchemaRows(t *testing.T) {
	ctx := context.Background()
	dsn := startPortalCompatMySQL(t, ctx, "portal_console_compat_it")

	rawDB, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	defer rawDB.Close()
	require.Eventually(t, func() bool {
		return rawDB.PingContext(ctx) == nil
	}, 30*time.Second, 500*time.Millisecond)

	require.NoError(t, shared.ApplyToSQL(ctx, rawDB))
	_, err = rawDB.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS billing_model_price_version (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			model_id VARCHAR(128) NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'active',
			effective_to DATETIME NULL,
			input_price_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			output_price_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			input_request_price_micro_yuan BIGINT NOT NULL DEFAULT 0,
			cache_creation_input_token_price_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			cache_creation_input_token_price_above_1hr_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			cache_read_input_token_price_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			input_token_price_above_200k_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			output_token_price_above_200k_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			cache_creation_input_token_price_above_200k_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			cache_read_input_token_price_above_200k_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			output_image_price_micro_yuan BIGINT NOT NULL DEFAULT 0,
			output_image_token_price_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			input_image_price_micro_yuan BIGINT NOT NULL DEFAULT 0,
			input_image_token_price_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			supports_prompt_caching TINYINT(1) NOT NULL DEFAULT 0
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
			('demo', 'Demo', 'demo@example.com', 'hash', 'active', 'console', 'pro', 0)`)
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
			'qwen-plus', 'qwen-plus', 'Qwen Plus', 'test model', '[\"chat\"]', '[\"text\"]', '[\"reasoning\"]', '[\"chat_completions\"]'
		)`)
	require.NoError(t, err)
	_, err = rawDB.ExecContext(ctx, `
		INSERT INTO portal_model_binding (
			binding_id, asset_id, model_id, provider_name, target_model, protocol, endpoint, pricing_json, rpm, tpm, context_window, status, published_at
		) VALUES (
			'binding-qwen-plus', 'qwen-plus', 'qwen-plus-model', 'aliyun', 'qwen-plus', 'openai/v1', '/v1/chat/completions',
			'{\"currency\":\"CNY\"}', 60, 120000, 8192, 'published', CURRENT_TIMESTAMP
		)`)
	require.NoError(t, err)
	_, err = rawDB.ExecContext(ctx, `
		INSERT INTO billing_model_price_version (
			model_id, status, effective_to, input_price_per_1k_micro_yuan, output_price_per_1k_micro_yuan, supports_prompt_caching
		) VALUES ('qwen-plus-model', 'active', NULL, 100, 200, 0)`)
	require.NoError(t, err)
	_, err = rawDB.ExecContext(ctx, `
		INSERT INTO asset_grant (asset_type, asset_id, subject_type, subject_id)
		VALUES
			('model_binding', 'binding-qwen-plus', 'consumer', 'demo'),
			('agent_catalog', 'agent-demo', 'consumer', 'demo')`)
	require.NoError(t, err)
	_, err = rawDB.ExecContext(ctx, `
		INSERT INTO portal_agent_catalog (
			agent_id, canonical_name, display_name, intro, description, icon_url, tags_json,
			mcp_server_name, tool_count, transport_types_json, resource_summary, prompt_summary, status, published_at
		) VALUES (
			'agent-demo', 'agent-demo', 'Agent Demo', 'agent intro', 'agent description', '',
			'[\"assistant\"]', 'mcp-demo', 2, '[\"http\",\"sse\"]', 'resource summary', 'prompt summary', 'published', CURRENT_TIMESTAMP
		)`)
	require.NoError(t, err)

	db, err := gdb.New(gdb.ConfigNode{Type: "mysql", Link: dsn})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close(ctx))
	}()

	svc := &Service{
		cfg: config.Config{
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

	agents, err := svc.ListAgents(ctx, user)
	require.NoError(t, err)
	require.Len(t, agents, 1)
	require.Equal(t, "agent-demo", agents[0].ID)
	require.EqualValues(t, 2, agents[0].ToolCount)

	policy, err := svc.loadUserQuotaPolicy(ctx, "demo")
	require.NoError(t, err)
	require.EqualValues(t, 1000, policy.LimitTotalMicroYuan)
	require.EqualValues(t, 500, policy.LimitMonthlyMicroYuan)
}

func startPortalCompatMySQL(t *testing.T, ctx context.Context, databaseName string) string {
	t.Helper()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "mysql:8.4",
			ExposedPorts: []string{"3306/tcp"},
			Env: map[string]string{
				"MYSQL_ROOT_PASSWORD": "root",
				"MYSQL_DATABASE":      databaseName,
			},
			WaitingFor: wait.ForListeningPort("3306/tcp").WithStartupTimeout(90 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "3306/tcp")
	require.NoError(t, err)
	return fmt.Sprintf("root:root@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&loc=UTC", host, port.Port(), databaseName)
}

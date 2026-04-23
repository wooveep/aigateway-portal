package portal

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	clientK8s "higress-portal-backend/internal/client/k8s"
	"higress-portal-backend/internal/config"
	"higress-portal-backend/internal/consts"
	"higress-portal-backend/internal/model"
	"higress-portal-backend/schema/shared"
)

type Service struct {
	cfg             config.Config
	db              gdb.DB
	httpClient      *http.Client
	streamClient    *http.Client
	modelK8s        *clientK8s.Client
	billingNodeName string
	usageReadSyncMu sync.Mutex
	usageReadSyncAt time.Time
}

func New(cfg config.Config) (*Service, error) {
	node := gdb.ConfigNode{
		Type:             "pgsql",
		Link:             goFramePostgresLink(cfg.DBDSN),
		MaxOpenConnCount: 20,
		MaxIdleConnCount: 10,
		MaxConnLifeTime:  30 * time.Minute,
	}
	db, err := gdb.New(node)
	if err != nil {
		return nil, gerror.Wrap(err, "initialize postgres failed")
	}
	if err = db.PingMaster(); err != nil {
		return nil, gerror.Wrap(err, "connect postgres failed")
	}

	s := &Service{
		cfg: cfg,
		db:  db,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		streamClient:    &http.Client{},
		modelK8s:        clientK8s.New(cfg),
		billingNodeName: "portal-" + randomString(8),
	}
	if initErr := s.modelK8s.InitError(); initErr != nil {
		g.Log().Warningf(context.Background(), "portal k8s model catalog init failed: %v", initErr)
	}

	if err = s.runMigrations(context.Background()); err != nil {
		return nil, err
	}
	if err = s.seedBootstrapData(context.Background()); err != nil {
		return nil, err
	}
	if err = s.bootstrapBillingState(context.Background()); err != nil {
		return nil, err
	}
	if err = s.backfillUsageDepartmentSnapshots(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Service) Close(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	return s.db.Close(ctx)
}

func (s *Service) runMigrations(ctx context.Context) error {
	if err := shared.ApplyToGDBWithDriver(ctx, s.db, s.cfg.DBDriver); err != nil {
		return err
	}

	migrations := s.portalMigrationDDLs()

	for _, ddl := range migrations {
		if _, err := s.db.Exec(ctx, ddl); err != nil {
			return gerror.Wrap(err, "migration failed")
		}
	}
	if err := s.ensurePortalAPIKeyColumns(ctx); err != nil {
		return err
	}
	if err := s.ensureBillingTransactionColumns(ctx); err != nil {
		return err
	}
	if err := s.ensureBillingUsageEventColumns(ctx); err != nil {
		return err
	}
	if err := s.ensurePortalUsageDailyColumns(ctx); err != nil {
		return err
	}
	if err := s.ensurePortalModelAssetColumns(ctx); err != nil {
		return err
	}
	if err := s.ensurePortalModelBindingColumns(ctx); err != nil {
		return err
	}
	if err := s.ensureBillingModelPriceColumns(ctx); err != nil {
		return err
	}
	if err := s.ensureOrganizationSchema(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Service) ensurePortalUserLevelColumn(ctx context.Context) error {
	return s.ensureTableColumn(ctx, "portal_user", "user_level",
		`ALTER TABLE portal_user ADD COLUMN user_level VARCHAR(16) NOT NULL DEFAULT 'normal'`)
}

func (s *Service) ensurePortalUserDeleteColumns(ctx context.Context) error {
	changes := []struct {
		column string
		sql    string
	}{
		{"is_deleted", `ALTER TABLE portal_user ADD COLUMN is_deleted BOOLEAN NOT NULL DEFAULT FALSE`},
		{"deleted_at", `ALTER TABLE portal_user ADD COLUMN deleted_at TIMESTAMP NULL`},
	}
	for _, item := range changes {
		if err := s.ensureTableColumn(ctx, "portal_user", item.column, item.sql); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensurePortalAPIKeyColumns(ctx context.Context) error {
	changes := []struct {
		column string
		sql    string
	}{
		{"expires_at", `ALTER TABLE portal_api_key ADD COLUMN expires_at TIMESTAMP NULL`},
		{"deleted_at", `ALTER TABLE portal_api_key ADD COLUMN deleted_at TIMESTAMP NULL`},
		{"limit_total_micro_yuan",
			`ALTER TABLE portal_api_key ADD COLUMN limit_total_micro_yuan BIGINT NOT NULL DEFAULT 0`},
		{"limit_5h_micro_yuan",
			`ALTER TABLE portal_api_key ADD COLUMN limit_5h_micro_yuan BIGINT NOT NULL DEFAULT 0`},
		{"limit_daily_micro_yuan",
			`ALTER TABLE portal_api_key ADD COLUMN limit_daily_micro_yuan BIGINT NOT NULL DEFAULT 0`},
		{"daily_reset_mode",
			`ALTER TABLE portal_api_key ADD COLUMN daily_reset_mode VARCHAR(16) NOT NULL DEFAULT 'fixed'`},
		{"daily_reset_time",
			`ALTER TABLE portal_api_key ADD COLUMN daily_reset_time VARCHAR(5) NOT NULL DEFAULT '00:00'`},
		{"limit_weekly_micro_yuan",
			`ALTER TABLE portal_api_key ADD COLUMN limit_weekly_micro_yuan BIGINT NOT NULL DEFAULT 0`},
		{"limit_monthly_micro_yuan",
			`ALTER TABLE portal_api_key ADD COLUMN limit_monthly_micro_yuan BIGINT NOT NULL DEFAULT 0`},
	}
	for _, item := range changes {
		if err := s.ensureTableColumn(ctx, "portal_api_key", item.column, item.sql); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureBillingTransactionColumns(ctx context.Context) error {
	changes := []struct {
		column string
		sql    string
	}{
		{"api_key_id", `ALTER TABLE billing_transaction ADD COLUMN api_key_id VARCHAR(64) NULL`},
		{"cache_creation_input_tokens",
			`ALTER TABLE billing_transaction ADD COLUMN cache_creation_input_tokens BIGINT NOT NULL DEFAULT 0`},
		{"cache_creation_5m_input_tokens",
			`ALTER TABLE billing_transaction ADD COLUMN cache_creation_5m_input_tokens BIGINT NOT NULL DEFAULT 0`},
		{"cache_creation_1h_input_tokens",
			`ALTER TABLE billing_transaction ADD COLUMN cache_creation_1h_input_tokens BIGINT NOT NULL DEFAULT 0`},
		{"cache_read_input_tokens",
			`ALTER TABLE billing_transaction ADD COLUMN cache_read_input_tokens BIGINT NOT NULL DEFAULT 0`},
		{"input_image_tokens",
			`ALTER TABLE billing_transaction ADD COLUMN input_image_tokens BIGINT NOT NULL DEFAULT 0`},
		{"output_image_tokens",
			`ALTER TABLE billing_transaction ADD COLUMN output_image_tokens BIGINT NOT NULL DEFAULT 0`},
		{"input_image_count",
			`ALTER TABLE billing_transaction ADD COLUMN input_image_count BIGINT NOT NULL DEFAULT 0`},
		{"output_image_count",
			`ALTER TABLE billing_transaction ADD COLUMN output_image_count BIGINT NOT NULL DEFAULT 0`},
		{"request_count",
			`ALTER TABLE billing_transaction ADD COLUMN request_count BIGINT NOT NULL DEFAULT 0`},
	}
	for _, item := range changes {
		if err := s.ensureTableColumn(ctx, "billing_transaction", item.column, item.sql); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureBillingUsageEventColumns(ctx context.Context) error {
	changes := []struct {
		column string
		sql    string
	}{
		{"trace_id", `ALTER TABLE billing_usage_event ADD COLUMN trace_id VARCHAR(128) NULL`},
		{"department_id", `ALTER TABLE billing_usage_event ADD COLUMN department_id VARCHAR(64) NOT NULL DEFAULT ''`},
		{"department_path", `ALTER TABLE billing_usage_event ADD COLUMN department_path VARCHAR(512) NOT NULL DEFAULT ''`},
		{"api_key_id", `ALTER TABLE billing_usage_event ADD COLUMN api_key_id VARCHAR(64) NULL`},
		{"request_path",
			`ALTER TABLE billing_usage_event ADD COLUMN request_path VARCHAR(255) NOT NULL DEFAULT ''`},
		{"request_kind",
			`ALTER TABLE billing_usage_event ADD COLUMN request_kind VARCHAR(64) NOT NULL DEFAULT ''`},
		{"request_status",
			`ALTER TABLE billing_usage_event ADD COLUMN request_status VARCHAR(16) NOT NULL DEFAULT 'success'`},
		{"usage_status",
			`ALTER TABLE billing_usage_event ADD COLUMN usage_status VARCHAR(16) NOT NULL DEFAULT 'parsed'`},
		{"http_status", `ALTER TABLE billing_usage_event ADD COLUMN http_status INT NOT NULL DEFAULT 200`},
		{"error_code",
			`ALTER TABLE billing_usage_event ADD COLUMN error_code VARCHAR(64) NOT NULL DEFAULT ''`},
		{"error_message",
			`ALTER TABLE billing_usage_event ADD COLUMN error_message VARCHAR(512) NOT NULL DEFAULT ''`},
		{"input_token_details_json",
			`ALTER TABLE billing_usage_event ADD COLUMN input_token_details_json TEXT NULL`},
		{"output_token_details_json",
			`ALTER TABLE billing_usage_event ADD COLUMN output_token_details_json TEXT NULL`},
		{"provider_usage_json",
			`ALTER TABLE billing_usage_event ADD COLUMN provider_usage_json TEXT NULL`},
		{"cache_creation_input_tokens",
			`ALTER TABLE billing_usage_event ADD COLUMN cache_creation_input_tokens BIGINT NOT NULL DEFAULT 0`},
		{"cache_creation_5m_input_tokens",
			`ALTER TABLE billing_usage_event ADD COLUMN cache_creation_5m_input_tokens BIGINT NOT NULL DEFAULT 0`},
		{"cache_creation_1h_input_tokens",
			`ALTER TABLE billing_usage_event ADD COLUMN cache_creation_1h_input_tokens BIGINT NOT NULL DEFAULT 0`},
		{"cache_read_input_tokens",
			`ALTER TABLE billing_usage_event ADD COLUMN cache_read_input_tokens BIGINT NOT NULL DEFAULT 0`},
		{"input_image_tokens",
			`ALTER TABLE billing_usage_event ADD COLUMN input_image_tokens BIGINT NOT NULL DEFAULT 0`},
		{"output_image_tokens",
			`ALTER TABLE billing_usage_event ADD COLUMN output_image_tokens BIGINT NOT NULL DEFAULT 0`},
		{"input_image_count",
			`ALTER TABLE billing_usage_event ADD COLUMN input_image_count BIGINT NOT NULL DEFAULT 0`},
		{"output_image_count",
			`ALTER TABLE billing_usage_event ADD COLUMN output_image_count BIGINT NOT NULL DEFAULT 0`},
		{"request_count",
			`ALTER TABLE billing_usage_event ADD COLUMN request_count BIGINT NOT NULL DEFAULT 0`},
		{"cache_ttl",
			`ALTER TABLE billing_usage_event ADD COLUMN cache_ttl VARCHAR(8) NOT NULL DEFAULT ''`},
		{"started_at", `ALTER TABLE billing_usage_event ADD COLUMN started_at TIMESTAMP NULL`},
		{"finished_at", `ALTER TABLE billing_usage_event ADD COLUMN finished_at TIMESTAMP NULL`},
	}
	for _, item := range changes {
		if err := s.ensureTableColumn(ctx, "billing_usage_event", item.column, item.sql); err != nil {
			return err
		}
	}
	for _, index := range []struct {
		name    string
		columns string
	}{
		{"idx_billing_usage_event_consumer_model_time", "consumer_name, model_id, occurred_at"},
		{"idx_billing_usage_event_api_key_time", "api_key_id, occurred_at"},
		{"idx_billing_usage_event_department_time", "department_id, occurred_at"},
	} {
		if err := s.ensureTableIndex(ctx, "billing_usage_event", index.name, index.columns); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensurePortalUsageDailyColumns(ctx context.Context) error {
	changes := []struct {
		column string
		sql    string
	}{
		{"department_id",
			`ALTER TABLE portal_usage_daily ADD COLUMN department_id VARCHAR(64) NOT NULL DEFAULT ''`},
		{"department_path",
			`ALTER TABLE portal_usage_daily ADD COLUMN department_path VARCHAR(512) NOT NULL DEFAULT ''`},
		{"cache_creation_input_tokens",
			`ALTER TABLE portal_usage_daily ADD COLUMN cache_creation_input_tokens BIGINT NOT NULL DEFAULT 0`},
		{"cache_creation_5m_input_tokens",
			`ALTER TABLE portal_usage_daily ADD COLUMN cache_creation_5m_input_tokens BIGINT NOT NULL DEFAULT 0`},
		{"cache_creation_1h_input_tokens",
			`ALTER TABLE portal_usage_daily ADD COLUMN cache_creation_1h_input_tokens BIGINT NOT NULL DEFAULT 0`},
		{"cache_read_input_tokens",
			`ALTER TABLE portal_usage_daily ADD COLUMN cache_read_input_tokens BIGINT NOT NULL DEFAULT 0`},
		{"input_image_tokens",
			`ALTER TABLE portal_usage_daily ADD COLUMN input_image_tokens BIGINT NOT NULL DEFAULT 0`},
		{"output_image_tokens",
			`ALTER TABLE portal_usage_daily ADD COLUMN output_image_tokens BIGINT NOT NULL DEFAULT 0`},
		{"input_image_count",
			`ALTER TABLE portal_usage_daily ADD COLUMN input_image_count BIGINT NOT NULL DEFAULT 0`},
		{"output_image_count",
			`ALTER TABLE portal_usage_daily ADD COLUMN output_image_count BIGINT NOT NULL DEFAULT 0`},
	}
	for _, item := range changes {
		if err := s.ensureTableColumn(ctx, "portal_usage_daily", item.column, item.sql); err != nil {
			return err
		}
	}
	if err := s.ensureTableIndex(ctx, "portal_usage_daily", "idx_portal_usage_daily_department_date",
		"department_id, billing_date"); err != nil {
		return err
	}
	return nil
}

func (s *Service) ensurePortalModelAssetColumns(ctx context.Context) error {
	changes := []struct {
		column string
		sql    string
	}{
		{"model_type",
			`ALTER TABLE portal_model_asset ADD COLUMN model_type VARCHAR(64) NULL`},
		{"input_modalities_json",
			`ALTER TABLE portal_model_asset ADD COLUMN input_modalities_json TEXT NULL`},
		{"output_modalities_json",
			`ALTER TABLE portal_model_asset ADD COLUMN output_modalities_json TEXT NULL`},
		{"feature_flags_json",
			`ALTER TABLE portal_model_asset ADD COLUMN feature_flags_json TEXT NULL`},
		{"request_kinds_json",
			`ALTER TABLE portal_model_asset ADD COLUMN request_kinds_json TEXT NULL`},
	}
	for _, item := range changes {
		if err := s.ensureTableColumn(ctx, "portal_model_asset", item.column, item.sql); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensurePortalModelBindingColumns(ctx context.Context) error {
	changes := []struct {
		column string
		sql    string
	}{
		{"limits_json",
			`ALTER TABLE portal_model_binding ADD COLUMN limits_json TEXT NULL`},
	}
	for _, item := range changes {
		if err := s.ensureTableColumn(ctx, "portal_model_binding", item.column, item.sql); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureBillingModelPriceColumns(ctx context.Context) error {
	addColumnSQL := func(column string, postgresType string) string {
		return s.sqlForDriver(`ALTER TABLE billing_model_price_version ADD COLUMN ` + column + ` ` + postgresType)
	}

	changes := []struct {
		column string
		sql    string
	}{
		{"input_price_micro_yuan_per_token",
			addColumnSQL("input_price_micro_yuan_per_token", "BIGINT NOT NULL DEFAULT 0")},
		{"output_price_micro_yuan_per_token",
			addColumnSQL("output_price_micro_yuan_per_token", "BIGINT NOT NULL DEFAULT 0")},
		{"input_price_per_1k_micro_yuan",
			addColumnSQL("input_price_per_1k_micro_yuan", "BIGINT NOT NULL DEFAULT 0")},
		{"output_price_per_1k_micro_yuan",
			addColumnSQL("output_price_per_1k_micro_yuan", "BIGINT NOT NULL DEFAULT 0")},
		{"input_request_price_micro_yuan",
			addColumnSQL("input_request_price_micro_yuan", "BIGINT NOT NULL DEFAULT 0")},
		{"cache_creation_input_token_price_micro_yuan_per_token",
			addColumnSQL("cache_creation_input_token_price_micro_yuan_per_token", "BIGINT NOT NULL DEFAULT 0")},
		{"cache_creation_input_token_price_above_1hr_micro_yuan_per_token",
			addColumnSQL("cache_creation_input_token_price_above_1hr_micro_yuan_per_token", "BIGINT NOT NULL DEFAULT 0")},
		{"cache_read_input_token_price_micro_yuan_per_token",
			addColumnSQL("cache_read_input_token_price_micro_yuan_per_token", "BIGINT NOT NULL DEFAULT 0")},
		{"input_token_price_above_200k_micro_yuan_per_token",
			addColumnSQL("input_token_price_above_200k_micro_yuan_per_token", "BIGINT NOT NULL DEFAULT 0")},
		{"output_token_price_above_200k_micro_yuan_per_token",
			addColumnSQL("output_token_price_above_200k_micro_yuan_per_token", "BIGINT NOT NULL DEFAULT 0")},
		{"cache_creation_input_token_price_above_200k_micro_yuan_per_token",
			addColumnSQL("cache_creation_input_token_price_above_200k_micro_yuan_per_token", "BIGINT NOT NULL DEFAULT 0")},
		{"cache_read_input_token_price_above_200k_micro_yuan_per_token",
			addColumnSQL("cache_read_input_token_price_above_200k_micro_yuan_per_token", "BIGINT NOT NULL DEFAULT 0")},
		{"cache_creation_input_token_price_per_1k_micro_yuan",
			addColumnSQL("cache_creation_input_token_price_per_1k_micro_yuan", "BIGINT NOT NULL DEFAULT 0")},
		{"cache_creation_input_token_price_above_1hr_per_1k_micro_yuan",
			addColumnSQL("cache_creation_input_token_price_above_1hr_per_1k_micro_yuan", "BIGINT NOT NULL DEFAULT 0")},
		{"cache_read_input_token_price_per_1k_micro_yuan",
			addColumnSQL("cache_read_input_token_price_per_1k_micro_yuan", "BIGINT NOT NULL DEFAULT 0")},
		{"input_token_price_above_200k_per_1k_micro_yuan",
			addColumnSQL("input_token_price_above_200k_per_1k_micro_yuan", "BIGINT NOT NULL DEFAULT 0")},
		{"output_token_price_above_200k_per_1k_micro_yuan",
			addColumnSQL("output_token_price_above_200k_per_1k_micro_yuan", "BIGINT NOT NULL DEFAULT 0")},
		{"cache_creation_input_token_price_above_200k_per_1k_micro_yuan",
			addColumnSQL("cache_creation_input_token_price_above_200k_per_1k_micro_yuan", "BIGINT NOT NULL DEFAULT 0")},
		{"cache_read_input_token_price_above_200k_per_1k_micro_yuan",
			addColumnSQL("cache_read_input_token_price_above_200k_per_1k_micro_yuan", "BIGINT NOT NULL DEFAULT 0")},
		{"output_image_price_micro_yuan",
			addColumnSQL("output_image_price_micro_yuan", "BIGINT NOT NULL DEFAULT 0")},
		{"output_image_token_price_micro_yuan_per_token",
			addColumnSQL("output_image_token_price_micro_yuan_per_token", "BIGINT NOT NULL DEFAULT 0")},
		{"output_image_token_price_per_1k_micro_yuan",
			addColumnSQL("output_image_token_price_per_1k_micro_yuan", "BIGINT NOT NULL DEFAULT 0")},
		{"input_image_price_micro_yuan",
			addColumnSQL("input_image_price_micro_yuan", "BIGINT NOT NULL DEFAULT 0")},
		{"input_image_token_price_micro_yuan_per_token",
			addColumnSQL("input_image_token_price_micro_yuan_per_token", "BIGINT NOT NULL DEFAULT 0")},
		{"input_image_token_price_per_1k_micro_yuan",
			addColumnSQL("input_image_token_price_per_1k_micro_yuan", "BIGINT NOT NULL DEFAULT 0")},
		{"supports_prompt_caching",
			addColumnSQL("supports_prompt_caching", "BOOLEAN NOT NULL DEFAULT FALSE")},
	}
	for _, item := range changes {
		if err := s.ensureTableColumn(ctx, "billing_model_price_version", item.column, item.sql); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureTableColumn(ctx context.Context, tableName string, columnName string, alterSQL string) error {
	existed, err := s.db.GetValue(ctx, s.tableColumnExistsQuery(), tableName, columnName)
	if err != nil {
		return gerror.Wrapf(err, "query %s.%s existence failed", tableName, columnName)
	}
	if existed.Int() > 0 {
		return nil
	}
	if _, err = s.db.Exec(ctx, alterSQL); err != nil {
		return gerror.Wrapf(err, "add %s.%s column failed", tableName, columnName)
	}
	return nil
}

func (s *Service) ensureTableIndex(ctx context.Context, tableName string, indexName string, columns string) error {
	existed, err := s.db.GetValue(ctx, s.tableIndexExistsQuery(), tableName, indexName)
	if err != nil {
		return gerror.Wrapf(err, "query %s.%s index existence failed", tableName, indexName)
	}
	if existed.Int() > 0 {
		return nil
	}
	if _, err = s.db.Exec(ctx, "CREATE INDEX "+indexName+" ON "+tableName+" ("+columns+")"); err != nil {
		return gerror.Wrapf(err, "add %s.%s index failed", tableName, indexName)
	}
	return nil
}

func (s *Service) seedBootstrapData(ctx context.Context) error {
	seedModels := []model.ModelInfo{
		{ID: "model-qwen-max", Name: "Qwen-Max", Vendor: "Alibaba Cloud", Capability: "general chat / coding", InputPricePerMillionTokens: 20.000, OutputPricePerMillionTokens: 60.000, Endpoint: "/v1/chat/completions", SDK: "OpenAI Compatible API", Summary: "High quality generation and complex reasoning."},
		{ID: "model-deepseek-v3", Name: "DeepSeek-V3", Vendor: "DeepSeek", Capability: "reasoning / low cost generation", InputPricePerMillionTokens: 8.000, OutputPricePerMillionTokens: 18.000, Endpoint: "/v1/chat/completions", SDK: "OpenAI Compatible API", Summary: "Balanced quality and cost for general use cases."},
		{ID: "model-higress-rerank", Name: "Higress-Rerank-1.0", Vendor: "Higress AI", Capability: "search rerank", InputPricePerMillionTokens: 4.000, OutputPricePerMillionTokens: 0.000, Endpoint: "/v1/rerank", SDK: "REST API", Summary: "Improve relevance in RAG retrieval ranking."},
	}

	for _, item := range seedModels {
		_, err := s.db.Exec(ctx, `
			INSERT INTO portal_model_catalog
			(model_id, name, vendor, capability, input_token_price, output_token_price, endpoint, sdk, summary, status)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active')
			`+s.upsertClause([]string{"model_id"},
			s.assignExcluded("vendor"),
			s.assignExcluded("capability"),
			s.assignExcluded("input_token_price"),
			s.assignExcluded("output_token_price"),
			s.assignExcluded("endpoint"),
			s.assignExcluded("sdk"),
			s.assignExcluded("summary"),
			"status = 'active'")+``,
			item.ID,
			item.Name,
			item.Vendor,
			item.Capability,
			item.InputPricePerMillionTokens,
			item.OutputPricePerMillionTokens,
			item.Endpoint,
			item.SDK,
			item.Summary,
		)
		if err != nil {
			return gerror.Wrap(err, "seed model catalog failed")
		}
	}

	if code := strings.TrimSpace(s.cfg.InviteCode); code != "" {
		expiresAt := time.Now().AddDate(0, 0, s.cfg.InviteExpireDays)
		inviteUpsert := s.upsertClause([]string{"invite_code"},
			"status = "+s.sqlForDriver(
				"CASE WHEN portal_invite_code.status = 'disabled' THEN portal_invite_code.status ELSE EXCLUDED.status END"),
			"expires_at = "+s.sqlForDriver(
				"CASE WHEN portal_invite_code.status = 'disabled' THEN portal_invite_code.expires_at ELSE EXCLUDED.expires_at END"),
		)
		_, err := s.db.Exec(ctx, `
			INSERT INTO portal_invite_code (invite_code, status, expires_at)
			VALUES (?, ?, ?)
			`+inviteUpsert,
			code,
			consts.InviteStatusActive,
			expiresAt,
		)
		if err != nil {
			return gerror.Wrap(err, "seed invite code failed")
		}
	}
	return nil
}

func randomString(length int) string {
	if length <= 0 {
		return ""
	}
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	max := big.NewInt(int64(len(chars)))
	out := make([]byte, length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			out[i] = chars[i%len(chars)]
			continue
		}
		out[i] = chars[n.Int64()]
	}
	return string(out)
}

func randomToken(prefix string) string {
	return prefix + randomString(24)
}

func sha256Hex(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

func (s *Service) cfgValue() config.Config {
	return s.cfg
}

func (s *Service) logf(ctx context.Context, format string, args ...any) {
	g.Log().Infof(ctx, format, args...)
}

func goFramePostgresLink(dsn string) string {
	trimmed := strings.TrimSpace(dsn)
	if trimmed == "" || strings.HasPrefix(trimmed, "pgsql:") {
		return trimmed
	}

	fields := map[string]string{}
	for _, field := range strings.Fields(trimmed) {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) == 2 {
			fields[parts[0]] = parts[1]
		}
	}
	if fields["host"] == "" || fields["user"] == "" || fields["dbname"] == "" {
		return trimmed
	}

	port := portalDefaultString(fields["port"], "5432")
	params := url.Values{}
	if sslmode := portalDefaultString(fields["sslmode"], "disable"); sslmode != "" {
		params.Set("sslmode", sslmode)
	}
	for key, value := range fields {
		switch key {
		case "host", "port", "user", "password", "dbname", "sslmode":
			continue
		default:
			params.Set(key, value)
		}
	}

	query := params.Encode()
	if query == "" {
		query = "sslmode=disable"
	}
	return "pgsql:" + fields["user"] + ":" + fields["password"] + "@tcp(" + fields["host"] + ":" + port + ")/" + fields["dbname"] + "?" + query
}

func portalDefaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

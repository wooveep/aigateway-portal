package portal

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	clientK8s "higress-portal-backend/internal/client/k8s"
	"higress-portal-backend/internal/config"
	"higress-portal-backend/internal/consts"
	"higress-portal-backend/internal/model"
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
		Type:             "mysql",
		Link:             cfg.MySQLDSN,
		MaxOpenConnCount: 20,
		MaxIdleConnCount: 10,
		MaxConnLifeTime:  30 * time.Minute,
	}
	db, err := gdb.New(node)
	if err != nil {
		return nil, gerror.Wrap(err, "initialize mysql failed")
	}
	if err = db.PingMaster(); err != nil {
		return nil, gerror.Wrap(err, "connect mysql failed")
	}

	s := &Service{
		cfg: cfg,
		db:  db,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		streamClient: &http.Client{},
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
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS portal_user (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			consumer_name VARCHAR(128) NOT NULL UNIQUE,
			display_name VARCHAR(128) NOT NULL,
			email VARCHAR(255) NOT NULL DEFAULT '',
			password_hash VARCHAR(255) NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'active',
			source VARCHAR(16) NOT NULL DEFAULT 'portal',
			user_level VARCHAR(16) NOT NULL DEFAULT 'normal',
			is_deleted TINYINT(1) NOT NULL DEFAULT 0,
			deleted_at DATETIME NULL,
			last_login_at DATETIME NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS portal_invite_code (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			invite_code VARCHAR(64) NOT NULL UNIQUE,
			status VARCHAR(16) NOT NULL DEFAULT 'active',
			expires_at DATETIME NOT NULL,
			used_by_consumer VARCHAR(128) NULL,
			used_at DATETIME NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS portal_session (
			session_token VARCHAR(96) PRIMARY KEY,
			consumer_name VARCHAR(128) NOT NULL,
			expires_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_session_consumer (consumer_name),
			INDEX idx_session_expire (expires_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS portal_api_key (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			key_id VARCHAR(64) NOT NULL UNIQUE,
			consumer_name VARCHAR(128) NOT NULL,
			name VARCHAR(128) NOT NULL,
			key_masked VARCHAR(128) NOT NULL,
			key_hash VARCHAR(128) NOT NULL,
			raw_key VARCHAR(256) NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'active',
			total_calls BIGINT NOT NULL DEFAULT 0,
			last_used_at DATETIME NULL,
			expires_at DATETIME NULL,
			deleted_at DATETIME NULL,
			limit_total_micro_yuan BIGINT NOT NULL DEFAULT 0,
			limit_5h_micro_yuan BIGINT NOT NULL DEFAULT 0,
			limit_daily_micro_yuan BIGINT NOT NULL DEFAULT 0,
			daily_reset_mode VARCHAR(16) NOT NULL DEFAULT 'fixed',
			daily_reset_time VARCHAR(5) NOT NULL DEFAULT '00:00',
			limit_weekly_micro_yuan BIGINT NOT NULL DEFAULT 0,
			limit_monthly_micro_yuan BIGINT NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_api_key_consumer_status (consumer_name, status)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS portal_model_catalog (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			model_id VARCHAR(64) NOT NULL UNIQUE,
			name VARCHAR(128) NOT NULL UNIQUE,
			vendor VARCHAR(128) NOT NULL,
			capability VARCHAR(255) NOT NULL,
			input_token_price DECIMAL(18,6) NOT NULL DEFAULT 0,
			output_token_price DECIMAL(18,6) NOT NULL DEFAULT 0,
			endpoint VARCHAR(255) NOT NULL,
			sdk VARCHAR(128) NOT NULL,
			summary TEXT NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'active',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS portal_usage_daily (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			billing_date DATE NOT NULL,
			consumer_name VARCHAR(128) NOT NULL,
			department_id VARCHAR(64) NOT NULL DEFAULT '',
			department_path VARCHAR(512) NOT NULL DEFAULT '',
			model_name VARCHAR(128) NOT NULL,
			request_count BIGINT NOT NULL DEFAULT 0,
			input_tokens BIGINT NOT NULL DEFAULT 0,
			output_tokens BIGINT NOT NULL DEFAULT 0,
			cache_creation_input_tokens BIGINT NOT NULL DEFAULT 0,
			cache_creation_5m_input_tokens BIGINT NOT NULL DEFAULT 0,
			cache_creation_1h_input_tokens BIGINT NOT NULL DEFAULT 0,
			cache_read_input_tokens BIGINT NOT NULL DEFAULT 0,
			input_image_tokens BIGINT NOT NULL DEFAULT 0,
			output_image_tokens BIGINT NOT NULL DEFAULT 0,
			input_image_count BIGINT NOT NULL DEFAULT 0,
			output_image_count BIGINT NOT NULL DEFAULT 0,
			total_tokens BIGINT NOT NULL DEFAULT 0,
			cost_amount DECIMAL(18,6) NOT NULL DEFAULT 0,
			source_from DATETIME NULL,
			source_to DATETIME NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_usage_consumer_model_date (billing_date, consumer_name, model_name),
			INDEX idx_portal_usage_daily_department_date (department_id, billing_date)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS portal_recharge_order (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			order_id VARCHAR(64) NOT NULL UNIQUE,
			consumer_name VARCHAR(128) NOT NULL,
			amount DECIMAL(18,2) NOT NULL,
			channel VARCHAR(32) NOT NULL,
			status VARCHAR(16) NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_recharge_consumer (consumer_name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS portal_balance_adjustment (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			adjustment_id VARCHAR(64) NOT NULL UNIQUE,
			operator_consumer_name VARCHAR(128) NOT NULL,
			target_consumer_name VARCHAR(128) NOT NULL,
			delta_micro_yuan BIGINT NOT NULL,
			reason VARCHAR(255) NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_balance_adjustment_target (target_consumer_name, created_at),
			INDEX idx_balance_adjustment_operator (operator_consumer_name, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS portal_invoice_profile (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			consumer_name VARCHAR(128) NOT NULL UNIQUE,
			company_name VARCHAR(255) NOT NULL DEFAULT '',
			tax_no VARCHAR(128) NOT NULL DEFAULT '',
			address VARCHAR(255) NOT NULL DEFAULT '',
			bank_account VARCHAR(255) NOT NULL DEFAULT '',
			receiver VARCHAR(128) NOT NULL DEFAULT '',
			email VARCHAR(255) NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS portal_invoice_record (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			invoice_id VARCHAR(64) NOT NULL UNIQUE,
			consumer_name VARCHAR(128) NOT NULL,
			title VARCHAR(255) NOT NULL,
			tax_no VARCHAR(128) NOT NULL,
			amount DECIMAL(18,2) NOT NULL,
			status VARCHAR(16) NOT NULL,
			remark VARCHAR(512) NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_invoice_consumer (consumer_name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS billing_wallet (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			consumer_name VARCHAR(128) NOT NULL UNIQUE,
			currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
			available_micro_yuan BIGINT NOT NULL DEFAULT 0,
			version BIGINT NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_billing_wallet_currency (currency)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS billing_transaction (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			tx_id VARCHAR(64) NOT NULL UNIQUE,
			consumer_name VARCHAR(128) NOT NULL,
			tx_type VARCHAR(16) NOT NULL,
			amount_micro_yuan BIGINT NOT NULL,
			currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
			source_type VARCHAR(64) NOT NULL,
			source_id VARCHAR(128) NOT NULL,
			request_id VARCHAR(128) NULL,
			api_key_id VARCHAR(64) NULL,
			model_id VARCHAR(128) NULL,
			price_version_id BIGINT NULL,
			input_tokens BIGINT NOT NULL DEFAULT 0,
			output_tokens BIGINT NOT NULL DEFAULT 0,
			cache_creation_input_tokens BIGINT NOT NULL DEFAULT 0,
			cache_creation_5m_input_tokens BIGINT NOT NULL DEFAULT 0,
			cache_creation_1h_input_tokens BIGINT NOT NULL DEFAULT 0,
			cache_read_input_tokens BIGINT NOT NULL DEFAULT 0,
			input_image_tokens BIGINT NOT NULL DEFAULT 0,
			output_image_tokens BIGINT NOT NULL DEFAULT 0,
			input_image_count BIGINT NOT NULL DEFAULT 0,
			output_image_count BIGINT NOT NULL DEFAULT 0,
			request_count BIGINT NOT NULL DEFAULT 0,
			occurred_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uk_billing_transaction_source (source_type, source_id),
			INDEX idx_billing_transaction_consumer_time (consumer_name, occurred_at),
			INDEX idx_billing_transaction_type (tx_type)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS billing_usage_event (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			event_id VARCHAR(128) NOT NULL UNIQUE,
			request_id VARCHAR(128) NULL,
			trace_id VARCHAR(128) NULL,
			consumer_name VARCHAR(128) NOT NULL,
			department_id VARCHAR(64) NOT NULL DEFAULT '',
			department_path VARCHAR(512) NOT NULL DEFAULT '',
			api_key_id VARCHAR(64) NULL,
			route_name VARCHAR(255) NOT NULL DEFAULT '',
			request_path VARCHAR(255) NOT NULL DEFAULT '',
			request_kind VARCHAR(64) NOT NULL DEFAULT '',
			model_id VARCHAR(128) NOT NULL,
			request_status VARCHAR(16) NOT NULL DEFAULT 'success',
			usage_status VARCHAR(16) NOT NULL DEFAULT 'parsed',
			http_status INT NOT NULL DEFAULT 200,
			error_code VARCHAR(64) NOT NULL DEFAULT '',
			error_message VARCHAR(512) NOT NULL DEFAULT '',
			input_tokens BIGINT NOT NULL DEFAULT 0,
			output_tokens BIGINT NOT NULL DEFAULT 0,
			cache_creation_input_tokens BIGINT NOT NULL DEFAULT 0,
			cache_creation_5m_input_tokens BIGINT NOT NULL DEFAULT 0,
			cache_creation_1h_input_tokens BIGINT NOT NULL DEFAULT 0,
			cache_read_input_tokens BIGINT NOT NULL DEFAULT 0,
			input_image_tokens BIGINT NOT NULL DEFAULT 0,
			output_image_tokens BIGINT NOT NULL DEFAULT 0,
			input_image_count BIGINT NOT NULL DEFAULT 0,
			output_image_count BIGINT NOT NULL DEFAULT 0,
			request_count BIGINT NOT NULL DEFAULT 0,
			cache_ttl VARCHAR(8) NOT NULL DEFAULT '',
			total_tokens BIGINT NOT NULL DEFAULT 0,
			input_token_details_json TEXT NULL,
			output_token_details_json TEXT NULL,
			provider_usage_json TEXT NULL,
			cost_micro_yuan BIGINT NOT NULL DEFAULT 0,
			price_version_id BIGINT NULL,
			started_at DATETIME NULL,
			finished_at DATETIME NULL,
			redis_stream_id VARCHAR(128) NOT NULL DEFAULT '',
			occurred_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_billing_usage_event_consumer_time (consumer_name, occurred_at),
			INDEX idx_billing_usage_event_consumer_model_time (consumer_name, model_id, occurred_at),
			INDEX idx_billing_usage_event_api_key_time (api_key_id, occurred_at),
			INDEX idx_billing_usage_event_department_time (department_id, occurred_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS quota_policy_user (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			consumer_name VARCHAR(128) NOT NULL UNIQUE,
			limit_total_micro_yuan BIGINT NOT NULL DEFAULT 0,
			limit_5h_micro_yuan BIGINT NOT NULL DEFAULT 0,
			limit_daily_micro_yuan BIGINT NOT NULL DEFAULT 0,
			daily_reset_mode VARCHAR(16) NOT NULL DEFAULT 'fixed',
			daily_reset_time VARCHAR(5) NOT NULL DEFAULT '00:00',
			limit_weekly_micro_yuan BIGINT NOT NULL DEFAULT 0,
			limit_monthly_micro_yuan BIGINT NOT NULL DEFAULT 0,
			cost_reset_at DATETIME NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS billing_model_catalog (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			model_id VARCHAR(128) NOT NULL UNIQUE,
			name VARCHAR(128) NOT NULL,
			vendor VARCHAR(128) NOT NULL,
			capability VARCHAR(255) NOT NULL,
			endpoint VARCHAR(255) NOT NULL,
			sdk VARCHAR(128) NOT NULL,
			summary TEXT NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'active',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_billing_model_status (status)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS portal_model_asset (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			asset_id VARCHAR(128) NOT NULL UNIQUE,
			canonical_name VARCHAR(128) NOT NULL UNIQUE,
			display_name VARCHAR(128) NOT NULL,
			intro TEXT NOT NULL,
			tags_json TEXT NULL,
			modalities_json TEXT NULL,
			features_json TEXT NULL,
			request_kinds_json TEXT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_portal_model_asset_display_name (display_name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS portal_model_binding (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			binding_id VARCHAR(128) NOT NULL UNIQUE,
			asset_id VARCHAR(128) NOT NULL,
			model_id VARCHAR(128) NOT NULL UNIQUE,
			provider_name VARCHAR(128) NOT NULL,
			target_model VARCHAR(128) NOT NULL,
			protocol VARCHAR(128) NOT NULL DEFAULT 'openai/v1',
			endpoint VARCHAR(255) NOT NULL DEFAULT '-',
			pricing_json TEXT NOT NULL,
			rpm BIGINT NULL,
			tpm BIGINT NULL,
			context_window BIGINT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'draft',
			published_at DATETIME NULL,
			unpublished_at DATETIME NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_portal_model_binding_target (asset_id, provider_name, target_model),
			INDEX idx_portal_model_binding_asset (asset_id),
			INDEX idx_portal_model_binding_status (status),
			INDEX idx_portal_model_binding_provider (provider_name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS portal_agent_catalog (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			agent_id VARCHAR(128) NOT NULL UNIQUE,
			canonical_name VARCHAR(128) NOT NULL UNIQUE,
			display_name VARCHAR(128) NOT NULL,
			intro TEXT NOT NULL,
			description TEXT NOT NULL,
			icon_url VARCHAR(512) NOT NULL DEFAULT '',
			tags_json TEXT NULL,
			mcp_server_name VARCHAR(128) NOT NULL UNIQUE,
			tool_count BIGINT NOT NULL DEFAULT 0,
			transport_types_json TEXT NULL,
			resource_summary TEXT NOT NULL,
			prompt_summary TEXT NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'draft',
			published_at DATETIME NULL,
			unpublished_at DATETIME NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_portal_agent_catalog_status (status),
			INDEX idx_portal_agent_catalog_display_name (display_name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS portal_ai_chat_session (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			session_id VARCHAR(128) NOT NULL UNIQUE,
			consumer_name VARCHAR(128) NOT NULL,
			operator_consumer_name VARCHAR(128) NOT NULL,
			title VARCHAR(255) NOT NULL,
			default_model_id VARCHAR(128) NOT NULL DEFAULT '',
			default_api_key_id VARCHAR(64) NOT NULL DEFAULT '',
			last_message_preview VARCHAR(512) NOT NULL DEFAULT '',
			last_message_at DATETIME NULL,
			deleted_at DATETIME NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_portal_ai_chat_session_consumer (consumer_name, deleted_at, last_message_at),
			INDEX idx_portal_ai_chat_session_operator (operator_consumer_name, deleted_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS portal_ai_chat_message (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			message_id VARCHAR(128) NOT NULL UNIQUE,
			session_id VARCHAR(128) NOT NULL,
			role VARCHAR(16) NOT NULL,
			content MEDIUMTEXT NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'succeeded',
			model_id VARCHAR(128) NOT NULL DEFAULT '',
			api_key_id VARCHAR(64) NOT NULL DEFAULT '',
			request_id VARCHAR(128) NOT NULL DEFAULT '',
			trace_id VARCHAR(128) NOT NULL DEFAULT '',
			http_status INT NOT NULL DEFAULT 0,
			error_message VARCHAR(1024) NOT NULL DEFAULT '',
			finished_at DATETIME NULL,
			deleted_at DATETIME NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_portal_ai_chat_message_session (session_id, deleted_at, created_at),
			INDEX idx_portal_ai_chat_message_request (request_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS billing_model_price_version (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			model_id VARCHAR(128) NOT NULL,
			currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
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
			supports_prompt_caching TINYINT(1) NOT NULL DEFAULT 0,
			effective_from DATETIME NOT NULL,
			effective_to DATETIME NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'active',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_billing_model_price_active (model_id, status, effective_to),
			INDEX idx_billing_model_price_time (effective_from)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}

	for _, ddl := range migrations {
		if _, err := s.db.Exec(ctx, ddl); err != nil {
			return gerror.Wrap(err, "migration failed")
		}
	}
	if err := s.ensurePortalUserLevelColumn(ctx); err != nil {
		return err
	}
	if err := s.ensurePortalUserDeleteColumns(ctx); err != nil {
		return err
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
	if err := s.ensureBillingModelPriceColumns(ctx); err != nil {
		return err
	}
	if err := s.ensurePortalModelAssetColumns(ctx); err != nil {
		return err
	}
	if err := s.ensureOrganizationSchema(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Service) ensurePortalUserLevelColumn(ctx context.Context) error {
	existed, err := s.db.GetValue(ctx, `
		SELECT COUNT(1)
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = 'portal_user'
		  AND COLUMN_NAME = 'user_level'`)
	if err != nil {
		return gerror.Wrap(err, "query portal_user.user_level existence failed")
	}
	if existed.Int() > 0 {
		return nil
	}
	if _, err = s.db.Exec(ctx,
		`ALTER TABLE portal_user ADD COLUMN user_level VARCHAR(16) NOT NULL DEFAULT 'normal'`); err != nil {
		return gerror.Wrap(err, "add portal_user.user_level column failed")
	}
	return nil
}

func (s *Service) ensurePortalUserDeleteColumns(ctx context.Context) error {
	changes := []struct {
		column string
		sql    string
	}{
		{"is_deleted", `ALTER TABLE portal_user ADD COLUMN is_deleted TINYINT(1) NOT NULL DEFAULT 0 AFTER user_level`},
		{"deleted_at", `ALTER TABLE portal_user ADD COLUMN deleted_at DATETIME NULL AFTER is_deleted`},
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
		{"expires_at", `ALTER TABLE portal_api_key ADD COLUMN expires_at DATETIME NULL AFTER last_used_at`},
		{"deleted_at", `ALTER TABLE portal_api_key ADD COLUMN deleted_at DATETIME NULL AFTER expires_at`},
		{"limit_total_micro_yuan",
			`ALTER TABLE portal_api_key ADD COLUMN limit_total_micro_yuan BIGINT NOT NULL DEFAULT 0 AFTER deleted_at`},
		{"limit_5h_micro_yuan",
			`ALTER TABLE portal_api_key ADD COLUMN limit_5h_micro_yuan BIGINT NOT NULL DEFAULT 0 AFTER limit_total_micro_yuan`},
		{"limit_daily_micro_yuan",
			`ALTER TABLE portal_api_key ADD COLUMN limit_daily_micro_yuan BIGINT NOT NULL DEFAULT 0 AFTER limit_5h_micro_yuan`},
		{"daily_reset_mode",
			`ALTER TABLE portal_api_key ADD COLUMN daily_reset_mode VARCHAR(16) NOT NULL DEFAULT 'fixed' AFTER limit_daily_micro_yuan`},
		{"daily_reset_time",
			`ALTER TABLE portal_api_key ADD COLUMN daily_reset_time VARCHAR(5) NOT NULL DEFAULT '00:00' AFTER daily_reset_mode`},
		{"limit_weekly_micro_yuan",
			`ALTER TABLE portal_api_key ADD COLUMN limit_weekly_micro_yuan BIGINT NOT NULL DEFAULT 0 AFTER daily_reset_time`},
		{"limit_monthly_micro_yuan",
			`ALTER TABLE portal_api_key ADD COLUMN limit_monthly_micro_yuan BIGINT NOT NULL DEFAULT 0 AFTER limit_weekly_micro_yuan`},
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
		{"api_key_id", `ALTER TABLE billing_transaction ADD COLUMN api_key_id VARCHAR(64) NULL AFTER request_id`},
		{"cache_creation_input_tokens",
			`ALTER TABLE billing_transaction ADD COLUMN cache_creation_input_tokens BIGINT NOT NULL DEFAULT 0 AFTER output_tokens`},
		{"cache_creation_5m_input_tokens",
			`ALTER TABLE billing_transaction ADD COLUMN cache_creation_5m_input_tokens BIGINT NOT NULL DEFAULT 0 AFTER cache_creation_input_tokens`},
		{"cache_creation_1h_input_tokens",
			`ALTER TABLE billing_transaction ADD COLUMN cache_creation_1h_input_tokens BIGINT NOT NULL DEFAULT 0 AFTER cache_creation_5m_input_tokens`},
		{"cache_read_input_tokens",
			`ALTER TABLE billing_transaction ADD COLUMN cache_read_input_tokens BIGINT NOT NULL DEFAULT 0 AFTER cache_creation_1h_input_tokens`},
		{"input_image_tokens",
			`ALTER TABLE billing_transaction ADD COLUMN input_image_tokens BIGINT NOT NULL DEFAULT 0 AFTER cache_read_input_tokens`},
		{"output_image_tokens",
			`ALTER TABLE billing_transaction ADD COLUMN output_image_tokens BIGINT NOT NULL DEFAULT 0 AFTER input_image_tokens`},
		{"input_image_count",
			`ALTER TABLE billing_transaction ADD COLUMN input_image_count BIGINT NOT NULL DEFAULT 0 AFTER output_image_tokens`},
		{"output_image_count",
			`ALTER TABLE billing_transaction ADD COLUMN output_image_count BIGINT NOT NULL DEFAULT 0 AFTER input_image_count`},
		{"request_count",
			`ALTER TABLE billing_transaction ADD COLUMN request_count BIGINT NOT NULL DEFAULT 0 AFTER output_image_count`},
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
		{"trace_id", `ALTER TABLE billing_usage_event ADD COLUMN trace_id VARCHAR(128) NULL AFTER request_id`},
		{"department_id", `ALTER TABLE billing_usage_event ADD COLUMN department_id VARCHAR(64) NOT NULL DEFAULT '' AFTER consumer_name`},
		{"department_path", `ALTER TABLE billing_usage_event ADD COLUMN department_path VARCHAR(512) NOT NULL DEFAULT '' AFTER department_id`},
		{"api_key_id", `ALTER TABLE billing_usage_event ADD COLUMN api_key_id VARCHAR(64) NULL AFTER consumer_name`},
		{"request_path",
			`ALTER TABLE billing_usage_event ADD COLUMN request_path VARCHAR(255) NOT NULL DEFAULT '' AFTER route_name`},
		{"request_kind",
			`ALTER TABLE billing_usage_event ADD COLUMN request_kind VARCHAR(64) NOT NULL DEFAULT '' AFTER request_path`},
		{"request_status",
			`ALTER TABLE billing_usage_event ADD COLUMN request_status VARCHAR(16) NOT NULL DEFAULT 'success' AFTER model_id`},
		{"usage_status",
			`ALTER TABLE billing_usage_event ADD COLUMN usage_status VARCHAR(16) NOT NULL DEFAULT 'parsed' AFTER request_status`},
		{"http_status", `ALTER TABLE billing_usage_event ADD COLUMN http_status INT NOT NULL DEFAULT 200 AFTER usage_status`},
		{"error_code",
			`ALTER TABLE billing_usage_event ADD COLUMN error_code VARCHAR(64) NOT NULL DEFAULT '' AFTER http_status`},
		{"error_message",
			`ALTER TABLE billing_usage_event ADD COLUMN error_message VARCHAR(512) NOT NULL DEFAULT '' AFTER error_code`},
		{"input_token_details_json",
			`ALTER TABLE billing_usage_event ADD COLUMN input_token_details_json TEXT NULL AFTER total_tokens`},
		{"output_token_details_json",
			`ALTER TABLE billing_usage_event ADD COLUMN output_token_details_json TEXT NULL AFTER input_token_details_json`},
		{"provider_usage_json",
			`ALTER TABLE billing_usage_event ADD COLUMN provider_usage_json TEXT NULL AFTER output_token_details_json`},
		{"cache_creation_input_tokens",
			`ALTER TABLE billing_usage_event ADD COLUMN cache_creation_input_tokens BIGINT NOT NULL DEFAULT 0 AFTER output_tokens`},
		{"cache_creation_5m_input_tokens",
			`ALTER TABLE billing_usage_event ADD COLUMN cache_creation_5m_input_tokens BIGINT NOT NULL DEFAULT 0 AFTER cache_creation_input_tokens`},
		{"cache_creation_1h_input_tokens",
			`ALTER TABLE billing_usage_event ADD COLUMN cache_creation_1h_input_tokens BIGINT NOT NULL DEFAULT 0 AFTER cache_creation_5m_input_tokens`},
		{"cache_read_input_tokens",
			`ALTER TABLE billing_usage_event ADD COLUMN cache_read_input_tokens BIGINT NOT NULL DEFAULT 0 AFTER cache_creation_1h_input_tokens`},
		{"input_image_tokens",
			`ALTER TABLE billing_usage_event ADD COLUMN input_image_tokens BIGINT NOT NULL DEFAULT 0 AFTER cache_read_input_tokens`},
		{"output_image_tokens",
			`ALTER TABLE billing_usage_event ADD COLUMN output_image_tokens BIGINT NOT NULL DEFAULT 0 AFTER input_image_tokens`},
		{"input_image_count",
			`ALTER TABLE billing_usage_event ADD COLUMN input_image_count BIGINT NOT NULL DEFAULT 0 AFTER output_image_tokens`},
		{"output_image_count",
			`ALTER TABLE billing_usage_event ADD COLUMN output_image_count BIGINT NOT NULL DEFAULT 0 AFTER input_image_count`},
		{"request_count",
			`ALTER TABLE billing_usage_event ADD COLUMN request_count BIGINT NOT NULL DEFAULT 0 AFTER output_image_count`},
		{"cache_ttl",
			`ALTER TABLE billing_usage_event ADD COLUMN cache_ttl VARCHAR(8) NOT NULL DEFAULT '' AFTER request_count`},
		{"started_at", `ALTER TABLE billing_usage_event ADD COLUMN started_at DATETIME NULL AFTER price_version_id`},
		{"finished_at", `ALTER TABLE billing_usage_event ADD COLUMN finished_at DATETIME NULL AFTER started_at`},
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
			`ALTER TABLE portal_usage_daily ADD COLUMN department_id VARCHAR(64) NOT NULL DEFAULT '' AFTER consumer_name`},
		{"department_path",
			`ALTER TABLE portal_usage_daily ADD COLUMN department_path VARCHAR(512) NOT NULL DEFAULT '' AFTER department_id`},
		{"cache_creation_input_tokens",
			`ALTER TABLE portal_usage_daily ADD COLUMN cache_creation_input_tokens BIGINT NOT NULL DEFAULT 0 AFTER output_tokens`},
		{"cache_creation_5m_input_tokens",
			`ALTER TABLE portal_usage_daily ADD COLUMN cache_creation_5m_input_tokens BIGINT NOT NULL DEFAULT 0 AFTER cache_creation_input_tokens`},
		{"cache_creation_1h_input_tokens",
			`ALTER TABLE portal_usage_daily ADD COLUMN cache_creation_1h_input_tokens BIGINT NOT NULL DEFAULT 0 AFTER cache_creation_5m_input_tokens`},
		{"cache_read_input_tokens",
			`ALTER TABLE portal_usage_daily ADD COLUMN cache_read_input_tokens BIGINT NOT NULL DEFAULT 0 AFTER cache_creation_1h_input_tokens`},
		{"input_image_tokens",
			`ALTER TABLE portal_usage_daily ADD COLUMN input_image_tokens BIGINT NOT NULL DEFAULT 0 AFTER cache_read_input_tokens`},
		{"output_image_tokens",
			`ALTER TABLE portal_usage_daily ADD COLUMN output_image_tokens BIGINT NOT NULL DEFAULT 0 AFTER input_image_tokens`},
		{"input_image_count",
			`ALTER TABLE portal_usage_daily ADD COLUMN input_image_count BIGINT NOT NULL DEFAULT 0 AFTER output_image_tokens`},
		{"output_image_count",
			`ALTER TABLE portal_usage_daily ADD COLUMN output_image_count BIGINT NOT NULL DEFAULT 0 AFTER input_image_count`},
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
		{"request_kinds_json",
			`ALTER TABLE portal_model_asset ADD COLUMN request_kinds_json TEXT NULL AFTER features_json`},
	}
	for _, item := range changes {
		if err := s.ensureTableColumn(ctx, "portal_model_asset", item.column, item.sql); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureBillingModelPriceColumns(ctx context.Context) error {
	changes := []struct {
		column string
		sql    string
	}{
		{"input_request_price_micro_yuan",
			`ALTER TABLE billing_model_price_version ADD COLUMN input_request_price_micro_yuan BIGINT NOT NULL DEFAULT 0 AFTER output_price_per_1k_micro_yuan`},
		{"cache_creation_input_token_price_per_1k_micro_yuan",
			`ALTER TABLE billing_model_price_version ADD COLUMN cache_creation_input_token_price_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0 AFTER input_request_price_micro_yuan`},
		{"cache_creation_input_token_price_above_1hr_per_1k_micro_yuan",
			`ALTER TABLE billing_model_price_version ADD COLUMN cache_creation_input_token_price_above_1hr_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0 AFTER cache_creation_input_token_price_per_1k_micro_yuan`},
		{"cache_read_input_token_price_per_1k_micro_yuan",
			`ALTER TABLE billing_model_price_version ADD COLUMN cache_read_input_token_price_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0 AFTER cache_creation_input_token_price_above_1hr_per_1k_micro_yuan`},
		{"input_token_price_above_200k_per_1k_micro_yuan",
			`ALTER TABLE billing_model_price_version ADD COLUMN input_token_price_above_200k_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0 AFTER cache_read_input_token_price_per_1k_micro_yuan`},
		{"output_token_price_above_200k_per_1k_micro_yuan",
			`ALTER TABLE billing_model_price_version ADD COLUMN output_token_price_above_200k_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0 AFTER input_token_price_above_200k_per_1k_micro_yuan`},
		{"cache_creation_input_token_price_above_200k_per_1k_micro_yuan",
			`ALTER TABLE billing_model_price_version ADD COLUMN cache_creation_input_token_price_above_200k_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0 AFTER output_token_price_above_200k_per_1k_micro_yuan`},
		{"cache_read_input_token_price_above_200k_per_1k_micro_yuan",
			`ALTER TABLE billing_model_price_version ADD COLUMN cache_read_input_token_price_above_200k_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0 AFTER cache_creation_input_token_price_above_200k_per_1k_micro_yuan`},
		{"output_image_price_micro_yuan",
			`ALTER TABLE billing_model_price_version ADD COLUMN output_image_price_micro_yuan BIGINT NOT NULL DEFAULT 0 AFTER cache_read_input_token_price_above_200k_per_1k_micro_yuan`},
		{"output_image_token_price_per_1k_micro_yuan",
			`ALTER TABLE billing_model_price_version ADD COLUMN output_image_token_price_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0 AFTER output_image_price_micro_yuan`},
		{"input_image_price_micro_yuan",
			`ALTER TABLE billing_model_price_version ADD COLUMN input_image_price_micro_yuan BIGINT NOT NULL DEFAULT 0 AFTER output_image_token_price_per_1k_micro_yuan`},
		{"input_image_token_price_per_1k_micro_yuan",
			`ALTER TABLE billing_model_price_version ADD COLUMN input_image_token_price_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0 AFTER input_image_price_micro_yuan`},
		{"supports_prompt_caching",
			`ALTER TABLE billing_model_price_version ADD COLUMN supports_prompt_caching TINYINT(1) NOT NULL DEFAULT 0 AFTER input_image_token_price_per_1k_micro_yuan`},
	}
	for _, item := range changes {
		if err := s.ensureTableColumn(ctx, "billing_model_price_version", item.column, item.sql); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureTableColumn(ctx context.Context, tableName string, columnName string, alterSQL string) error {
	existed, err := s.db.GetValue(ctx, `
		SELECT COUNT(1)
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = ?
		  AND COLUMN_NAME = ?`, tableName, columnName)
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
	existed, err := s.db.GetValue(ctx, `
		SELECT COUNT(1)
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = ?
		  AND INDEX_NAME = ?`, tableName, indexName)
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
		{ID: "model-qwen-max", Name: "Qwen-Max", Vendor: "Alibaba Cloud", Capability: "general chat / coding", InputTokenPrice: 0.020000, OutputTokenPrice: 0.060000, Endpoint: "/v1/chat/completions", SDK: "OpenAI Compatible API", Summary: "High quality generation and complex reasoning."},
		{ID: "model-deepseek-v3", Name: "DeepSeek-V3", Vendor: "DeepSeek", Capability: "reasoning / low cost generation", InputTokenPrice: 0.008000, OutputTokenPrice: 0.018000, Endpoint: "/v1/chat/completions", SDK: "OpenAI Compatible API", Summary: "Balanced quality and cost for general use cases."},
		{ID: "model-higress-rerank", Name: "Higress-Rerank-1.0", Vendor: "Higress AI", Capability: "search rerank", InputTokenPrice: 0.004000, OutputTokenPrice: 0.000000, Endpoint: "/v1/rerank", SDK: "REST API", Summary: "Improve relevance in RAG retrieval ranking."},
	}

	for _, item := range seedModels {
		_, err := s.db.Exec(ctx, `
			INSERT INTO portal_model_catalog
			(model_id, name, vendor, capability, input_token_price, output_token_price, endpoint, sdk, summary, status)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active')
			ON DUPLICATE KEY UPDATE
			vendor = VALUES(vendor),
			capability = VALUES(capability),
			input_token_price = VALUES(input_token_price),
			output_token_price = VALUES(output_token_price),
			endpoint = VALUES(endpoint),
			sdk = VALUES(sdk),
			summary = VALUES(summary),
			status = 'active'`,
			item.ID,
			item.Name,
			item.Vendor,
			item.Capability,
			item.InputTokenPrice,
			item.OutputTokenPrice,
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
		_, err := s.db.Exec(ctx, `
			INSERT INTO portal_invite_code (invite_code, status, expires_at)
			VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE
			status = IF(status = ?, status, VALUES(status)),
			expires_at = IF(status = ?, expires_at, VALUES(expires_at))`,
			code,
			consts.InviteStatusActive,
			expiresAt,
			consts.InviteStatusDisabled,
			consts.InviteStatusDisabled,
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

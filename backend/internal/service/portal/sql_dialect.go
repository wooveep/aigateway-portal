package portal

import (
	"fmt"
	"strings"
)

func (s *Service) sqlForDriver(postgresSQL string) string {
	return postgresSQL
}

func (s *Service) epochTimestampLiteral() string {
	return "TIMESTAMP '1970-01-01 00:00:00'"
}

func (s *Service) assignExcluded(column string) string {
	return fmt.Sprintf("%s = EXCLUDED.%s", column, column)
}

func (s *Service) upsertClause(conflictColumns []string, assignments ...string) string {
	return fmt.Sprintf("ON CONFLICT (%s) DO UPDATE SET %s", strings.Join(conflictColumns, ", "), strings.Join(assignments, ", "))
}

func (s *Service) upsertAdd(tableName string, column string) string {
	return fmt.Sprintf("%s = %s.%s + EXCLUDED.%s", column, tableName, column, column)
}

func (s *Service) upsertPreserveNonEmpty(tableName string, column string) string {
	return fmt.Sprintf("%s = CASE WHEN EXCLUDED.%s <> '' THEN EXCLUDED.%s ELSE %s.%s END", column, column, column, tableName, column)
}

func (s *Service) upsertLeastTimestamp(tableName string, column string) string {
	return fmt.Sprintf("%s = CASE WHEN %s.%s IS NULL OR EXCLUDED.%s < %s.%s THEN EXCLUDED.%s ELSE %s.%s END", column, tableName, column, column, tableName, column, column, tableName, column)
}

func (s *Service) upsertGreatestTimestamp(tableName string, column string) string {
	return fmt.Sprintf("%s = CASE WHEN %s.%s IS NULL OR EXCLUDED.%s > %s.%s THEN EXCLUDED.%s ELSE %s.%s END", column, tableName, column, column, tableName, column, column, tableName, column)
}

func (s *Service) tableColumnExistsQuery() string {
	return `
		SELECT COUNT(1)
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = current_schema()
		  AND TABLE_NAME = ?
		  AND COLUMN_NAME = ?`
}

func (s *Service) tableIndexExistsQuery() string {
	return `
		SELECT COUNT(1)
		FROM pg_indexes
		WHERE schemaname = current_schema()
		  AND tablename = ?
		  AND indexname = ?`
}

func (s *Service) portalMigrationDDLs() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS portal_session (
			session_token VARCHAR(96) PRIMARY KEY,
			consumer_name VARCHAR(128) NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_seen_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_session_consumer ON portal_session (consumer_name)`,
		`CREATE INDEX IF NOT EXISTS idx_session_expire ON portal_session (expires_at)`,
		`CREATE TABLE IF NOT EXISTS portal_api_key (
			id BIGSERIAL PRIMARY KEY,
			key_id VARCHAR(64) NOT NULL UNIQUE,
			consumer_name VARCHAR(128) NOT NULL,
			name VARCHAR(128) NOT NULL,
			key_masked VARCHAR(128) NOT NULL,
			key_hash VARCHAR(128) NOT NULL,
			raw_key VARCHAR(256) NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'active',
			total_calls BIGINT NOT NULL DEFAULT 0,
			last_used_at TIMESTAMP NULL,
			expires_at TIMESTAMP NULL,
			deleted_at TIMESTAMP NULL,
			limit_total_micro_yuan BIGINT NOT NULL DEFAULT 0,
			limit_5h_micro_yuan BIGINT NOT NULL DEFAULT 0,
			limit_daily_micro_yuan BIGINT NOT NULL DEFAULT 0,
			daily_reset_mode VARCHAR(16) NOT NULL DEFAULT 'fixed',
			daily_reset_time VARCHAR(5) NOT NULL DEFAULT '00:00',
			limit_weekly_micro_yuan BIGINT NOT NULL DEFAULT 0,
			limit_monthly_micro_yuan BIGINT NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_api_key_consumer_status ON portal_api_key (consumer_name, status)`,
		`CREATE TABLE IF NOT EXISTS portal_model_catalog (
			id BIGSERIAL PRIMARY KEY,
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
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS portal_usage_daily (
			id BIGSERIAL PRIMARY KEY,
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
			source_from TIMESTAMP NULL,
			source_to TIMESTAMP NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT uk_usage_consumer_model_date UNIQUE (billing_date, consumer_name, model_name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_portal_usage_daily_department_date ON portal_usage_daily (department_id, billing_date)`,
		`CREATE TABLE IF NOT EXISTS portal_recharge_order (
			id BIGSERIAL PRIMARY KEY,
			order_id VARCHAR(64) NOT NULL UNIQUE,
			consumer_name VARCHAR(128) NOT NULL,
			amount DECIMAL(18,2) NOT NULL,
			channel VARCHAR(32) NOT NULL,
			status VARCHAR(16) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_recharge_consumer ON portal_recharge_order (consumer_name)`,
		`CREATE TABLE IF NOT EXISTS portal_balance_adjustment (
			id BIGSERIAL PRIMARY KEY,
			adjustment_id VARCHAR(64) NOT NULL UNIQUE,
			operator_consumer_name VARCHAR(128) NOT NULL,
			target_consumer_name VARCHAR(128) NOT NULL,
			delta_micro_yuan BIGINT NOT NULL,
			reason VARCHAR(255) NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_balance_adjustment_target ON portal_balance_adjustment (target_consumer_name, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_balance_adjustment_operator ON portal_balance_adjustment (operator_consumer_name, created_at)`,
		`CREATE TABLE IF NOT EXISTS portal_invoice_profile (
			id BIGSERIAL PRIMARY KEY,
			consumer_name VARCHAR(128) NOT NULL UNIQUE,
			company_name VARCHAR(255) NOT NULL DEFAULT '',
			tax_no VARCHAR(128) NOT NULL DEFAULT '',
			address VARCHAR(255) NOT NULL DEFAULT '',
			bank_account VARCHAR(255) NOT NULL DEFAULT '',
			receiver VARCHAR(128) NOT NULL DEFAULT '',
			email VARCHAR(255) NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS portal_invoice_record (
			id BIGSERIAL PRIMARY KEY,
			invoice_id VARCHAR(64) NOT NULL UNIQUE,
			consumer_name VARCHAR(128) NOT NULL,
			title VARCHAR(255) NOT NULL,
			tax_no VARCHAR(128) NOT NULL,
			amount DECIMAL(18,2) NOT NULL,
			status VARCHAR(16) NOT NULL,
			remark VARCHAR(512) NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_invoice_consumer ON portal_invoice_record (consumer_name)`,
		`CREATE TABLE IF NOT EXISTS billing_wallet (
			id BIGSERIAL PRIMARY KEY,
			consumer_name VARCHAR(128) NOT NULL UNIQUE,
			currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
			available_micro_yuan BIGINT NOT NULL DEFAULT 0,
			version BIGINT NOT NULL DEFAULT 1,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_billing_wallet_currency ON billing_wallet (currency)`,
		`CREATE TABLE IF NOT EXISTS billing_transaction (
			id BIGSERIAL PRIMARY KEY,
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
			occurred_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT uk_billing_transaction_source UNIQUE (source_type, source_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_billing_transaction_consumer_time ON billing_transaction (consumer_name, occurred_at)`,
		`CREATE INDEX IF NOT EXISTS idx_billing_transaction_type ON billing_transaction (tx_type)`,
		`CREATE TABLE IF NOT EXISTS billing_usage_event (
			id BIGSERIAL PRIMARY KEY,
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
			started_at TIMESTAMP NULL,
			finished_at TIMESTAMP NULL,
			redis_stream_id VARCHAR(128) NOT NULL DEFAULT '',
			occurred_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_billing_usage_event_consumer_time ON billing_usage_event (consumer_name, occurred_at)`,
		`CREATE INDEX IF NOT EXISTS idx_billing_usage_event_consumer_model_time ON billing_usage_event (consumer_name, model_id, occurred_at)`,
		`CREATE INDEX IF NOT EXISTS idx_billing_usage_event_api_key_time ON billing_usage_event (api_key_id, occurred_at)`,
		`CREATE INDEX IF NOT EXISTS idx_billing_usage_event_department_time ON billing_usage_event (department_id, occurred_at)`,
		`CREATE TABLE IF NOT EXISTS billing_model_catalog (
			id BIGSERIAL PRIMARY KEY,
			model_id VARCHAR(128) NOT NULL UNIQUE,
			name VARCHAR(128) NOT NULL,
			vendor VARCHAR(128) NOT NULL,
			capability VARCHAR(255) NOT NULL,
			endpoint VARCHAR(255) NOT NULL,
			sdk VARCHAR(128) NOT NULL,
			summary TEXT NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'active',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_billing_model_status ON billing_model_catalog (status)`,
		`CREATE TABLE IF NOT EXISTS portal_ai_chat_session (
			id BIGSERIAL PRIMARY KEY,
			session_id VARCHAR(128) NOT NULL UNIQUE,
			consumer_name VARCHAR(128) NOT NULL,
			operator_consumer_name VARCHAR(128) NOT NULL,
			title VARCHAR(255) NOT NULL,
			default_model_id VARCHAR(128) NOT NULL DEFAULT '',
			default_api_key_id VARCHAR(64) NOT NULL DEFAULT '',
			last_message_preview VARCHAR(512) NOT NULL DEFAULT '',
			last_message_at TIMESTAMP NULL,
			deleted_at TIMESTAMP NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_portal_ai_chat_session_consumer ON portal_ai_chat_session (consumer_name, deleted_at, last_message_at)`,
		`CREATE INDEX IF NOT EXISTS idx_portal_ai_chat_session_operator ON portal_ai_chat_session (operator_consumer_name, deleted_at)`,
		`CREATE TABLE IF NOT EXISTS portal_ai_chat_message (
			id BIGSERIAL PRIMARY KEY,
			message_id VARCHAR(128) NOT NULL UNIQUE,
			session_id VARCHAR(128) NOT NULL,
			role VARCHAR(16) NOT NULL,
			content TEXT NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'succeeded',
			model_id VARCHAR(128) NOT NULL DEFAULT '',
			api_key_id VARCHAR(64) NOT NULL DEFAULT '',
			request_id VARCHAR(128) NOT NULL DEFAULT '',
			trace_id VARCHAR(128) NOT NULL DEFAULT '',
			http_status INT NOT NULL DEFAULT 0,
			error_message VARCHAR(1024) NOT NULL DEFAULT '',
			finished_at TIMESTAMP NULL,
			deleted_at TIMESTAMP NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_portal_ai_chat_message_session ON portal_ai_chat_message (session_id, deleted_at, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_portal_ai_chat_message_request ON portal_ai_chat_message (request_id)`,
		`CREATE TABLE IF NOT EXISTS billing_model_price_version (
			id BIGSERIAL PRIMARY KEY,
			model_id VARCHAR(128) NOT NULL,
			currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
			input_price_micro_yuan_per_token BIGINT NOT NULL DEFAULT 0,
			output_price_micro_yuan_per_token BIGINT NOT NULL DEFAULT 0,
			input_price_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			output_price_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			input_request_price_micro_yuan BIGINT NOT NULL DEFAULT 0,
			cache_creation_input_token_price_micro_yuan_per_token BIGINT NOT NULL DEFAULT 0,
			cache_creation_input_token_price_above_1hr_micro_yuan_per_token BIGINT NOT NULL DEFAULT 0,
			cache_read_input_token_price_micro_yuan_per_token BIGINT NOT NULL DEFAULT 0,
			input_token_price_above_200k_micro_yuan_per_token BIGINT NOT NULL DEFAULT 0,
			output_token_price_above_200k_micro_yuan_per_token BIGINT NOT NULL DEFAULT 0,
			cache_creation_input_token_price_above_200k_micro_yuan_per_token BIGINT NOT NULL DEFAULT 0,
			cache_read_input_token_price_above_200k_micro_yuan_per_token BIGINT NOT NULL DEFAULT 0,
			cache_creation_input_token_price_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			cache_creation_input_token_price_above_1hr_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			cache_read_input_token_price_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			input_token_price_above_200k_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			output_token_price_above_200k_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			cache_creation_input_token_price_above_200k_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			cache_read_input_token_price_above_200k_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			output_image_price_micro_yuan BIGINT NOT NULL DEFAULT 0,
			output_image_token_price_micro_yuan_per_token BIGINT NOT NULL DEFAULT 0,
			output_image_token_price_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			input_image_price_micro_yuan BIGINT NOT NULL DEFAULT 0,
			input_image_token_price_micro_yuan_per_token BIGINT NOT NULL DEFAULT 0,
			input_image_token_price_per_1k_micro_yuan BIGINT NOT NULL DEFAULT 0,
			supports_prompt_caching BOOLEAN NOT NULL DEFAULT FALSE,
			effective_from TIMESTAMP NOT NULL,
			effective_to TIMESTAMP NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'active',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_billing_model_price_active ON billing_model_price_version (model_id, status, effective_to)`,
		`CREATE INDEX IF NOT EXISTS idx_billing_model_price_time ON billing_model_price_version (effective_from)`,
	}
}

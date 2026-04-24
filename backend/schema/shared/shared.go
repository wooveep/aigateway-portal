package shared

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
)

type columnMigration struct {
	table  string
	column string
	ddl    string
}

var tableDDLs = []string{
	`CREATE TABLE IF NOT EXISTS portal_user (
		id BIGSERIAL PRIMARY KEY,
		consumer_name VARCHAR(128) NOT NULL UNIQUE,
		display_name VARCHAR(128) NOT NULL,
		email VARCHAR(255) NOT NULL DEFAULT '',
		password_hash VARCHAR(255) NOT NULL,
		status VARCHAR(16) NOT NULL DEFAULT 'active',
		source VARCHAR(16) NOT NULL DEFAULT 'portal',
		user_level VARCHAR(16) NOT NULL DEFAULT 'normal',
		is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
		deleted_at TIMESTAMP NULL,
		last_login_at TIMESTAMP NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS portal_invite_code (
		id BIGSERIAL PRIMARY KEY,
		invite_code VARCHAR(64) NOT NULL UNIQUE,
		status VARCHAR(16) NOT NULL DEFAULT 'active',
		expires_at TIMESTAMP NOT NULL,
		used_by_consumer VARCHAR(128) NULL,
		used_at TIMESTAMP NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS org_department (
		id BIGSERIAL PRIMARY KEY,
		department_id VARCHAR(64) NOT NULL UNIQUE,
		name VARCHAR(128) NOT NULL,
		parent_department_id VARCHAR(64) NULL,
		admin_consumer_name VARCHAR(128) NULL,
		path VARCHAR(512) NOT NULL,
		level INT NOT NULL DEFAULT 0,
		sort_order INT NOT NULL DEFAULT 0,
		status VARCHAR(16) NOT NULL DEFAULT 'active',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT uk_org_department_admin_consumer UNIQUE (admin_consumer_name)
	)`,
	`CREATE TABLE IF NOT EXISTS org_account_membership (
		id BIGSERIAL PRIMARY KEY,
		consumer_name VARCHAR(128) NOT NULL UNIQUE,
		department_id VARCHAR(64) NULL,
		parent_consumer_name VARCHAR(128) NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS asset_grant (
		id BIGSERIAL PRIMARY KEY,
		asset_type VARCHAR(32) NOT NULL,
		asset_id VARCHAR(128) NOT NULL,
		subject_type VARCHAR(32) NOT NULL,
		subject_id VARCHAR(128) NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT uk_asset_grant_subject UNIQUE (asset_type, asset_id, subject_type, subject_id)
	)`,
	`CREATE TABLE IF NOT EXISTS quota_policy_user (
		id BIGSERIAL PRIMARY KEY,
		consumer_name VARCHAR(128) NOT NULL UNIQUE,
		limit_total_micro_yuan BIGINT NOT NULL DEFAULT 0,
		limit_5h_micro_yuan BIGINT NOT NULL DEFAULT 0,
		limit_daily_micro_yuan BIGINT NOT NULL DEFAULT 0,
		daily_reset_mode VARCHAR(16) NOT NULL DEFAULT 'fixed',
		daily_reset_time VARCHAR(5) NOT NULL DEFAULT '00:00',
		limit_weekly_micro_yuan BIGINT NOT NULL DEFAULT 0,
		limit_monthly_micro_yuan BIGINT NOT NULL DEFAULT 0,
		cost_reset_at TIMESTAMP NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS portal_model_asset (
		id BIGSERIAL PRIMARY KEY,
		asset_id VARCHAR(128) NOT NULL UNIQUE,
		canonical_name VARCHAR(128) NOT NULL UNIQUE,
		display_name VARCHAR(128) NOT NULL,
		intro TEXT NOT NULL,
		model_type VARCHAR(64) NULL,
		tags_json TEXT NULL,
		input_modalities_json TEXT NULL,
		output_modalities_json TEXT NULL,
		feature_flags_json TEXT NULL,
		modalities_json TEXT NULL,
		features_json TEXT NULL,
		request_kinds_json TEXT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS portal_model_binding (
		id BIGSERIAL PRIMARY KEY,
		binding_id VARCHAR(128) NOT NULL UNIQUE,
		asset_id VARCHAR(128) NOT NULL,
		model_id VARCHAR(128) NOT NULL UNIQUE,
		provider_name VARCHAR(128) NOT NULL,
		target_model VARCHAR(128) NOT NULL,
		protocol VARCHAR(128) NOT NULL DEFAULT 'openai/v1',
		endpoint VARCHAR(255) NOT NULL DEFAULT '-',
		pricing_json TEXT NOT NULL,
		limits_json TEXT NULL,
		rpm BIGINT NULL,
		tpm BIGINT NULL,
		context_window BIGINT NULL,
		status VARCHAR(16) NOT NULL DEFAULT 'draft',
		published_at TIMESTAMP NULL,
		unpublished_at TIMESTAMP NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT uk_portal_model_binding_target UNIQUE (asset_id, provider_name, target_model)
	)`,
	`CREATE TABLE IF NOT EXISTS portal_agent_catalog (
		id BIGSERIAL PRIMARY KEY,
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
		published_at TIMESTAMP NULL,
		unpublished_at TIMESTAMP NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS portal_sso_config (
		provider_type VARCHAR(16) PRIMARY KEY,
		enabled BOOLEAN NOT NULL DEFAULT FALSE,
		display_name VARCHAR(128) NOT NULL DEFAULT '',
		issuer_url VARCHAR(512) NOT NULL DEFAULT '',
		client_id VARCHAR(255) NOT NULL DEFAULT '',
		client_secret_encrypted TEXT NOT NULL DEFAULT '',
		scopes_json TEXT NULL,
		claim_mapping_json TEXT NULL,
		updated_by VARCHAR(128) NOT NULL DEFAULT '',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS portal_user_sso_identity (
		id BIGSERIAL PRIMARY KEY,
		provider_key VARCHAR(64) NOT NULL,
		issuer VARCHAR(512) NOT NULL,
		subject VARCHAR(512) NOT NULL,
		consumer_name VARCHAR(128) NOT NULL,
		email VARCHAR(255) NOT NULL DEFAULT '',
		email_verified BOOLEAN NOT NULL DEFAULT FALSE,
		display_name VARCHAR(255) NOT NULL DEFAULT '',
		claims_json TEXT NULL,
		linked_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_login_at TIMESTAMP NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT uk_portal_user_sso_identity_subject UNIQUE (provider_key, issuer, subject),
		CONSTRAINT uk_portal_user_sso_identity_consumer UNIQUE (provider_key, consumer_name)
	)`,
}

var columnDDLs = []columnMigration{
	{
		table:  "portal_user",
		column: "user_level",
		ddl:    `ALTER TABLE portal_user ADD COLUMN user_level VARCHAR(16) NOT NULL DEFAULT 'normal'`,
	},
	{
		table:  "portal_user",
		column: "is_deleted",
		ddl:    `ALTER TABLE portal_user ADD COLUMN is_deleted BOOLEAN NOT NULL DEFAULT FALSE`,
	},
	{
		table:  "portal_user",
		column: "deleted_at",
		ddl:    `ALTER TABLE portal_user ADD COLUMN deleted_at TIMESTAMP NULL`,
	},
	{
		table:  "portal_model_asset",
		column: "request_kinds_json",
		ddl:    `ALTER TABLE portal_model_asset ADD COLUMN request_kinds_json TEXT NULL`,
	},
	{
		table:  "portal_model_asset",
		column: "model_type",
		ddl:    `ALTER TABLE portal_model_asset ADD COLUMN model_type VARCHAR(64) NULL`,
	},
	{
		table:  "portal_model_asset",
		column: "input_modalities_json",
		ddl:    `ALTER TABLE portal_model_asset ADD COLUMN input_modalities_json TEXT NULL`,
	},
	{
		table:  "portal_model_asset",
		column: "output_modalities_json",
		ddl:    `ALTER TABLE portal_model_asset ADD COLUMN output_modalities_json TEXT NULL`,
	},
	{
		table:  "portal_model_asset",
		column: "feature_flags_json",
		ddl:    `ALTER TABLE portal_model_asset ADD COLUMN feature_flags_json TEXT NULL`,
	},
	{
		table:  "portal_model_binding",
		column: "limits_json",
		ddl:    `ALTER TABLE portal_model_binding ADD COLUMN limits_json TEXT NULL`,
	},
	{
		table:  "org_department",
		column: "admin_consumer_name",
		ddl:    `ALTER TABLE org_department ADD COLUMN admin_consumer_name VARCHAR(128) NULL`,
	},
}

var rawAdjustments = []string{
	`ALTER TABLE org_department ADD CONSTRAINT uk_org_department_admin_consumer UNIQUE (admin_consumer_name)`,
	`CREATE INDEX IF NOT EXISTS idx_org_department_parent ON org_department (parent_department_id)`,
	`CREATE INDEX IF NOT EXISTS idx_org_department_status ON org_department (status)`,
	`CREATE INDEX IF NOT EXISTS idx_org_account_department ON org_account_membership (department_id)`,
	`CREATE INDEX IF NOT EXISTS idx_org_account_parent ON org_account_membership (parent_consumer_name)`,
	`UPDATE org_account_membership SET parent_consumer_name = NULL WHERE parent_consumer_name IS NOT NULL`,
	`CREATE INDEX IF NOT EXISTS idx_asset_grant_asset ON asset_grant (asset_type, asset_id)`,
	`CREATE INDEX IF NOT EXISTS idx_asset_grant_subject_lookup ON asset_grant (subject_type, subject_id)`,
	`CREATE INDEX IF NOT EXISTS idx_portal_model_asset_display_name ON portal_model_asset (display_name)`,
	`CREATE INDEX IF NOT EXISTS idx_portal_model_binding_asset ON portal_model_binding (asset_id)`,
	`CREATE INDEX IF NOT EXISTS idx_portal_model_binding_status ON portal_model_binding (status)`,
	`CREATE INDEX IF NOT EXISTS idx_portal_model_binding_provider ON portal_model_binding (provider_name)`,
	`CREATE INDEX IF NOT EXISTS idx_portal_agent_catalog_status ON portal_agent_catalog (status)`,
	`CREATE INDEX IF NOT EXISTS idx_portal_agent_catalog_display_name ON portal_agent_catalog (display_name)`,
	`CREATE INDEX IF NOT EXISTS idx_portal_user_sso_identity_consumer_name ON portal_user_sso_identity (consumer_name)`,
	`CREATE INDEX IF NOT EXISTS idx_portal_user_sso_identity_email ON portal_user_sso_identity (email)`,
}

func ApplyToGDB(ctx context.Context, db gdb.DB) error {
	return ApplyToGDBWithDriver(ctx, db, "postgres")
}

func ApplyToGDBWithDriver(ctx context.Context, db gdb.DB, driver string) error {
	if !IsPostgresDriver(driver) {
		return gerror.New("shared schema only supports PostgreSQL")
	}
	for _, ddl := range tableDDLs {
		if _, err := db.Exec(ctx, ddl); err != nil {
			return gerror.Wrap(err, "shared schema migration failed")
		}
	}
	for _, item := range columnDDLs {
		if err := ensureGDBColumn(ctx, db, driver, item.table, item.column, item.ddl); err != nil {
			return err
		}
	}
	for _, ddl := range rawAdjustments {
		if _, err := db.Exec(ctx, ddl); err != nil {
			// best effort to preserve compatibility across already-migrated DBs
		}
	}
	return nil
}

func ApplyToSQL(ctx context.Context, db *sql.DB) error {
	return ApplyToSQLWithDriver(ctx, db, "postgres")
}

func ApplyToSQLWithDriver(ctx context.Context, db *sql.DB, driver string) error {
	if !IsPostgresDriver(driver) {
		return gerror.New("shared schema only supports PostgreSQL")
	}
	for _, ddl := range tableDDLs {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			return err
		}
	}
	for _, item := range columnDDLs {
		if err := ensureSQLColumn(ctx, db, driver, item.table, item.column, item.ddl); err != nil {
			return err
		}
	}
	for _, ddl := range rawAdjustments {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			// best effort to preserve compatibility across already-migrated DBs
		}
	}
	return nil
}

func RequiredTables() []string {
	return []string{
		"portal_user",
		"portal_invite_code",
		"org_department",
		"org_account_membership",
		"asset_grant",
		"quota_policy_user",
		"portal_model_asset",
		"portal_model_binding",
		"portal_agent_catalog",
		"portal_sso_config",
		"portal_user_sso_identity",
	}
}

func IsPostgresDriver(driver string) bool {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "postgres", "postgresql", "pgx", "pgsql", "pgx-rebind":
		return true
	default:
		return false
	}
}

func ensureGDBColumn(ctx context.Context, db gdb.DB, driver, table, column, ddl string) error {
	value, err := db.GetValue(ctx, columnExistenceQuery(driver), table, column)
	if err != nil {
		return gerror.Wrap(err, "query shared schema column existence failed")
	}
	if value.Int() > 0 {
		return nil
	}
	_, err = db.Exec(ctx, ddl)
	return gerror.Wrap(err, "apply shared schema column migration failed")
}

func ensureSQLColumn(ctx context.Context, db *sql.DB, driver, table, column, ddl string) error {
	var count int
	if err := db.QueryRowContext(ctx, rebindSQL(driver, columnExistenceQuery(driver)), table, column).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := db.ExecContext(ctx, ddl)
	return err
}

func columnExistenceQuery(driver string) string {
	if !IsPostgresDriver(driver) {
		panic("shared schema only supports PostgreSQL")
	}
	return `
		SELECT COUNT(1)
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = current_schema()
		  AND TABLE_NAME = ?
		  AND COLUMN_NAME = ?`
}

func rebindSQL(driver, query string) string {
	if !IsPostgresDriver(driver) {
		panic("shared schema only supports PostgreSQL")
	}
	var (
		builder        strings.Builder
		index          int
		inSingleQuotes bool
	)
	for i := 0; i < len(query); i++ {
		ch := query[i]
		if ch == '\'' {
			if inSingleQuotes && i+1 < len(query) && query[i+1] == '\'' {
				builder.WriteByte(ch)
				builder.WriteByte(query[i+1])
				i++
				continue
			}
			inSingleQuotes = !inSingleQuotes
			builder.WriteByte(ch)
			continue
		}
		if ch == '?' && !inSingleQuotes {
			index++
			builder.WriteByte('$')
			builder.WriteString(sqlIndex(index))
			continue
		}
		builder.WriteByte(ch)
	}
	return builder.String()
}

func sqlIndex(index int) string {
	return strconv.Itoa(index)
}

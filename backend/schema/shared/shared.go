package shared

import (
	"context"
	"database/sql"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
)

var tableDDLs = []string{
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
	`CREATE TABLE IF NOT EXISTS org_department (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		department_id VARCHAR(64) NOT NULL UNIQUE,
		name VARCHAR(128) NOT NULL,
		parent_department_id VARCHAR(64) NULL,
		admin_consumer_name VARCHAR(128) NULL,
		path VARCHAR(512) NOT NULL,
		level INT NOT NULL DEFAULT 0,
		sort_order INT NOT NULL DEFAULT 0,
		status VARCHAR(16) NOT NULL DEFAULT 'active',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		INDEX idx_org_department_parent (parent_department_id),
		INDEX idx_org_department_status (status),
		UNIQUE KEY uk_org_department_admin_consumer (admin_consumer_name)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS org_account_membership (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		consumer_name VARCHAR(128) NOT NULL UNIQUE,
		department_id VARCHAR(64) NULL,
		parent_consumer_name VARCHAR(128) NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		INDEX idx_org_account_department (department_id),
		INDEX idx_org_account_parent (parent_consumer_name)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS asset_grant (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		asset_type VARCHAR(32) NOT NULL,
		asset_id VARCHAR(128) NOT NULL,
		subject_type VARCHAR(32) NOT NULL,
		subject_id VARCHAR(128) NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		UNIQUE KEY uk_asset_grant_subject (asset_type, asset_id, subject_type, subject_id),
		INDEX idx_asset_grant_asset (asset_type, asset_id),
		INDEX idx_asset_grant_subject_lookup (subject_type, subject_id)
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
}

var columnDDLs = []struct {
	table  string
	column string
	sql    string
}{
	{"portal_user", "user_level", `ALTER TABLE portal_user ADD COLUMN user_level VARCHAR(16) NOT NULL DEFAULT 'normal'`},
	{"portal_user", "is_deleted", `ALTER TABLE portal_user ADD COLUMN is_deleted TINYINT(1) NOT NULL DEFAULT 0 AFTER user_level`},
	{"portal_user", "deleted_at", `ALTER TABLE portal_user ADD COLUMN deleted_at DATETIME NULL AFTER is_deleted`},
	{"portal_model_asset", "request_kinds_json", `ALTER TABLE portal_model_asset ADD COLUMN request_kinds_json TEXT NULL AFTER features_json`},
	{"org_department", "admin_consumer_name", `ALTER TABLE org_department ADD COLUMN admin_consumer_name VARCHAR(128) NULL AFTER parent_department_id`},
}

var rawAdjustments = []string{
	`ALTER TABLE org_department ADD UNIQUE KEY uk_org_department_admin_consumer (admin_consumer_name)`,
}

func ApplyToGDB(ctx context.Context, db gdb.DB) error {
	for _, ddl := range tableDDLs {
		if _, err := db.Exec(ctx, ddl); err != nil {
			return gerror.Wrap(err, "shared schema migration failed")
		}
	}
	for _, item := range columnDDLs {
		if err := ensureGDBColumn(ctx, db, item.table, item.column, item.sql); err != nil {
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
	for _, ddl := range tableDDLs {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			return err
		}
	}
	for _, item := range columnDDLs {
		if err := ensureSQLColumn(ctx, db, item.table, item.column, item.sql); err != nil {
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
	}
}

func ensureGDBColumn(ctx context.Context, db gdb.DB, table, column, ddl string) error {
	value, err := db.GetValue(ctx, `
		SELECT COUNT(1)
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = ?
		  AND COLUMN_NAME = ?`, table, column)
	if err != nil {
		return gerror.Wrap(err, "query shared schema column existence failed")
	}
	if value.Int() > 0 {
		return nil
	}
	_, err = db.Exec(ctx, ddl)
	return gerror.Wrap(err, "apply shared schema column migration failed")
}

func ensureSQLColumn(ctx context.Context, db *sql.DB, table, column, ddl string) error {
	var count int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = ?
		  AND COLUMN_NAME = ?`, table, column).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := db.ExecContext(ctx, ddl)
	return err
}

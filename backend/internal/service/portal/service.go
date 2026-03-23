package portal

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"net/http"
	"strings"
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
	cfg        config.Config
	db         gdb.DB
	httpClient *http.Client
	modelK8s   *clientK8s.Client
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
		modelK8s: clientK8s.New(cfg),
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
			department VARCHAR(128) NOT NULL DEFAULT '',
			password_hash VARCHAR(255) NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'active',
			source VARCHAR(16) NOT NULL DEFAULT 'portal',
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
			model_name VARCHAR(128) NOT NULL,
			request_count BIGINT NOT NULL DEFAULT 0,
			input_tokens BIGINT NOT NULL DEFAULT 0,
			output_tokens BIGINT NOT NULL DEFAULT 0,
			total_tokens BIGINT NOT NULL DEFAULT 0,
			cost_amount DECIMAL(18,6) NOT NULL DEFAULT 0,
			source_from DATETIME NULL,
			source_to DATETIME NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_usage_consumer_model_date (billing_date, consumer_name, model_name)
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
	}

	for _, ddl := range migrations {
		if _, err := s.db.Exec(ctx, ddl); err != nil {
			return gerror.Wrap(err, "migration failed")
		}
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

package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

const ctxUserKey = "authUser"

type App struct {
	cfg     Config
	db      *sql.DB
	console *ConsoleClient
	logger  *log.Logger
	router  *gin.Engine
}

type portalUserRow struct {
	PortalUser
	PasswordHash string
}

type apiKeyRow struct {
	KeyID      string
	Name       string
	RawKey     string
	Masked     string
	Status     string
	TotalCalls int64
	LastUsedAt *time.Time
	CreatedAt  time.Time
}

func NewApp(cfg Config) (*App, error) {
	db, err := sql.Open("mysql", cfg.MySQLDSN)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect mysql: %w", err)
	}

	app := &App{
		cfg:     cfg,
		db:      db,
		console: NewConsoleClient(cfg),
		logger:  log.Default(),
	}

	if err := app.runMigrations(context.Background()); err != nil {
		return nil, err
	}
	if err := app.seedBootstrapData(context.Background()); err != nil {
		return nil, err
	}

	app.router = app.buildRouter()
	return app, nil
}

func (a *App) Close() error {
	if a.db != nil {
		return a.db.Close()
	}
	return nil
}

func (a *App) buildRouter() *gin.Engine {
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	api := r.Group("/api")
	api.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "aigateway-portal-backend"})
	})

	auth := api.Group("/auth")
	auth.POST("/register", a.handleRegister)
	auth.POST("/login", a.handleLogin)
	auth.POST("/logout", a.handleLogout)
	auth.GET("/me", a.authMiddleware(), a.handleMe)

	biz := api.Group("/")
	biz.Use(a.authMiddleware())
	biz.GET("/billing/overview", a.handleBillingOverview)
	biz.GET("/billing/consumptions", a.handleConsumptions)
	biz.GET("/billing/recharges", a.handleRecharges)
	biz.POST("/billing/recharges", a.handleCreateRecharge)

	biz.GET("/models", a.handleListModels)
	biz.GET("/models/:id", a.handleModelDetail)

	biz.GET("/open-platform/keys", a.handleListAPIKeys)
	biz.POST("/open-platform/keys", a.handleCreateAPIKey)
	biz.PATCH("/open-platform/keys/:id/status", a.handleUpdateAPIKeyStatus)
	biz.DELETE("/open-platform/keys/:id", a.handleDeleteAPIKey)
	biz.GET("/open-platform/stats", a.handleOpenStats)
	biz.GET("/open-platform/cost-details", a.handleCostDetails)

	biz.GET("/invoices/profile", a.handleGetInvoiceProfile)
	biz.PUT("/invoices/profile", a.handleUpdateInvoiceProfile)
	biz.GET("/invoices/records", a.handleInvoiceRecords)
	biz.POST("/invoices/records", a.handleCreateInvoice)

	a.mountFrontend(r)

	return r
}

func (a *App) mountFrontend(r *gin.Engine) {
	webRoot := strings.TrimSpace(a.cfg.WebRoot)
	if webRoot == "" {
		return
	}

	info, err := os.Stat(webRoot)
	if err != nil || !info.IsDir() {
		a.logger.Printf("portal frontend root %q not available, skip static hosting", webRoot)
		return
	}

	indexFile := filepath.Join(webRoot, "index.html")
	if _, err := os.Stat(indexFile); err != nil {
		a.logger.Printf("portal frontend index %q not found, skip static hosting", indexFile)
		return
	}

	assetsDir := filepath.Join(webRoot, "assets")
	if assetsInfo, err := os.Stat(assetsDir); err == nil && assetsInfo.IsDir() {
		r.Static("/assets", assetsDir)
	}

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"message": "not found"})
			return
		}

		cleaned := filepath.Clean(c.Request.URL.Path)
		if cleaned == "." || cleaned == "/" {
			c.File(indexFile)
			return
		}

		candidate := filepath.Join(webRoot, strings.TrimPrefix(cleaned, "/"))
		if fileInfo, err := os.Stat(candidate); err == nil && !fileInfo.IsDir() {
			c.File(candidate)
			return
		}

		c.File(indexFile)
	})
}

func (a *App) runMigrations(ctx context.Context) error {
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
		if _, err := a.db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	return nil
}

func (a *App) seedBootstrapData(ctx context.Context) error {
	seedModels := []ModelInfo{
		{ID: "model-qwen-max", Name: "Qwen-Max", Vendor: "Alibaba Cloud", Capability: "general chat / coding", InputTokenPrice: 0.020000, OutputTokenPrice: 0.060000, Endpoint: "/v1/chat/completions", SDK: "OpenAI Compatible API", Summary: "High quality generation and complex reasoning."},
		{ID: "model-deepseek-v3", Name: "DeepSeek-V3", Vendor: "DeepSeek", Capability: "reasoning / low cost generation", InputTokenPrice: 0.008000, OutputTokenPrice: 0.018000, Endpoint: "/v1/chat/completions", SDK: "OpenAI Compatible API", Summary: "Balanced quality and cost for general use cases."},
		{ID: "model-higress-rerank", Name: "Higress-Rerank-1.0", Vendor: "Higress AI", Capability: "search rerank", InputTokenPrice: 0.004000, OutputTokenPrice: 0.000000, Endpoint: "/v1/rerank", SDK: "REST API", Summary: "Improve relevance in RAG retrieval ranking."},
	}

	for _, model := range seedModels {
		_, err := a.db.ExecContext(ctx, `
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
			model.ID, model.Name, model.Vendor, model.Capability,
			model.InputTokenPrice, model.OutputTokenPrice,
			model.Endpoint, model.SDK, model.Summary,
		)
		if err != nil {
			return err
		}
	}

	expiresAt := time.Now().AddDate(0, 0, a.cfg.InviteExpireDays)
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO portal_invite_code (invite_code, status, expires_at)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE
		status = IF(status = 'used', status, VALUES(status)),
		expires_at = IF(status = 'used', expires_at, VALUES(expires_at))`,
		a.cfg.InviteCode, inviteStatusActive, expiresAt,
	)
	return err
}

func hashPassword(raw string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func comparePassword(hash string, raw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(raw)) == nil
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

func (a *App) getUserByName(ctx context.Context, consumerName string) (*portalUserRow, error) {
	row := a.db.QueryRowContext(ctx, `
		SELECT consumer_name, display_name, email, department, status, source, password_hash, last_login_at
		FROM portal_user WHERE consumer_name = ?`, consumerName)
	var user portalUserRow
	var lastLoginAt sql.NullTime
	if err := row.Scan(
		&user.ConsumerName,
		&user.DisplayName,
		&user.Email,
		&user.Department,
		&user.Status,
		&user.Source,
		&user.PasswordHash,
		&lastLoginAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if lastLoginAt.Valid {
		t := lastLoginAt.Time
		user.LastLoginAt = &t
	}
	return &user, nil
}

func (a *App) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(a.cfg.SessionCookieName)
		if err != nil || token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
			c.Abort()
			return
		}

		row := a.db.QueryRowContext(c.Request.Context(), `
			SELECT u.consumer_name, u.display_name, u.email, u.department, u.status
			FROM portal_session s
			JOIN portal_user u ON u.consumer_name = s.consumer_name
			WHERE s.session_token = ? AND s.expires_at > NOW()`, token)
		var user AuthUser
		if err := row.Scan(&user.ConsumerName, &user.DisplayName, &user.Email, &user.Department, &user.Status); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
			c.Abort()
			return
		}
		if user.Status != userStatusActive {
			c.JSON(http.StatusForbidden, gin.H{"message": "account disabled"})
			c.Abort()
			return
		}

		_, _ = a.db.ExecContext(c.Request.Context(), `UPDATE portal_session SET last_seen_at = NOW() WHERE session_token = ?`, token)
		c.Set(ctxUserKey, user)
		c.Next()
	}
}

func getAuthUser(c *gin.Context) AuthUser {
	v, _ := c.Get(ctxUserKey)
	user, _ := v.(AuthUser)
	return user
}

func (a *App) saveSession(c *gin.Context, consumerName string) error {
	token := "sess_" + sha256Hex(fmt.Sprintf("%s:%s:%d:%s",
		consumerName, randomString(24), time.Now().UnixNano(), a.cfg.SessionSecret))[:48]
	expireAt := time.Now().Add(a.cfg.SessionTTL)
	if _, err := a.db.ExecContext(c.Request.Context(), `
		INSERT INTO portal_session (session_token, consumer_name, expires_at)
		VALUES (?, ?, ?)`, token, consumerName, expireAt); err != nil {
		return err
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     a.cfg.SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cfg.SessionSecureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(a.cfg.SessionTTL.Seconds()),
	})
	return nil
}

func (a *App) clearSession(c *gin.Context) {
	token, _ := c.Cookie(a.cfg.SessionCookieName)
	if token != "" {
		_, _ = a.db.ExecContext(c.Request.Context(), `DELETE FROM portal_session WHERE session_token = ?`, token)
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     a.cfg.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cfg.SessionSecureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (a *App) listAPIKeys(ctx context.Context, consumerName string) ([]apiKeyRow, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT key_id, name, raw_key, key_masked, status, total_calls, last_used_at, created_at
		FROM portal_api_key
		WHERE consumer_name = ?
		ORDER BY created_at DESC`, consumerName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []apiKeyRow
	for rows.Next() {
		var item apiKeyRow
		var lastUsedAt sql.NullTime
		if err := rows.Scan(
			&item.KeyID,
			&item.Name,
			&item.RawKey,
			&item.Masked,
			&item.Status,
			&item.TotalCalls,
			&lastUsedAt,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		if lastUsedAt.Valid {
			t := lastUsedAt.Time
			item.LastUsedAt = &t
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (a *App) activeRawKeys(ctx context.Context, consumerName string) ([]string, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT raw_key FROM portal_api_key
		WHERE consumer_name = ? AND status = 'active'
		ORDER BY created_at ASC`, consumerName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		if raw != "" {
			keys = append(keys, raw)
		}
	}
	return keys, rows.Err()
}

func ensureNonEmptyKeys(keys []string) []string {
	if len(keys) > 0 {
		sort.Strings(keys)
		return keys
	}
	return []string{"revoked_" + randomString(24)}
}

func (a *App) syncConsumerKeys(ctx context.Context, consumerName string, department string) error {
	keys, err := a.activeRawKeys(ctx, consumerName)
	if err != nil {
		return err
	}
	return a.console.UpsertConsumer(ctx, consumerName, department, ensureNonEmptyKeys(keys))
}

func (a *App) getModelPrices(ctx context.Context, modelName string) (float64, float64, error) {
	row := a.db.QueryRowContext(ctx, `
		SELECT input_token_price, output_token_price
		FROM portal_model_catalog
		WHERE name = ? OR model_id = ?
		LIMIT 1`, modelName, modelName)
	var inputPrice, outputPrice float64
	if err := row.Scan(&inputPrice, &outputPrice); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	return inputPrice, outputPrice, nil
}

func calculateCost(inputTokens int64, outputTokens int64, inputPrice float64, outputPrice float64) float64 {
	return (float64(inputTokens)/1000.0)*inputPrice + (float64(outputTokens)/1000.0)*outputPrice
}

func (a *App) syncUsageOnce(ctx context.Context) error {
	if !a.cfg.UsageSyncEnabled || !a.console.Enabled() {
		return nil
	}
	now := time.Now()
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	stats, err := a.console.FetchUsageStats(ctx, from, now)
	if err != nil {
		return err
	}
	if len(stats) == 0 {
		return nil
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	for _, item := range stats {
		consumerName := normalizeUsername(item.ConsumerName)
		if consumerName == "" {
			continue
		}
		inputPrice, outputPrice, priceErr := a.getModelPrices(ctx, item.ModelName)
		if priceErr != nil {
			a.logger.Printf("get model price failed: model=%s err=%v", item.ModelName, priceErr)
			inputPrice = 0
			outputPrice = 0
		}
		cost := calculateCost(item.InputTokens, item.OutputTokens, inputPrice, outputPrice)

		_, err = tx.ExecContext(ctx, `
			INSERT INTO portal_usage_daily
			(billing_date, consumer_name, model_name, request_count, input_tokens, output_tokens, total_tokens, cost_amount, source_from, source_to)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
			request_count = VALUES(request_count),
			input_tokens = VALUES(input_tokens),
			output_tokens = VALUES(output_tokens),
			total_tokens = VALUES(total_tokens),
			cost_amount = VALUES(cost_amount),
			source_from = VALUES(source_from),
			source_to = VALUES(source_to)`,
			from.Format("2006-01-02"),
			consumerName,
			item.ModelName,
			item.RequestCount,
			item.InputTokens,
			item.OutputTokens,
			item.TotalTokens,
			cost,
			from,
			now,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (a *App) startUsageSync(ctx context.Context) {
	if !a.cfg.UsageSyncEnabled {
		return
	}
	if err := a.syncUsageOnce(ctx); err != nil {
		a.logger.Printf("initial usage sync failed: %v", err)
	}

	ticker := time.NewTicker(a.cfg.UsageSyncInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := a.syncUsageOnce(ctx); err != nil {
					a.logger.Printf("usage sync failed: %v", err)
				}
			}
		}
	}()
}

package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func (a *App) handleRegister(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
		return
	}

	username := normalizeUsername(req.Username)
	if username == "" || len(req.Password) < 8 || strings.TrimSpace(req.InviteCode) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "inviteCode, username and password(>=8) are required"})
		return
	}
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = username
	}

	ctx := c.Request.Context()

	inviteRow := a.db.QueryRowContext(ctx, `
		SELECT id FROM portal_invite_code
		WHERE invite_code = ? AND status = 'active' AND expires_at > NOW()`, strings.TrimSpace(req.InviteCode))
	var inviteID int64
	if err := inviteRow.Scan(&inviteID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invite code invalid or expired"})
		return
	}

	existing, err := a.getUserByName(ctx, username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "query user failed"})
		return
	}
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"message": "username already exists"})
		return
	}

	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "encrypt password failed"})
		return
	}

	defaultKeyRaw := randomToken("hgpk_live_")
	if err := a.console.UpsertConsumer(ctx, username, strings.TrimSpace(req.Department), []string{defaultKeyRaw}); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "create consumer in higress-console failed", "error": err.Error()})
		return
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "begin tx failed"})
		return
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO portal_user
		(consumer_name, display_name, email, department, password_hash, status, source)
		VALUES (?, ?, ?, ?, ?, 'active', 'portal')`,
		username, displayName, strings.TrimSpace(req.Email), strings.TrimSpace(req.Department), passwordHash,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "create user failed"})
		return
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE portal_invite_code
		SET status = 'used', used_by_consumer = ?, used_at = NOW()
		WHERE id = ?`, username, inviteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "consume invite code failed"})
		return
	}

	keyID := fmt.Sprintf("KEY%d", time.Now().UnixMilli())
	_, err = tx.ExecContext(ctx, `
		INSERT INTO portal_api_key
		(key_id, consumer_name, name, key_masked, key_hash, raw_key, status)
		VALUES (?, ?, ?, ?, ?, ?, 'active')`,
		keyID, username, "Default Key", maskKey(defaultKeyRaw), sha256Hex(defaultKeyRaw), defaultKeyRaw,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "save default api key failed"})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "commit failed"})
		return
	}

	if err := a.saveSession(c, username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "create session failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user": AuthUser{
			ConsumerName: username,
			DisplayName:  displayName,
			Email:        strings.TrimSpace(req.Email),
			Department:   strings.TrimSpace(req.Department),
			Status:       userStatusActive,
		},
		"defaultApiKey": defaultKeyRaw,
	})
}

func (a *App) handleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
		return
	}
	username := normalizeUsername(req.Username)
	if username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "username and password are required"})
		return
	}

	ctx := c.Request.Context()
	user, err := a.getUserByName(ctx, username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "query user failed"})
		return
	}
	if user == nil || !comparePassword(user.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "incorrect username or password"})
		return
	}
	if user.Status != userStatusActive {
		c.JSON(http.StatusForbidden, gin.H{"message": "account disabled"})
		return
	}

	_, _ = a.db.ExecContext(ctx, `UPDATE portal_user SET last_login_at = NOW() WHERE consumer_name = ?`, username)
	if err := a.saveSession(c, username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "create session failed"})
		return
	}

	c.JSON(http.StatusOK, AuthUser{
		ConsumerName: user.ConsumerName,
		DisplayName:  user.DisplayName,
		Email:        user.Email,
		Department:   user.Department,
		Status:       user.Status,
	})
}

func (a *App) handleLogout(c *gin.Context) {
	a.clearSession(c)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (a *App) handleMe(c *gin.Context) {
	c.JSON(http.StatusOK, getAuthUser(c))
}

func (a *App) handleBillingOverview(c *gin.Context) {
	user := getAuthUser(c)
	ctx := c.Request.Context()

	var totalRecharge sql.NullFloat64
	if err := a.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount), 0)
		FROM portal_recharge_order
		WHERE consumer_name = ? AND status = 'success'`, user.ConsumerName).Scan(&totalRecharge); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "query recharge failed"})
		return
	}

	var totalConsumption sql.NullFloat64
	if err := a.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(cost_amount), 0)
		FROM portal_usage_daily
		WHERE consumer_name = ?`, user.ConsumerName).Scan(&totalConsumption); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "query consumption failed"})
		return
	}

	recharge := totalRecharge.Float64
	consumption := totalConsumption.Float64
	balance := recharge - consumption

	c.JSON(http.StatusOK, BillingOverview{
		Balance:          fmt.Sprintf("%.2f", balance),
		TotalRecharge:    fmt.Sprintf("%.2f", recharge),
		TotalConsumption: fmt.Sprintf("%.2f", consumption),
	})
}

func (a *App) handleConsumptions(c *gin.Context) {
	user := getAuthUser(c)
	rows, err := a.db.QueryContext(c.Request.Context(), `
		SELECT id, model_name, total_tokens, cost_amount, updated_at
		FROM portal_usage_daily
		WHERE consumer_name = ?
		ORDER BY billing_date DESC, id DESC`, user.ConsumerName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "query consumptions failed"})
		return
	}
	defer rows.Close()

	items := make([]ConsumptionRecord, 0)
	for rows.Next() {
		var (
			id         int64
			modelName  string
			tokens     int64
			cost       float64
			updatedAt  time.Time
		)
		if err := rows.Scan(&id, &modelName, &tokens, &cost, &updatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "scan consumptions failed"})
			return
		}
		items = append(items, ConsumptionRecord{
			ID:        fmt.Sprintf("CS%d", id),
			Model:     modelName,
			Tokens:    tokens,
			Cost:      cost,
			CreatedAt: nowText(updatedAt),
		})
	}

	c.JSON(http.StatusOK, items)
}

func (a *App) handleRecharges(c *gin.Context) {
	user := getAuthUser(c)
	rows, err := a.db.QueryContext(c.Request.Context(), `
		SELECT order_id, amount, channel, status, created_at
		FROM portal_recharge_order
		WHERE consumer_name = ?
		ORDER BY id DESC`, user.ConsumerName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "query recharges failed"})
		return
	}
	defer rows.Close()

	items := make([]RechargeRecord, 0)
	for rows.Next() {
		var item RechargeRecord
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.Amount, &item.Channel, &item.Status, &createdAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "scan recharges failed"})
			return
		}
		item.CreatedAt = nowText(createdAt)
		items = append(items, item)
	}

	c.JSON(http.StatusOK, items)
}

func (a *App) handleCreateRecharge(c *gin.Context) {
	user := getAuthUser(c)
	var req CreateRechargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
		return
	}
	if req.Amount <= 0 || strings.TrimSpace(req.Channel) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "amount and channel are required"})
		return
	}

	orderID := fmt.Sprintf("RC%d", time.Now().UnixMilli())
	_, err := a.db.ExecContext(c.Request.Context(), `
		INSERT INTO portal_recharge_order
		(order_id, consumer_name, amount, channel, status)
		VALUES (?, ?, ?, ?, 'success')`, orderID, user.ConsumerName, req.Amount, strings.TrimSpace(req.Channel))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "create recharge failed"})
		return
	}

	c.JSON(http.StatusCreated, RechargeRecord{
		ID:        orderID,
		Amount:    req.Amount,
		Channel:   strings.TrimSpace(req.Channel),
		Status:    "success",
		CreatedAt: nowText(time.Now()),
	})
}

func (a *App) handleListModels(c *gin.Context) {
	rows, err := a.db.QueryContext(c.Request.Context(), `
		SELECT model_id, name, vendor, capability, input_token_price, output_token_price, endpoint, sdk, summary, updated_at
		FROM portal_model_catalog
		WHERE status = 'active'
		ORDER BY id ASC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "query models failed"})
		return
	}
	defer rows.Close()

	models := make([]ModelInfo, 0)
	for rows.Next() {
		var item ModelInfo
		var updatedAt time.Time
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Vendor,
			&item.Capability,
			&item.InputTokenPrice,
			&item.OutputTokenPrice,
			&item.Endpoint,
			&item.SDK,
			&item.Summary,
			&updatedAt,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "scan models failed"})
			return
		}
		item.UpdatedAt = updatedAt.Format("2006-01-02")
		models = append(models, item)
	}

	c.JSON(http.StatusOK, models)
}

func (a *App) handleModelDetail(c *gin.Context) {
	id := c.Param("id")
	row := a.db.QueryRowContext(c.Request.Context(), `
		SELECT model_id, name, vendor, capability, input_token_price, output_token_price, endpoint, sdk, summary, updated_at
		FROM portal_model_catalog
		WHERE model_id = ? AND status = 'active'`, id)
	var model ModelInfo
	var updatedAt time.Time
	if err := row.Scan(
		&model.ID,
		&model.Name,
		&model.Vendor,
		&model.Capability,
		&model.InputTokenPrice,
		&model.OutputTokenPrice,
		&model.Endpoint,
		&model.SDK,
		&model.Summary,
		&updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"message": "model not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "query model failed"})
		return
	}
	model.UpdatedAt = updatedAt.Format("2006-01-02")
	c.JSON(http.StatusOK, model)
}

func (a *App) handleListAPIKeys(c *gin.Context) {
	user := getAuthUser(c)
	rows, err := a.listAPIKeys(c.Request.Context(), user.ConsumerName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "query api keys failed"})
		return
	}

	items := make([]ApiKeyRecord, 0, len(rows))
	for _, item := range rows {
		lastUsed := "-"
		if item.LastUsedAt != nil {
			lastUsed = nowText(*item.LastUsedAt)
		}
		items = append(items, ApiKeyRecord{
			ID:         item.KeyID,
			Name:       item.Name,
			Key:        item.Masked,
			Status:     item.Status,
			CreatedAt:  nowText(item.CreatedAt),
			LastUsed:   lastUsed,
			TotalCalls: item.TotalCalls,
		})
	}

	c.JSON(http.StatusOK, items)
}

func (a *App) handleCreateAPIKey(c *gin.Context) {
	user := getAuthUser(c)
	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "name is required"})
		return
	}

	rawKey := randomToken("hgpk_live_")
	keyID := fmt.Sprintf("KEY%d", time.Now().UnixMilli())
	ctx := c.Request.Context()
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO portal_api_key
		(key_id, consumer_name, name, key_masked, key_hash, raw_key, status)
		VALUES (?, ?, ?, ?, ?, ?, 'active')`,
		keyID, user.ConsumerName, name, maskKey(rawKey), sha256Hex(rawKey), rawKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "create api key failed"})
		return
	}

	if err := a.syncConsumerKeys(ctx, user.ConsumerName, user.Department); err != nil {
		_, _ = a.db.ExecContext(ctx, `UPDATE portal_api_key SET status='disabled' WHERE key_id=?`, keyID)
		c.JSON(http.StatusBadGateway, gin.H{"message": "sync consumer key to higress-console failed", "error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, ApiKeyRecord{
		ID:         keyID,
		Name:       name,
		Key:        rawKey,
		Status:     apiKeyStatusActive,
		CreatedAt:  nowText(time.Now()),
		LastUsed:   "-",
		TotalCalls: 0,
	})
}

func (a *App) handleUpdateAPIKeyStatus(c *gin.Context) {
	user := getAuthUser(c)
	keyID := c.Param("id")

	var req UpdateAPIKeyStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
		return
	}
	status := strings.TrimSpace(req.Status)
	if status != apiKeyStatusActive && status != apiKeyStatusDisabled {
		c.JSON(http.StatusBadRequest, gin.H{"message": "status must be active or disabled"})
		return
	}

	result, err := a.db.ExecContext(c.Request.Context(), `
		UPDATE portal_api_key
		SET status = ?
		WHERE key_id = ? AND consumer_name = ?`, status, keyID, user.ConsumerName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "update api key failed"})
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"message": "api key not found"})
		return
	}

	if err := a.syncConsumerKeys(c.Request.Context(), user.ConsumerName, user.Department); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "sync consumer key to higress-console failed", "error": err.Error()})
		return
	}

	row := a.db.QueryRowContext(c.Request.Context(), `
		SELECT key_id, name, key_masked, status, created_at, total_calls, last_used_at
		FROM portal_api_key
		WHERE key_id = ? AND consumer_name = ?`, keyID, user.ConsumerName)
	var resp ApiKeyRecord
	var createdAt time.Time
	var lastUsedAt sql.NullTime
	if err := row.Scan(&resp.ID, &resp.Name, &resp.Key, &resp.Status, &createdAt, &resp.TotalCalls, &lastUsedAt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "query api key failed"})
		return
	}
	resp.CreatedAt = nowText(createdAt)
	resp.LastUsed = "-"
	if lastUsedAt.Valid {
		resp.LastUsed = nowText(lastUsedAt.Time)
	}
	c.JSON(http.StatusOK, resp)
}

func (a *App) handleDeleteAPIKey(c *gin.Context) {
	user := getAuthUser(c)
	keyID := c.Param("id")

	result, err := a.db.ExecContext(c.Request.Context(), `
		DELETE FROM portal_api_key
		WHERE key_id = ? AND consumer_name = ?`, keyID, user.ConsumerName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "delete api key failed"})
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"message": "api key not found"})
		return
	}

	if err := a.syncConsumerKeys(c.Request.Context(), user.ConsumerName, user.Department); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "sync consumer key to higress-console failed", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": keyID})
}

func (a *App) handleOpenStats(c *gin.Context) {
	user := getAuthUser(c)
	ctx := c.Request.Context()

	var todayCalls int64
	var todayCost float64
	if err := a.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(request_count),0), COALESCE(SUM(cost_amount),0)
		FROM portal_usage_daily
		WHERE consumer_name = ? AND billing_date = CURDATE()`, user.ConsumerName).Scan(&todayCalls, &todayCost); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "query today stats failed"})
		return
	}

	if todayCalls == 0 {
		_ = a.db.QueryRowContext(ctx, `
			SELECT COALESCE(SUM(total_tokens),0)
			FROM portal_usage_daily
			WHERE consumer_name = ? AND billing_date = CURDATE()`, user.ConsumerName).Scan(&todayCalls)
	}

	var last7DaysCalls int64
	if err := a.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(request_count),0)
		FROM portal_usage_daily
		WHERE consumer_name = ? AND billing_date >= DATE_SUB(CURDATE(), INTERVAL 6 DAY)`, user.ConsumerName).Scan(&last7DaysCalls); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "query 7days stats failed"})
		return
	}
	if last7DaysCalls == 0 {
		_ = a.db.QueryRowContext(ctx, `
			SELECT COALESCE(SUM(total_tokens),0)
			FROM portal_usage_daily
			WHERE consumer_name = ? AND billing_date >= DATE_SUB(CURDATE(), INTERVAL 6 DAY)`, user.ConsumerName).Scan(&last7DaysCalls)
	}

	var activeKeys int64
	if err := a.db.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM portal_api_key
		WHERE consumer_name = ? AND status = 'active'`, user.ConsumerName).Scan(&activeKeys); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "query active key failed"})
		return
	}

	c.JSON(http.StatusOK, OpenStats{
		TodayCalls:     todayCalls,
		TodayCost:      fmt.Sprintf("%.2f", todayCost),
		Last7DaysCalls: last7DaysCalls,
		ActiveKeys:     activeKeys,
	})
}

func (a *App) handleCostDetails(c *gin.Context) {
	user := getAuthUser(c)
	rows, err := a.db.QueryContext(c.Request.Context(), `
		SELECT id, billing_date, model_name, request_count, total_tokens, cost_amount
		FROM portal_usage_daily
		WHERE consumer_name = ?
		ORDER BY billing_date DESC, id DESC`, user.ConsumerName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "query cost details failed"})
		return
	}
	defer rows.Close()

	items := make([]CostDetailRecord, 0)
	for rows.Next() {
		var item CostDetailRecord
		var id int64
		var billingDate time.Time
		if err := rows.Scan(&id, &billingDate, &item.Model, &item.Calls, &item.Tokens, &item.Cost); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "scan cost details failed"})
			return
		}
		item.ID = fmt.Sprintf("COST%d", id)
		item.Date = billingDate.Format("2006-01-02")
		items = append(items, item)
	}

	c.JSON(http.StatusOK, items)
}

func (a *App) handleGetInvoiceProfile(c *gin.Context) {
	user := getAuthUser(c)
	row := a.db.QueryRowContext(c.Request.Context(), `
		SELECT company_name, tax_no, address, bank_account, receiver, email
		FROM portal_invoice_profile
		WHERE consumer_name = ?`, user.ConsumerName)
	var profile InvoiceProfile
	if err := row.Scan(
		&profile.CompanyName,
		&profile.TaxNo,
		&profile.Address,
		&profile.BankAccount,
		&profile.Receiver,
		&profile.Email,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusOK, InvoiceProfile{})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "query invoice profile failed"})
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (a *App) handleUpdateInvoiceProfile(c *gin.Context) {
	user := getAuthUser(c)
	var req InvoiceProfile
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
		return
	}

	_, err := a.db.ExecContext(c.Request.Context(), `
		INSERT INTO portal_invoice_profile
		(consumer_name, company_name, tax_no, address, bank_account, receiver, email)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		company_name = VALUES(company_name),
		tax_no = VALUES(tax_no),
		address = VALUES(address),
		bank_account = VALUES(bank_account),
		receiver = VALUES(receiver),
		email = VALUES(email)`,
		user.ConsumerName,
		strings.TrimSpace(req.CompanyName),
		strings.TrimSpace(req.TaxNo),
		strings.TrimSpace(req.Address),
		strings.TrimSpace(req.BankAccount),
		strings.TrimSpace(req.Receiver),
		strings.TrimSpace(req.Email),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "update invoice profile failed"})
		return
	}
	c.JSON(http.StatusOK, req)
}

func (a *App) handleInvoiceRecords(c *gin.Context) {
	user := getAuthUser(c)
	rows, err := a.db.QueryContext(c.Request.Context(), `
		SELECT invoice_id, title, tax_no, amount, status, remark, created_at
		FROM portal_invoice_record
		WHERE consumer_name = ?
		ORDER BY id DESC`, user.ConsumerName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "query invoice records failed"})
		return
	}
	defer rows.Close()

	items := make([]InvoiceRecord, 0)
	for rows.Next() {
		var item InvoiceRecord
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.Title, &item.TaxNo, &item.Amount, &item.Status, &item.Remark, &createdAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "scan invoice records failed"})
			return
		}
		item.CreatedAt = nowText(createdAt)
		items = append(items, item)
	}
	c.JSON(http.StatusOK, items)
}

func (a *App) handleCreateInvoice(c *gin.Context) {
	user := getAuthUser(c)
	var req CreateInvoiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
		return
	}
	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "amount must be greater than 0"})
		return
	}

	profileRow := a.db.QueryRowContext(c.Request.Context(), `
		SELECT company_name, tax_no
		FROM portal_invoice_profile
		WHERE consumer_name = ?`, user.ConsumerName)
	var title string
	var taxNo string
	if err := profileRow.Scan(&title, &taxNo); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "please set invoice profile first"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "query invoice profile failed"})
		return
	}

	invoiceID := fmt.Sprintf("INV%d", time.Now().UnixMilli())
	_, err := a.db.ExecContext(c.Request.Context(), `
		INSERT INTO portal_invoice_record
		(invoice_id, consumer_name, title, tax_no, amount, status, remark)
		VALUES (?, ?, ?, ?, ?, 'pending', ?)`,
		invoiceID, user.ConsumerName, title, taxNo, req.Amount, strings.TrimSpace(req.Remark),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "create invoice failed"})
		return
	}

	c.JSON(http.StatusCreated, InvoiceRecord{
		ID:        invoiceID,
		Title:     title,
		TaxNo:     taxNo,
		Amount:    req.Amount,
		Status:    "pending",
		CreatedAt: nowText(time.Now()),
		Remark:    strings.TrimSpace(req.Remark),
	})
}

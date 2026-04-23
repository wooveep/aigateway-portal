package portal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"higress-portal-backend/internal/apperr"
	"higress-portal-backend/internal/consts"
	"higress-portal-backend/internal/model"
)

const (
	apiKeyDailyResetModeFixed = "fixed"
)

type apiKeyPolicyInput struct {
	Name                  string
	ExpiresAt             *time.Time
	LimitTotalMicroYuan   int64
	Limit5hMicroYuan      int64
	LimitDailyMicroYuan   int64
	DailyResetMode        string
	DailyResetTime        string
	LimitWeeklyMicroYuan  int64
	LimitMonthlyMicroYuan int64
}

type userQuotaPolicy struct {
	LimitTotalMicroYuan   int64
	Limit5hMicroYuan      int64
	LimitDailyMicroYuan   int64
	LimitWeeklyMicroYuan  int64
	LimitMonthlyMicroYuan int64
}

func (s *Service) ListAPIKeys(ctx context.Context, consumerName string, includeRaw bool) ([]model.APIKeyRecord, error) {
	rows, err := s.listAPIKeyRows(ctx, consumerName)
	if err != nil {
		return nil, err
	}

	items := make([]model.APIKeyRecord, 0, len(rows))
	for _, item := range rows {
		items = append(items, toAPIKeyRecord(item, includeRaw))
	}
	return items, nil
}

func (s *Service) CreateAPIKey(ctx context.Context, consumerName string, req model.CreateAPIKeyRequest) (model.APIKeyRecord, error) {
	normalizedConsumer := model.NormalizeUsername(consumerName)
	if strings.EqualFold(normalizedConsumer, builtinAdministratorUser) {
		return model.APIKeyRecord{}, apperr.New(403, "administrator api key management is not supported")
	}

	policy, err := s.normalizeAPIKeyPolicyInput(ctx, normalizedConsumer, req.Name, req.ExpiresAt,
		req.LimitTotal, req.Limit5h, req.LimitDaily, req.DailyResetMode, req.DailyResetTime, req.LimitWeekly,
		req.LimitMonthly)
	if err != nil {
		return model.APIKeyRecord{}, err
	}

	rawKey := randomToken("hgpk_live_")
	keyID := fmt.Sprintf("KEY%d", time.Now().UnixMilli())
	now := time.Now()

	if _, err = s.db.Exec(ctx, `
		INSERT INTO portal_api_key
		(key_id, consumer_name, name, key_masked, key_hash, raw_key, status, expires_at,
		 limit_total_micro_yuan, limit_5h_micro_yuan, limit_daily_micro_yuan, daily_reset_mode, daily_reset_time,
		 limit_weekly_micro_yuan, limit_monthly_micro_yuan, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		keyID,
		normalizedConsumer,
		policy.Name,
		model.MaskKey(rawKey),
		sha256Hex(rawKey),
		rawKey,
		consts.APIKeyStatusActive,
		policy.ExpiresAt,
		policy.LimitTotalMicroYuan,
		policy.Limit5hMicroYuan,
		policy.LimitDailyMicroYuan,
		policy.DailyResetMode,
		policy.DailyResetTime,
		policy.LimitWeeklyMicroYuan,
		policy.LimitMonthlyMicroYuan,
		now,
		now,
	); err != nil {
		return model.APIKeyRecord{}, gerror.Wrap(err, "create api key failed")
	}
	if err = s.syncKeyAuthConsumers(ctx); err != nil {
		return model.APIKeyRecord{}, apperr.New(503, "api key created but failed to sync gateway key-auth", err.Error())
	}

	row, err := s.getAPIKeyRow(ctx, normalizedConsumer, keyID, true)
	if err != nil {
		return model.APIKeyRecord{}, err
	}
	if row == nil {
		return model.APIKeyRecord{}, apperr.New(404, "api key not found")
	}
	return toAPIKeyRecord(*row, true), nil
}

func (s *Service) UpdateAPIKeyStatus(ctx context.Context, consumerName string, keyID string, status string) (model.APIKeyRecord, error) {
	normalizedConsumer := model.NormalizeUsername(consumerName)
	normalizedStatus := strings.ToLower(strings.TrimSpace(status))
	if normalizedStatus != consts.APIKeyStatusActive && normalizedStatus != consts.APIKeyStatusDisabled {
		return model.APIKeyRecord{}, apperr.New(400, "status must be active or disabled")
	}

	row, err := s.getAPIKeyRow(ctx, normalizedConsumer, keyID, false)
	if err != nil {
		return model.APIKeyRecord{}, err
	}
	if row == nil {
		return model.APIKeyRecord{}, apperr.New(404, "api key not found")
	}

	if _, err = s.db.Exec(ctx, `
		UPDATE portal_api_key
		SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE key_id = ? AND consumer_name = ? AND deleted_at IS NULL`,
		normalizedStatus,
		keyID,
		normalizedConsumer,
	); err != nil {
		return model.APIKeyRecord{}, gerror.Wrap(err, "update api key failed")
	}
	if err = s.syncKeyAuthConsumers(ctx); err != nil {
		return model.APIKeyRecord{}, apperr.New(503, "api key updated but failed to sync gateway key-auth", err.Error())
	}

	updated, err := s.getAPIKeyRow(ctx, normalizedConsumer, keyID, false)
	if err != nil {
		return model.APIKeyRecord{}, err
	}
	if updated == nil {
		return model.APIKeyRecord{}, apperr.New(404, "api key not found")
	}
	return toAPIKeyRecord(*updated, false), nil
}

func (s *Service) UpdateAPIKey(ctx context.Context, consumerName string, keyID string, req model.UpdateAPIKeyRequest) (model.APIKeyRecord, error) {
	normalizedConsumer := model.NormalizeUsername(consumerName)

	row, err := s.getAPIKeyRow(ctx, normalizedConsumer, keyID, false)
	if err != nil {
		return model.APIKeyRecord{}, err
	}
	if row == nil {
		return model.APIKeyRecord{}, apperr.New(404, "api key not found")
	}

	policy, err := s.normalizeAPIKeyPolicyInput(ctx, normalizedConsumer, req.Name, req.ExpiresAt,
		req.LimitTotal, req.Limit5h, req.LimitDaily, req.DailyResetMode, req.DailyResetTime, req.LimitWeekly,
		req.LimitMonthly)
	if err != nil {
		return model.APIKeyRecord{}, err
	}

	if _, err = s.db.Exec(ctx, `
		UPDATE portal_api_key
		SET name = ?, expires_at = ?, limit_total_micro_yuan = ?, limit_5h_micro_yuan = ?,
			limit_daily_micro_yuan = ?, daily_reset_mode = ?, daily_reset_time = ?,
			limit_weekly_micro_yuan = ?, limit_monthly_micro_yuan = ?, updated_at = CURRENT_TIMESTAMP
		WHERE key_id = ? AND consumer_name = ? AND deleted_at IS NULL`,
		policy.Name,
		policy.ExpiresAt,
		policy.LimitTotalMicroYuan,
		policy.Limit5hMicroYuan,
		policy.LimitDailyMicroYuan,
		policy.DailyResetMode,
		policy.DailyResetTime,
		policy.LimitWeeklyMicroYuan,
		policy.LimitMonthlyMicroYuan,
		keyID,
		normalizedConsumer,
	); err != nil {
		return model.APIKeyRecord{}, gerror.Wrap(err, "update api key failed")
	}
	if err = s.syncKeyAuthConsumers(ctx); err != nil {
		return model.APIKeyRecord{}, apperr.New(503, "api key updated but failed to sync gateway key-auth", err.Error())
	}

	updated, err := s.getAPIKeyRow(ctx, normalizedConsumer, keyID, false)
	if err != nil {
		return model.APIKeyRecord{}, err
	}
	if updated == nil {
		return model.APIKeyRecord{}, apperr.New(404, "api key not found")
	}
	return toAPIKeyRecord(*updated, false), nil
}

func (s *Service) DeleteAPIKey(ctx context.Context, consumerName string, keyID string) error {
	normalizedConsumer := model.NormalizeUsername(consumerName)

	row, err := s.getAPIKeyRow(ctx, normalizedConsumer, keyID, false)
	if err != nil {
		return err
	}
	if row == nil {
		return apperr.New(404, "api key not found")
	}
	if _, err = s.db.Exec(ctx, `
		UPDATE portal_api_key
		SET status = ?, deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE key_id = ? AND consumer_name = ? AND deleted_at IS NULL`,
		consts.APIKeyStatusDisabled,
		keyID,
		normalizedConsumer,
	); err != nil {
		return gerror.Wrap(err, "delete api key failed")
	}
	if err = s.syncKeyAuthConsumers(ctx); err != nil {
		return apperr.New(503, "api key deleted but failed to sync gateway key-auth", err.Error())
	}
	return nil
}

func (s *Service) listAPIKeyRows(ctx context.Context, consumerName string) ([]model.APIKeyRow, error) {
	var rows []model.APIKeyRow
	err := s.db.GetScan(ctx, &rows, `
		SELECT key_id, name, raw_key, key_masked, status, total_calls, last_used_at, expires_at, deleted_at,
			limit_total_micro_yuan, limit_5h_micro_yuan, limit_daily_micro_yuan, daily_reset_mode, daily_reset_time,
			limit_weekly_micro_yuan, limit_monthly_micro_yuan, created_at
		FROM portal_api_key
		WHERE consumer_name = ? AND deleted_at IS NULL
		ORDER BY created_at DESC`, consumerName)
	if err != nil {
		return nil, gerror.Wrap(err, "query api keys failed")
	}
	return rows, nil
}

func (s *Service) getAPIKeyRow(ctx context.Context, consumerName string, keyID string, includeRaw bool) (*model.APIKeyRow, error) {
	selectKey := "raw_key"
	if !includeRaw {
		selectKey = "raw_key"
	}
	record, err := s.db.GetOne(ctx, `
		SELECT key_id, name, `+selectKey+`, key_masked, status, total_calls, last_used_at, expires_at, deleted_at,
			limit_total_micro_yuan, limit_5h_micro_yuan, limit_daily_micro_yuan, daily_reset_mode, daily_reset_time,
			limit_weekly_micro_yuan, limit_monthly_micro_yuan, created_at
		FROM portal_api_key
		WHERE key_id = ? AND consumer_name = ? AND deleted_at IS NULL
		LIMIT 1`, keyID, consumerName)
	if err != nil {
		return nil, gerror.Wrap(err, "query api key failed")
	}
	if record.IsEmpty() {
		return nil, nil
	}
	row := &model.APIKeyRow{
		KeyID:                 record["key_id"].String(),
		Name:                  record["name"].String(),
		RawKey:                record["raw_key"].String(),
		Masked:                record["key_masked"].String(),
		Status:                record["status"].String(),
		TotalCalls:            record["total_calls"].Int64(),
		LimitTotalMicroYuan:   record["limit_total_micro_yuan"].Int64(),
		Limit5hMicroYuan:      record["limit_5h_micro_yuan"].Int64(),
		LimitDailyMicroYuan:   record["limit_daily_micro_yuan"].Int64(),
		DailyResetMode:        record["daily_reset_mode"].String(),
		DailyResetTime:        record["daily_reset_time"].String(),
		LimitWeeklyMicroYuan:  record["limit_weekly_micro_yuan"].Int64(),
		LimitMonthlyMicroYuan: record["limit_monthly_micro_yuan"].Int64(),
		CreatedAt:             record["created_at"].Time(),
	}
	if !record["last_used_at"].IsEmpty() {
		lastUsed := record["last_used_at"].Time()
		row.LastUsedAt = &lastUsed
	}
	if !record["expires_at"].IsEmpty() {
		expiresAt := record["expires_at"].Time()
		row.ExpiresAt = &expiresAt
	}
	if !record["deleted_at"].IsEmpty() {
		deletedAt := record["deleted_at"].Time()
		row.DeletedAt = &deletedAt
	}
	return row, nil
}

func toAPIKeyRecord(item model.APIKeyRow, includeRaw bool) model.APIKeyRecord {
	lastUsed := "-"
	if item.LastUsedAt != nil {
		lastUsed = model.NowText(*item.LastUsedAt)
	}
	expiresAt := "-"
	if item.ExpiresAt != nil {
		expiresAt = model.NowText(*item.ExpiresAt)
	}
	keyValue := item.Masked
	if includeRaw {
		keyValue = item.RawKey
	}
	return model.APIKeyRecord{
		ID:             item.KeyID,
		Name:           item.Name,
		Key:            keyValue,
		Status:         item.Status,
		CreatedAt:      model.NowText(item.CreatedAt),
		LastUsed:       lastUsed,
		ExpiresAt:      expiresAt,
		TotalCalls:     item.TotalCalls,
		LimitTotal:     microYuanToRMB(item.LimitTotalMicroYuan),
		Limit5h:        microYuanToRMB(item.Limit5hMicroYuan),
		LimitDaily:     microYuanToRMB(item.LimitDailyMicroYuan),
		DailyResetMode: defaultDailyResetMode(item.DailyResetMode),
		DailyResetTime: defaultDailyResetTime(item.DailyResetTime),
		LimitWeekly:    microYuanToRMB(item.LimitWeeklyMicroYuan),
		LimitMonthly:   microYuanToRMB(item.LimitMonthlyMicroYuan),
	}
}

func (s *Service) normalizeAPIKeyPolicyInput(ctx context.Context, consumerName string, name string, expiresAt string,
	limitTotal float64, limit5h float64, limitDaily float64, dailyResetMode string, dailyResetTime string,
	limitWeekly float64, limitMonthly float64) (apiKeyPolicyInput, error) {
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" {
		return apiKeyPolicyInput{}, apperr.New(400, "name is required")
	}

	parsedExpiresAt, err := parseOptionalDateTime(expiresAt)
	if err != nil {
		return apiKeyPolicyInput{}, apperr.New(400, "expiresAt is invalid", err.Error())
	}

	policy := apiKeyPolicyInput{
		Name:           normalizedName,
		ExpiresAt:      parsedExpiresAt,
		DailyResetMode: defaultDailyResetMode(dailyResetMode),
		DailyResetTime: defaultDailyResetTime(dailyResetTime),
	}
	if policy.LimitTotalMicroYuan, err = validateQuotaAmount(limitTotal, "limitTotal"); err != nil {
		return apiKeyPolicyInput{}, err
	}
	if policy.Limit5hMicroYuan, err = validateQuotaAmount(limit5h, "limit5h"); err != nil {
		return apiKeyPolicyInput{}, err
	}
	if policy.LimitDailyMicroYuan, err = validateQuotaAmount(limitDaily, "limitDaily"); err != nil {
		return apiKeyPolicyInput{}, err
	}
	if policy.LimitWeeklyMicroYuan, err = validateQuotaAmount(limitWeekly, "limitWeekly"); err != nil {
		return apiKeyPolicyInput{}, err
	}
	if policy.LimitMonthlyMicroYuan, err = validateQuotaAmount(limitMonthly, "limitMonthly"); err != nil {
		return apiKeyPolicyInput{}, err
	}

	if policy.DailyResetMode != apiKeyDailyResetModeFixed {
		return apiKeyPolicyInput{}, apperr.New(400, "dailyResetMode must be fixed")
	}
	if _, err = time.Parse("15:04", policy.DailyResetTime); err != nil {
		return apiKeyPolicyInput{}, apperr.New(400, "dailyResetTime must be HH:MM")
	}
	if err = s.validateKeyLimitsAgainstUserPolicy(ctx, consumerName, policy); err != nil {
		return apiKeyPolicyInput{}, err
	}
	return policy, nil
}

func validateQuotaAmount(amount float64, field string) (int64, error) {
	if amount < 0 {
		return 0, apperr.New(400, field+" cannot be negative")
	}
	return rmbToMicroYuan(amount), nil
}

func (s *Service) validateKeyLimitsAgainstUserPolicy(ctx context.Context, consumerName string, policy apiKeyPolicyInput) error {
	quotaPolicy, err := s.loadUserQuotaPolicy(ctx, consumerName)
	if err != nil {
		return err
	}
	checks := []struct {
		keyLimit  int64
		userLimit int64
		name      string
	}{
		{policy.LimitTotalMicroYuan, quotaPolicy.LimitTotalMicroYuan, "limitTotal"},
		{policy.Limit5hMicroYuan, quotaPolicy.Limit5hMicroYuan, "limit5h"},
		{policy.LimitDailyMicroYuan, quotaPolicy.LimitDailyMicroYuan, "limitDaily"},
		{policy.LimitWeeklyMicroYuan, quotaPolicy.LimitWeeklyMicroYuan, "limitWeekly"},
		{policy.LimitMonthlyMicroYuan, quotaPolicy.LimitMonthlyMicroYuan, "limitMonthly"},
	}
	for _, item := range checks {
		if item.userLimit <= 0 {
			continue
		}
		if item.keyLimit <= 0 {
			return apperr.New(400, item.name+" cannot exceed user quota policy")
		}
		if item.keyLimit > item.userLimit {
			return apperr.New(400, item.name+" cannot exceed user quota policy")
		}
	}
	return nil
}

func (s *Service) loadUserQuotaPolicy(ctx context.Context, consumerName string) (userQuotaPolicy, error) {
	record, err := s.db.GetOne(ctx, `
		SELECT limit_total_micro_yuan, limit_5h_micro_yuan, limit_daily_micro_yuan, limit_weekly_micro_yuan, limit_monthly_micro_yuan
		FROM quota_policy_user
		WHERE consumer_name = ?
		LIMIT 1`, consumerName)
	if err != nil {
		return userQuotaPolicy{}, gerror.Wrap(err, "query user quota policy failed")
	}
	if record.IsEmpty() {
		return userQuotaPolicy{}, nil
	}
	return userQuotaPolicy{
		LimitTotalMicroYuan:   record["limit_total_micro_yuan"].Int64(),
		Limit5hMicroYuan:      record["limit_5h_micro_yuan"].Int64(),
		LimitDailyMicroYuan:   record["limit_daily_micro_yuan"].Int64(),
		LimitWeeklyMicroYuan:  record["limit_weekly_micro_yuan"].Int64(),
		LimitMonthlyMicroYuan: record["limit_monthly_micro_yuan"].Int64(),
	}, nil
}

func (s *Service) ensureHasAnotherUsableAPIKey(ctx context.Context, consumerName string, excludedKeyID string) error {
	count, err := s.db.GetValue(ctx, `
		SELECT COUNT(1)
		FROM portal_api_key
		WHERE consumer_name = ?
		  AND deleted_at IS NULL
		  AND status = 'active'
		  AND (expires_at IS NULL OR expires_at > ?)
		  AND key_id <> ?`, consumerName, model.NowInAppLocation(), excludedKeyID)
	if err != nil {
		return gerror.Wrap(err, "query usable api keys failed")
	}
	if count.Int64() <= 0 {
		return apperr.New(409, "cannot disable or delete the last usable api key")
	}
	return nil
}

func isUsableAPIKey(row model.APIKeyRow, now time.Time) bool {
	if !strings.EqualFold(row.Status, consts.APIKeyStatusActive) {
		return false
	}
	if row.DeletedAt != nil {
		return false
	}
	if row.ExpiresAt != nil && !row.ExpiresAt.After(now) {
		return false
	}
	return true
}

func defaultDailyResetMode(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), apiKeyDailyResetModeFixed) {
		return apiKeyDailyResetModeFixed
	}
	return apiKeyDailyResetModeFixed
}

func defaultDailyResetTime(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "00:00"
	}
	return normalized
}

func parseOptionalDateTime(value string) (*time.Time, error) {
	parsed, err := model.ParseDateTime(value)
	if err != nil {
		return nil, gerror.New(err.Error())
	}
	return parsed, nil
}

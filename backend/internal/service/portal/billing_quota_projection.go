package portal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/redis/go-redis/v9"
)

const (
	billingQuotaWindowTotal   = "total"
	billingQuotaWindow5h      = "5h"
	billingQuotaWindowDaily   = "daily"
	billingQuotaWindowWeekly  = "weekly"
	billingQuotaWindowMonthly = "monthly"
)

type userQuotaPolicyProjection struct {
	ConsumerName           string
	LimitTotalMicroYuan    int64
	Limit5hMicroYuan       int64
	LimitDailyMicroYuan    int64
	DailyResetMode         string
	DailyResetTime         string
	LimitWeeklyMicroYuan   int64
	LimitMonthlyMicroYuan  int64
	CostResetAtRFC3339Nano string
}

type keyQuotaPolicyProjection struct {
	KeyID                 string
	ConsumerName          string
	LimitTotalMicroYuan   int64
	Limit5hMicroYuan      int64
	LimitDailyMicroYuan   int64
	DailyResetMode        string
	DailyResetTime        string
	LimitWeeklyMicroYuan  int64
	LimitMonthlyMicroYuan int64
}

type billingQuotaUsageWindow struct {
	Name      string
	TTL       time.Duration
	UserUsage map[string]int64
	KeyUsage  map[string]int64
}

func (s *Service) loadUserQuotaPolicyProjections(ctx context.Context) ([]userQuotaPolicyProjection, error) {
	records, err := s.db.GetAll(ctx, `
		SELECT consumer_name, limit_total_micro_yuan, limit_5h_micro_yuan, limit_daily_micro_yuan,
			daily_reset_mode, daily_reset_time, limit_weekly_micro_yuan, limit_monthly_micro_yuan, cost_reset_at
		FROM quota_policy_user
		WHERE LOWER(TRIM(consumer_name)) <> 'administrator'`)
	if err != nil {
		return nil, err
	}
	items := make([]userQuotaPolicyProjection, 0, len(records))
	for _, record := range records {
		consumerName := strings.TrimSpace(record["consumer_name"].String())
		if consumerName == "" {
			continue
		}
		costResetAt := ""
		if !record["cost_reset_at"].IsEmpty() {
			costResetAt = record["cost_reset_at"].Time().UTC().Format(time.RFC3339Nano)
		}
		items = append(items, userQuotaPolicyProjection{
			ConsumerName:           consumerName,
			LimitTotalMicroYuan:    record["limit_total_micro_yuan"].Int64(),
			Limit5hMicroYuan:       record["limit_5h_micro_yuan"].Int64(),
			LimitDailyMicroYuan:    record["limit_daily_micro_yuan"].Int64(),
			DailyResetMode:         strings.TrimSpace(record["daily_reset_mode"].String()),
			DailyResetTime:         strings.TrimSpace(record["daily_reset_time"].String()),
			LimitWeeklyMicroYuan:   record["limit_weekly_micro_yuan"].Int64(),
			LimitMonthlyMicroYuan:  record["limit_monthly_micro_yuan"].Int64(),
			CostResetAtRFC3339Nano: costResetAt,
		})
	}
	return items, nil
}

func (s *Service) loadKeyQuotaPolicyProjections(ctx context.Context) ([]keyQuotaPolicyProjection, error) {
	records, err := s.db.GetAll(ctx, `
		SELECT key_id, consumer_name, limit_total_micro_yuan, limit_5h_micro_yuan, limit_daily_micro_yuan,
			daily_reset_mode, daily_reset_time, limit_weekly_micro_yuan, limit_monthly_micro_yuan
		FROM portal_api_key
		WHERE deleted_at IS NULL
		  AND status = 'active'
		  AND (expires_at IS NULL OR expires_at > NOW())
		  AND LOWER(TRIM(consumer_name)) <> 'administrator'`)
	if err != nil {
		return nil, err
	}
	items := make([]keyQuotaPolicyProjection, 0, len(records))
	for _, record := range records {
		keyID := strings.TrimSpace(record["key_id"].String())
		consumerName := strings.TrimSpace(record["consumer_name"].String())
		if keyID == "" || consumerName == "" {
			continue
		}
		items = append(items, keyQuotaPolicyProjection{
			KeyID:                 keyID,
			ConsumerName:          consumerName,
			LimitTotalMicroYuan:   record["limit_total_micro_yuan"].Int64(),
			Limit5hMicroYuan:      record["limit_5h_micro_yuan"].Int64(),
			LimitDailyMicroYuan:   record["limit_daily_micro_yuan"].Int64(),
			DailyResetMode:        strings.TrimSpace(record["daily_reset_mode"].String()),
			DailyResetTime:        strings.TrimSpace(record["daily_reset_time"].String()),
			LimitWeeklyMicroYuan:  record["limit_weekly_micro_yuan"].Int64(),
			LimitMonthlyMicroYuan: record["limit_monthly_micro_yuan"].Int64(),
		})
	}
	return items, nil
}

func (s *Service) loadBillingQuotaUsageWindows(ctx context.Context, now time.Time) ([]billingQuotaUsageWindow, error) {
	now = now.In(billingQuotaLocation())
	window5hStart := now.Add(-5 * time.Hour)
	dayStart := billingDayStart(now)
	weekStart := billingWeekStart(now)
	monthStart := billingMonthStart(now)

	userTotal, err := s.queryBillingUsageByConsumer(ctx, time.Time{})
	if err != nil {
		return nil, err
	}
	keyTotal, err := s.queryBillingUsageByKey(ctx, time.Time{})
	if err != nil {
		return nil, err
	}
	user5h, err := s.queryBillingUsageByConsumer(ctx, window5hStart)
	if err != nil {
		return nil, err
	}
	key5h, err := s.queryBillingUsageByKey(ctx, window5hStart)
	if err != nil {
		return nil, err
	}
	userDaily, err := s.queryBillingUsageByConsumer(ctx, dayStart)
	if err != nil {
		return nil, err
	}
	keyDaily, err := s.queryBillingUsageByKey(ctx, dayStart)
	if err != nil {
		return nil, err
	}
	userWeekly, err := s.queryBillingUsageByConsumer(ctx, weekStart)
	if err != nil {
		return nil, err
	}
	keyWeekly, err := s.queryBillingUsageByKey(ctx, weekStart)
	if err != nil {
		return nil, err
	}
	userMonthly, err := s.queryBillingUsageByConsumer(ctx, monthStart)
	if err != nil {
		return nil, err
	}
	keyMonthly, err := s.queryBillingUsageByKey(ctx, monthStart)
	if err != nil {
		return nil, err
	}

	return []billingQuotaUsageWindow{
		{
			Name:      billingQuotaWindowTotal,
			TTL:       0,
			UserUsage: userTotal,
			KeyUsage:  keyTotal,
		},
		{
			Name:      billingQuotaWindow5h,
			TTL:       ttlUntil(now.Add(5*time.Hour), now),
			UserUsage: user5h,
			KeyUsage:  key5h,
		},
		{
			Name:      billingQuotaWindowDaily,
			TTL:       ttlUntil(dayStart.Add(24*time.Hour), now),
			UserUsage: userDaily,
			KeyUsage:  keyDaily,
		},
		{
			Name:      billingQuotaWindowWeekly,
			TTL:       ttlUntil(weekStart.Add(7*24*time.Hour), now),
			UserUsage: userWeekly,
			KeyUsage:  keyWeekly,
		},
		{
			Name:      billingQuotaWindowMonthly,
			TTL:       ttlUntil(monthStart.AddDate(0, 1, 0), now),
			UserUsage: userMonthly,
			KeyUsage:  keyMonthly,
		},
	}, nil
}

func (s *Service) syncQuotaPoliciesToRedis(ctx context.Context, client *redis.Client,
	userPolicies []userQuotaPolicyProjection, keyPolicies []keyQuotaPolicyProjection) error {
	policyTTL := 3 * s.cfg.BillingSyncInterval
	if policyTTL < time.Minute {
		policyTTL = time.Minute
	}
	for _, policy := range userPolicies {
		key := billingDefaultUserPolicyKey + policy.ConsumerName
		if err := client.HSet(ctx, key, map[string]any{
			"consumer_name":            policy.ConsumerName,
			"limit_total_micro_yuan":   policy.LimitTotalMicroYuan,
			"limit_5h_micro_yuan":      policy.Limit5hMicroYuan,
			"limit_daily_micro_yuan":   policy.LimitDailyMicroYuan,
			"daily_reset_mode":         policy.DailyResetMode,
			"daily_reset_time":         policy.DailyResetTime,
			"limit_weekly_micro_yuan":  policy.LimitWeeklyMicroYuan,
			"limit_monthly_micro_yuan": policy.LimitMonthlyMicroYuan,
			"cost_reset_at":            policy.CostResetAtRFC3339Nano,
		}).Err(); err != nil {
			return gerror.Wrapf(err, "sync user quota policy failed: %s", policy.ConsumerName)
		}
		if err := client.Expire(ctx, key, policyTTL).Err(); err != nil {
			return gerror.Wrapf(err, "expire user quota policy failed: %s", policy.ConsumerName)
		}
	}
	for _, policy := range keyPolicies {
		key := billingDefaultKeyPolicyKey + policy.KeyID
		if err := client.HSet(ctx, key, map[string]any{
			"consumer_name":            policy.ConsumerName,
			"limit_total_micro_yuan":   policy.LimitTotalMicroYuan,
			"limit_5h_micro_yuan":      policy.Limit5hMicroYuan,
			"limit_daily_micro_yuan":   policy.LimitDailyMicroYuan,
			"daily_reset_mode":         policy.DailyResetMode,
			"daily_reset_time":         policy.DailyResetTime,
			"limit_weekly_micro_yuan":  policy.LimitWeeklyMicroYuan,
			"limit_monthly_micro_yuan": policy.LimitMonthlyMicroYuan,
		}).Err(); err != nil {
			return gerror.Wrapf(err, "sync key quota policy failed: %s", policy.KeyID)
		}
		if err := client.Expire(ctx, key, policyTTL).Err(); err != nil {
			return gerror.Wrapf(err, "expire key quota policy failed: %s", policy.KeyID)
		}
	}
	return nil
}

func (s *Service) syncQuotaUsageWindowsToRedis(ctx context.Context, client *redis.Client,
	windows []billingQuotaUsageWindow) error {
	for _, window := range windows {
		for consumerName, usage := range window.UserUsage {
			key := billingUsageWindowKey(billingDefaultUserUsageKey, window.Name, consumerName)
			if err := client.Set(ctx, key, usage, window.TTL).Err(); err != nil {
				return gerror.Wrapf(err, "sync user quota usage failed: window=%s consumer=%s", window.Name, consumerName)
			}
		}
		for keyID, usage := range window.KeyUsage {
			key := billingUsageWindowKey(billingDefaultKeyUsageKey, window.Name, keyID)
			if err := client.Set(ctx, key, usage, window.TTL).Err(); err != nil {
				return gerror.Wrapf(err, "sync key quota usage failed: window=%s key=%s", window.Name, keyID)
			}
		}
	}
	return nil
}

func (s *Service) queryBillingUsageByConsumer(ctx context.Context, start time.Time) (map[string]int64, error) {
	args := make([]any, 0, 2)
	startSQL := "TIMESTAMP('1970-01-01 00:00:00')"
	if !start.IsZero() {
		startSQL = "?"
		args = append(args, start)
	}
	records, err := s.db.GetAll(ctx, fmt.Sprintf(`
		SELECT
			t.consumer_name AS target_name,
			COALESCE(SUM(0 - t.amount_micro_yuan), 0) AS used_micro_yuan
		FROM billing_transaction t
		LEFT JOIN quota_policy_user p
			ON p.consumer_name = t.consumer_name
		WHERE t.tx_type IN ('consume', 'reconcile')
		  AND t.amount_micro_yuan < 0
		  AND t.occurred_at >= GREATEST(%s, COALESCE(p.cost_reset_at, TIMESTAMP('1970-01-01 00:00:00')))
		GROUP BY t.consumer_name`, startSQL), args...)
	if err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(records))
	for _, record := range records {
		name := strings.TrimSpace(record["target_name"].String())
		if name == "" {
			continue
		}
		result[name] = record["used_micro_yuan"].Int64()
	}
	return result, nil
}

func (s *Service) queryBillingUsageByKey(ctx context.Context, start time.Time) (map[string]int64, error) {
	return s.queryBillingUsage(ctx, "api_key_id", start, true)
}

func (s *Service) queryBillingUsage(ctx context.Context, groupBy string, start time.Time, requireNonEmptyGroup bool) (map[string]int64, error) {
	where := ""
	args := make([]any, 0, 2)
	if !start.IsZero() {
		where = " AND occurred_at >= ?"
		args = append(args, start)
	}
	if requireNonEmptyGroup {
		where += " AND " + groupBy + " IS NOT NULL AND TRIM(" + groupBy + ") <> ''"
	}
	query := fmt.Sprintf(`
		SELECT %s AS target_name, COALESCE(SUM(0 - amount_micro_yuan), 0) AS used_micro_yuan
		FROM billing_transaction
		WHERE tx_type IN ('consume', 'reconcile')
		  AND amount_micro_yuan < 0%s
		GROUP BY %s`, groupBy, where, groupBy)
	gdbRecords, queryErr := s.db.GetAll(ctx, query, args...)
	if queryErr != nil {
		return nil, queryErr
	}
	result := make(map[string]int64, len(gdbRecords))
	for _, record := range gdbRecords {
		name := strings.TrimSpace(record["target_name"].String())
		if name == "" {
			continue
		}
		result[name] = record["used_micro_yuan"].Int64()
	}
	return result, nil
}

func billingUsageWindowKey(prefix string, window string, target string) string {
	return prefix + window + ":" + target
}

func billingQuotaLocation() *time.Location {
	return time.FixedZone("Asia/Shanghai", 8*60*60)
}

func billingDayStart(now time.Time) time.Time {
	localNow := now.In(billingQuotaLocation())
	return time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, localNow.Location())
}

func billingWeekStart(now time.Time) time.Time {
	dayStart := billingDayStart(now)
	offset := (int(dayStart.Weekday()) + 6) % 7
	return dayStart.AddDate(0, 0, -offset)
}

func billingMonthStart(now time.Time) time.Time {
	localNow := now.In(billingQuotaLocation())
	return time.Date(localNow.Year(), localNow.Month(), 1, 0, 0, 0, 0, localNow.Location())
}

func ttlUntil(deadline time.Time, now time.Time) time.Duration {
	ttl := deadline.Sub(now.In(deadline.Location()))
	if ttl <= 0 {
		return time.Second
	}
	return ttl
}

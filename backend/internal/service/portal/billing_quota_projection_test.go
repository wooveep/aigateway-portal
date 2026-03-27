package portal

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"higress-portal-backend/internal/config"
)

func TestProjectBillingRuntimeToRedisRestoresRuntimeAfterRedisClear(t *testing.T) {
	redisServer, err := miniredis.Run()
	require.NoError(t, err)
	defer redisServer.Close()

	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	defer client.Close()

	service := &Service{
		cfg: config.Config{
			BillingSyncInterval: 15 * time.Second,
		},
	}
	ctx := context.Background()
	resetAt := time.Date(2026, time.March, 27, 9, 30, 0, 0, time.UTC).Format(time.RFC3339Nano)
	wallets := []billingWalletProjection{
		{ConsumerName: "alice", AvailableMicroYuan: 1_230_000},
		{ConsumerName: builtinAdministratorUser, AvailableMicroYuan: 999_999},
	}
	prices := []billingModelPriceProjection{
		{ModelID: "qwen-plus", PriceVersion: 9, InputPer1K: 1234, OutputPer1K: 4321},
	}
	userPolicies := []userQuotaPolicyProjection{
		{
			ConsumerName:           "alice",
			LimitTotalMicroYuan:    9_900_000,
			Limit5hMicroYuan:       1_200_000,
			LimitDailyMicroYuan:    2_300_000,
			DailyResetMode:         "fixed",
			DailyResetTime:         "00:00",
			LimitWeeklyMicroYuan:   7_700_000,
			LimitMonthlyMicroYuan:  12_300_000,
			CostResetAtRFC3339Nano: resetAt,
		},
	}
	keyPolicies := []keyQuotaPolicyProjection{
		{
			KeyID:                 "key-1",
			ConsumerName:          "alice",
			LimitTotalMicroYuan:   4_400_000,
			Limit5hMicroYuan:      550_000,
			LimitDailyMicroYuan:   660_000,
			DailyResetMode:        "fixed",
			DailyResetTime:        "00:00",
			LimitWeeklyMicroYuan:  770_000,
			LimitMonthlyMicroYuan: 880_000,
		},
	}
	windows := []billingQuotaUsageWindow{
		{
			Name:      billingQuotaWindowTotal,
			TTL:       0,
			UserUsage: map[string]int64{"alice": 333},
			KeyUsage:  map[string]int64{"key-1": 222},
		},
		{
			Name:      billingQuotaWindow5h,
			TTL:       5 * time.Minute,
			UserUsage: map[string]int64{"alice": 111},
			KeyUsage:  map[string]int64{"key-1": 101},
		},
		{
			Name:      billingQuotaWindowDaily,
			TTL:       10 * time.Minute,
			UserUsage: map[string]int64{"alice": 444},
			KeyUsage:  map[string]int64{"key-1": 555},
		},
		{
			Name:      billingQuotaWindowWeekly,
			TTL:       15 * time.Minute,
			UserUsage: map[string]int64{"alice": 666},
			KeyUsage:  map[string]int64{"key-1": 777},
		},
		{
			Name:      billingQuotaWindowMonthly,
			TTL:       20 * time.Minute,
			UserUsage: map[string]int64{"alice": 888},
			KeyUsage:  map[string]int64{"key-1": 999},
		},
	}

	require.NoError(t, client.Set(ctx, "stale", "value", 0).Err())
	redisServer.FlushAll()
	require.NoError(t, service.projectBillingRuntimeToRedis(ctx, client, billingDefaultBalanceKey, billingDefaultPriceKey,
		wallets, prices, userPolicies, keyPolicies, windows))
	assertProjectedBillingRuntime(t, client, resetAt)

	redisServer.FlushAll()
	require.False(t, redisServer.Exists(billingDefaultBalanceKey+"alice"))
	require.NoError(t, service.projectBillingRuntimeToRedis(ctx, client, billingDefaultBalanceKey, billingDefaultPriceKey,
		wallets, prices, userPolicies, keyPolicies, windows))
	assertProjectedBillingRuntime(t, client, resetAt)
}

func assertProjectedBillingRuntime(t *testing.T, client *redis.Client, resetAt string) {
	t.Helper()
	ctx := context.Background()

	balance, err := client.Get(ctx, billingDefaultBalanceKey+"alice").Int64()
	require.NoError(t, err)
	require.Equal(t, int64(1_230_000), balance)
	require.False(t, redisKeyExists(ctx, client, billingDefaultBalanceKey+builtinAdministratorUser))

	price, err := client.HGetAll(ctx, billingDefaultPriceKey+"qwen-plus").Result()
	require.NoError(t, err)
	require.Equal(t, "qwen-plus", price["model_id"])
	require.Equal(t, "9", price["price_version_id"])
	require.Equal(t, "1234", price["input_price_per_1k_micro_yuan"])
	require.Equal(t, "4321", price["output_price_per_1k_micro_yuan"])

	userPolicy, err := client.HGetAll(ctx, billingDefaultUserPolicyKey+"alice").Result()
	require.NoError(t, err)
	require.Equal(t, "9900000", userPolicy["limit_total_micro_yuan"])
	require.Equal(t, "fixed", userPolicy["daily_reset_mode"])
	require.Equal(t, "00:00", userPolicy["daily_reset_time"])
	require.Equal(t, resetAt, userPolicy["cost_reset_at"])
	policyTTL, err := client.TTL(ctx, billingDefaultUserPolicyKey+"alice").Result()
	require.NoError(t, err)
	require.Greater(t, policyTTL, time.Duration(0))

	keyPolicy, err := client.HGetAll(ctx, billingDefaultKeyPolicyKey+"key-1").Result()
	require.NoError(t, err)
	require.Equal(t, "alice", keyPolicy["consumer_name"])
	require.Equal(t, "4400000", keyPolicy["limit_total_micro_yuan"])

	assertUsageKey(t, client, billingUsageWindowKey(billingDefaultUserUsageKey, billingQuotaWindowTotal, "alice"), "333", false)
	assertUsageKey(t, client, billingUsageWindowKey(billingDefaultKeyUsageKey, billingQuotaWindowTotal, "key-1"), "222", false)
	assertUsageKey(t, client, billingUsageWindowKey(billingDefaultUserUsageKey, billingQuotaWindow5h, "alice"), "111", true)
	assertUsageKey(t, client, billingUsageWindowKey(billingDefaultKeyUsageKey, billingQuotaWindow5h, "key-1"), "101", true)
	assertUsageKey(t, client, billingUsageWindowKey(billingDefaultUserUsageKey, billingQuotaWindowDaily, "alice"), "444", true)
	assertUsageKey(t, client, billingUsageWindowKey(billingDefaultKeyUsageKey, billingQuotaWindowDaily, "key-1"), "555", true)
	assertUsageKey(t, client, billingUsageWindowKey(billingDefaultUserUsageKey, billingQuotaWindowWeekly, "alice"), "666", true)
	assertUsageKey(t, client, billingUsageWindowKey(billingDefaultKeyUsageKey, billingQuotaWindowWeekly, "key-1"), "777", true)
	assertUsageKey(t, client, billingUsageWindowKey(billingDefaultUserUsageKey, billingQuotaWindowMonthly, "alice"), "888", true)
	assertUsageKey(t, client, billingUsageWindowKey(billingDefaultKeyUsageKey, billingQuotaWindowMonthly, "key-1"), "999", true)
}

func assertUsageKey(t *testing.T, client *redis.Client, key string, expected string, expectTTL bool) {
	t.Helper()
	ctx := context.Background()
	value, err := client.Get(ctx, key).Result()
	require.NoError(t, err)
	require.Equal(t, expected, value)

	ttl, err := client.TTL(ctx, key).Result()
	require.NoError(t, err)
	if expectTTL {
		require.Greater(t, ttl, time.Duration(0))
		return
	}
	require.Equal(t, time.Duration(-1), ttl)
}

func redisKeyExists(ctx context.Context, client *redis.Client, key string) bool {
	exists, err := client.Exists(ctx, key).Result()
	return err == nil && exists > 0
}

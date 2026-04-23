package portal

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestBillingUsageEventInsertSQLPlaceholderCount(t *testing.T) {
	if got, want := strings.Count(billingUsageEventInsertSQL, "?"), 38; got != want {
		t.Fatalf("billingUsageEventInsertSQL placeholder count = %d, want %d", got, want)
	}
}

func TestBillingUsageEventInsertArgsPreserveEmptyCacheTTL(t *testing.T) {
	args := billingUsageEventInsertArgs(billingUsageEventPayload{
		EventID:       "event-1",
		RequestID:     "request-1",
		ConsumerName:  "alice",
		RouteName:     "route-a",
		RequestPath:   "/v1/chat/completions",
		RequestKind:   "chat.completions",
		ModelID:       "qwen-plus",
		RequestStatus: "success",
		UsageStatus:   "parsed",
		HTTPStatus:    200,
		CacheTTL:      "",
		OccurredAt:    time.Now().UTC(),
	}, "stream-1")

	if got, want := len(args), 38; got != want {
		t.Fatalf("billingUsageEventInsertArgs len = %d, want %d", got, want)
	}
	if got, ok := args[28].(string); !ok || got != "" {
		t.Fatalf("billingUsageEventInsertArgs cache_ttl arg = %#v, want empty string", args[28])
	}
}

func TestEnsureBillingUsageConsumerGroupRestoresAfterRedisFlush(t *testing.T) {
	redisServer, err := miniredis.Run()
	require.NoError(t, err)
	defer redisServer.Close()

	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	defer client.Close()

	ctx := context.Background()
	stream := "billing:test:usage"

	require.NoError(t, ensureBillingUsageConsumerGroup(ctx, client, stream))

	require.NoError(t, client.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{"request_id": "before-flush"},
	}).Err())

	redisServer.FlushAll()

	require.NoError(t, client.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{"request_id": "after-flush"},
	}).Err())

	require.NoError(t, ensureBillingUsageConsumerGroup(ctx, client, stream))

	groups, err := client.XInfoGroups(ctx, stream).Result()
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, billingConsumerGroup, groups[0].Name)
	require.EqualValues(t, 0, groups[0].Pending)

	streams, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    billingConsumerGroup,
		Consumer: "test-node",
		Streams:  []string{stream, ">"},
		Count:    10,
	}).Result()
	require.NoError(t, err)
	require.Len(t, streams, 1)
	require.Len(t, streams[0].Messages, 1)
	require.Equal(t, "after-flush", streams[0].Messages[0].Values["request_id"])
}

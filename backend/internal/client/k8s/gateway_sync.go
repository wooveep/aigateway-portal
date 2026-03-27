package k8s

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/retry"
)

const (
	keyAuthPluginLabelSelector = "higress.io/wasm-plugin-name=key-auth"
	aiQuotaPluginLabelSelector = "higress.io/wasm-plugin-name=ai-quota"
	defaultQuotaRedisPrefix    = "chat_quota:"
	defaultBalanceRedisPrefix  = "billing:balance:"
	defaultModelPriceRedisKey  = "billing:model-price:"
	defaultUsageEventStream    = "billing:usage:stream"
)

type KeyAuthConsumer struct {
	Name       string
	Credential string
	KeyID      string
}

type AIQuotaBinding struct {
	ID               string
	QuotaUnit        string
	RedisKeyPrefix   string
	BalanceKeyPrefix string
	PriceKeyPrefix   string
	UsageEventStream string
	Redis            AIQuotaRedisConfig
}

type AIQuotaRedisConfig struct {
	ServiceName   string
	ServicePort   int
	Username      string
	Password      string
	TimeoutMillis int
	Database      int
}

func (c *Client) UpdateKeyAuthConsumers(ctx context.Context, consumers []KeyAuthConsumer) error {
	if c.dynamicClient == nil {
		if c.initErr != nil {
			return gerror.Wrap(c.initErr, "kubernetes key-auth sync unavailable")
		}
		return gerror.New("kubernetes key-auth sync unavailable")
	}

	normalized := make([]KeyAuthConsumer, 0, len(consumers))
	seen := make(map[string]struct{}, len(consumers))
	for _, item := range consumers {
		name := strings.TrimSpace(item.Name)
		credential := strings.TrimSpace(item.Credential)
		keyID := strings.TrimSpace(item.KeyID)
		if name == "" || credential == "" {
			continue
		}
		signature := name + "\x00" + credential + "\x00" + keyID
		if _, ok := seen[signature]; ok {
			continue
		}
		seen[signature] = struct{}{}
		normalized = append(normalized, KeyAuthConsumer{
			Name:       name,
			Credential: credential,
			KeyID:      keyID,
		})
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Name == normalized[j].Name {
			if normalized[i].Credential == normalized[j].Credential {
				return normalized[i].KeyID < normalized[j].KeyID
			}
			return normalized[i].Credential < normalized[j].Credential
		}
		return normalized[i].Name < normalized[j].Name
	})

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		plugin, err := c.getKeyAuthPlugin(ctx)
		if err != nil {
			return err
		}
		spec, found, err := unstructured.NestedMap(plugin.Object, "spec")
		if err != nil {
			return gerror.Wrap(err, "read key-auth wasmplugin spec failed")
		}
		if !found {
			spec = make(map[string]any)
		}
		defaultConfig, found, err := unstructured.NestedMap(spec, "defaultConfig")
		if err != nil {
			return gerror.Wrap(err, "read key-auth wasmplugin default config failed")
		}
		if !found || defaultConfig == nil {
			defaultConfig = make(map[string]any)
		}

		payload := make([]any, 0, len(normalized))
		for _, item := range normalized {
			consumer := map[string]any{
				"name":       item.Name,
				"credential": item.Credential,
			}
			if item.KeyID != "" {
				consumer["key_id"] = item.KeyID
			}
			payload = append(payload, consumer)
		}
		defaultConfig["keys"] = []any{"Authorization", "x-api-key", "x-goog-api-key", "key"}
		defaultConfig["in_header"] = true
		defaultConfig["in_query"] = true
		defaultConfig["consumers"] = payload
		spec["defaultConfig"] = defaultConfig
		plugin.Object["spec"] = spec
		if _, err = c.dynamicClient.Resource(wasmPluginGVR).Namespace(c.namespace).Update(
			ctx, plugin, metav1.UpdateOptions{},
		); err != nil {
			return gerror.Wrap(err, "update key-auth wasmplugin failed")
		}
		return nil
	})
}

func (c *Client) ListEnabledAIQuotaBindings(ctx context.Context) ([]AIQuotaBinding, error) {
	if c.dynamicClient == nil {
		if c.initErr != nil {
			return nil, gerror.Wrap(c.initErr, "kubernetes ai-quota discovery unavailable")
		}
		return nil, gerror.New("kubernetes ai-quota discovery unavailable")
	}

	list, err := c.dynamicClient.Resource(wasmPluginGVR).Namespace(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: aiQuotaPluginLabelSelector,
	})
	if err != nil {
		return nil, gerror.Wrap(err, "list ai-quota wasmplugins failed")
	}
	items := make([]unstructured.Unstructured, 0)
	if list != nil {
		items = append(items, list.Items...)
	}
	if len(items) == 0 {
		if fallback, getErr := c.dynamicClient.Resource(wasmPluginGVR).Namespace(c.namespace).Get(
			ctx, "ai-quota", metav1.GetOptions{},
		); getErr == nil && fallback != nil {
			items = append(items, *fallback)
		}
	}
	if len(items) == 0 {
		return []AIQuotaBinding{}, nil
	}

	result := make([]AIQuotaBinding, 0)
	for _, item := range items {
		spec, found, nestedErr := unstructured.NestedMap(item.Object, "spec")
		if nestedErr != nil || !found || len(spec) == 0 {
			continue
		}
		rules, found, nestedErr := unstructured.NestedSlice(spec, "matchRules")
		if nestedErr != nil || !found || len(rules) == 0 {
			continue
		}
		for idx, ruleItem := range rules {
			ruleMap, ok := ruleItem.(map[string]any)
			if !ok {
				continue
			}
			disabled, hasDisable, disableErr := unstructured.NestedBool(ruleMap, "configDisable")
			if disableErr == nil && hasDisable && disabled {
				continue
			}
			configMap, found, configErr := unstructured.NestedMap(ruleMap, "config")
			if configErr != nil || !found || len(configMap) == 0 {
				continue
			}
			binding, err := parseAIQuotaBinding(item.GetName(), idx, configMap)
			if err != nil {
				return nil, err
			}
			result = append(result, binding)
		}
	}
	return result, nil
}

func (c *Client) getKeyAuthPlugin(ctx context.Context) (*unstructured.Unstructured, error) {
	list, err := c.dynamicClient.Resource(wasmPluginGVR).Namespace(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: keyAuthPluginLabelSelector,
	})
	if err != nil {
		return nil, gerror.Wrap(err, "list key-auth wasmplugins failed")
	}
	items := make([]unstructured.Unstructured, 0, len(list.Items))
	if list != nil {
		items = append(items, list.Items...)
	}
	if len(items) == 0 {
		candidates := []string{"key-auth", "wasm-keyauth"}
		for _, name := range candidates {
			item, getErr := c.dynamicClient.Resource(wasmPluginGVR).Namespace(c.namespace).Get(
				ctx, name, metav1.GetOptions{},
			)
			if getErr == nil && item != nil {
				items = append(items, *item)
				break
			}
		}
	}
	if len(items) == 0 {
		return nil, gerror.New("key-auth wasmplugin not found")
	}

	sort.Slice(items, func(i, j int) bool {
		return keyAuthPluginRank(items[i].GetName()) < keyAuthPluginRank(items[j].GetName())
	})
	chosen := items[0].DeepCopy()
	return chosen, nil
}

func keyAuthPluginRank(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	switch normalized {
	case "key-auth.internal":
		return "0:" + normalized
	case "key-auth":
		return "1:" + normalized
	case "wasm-keyauth":
		return "2:" + normalized
	default:
		return "9:" + normalized
	}
}

func parseAIQuotaBinding(pluginName string, ruleIndex int, config map[string]any) (AIQuotaBinding, error) {
	redisMap, ok := config["redis"].(map[string]any)
	if !ok || len(redisMap) == 0 {
		return AIQuotaBinding{}, gerror.Newf("ai-quota rule missing redis config: plugin=%s rule=%d", pluginName, ruleIndex)
	}
	serviceName := asString(redisMap["service_name"])
	if serviceName == "" {
		return AIQuotaBinding{}, gerror.Newf(
			"ai-quota rule missing redis.service_name: plugin=%s rule=%d", pluginName, ruleIndex,
		)
	}
	servicePort := parseIntValue(redisMap["service_port"], 0)
	if servicePort <= 0 {
		if strings.HasSuffix(serviceName, ".static") {
			servicePort = 80
		} else {
			servicePort = 6379
		}
	}
	timeoutMillis := parseIntValue(redisMap["timeout"], 1000)
	if timeoutMillis <= 0 {
		timeoutMillis = 1000
	}

	redisKeyPrefix := asString(config["redis_key_prefix"])
	if redisKeyPrefix == "" {
		redisKeyPrefix = defaultQuotaRedisPrefix
	}
	quotaUnit := strings.ToLower(strings.TrimSpace(asString(config["quota_unit"])))
	if quotaUnit == "" {
		quotaUnit = "token"
	}
	balanceKeyPrefix := asString(config["balance_key_prefix"])
	if balanceKeyPrefix == "" && quotaUnit == "amount" {
		balanceKeyPrefix = defaultBalanceRedisPrefix
	}
	priceKeyPrefix := asString(config["price_key_prefix"])
	if priceKeyPrefix == "" && quotaUnit == "amount" {
		priceKeyPrefix = defaultModelPriceRedisKey
	}
	usageEventStream := asString(config["usage_event_stream"])
	if usageEventStream == "" && quotaUnit == "amount" {
		usageEventStream = defaultUsageEventStream
	}

	return AIQuotaBinding{
		ID:               fmt.Sprintf("%s#%d", pluginName, ruleIndex),
		QuotaUnit:        quotaUnit,
		RedisKeyPrefix:   redisKeyPrefix,
		BalanceKeyPrefix: balanceKeyPrefix,
		PriceKeyPrefix:   priceKeyPrefix,
		UsageEventStream: usageEventStream,
		Redis: AIQuotaRedisConfig{
			ServiceName:   serviceName,
			ServicePort:   servicePort,
			Username:      asString(redisMap["username"]),
			Password:      asString(redisMap["password"]),
			TimeoutMillis: timeoutMillis,
			Database:      parseIntValue(redisMap["database"], 0),
		},
	}, nil
}

func parseIntValue(v any, defaultValue int) int {
	switch value := v.(type) {
	case int:
		return value
	case int8:
		return int(value)
	case int16:
		return int(value)
	case int32:
		return int(value)
	case int64:
		return int(value)
	case uint:
		return int(value)
	case uint8:
		return int(value)
	case uint16:
		return int(value)
	case uint32:
		return int(value)
	case uint64:
		return int(value)
	case float32:
		return int(value)
	case float64:
		return int(value)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return defaultValue
		}
		return parsed
	default:
		return defaultValue
	}
}

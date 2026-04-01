package portal

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/redis/go-redis/v9"

	clientK8s "higress-portal-backend/internal/client/k8s"
	"higress-portal-backend/internal/model"
)

const (
	billingCurrencyCNY                = "CNY"
	billingQuotaUnitAmount            = "amount"
	billingDefaultBalanceKey          = "billing:balance:"
	billingDefaultPriceKey            = "billing:model-price:"
	billingDefaultUsageStream         = "billing:usage:stream"
	billingDefaultUserPolicyKey       = "billing:quota-policy:user:"
	billingDefaultKeyPolicyKey        = "billing:quota-policy:key:"
	billingDefaultUserUsageKey        = "billing:quota-usage:user:"
	billingDefaultKeyUsageKey         = "billing:quota-usage:key:"
	billingConsumerGroup              = "portal-billing"
	builtinAdministratorUser          = "administrator"
	microYuanPerRMB             int64 = 1_000_000
)

const billingUsageEventInsertSQL = `
			INSERT INTO billing_usage_event
			(event_id, request_id, trace_id, consumer_name, api_key_id, route_name, request_path, request_kind, model_id,
			 request_status, usage_status, http_status, error_code, error_message,
			 input_tokens, output_tokens, total_tokens, cache_creation_input_tokens, cache_creation_5m_input_tokens,
			 cache_creation_1h_input_tokens, cache_read_input_tokens, input_image_tokens, output_image_tokens,
			 input_image_count, output_image_count, request_count, cache_ttl,
			 input_token_details_json, output_token_details_json, provider_usage_json,
			 cost_micro_yuan, price_version_id, started_at, finished_at, redis_stream_id, occurred_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

type billingModelPriceProjection struct {
	ModelID                                             string
	PriceVersion                                        int64
	InputPer1K                                          int64
	OutputPer1K                                         int64
	InputRequestPriceMicroYuan                          int64
	CacheCreationInputTokenPricePer1KMicroYuan          int64
	CacheCreationInputTokenPriceAbove1hrPer1KMicroYuan  int64
	CacheReadInputTokenPricePer1KMicroYuan              int64
	InputTokenPriceAbove200kPer1KMicroYuan              int64
	OutputTokenPriceAbove200kPer1KMicroYuan             int64
	CacheCreationInputTokenPriceAbove200kPer1KMicroYuan int64
	CacheReadInputTokenPriceAbove200kPer1KMicroYuan     int64
	OutputImagePriceMicroYuan                           int64
	OutputImageTokenPricePer1KMicroYuan                 int64
	InputImagePriceMicroYuan                            int64
	InputImageTokenPricePer1KMicroYuan                  int64
	SupportsPromptCaching                               bool
}

type billingWalletProjection struct {
	ConsumerName       string
	AvailableMicroYuan int64
}

type billingUsageEventPayload struct {
	EventID                    string
	RequestID                  string
	TraceID                    string
	ConsumerName               string
	RouteName                  string
	RequestPath                string
	RequestKind                string
	APIKeyID                   string
	ModelID                    string
	RequestStatus              string
	UsageStatus                string
	HTTPStatus                 int
	ErrorCode                  string
	ErrorMessage               string
	InputTokens                int64
	OutputTokens               int64
	TotalTokens                int64
	CacheCreationInputTokens   int64
	CacheCreation5mInputTokens int64
	CacheCreation1hInputTokens int64
	CacheReadInputTokens       int64
	InputImageTokens           int64
	OutputImageTokens          int64
	InputImageCount            int64
	OutputImageCount           int64
	RequestCount               int64
	CacheTTL                   string
	InputTokenDetailsJSON      string
	OutputTokenDetailsJSON     string
	ProviderUsageJSON          string
	CostMicroYuan              int64
	PriceVersionID             int64
	StartedAt                  time.Time
	FinishedAt                 time.Time
	OccurredAt                 time.Time
}

func rmbToMicroYuan(amount float64) int64 {
	return int64(math.Round(amount * float64(microYuanPerRMB)))
}

func microYuanToRMB(amount int64) float64 {
	return float64(amount) / float64(microYuanPerRMB)
}

func microYuanToText(amount int64) string {
	return fmt.Sprintf("%.2f", microYuanToRMB(amount))
}

func (s *Service) bootstrapBillingState(ctx context.Context) error {
	if err := s.bootstrapBillingModels(ctx); err != nil {
		return err
	}
	if err := s.bootstrapLegacyBillingTransactions(ctx); err != nil {
		return err
	}
	if err := s.rebuildBillingWallets(ctx); err != nil {
		return err
	}
	summary, err := s.collectBillingBackfillSummary(ctx)
	if err != nil {
		return err
	}
	s.logBillingBackfillSummary(ctx, summary)
	if err = validateBillingBackfillSummary(summary); err != nil {
		return err
	}
	return nil
}

func (s *Service) bootstrapBillingModels(ctx context.Context) error {
	records, err := s.db.GetAll(ctx, `
		SELECT model_id, name, vendor, capability, input_token_price, output_token_price, endpoint, sdk, summary
		FROM portal_model_catalog
		WHERE status = 'active'`)
	if err != nil {
		return gerror.Wrap(err, "query portal model catalog failed")
	}

	entries := make([]activeModelPrice, 0, len(records))
	for _, record := range records {
		modelID := strings.TrimSpace(record["model_id"].String())
		if modelID == "" {
			continue
		}
		entries = append(entries, activeModelPrice{
			ModelID:    modelID,
			Name:       strings.TrimSpace(record["name"].String()),
			Vendor:     strings.TrimSpace(record["vendor"].String()),
			Capability: strings.TrimSpace(record["capability"].String()),
			Pricing: materializeModelPricing(model.ModelPricing{
				Currency:           billingCurrencyCNY,
				InputPer1K:         record["input_token_price"].Float64(),
				OutputPer1K:        record["output_token_price"].Float64(),
				InputCostPerToken:  record["input_token_price"].Float64() / 1000,
				OutputCostPerToken: record["output_token_price"].Float64() / 1000,
			}),
			Endpoint: strings.TrimSpace(record["endpoint"].String()),
			SDK:      strings.TrimSpace(record["sdk"].String()),
			Summary:  strings.TrimSpace(record["summary"].String()),
		})
	}
	return s.upsertBillingModels(ctx, entries)
}

func (s *Service) bootstrapLegacyBillingTransactions(ctx context.Context) error {
	if _, err := s.db.Exec(ctx, `
		INSERT INTO billing_transaction
		(tx_id, consumer_name, tx_type, amount_micro_yuan, currency, source_type, source_id, occurred_at, created_at)
		SELECT
			CONCAT('r', SUBSTRING(SHA2(CONCAT('portal_recharge_order:', order_id), 256), 1, 32)),
			consumer_name,
			'recharge',
			ROUND(amount * 1000000),
			'CNY',
			'portal_recharge_order',
			order_id,
			created_at,
			created_at
		FROM portal_recharge_order
		WHERE status = 'success'
		  AND LOWER(TRIM(consumer_name)) <> 'administrator'
		ON DUPLICATE KEY UPDATE
			consumer_name = VALUES(consumer_name),
			tx_type = VALUES(tx_type),
			amount_micro_yuan = VALUES(amount_micro_yuan),
			currency = VALUES(currency),
			occurred_at = VALUES(occurred_at),
			created_at = VALUES(created_at)`); err != nil {
		return gerror.Wrap(err, "bootstrap recharge transactions failed")
	}
	if _, err := s.db.Exec(ctx, `
		INSERT INTO billing_transaction
		(tx_id, consumer_name, tx_type, amount_micro_yuan, currency, source_type, source_id, model_id,
		 input_tokens, output_tokens, cache_creation_input_tokens, cache_creation_5m_input_tokens,
		 cache_creation_1h_input_tokens, cache_read_input_tokens, input_image_tokens, output_image_tokens,
		 input_image_count, output_image_count, request_count, occurred_at, created_at)
		SELECT
			CONCAT('u', SUBSTRING(SHA2(CONCAT('portal_usage_daily:', id), 256), 1, 32)),
			consumer_name,
			'consume',
			0 - ROUND(cost_amount * 1000000),
			'CNY',
			'portal_usage_daily',
			CAST(id AS CHAR),
			model_name,
			input_tokens,
			output_tokens,
			cache_creation_input_tokens,
			cache_creation_5m_input_tokens,
			cache_creation_1h_input_tokens,
			cache_read_input_tokens,
			input_image_tokens,
			output_image_tokens,
			input_image_count,
			output_image_count,
			request_count,
			updated_at,
			updated_at
		FROM portal_usage_daily
		WHERE LOWER(TRIM(consumer_name)) <> 'administrator'
		ON DUPLICATE KEY UPDATE
			consumer_name = VALUES(consumer_name),
			tx_type = VALUES(tx_type),
			amount_micro_yuan = VALUES(amount_micro_yuan),
			currency = VALUES(currency),
			model_id = VALUES(model_id),
			input_tokens = VALUES(input_tokens),
			output_tokens = VALUES(output_tokens),
			cache_creation_input_tokens = VALUES(cache_creation_input_tokens),
			cache_creation_5m_input_tokens = VALUES(cache_creation_5m_input_tokens),
			cache_creation_1h_input_tokens = VALUES(cache_creation_1h_input_tokens),
			cache_read_input_tokens = VALUES(cache_read_input_tokens),
			input_image_tokens = VALUES(input_image_tokens),
			output_image_tokens = VALUES(output_image_tokens),
			input_image_count = VALUES(input_image_count),
			output_image_count = VALUES(output_image_count),
			request_count = VALUES(request_count),
			occurred_at = VALUES(occurred_at),
			created_at = VALUES(created_at)`); err != nil {
		return gerror.Wrap(err, "bootstrap usage transactions failed")
	}
	return nil
}

func (s *Service) rebuildBillingWallets(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO billing_wallet
		(consumer_name, currency, available_micro_yuan, version)
		SELECT consumer_name, 'CNY', COALESCE(SUM(amount_micro_yuan), 0), 1
		FROM billing_transaction
		WHERE LOWER(TRIM(consumer_name)) <> 'administrator'
		GROUP BY consumer_name
		ON DUPLICATE KEY UPDATE
			currency = VALUES(currency),
			available_micro_yuan = VALUES(available_micro_yuan),
			version = version + 1`)
	if err != nil {
		return gerror.Wrap(err, "rebuild billing wallets failed")
	}
	return nil
}

func (s *Service) StartBillingSync(ctx context.Context) {
	if !s.cfg.BillingSyncEnabled {
		return
	}
	if err := s.syncBillingStateOnce(ctx); err != nil {
		s.logf(ctx, "initial billing sync failed: %v", err)
	}

	bindings, err := s.listAmountQuotaBindings(ctx)
	if err != nil {
		s.logf(ctx, "discover amount ai-quota bindings failed: %v", err)
	} else {
		for _, binding := range bindings {
			go s.consumeBillingUsageEventsLoop(ctx, binding)
		}
	}

	interval := s.cfg.BillingSyncInterval
	if interval < time.Second {
		interval = 15 * time.Second
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.syncBillingStateOnce(ctx); err != nil {
					s.logf(ctx, "billing sync failed: %v", err)
				}
			}
		}
	}()
}

func (s *Service) syncBillingStateOnce(ctx context.Context) error {
	if entries, err := s.loadActiveModelPricesFromGateway(ctx); err != nil {
		s.logf(ctx, "sync billing models from gateway skipped: %v", err)
	} else if len(entries) > 0 {
		if err = s.syncModelCatalogFromGateway(ctx, entries); err != nil {
			return err
		}
	}
	return s.syncBillingRuntimeProjection(ctx)
}

func (s *Service) listAmountQuotaBindings(ctx context.Context) ([]clientK8s.AIQuotaBinding, error) {
	bindings, err := s.modelK8s.ListEnabledAIQuotaBindings(ctx)
	if err != nil {
		return nil, err
	}
	filtered := make([]clientK8s.AIQuotaBinding, 0, len(bindings))
	seen := make(map[string]struct{}, len(bindings))
	for _, binding := range bindings {
		if !strings.EqualFold(strings.TrimSpace(binding.QuotaUnit), billingQuotaUnitAmount) {
			continue
		}
		if strings.TrimSpace(binding.BalanceKeyPrefix) == "" {
			binding.BalanceKeyPrefix = billingDefaultBalanceKey
		}
		if strings.TrimSpace(binding.PriceKeyPrefix) == "" {
			binding.PriceKeyPrefix = billingDefaultPriceKey
		}
		if strings.TrimSpace(binding.UsageEventStream) == "" {
			binding.UsageEventStream = billingDefaultUsageStream
		}
		signature := fmt.Sprintf("%s|%d|%s|%d|%s|%s|%s",
			binding.Redis.ServiceName,
			binding.Redis.ServicePort,
			binding.Redis.Username,
			binding.Redis.Database,
			binding.BalanceKeyPrefix,
			binding.PriceKeyPrefix,
			binding.UsageEventStream,
		)
		if _, ok := seen[signature]; ok {
			continue
		}
		seen[signature] = struct{}{}
		filtered = append(filtered, binding)
	}
	return filtered, nil
}

func (s *Service) syncBillingRuntimeProjection(ctx context.Context) error {
	bindings, err := s.listAmountQuotaBindings(ctx)
	if err != nil {
		return gerror.Wrap(err, "list amount ai-quota bindings failed")
	}
	if len(bindings) == 0 {
		return nil
	}

	walletRows, err := s.db.GetAll(ctx, `
		SELECT consumer_name, available_micro_yuan
		FROM billing_wallet`)
	if err != nil {
		return gerror.Wrap(err, "query billing wallets failed")
	}
	wallets := make([]billingWalletProjection, 0, len(walletRows))
	for _, wallet := range walletRows {
		consumerName := model.NormalizeUsername(wallet["consumer_name"].String())
		if consumerName == "" {
			continue
		}
		wallets = append(wallets, billingWalletProjection{
			ConsumerName:       consumerName,
			AvailableMicroYuan: wallet["available_micro_yuan"].Int64(),
		})
	}
	priceRows, err := s.loadBillingModelPriceProjections(ctx)
	if err != nil {
		return gerror.Wrap(err, "query billing model prices failed")
	}
	userPolicies, err := s.loadUserQuotaPolicyProjections(ctx)
	if err != nil {
		return gerror.Wrap(err, "query user quota policies failed")
	}
	keyPolicies, err := s.loadKeyQuotaPolicyProjections(ctx)
	if err != nil {
		return gerror.Wrap(err, "query key quota policies failed")
	}
	usageWindows, err := s.loadBillingQuotaUsageWindows(ctx, time.Now())
	if err != nil {
		return gerror.Wrap(err, "query billing quota usage windows failed")
	}

	for _, binding := range bindings {
		client := newRedisClient(binding.Redis)
		if err = s.projectBillingRuntimeToRedis(ctx, client, binding.BalanceKeyPrefix, binding.PriceKeyPrefix,
			wallets, priceRows, userPolicies, keyPolicies, usageWindows); err != nil {
			_ = client.Close()
			return err
		}
		_ = client.Close()
	}
	return nil
}

func (s *Service) projectBillingRuntimeToRedis(ctx context.Context, client *redis.Client,
	balanceKeyPrefix string, priceKeyPrefix string,
	wallets []billingWalletProjection, priceRows []billingModelPriceProjection,
	userPolicies []userQuotaPolicyProjection, keyPolicies []keyQuotaPolicyProjection,
	usageWindows []billingQuotaUsageWindow) error {
	if strings.TrimSpace(balanceKeyPrefix) == "" {
		balanceKeyPrefix = billingDefaultBalanceKey
	}
	if strings.TrimSpace(priceKeyPrefix) == "" {
		priceKeyPrefix = billingDefaultPriceKey
	}
	for _, wallet := range wallets {
		consumerName := model.NormalizeUsername(wallet.ConsumerName)
		if consumerName == "" || strings.EqualFold(consumerName, builtinAdministratorUser) {
			continue
		}
		if err := client.Set(ctx, balanceKeyPrefix+consumerName, wallet.AvailableMicroYuan, 0).Err(); err != nil {
			return gerror.Wrapf(err, "sync billing wallet to redis failed: %s", consumerName)
		}
	}
	for _, price := range priceRows {
		if err := client.HSet(ctx, priceKeyPrefix+price.ModelID, map[string]any{
			"model_id":                       price.ModelID,
			"price_version_id":               price.PriceVersion,
			"currency":                       billingCurrencyCNY,
			"input_price_per_1k_micro_yuan":  price.InputPer1K,
			"output_price_per_1k_micro_yuan": price.OutputPer1K,
			"input_request_price_micro_yuan": price.InputRequestPriceMicroYuan,
			"cache_creation_input_token_price_per_1k_micro_yuan":            price.CacheCreationInputTokenPricePer1KMicroYuan,
			"cache_creation_input_token_price_above_1hr_per_1k_micro_yuan":  price.CacheCreationInputTokenPriceAbove1hrPer1KMicroYuan,
			"cache_read_input_token_price_per_1k_micro_yuan":                price.CacheReadInputTokenPricePer1KMicroYuan,
			"input_token_price_above_200k_per_1k_micro_yuan":                price.InputTokenPriceAbove200kPer1KMicroYuan,
			"output_token_price_above_200k_per_1k_micro_yuan":               price.OutputTokenPriceAbove200kPer1KMicroYuan,
			"cache_creation_input_token_price_above_200k_per_1k_micro_yuan": price.CacheCreationInputTokenPriceAbove200kPer1KMicroYuan,
			"cache_read_input_token_price_above_200k_per_1k_micro_yuan":     price.CacheReadInputTokenPriceAbove200kPer1KMicroYuan,
			"output_image_price_micro_yuan":                                 price.OutputImagePriceMicroYuan,
			"output_image_token_price_per_1k_micro_yuan":                    price.OutputImageTokenPricePer1KMicroYuan,
			"input_image_price_micro_yuan":                                  price.InputImagePriceMicroYuan,
			"input_image_token_price_per_1k_micro_yuan":                     price.InputImageTokenPricePer1KMicroYuan,
			"supports_prompt_caching":                                       boolToInt(price.SupportsPromptCaching),
		}).Err(); err != nil {
			return gerror.Wrapf(err, "sync billing model price to redis failed: %s", price.ModelID)
		}
	}
	if err := s.syncQuotaPoliciesToRedis(ctx, client, userPolicies, keyPolicies); err != nil {
		return err
	}
	if err := s.syncQuotaUsageWindowsToRedis(ctx, client, usageWindows); err != nil {
		return err
	}
	return nil
}

func (s *Service) syncConsumerBalanceToRedis(ctx context.Context, consumerName string) error {
	bindings, err := s.listAmountQuotaBindings(ctx)
	if err != nil || len(bindings) == 0 {
		return err
	}
	value, err := s.db.GetValue(ctx, `
		SELECT available_micro_yuan
		FROM billing_wallet
		WHERE consumer_name = ?`, consumerName)
	if err != nil {
		return gerror.Wrap(err, "query wallet balance failed")
	}
	for _, binding := range bindings {
		client := newRedisClient(binding.Redis)
		if err = client.Set(ctx, binding.BalanceKeyPrefix+consumerName, value.Int64(), 0).Err(); err != nil {
			_ = client.Close()
			return gerror.Wrapf(err, "sync consumer balance failed: %s", consumerName)
		}
		_ = client.Close()
	}
	return nil
}

func (s *Service) loadBillingModelPriceProjections(ctx context.Context) ([]billingModelPriceProjection, error) {
	records, err := s.db.GetAll(ctx, `
		SELECT c.model_id, p.id AS price_version_id, p.input_price_per_1k_micro_yuan, p.output_price_per_1k_micro_yuan,
			p.input_request_price_micro_yuan,
			p.cache_creation_input_token_price_per_1k_micro_yuan,
			p.cache_creation_input_token_price_above_1hr_per_1k_micro_yuan,
			p.cache_read_input_token_price_per_1k_micro_yuan,
			p.input_token_price_above_200k_per_1k_micro_yuan,
			p.output_token_price_above_200k_per_1k_micro_yuan,
			p.cache_creation_input_token_price_above_200k_per_1k_micro_yuan,
			p.cache_read_input_token_price_above_200k_per_1k_micro_yuan,
			p.output_image_price_micro_yuan,
			p.output_image_token_price_per_1k_micro_yuan,
			p.input_image_price_micro_yuan,
			p.input_image_token_price_per_1k_micro_yuan,
			p.supports_prompt_caching
		FROM billing_model_catalog c
		INNER JOIN billing_model_price_version p
			ON p.model_id = c.model_id
		WHERE c.status = 'active'
		  AND p.status = 'active'
		  AND p.effective_to IS NULL`)
	if err != nil {
		return nil, err
	}
	items := make([]billingModelPriceProjection, 0, len(records))
	for _, record := range records {
		modelID := strings.TrimSpace(record["model_id"].String())
		if modelID == "" {
			continue
		}
		items = append(items, billingModelPriceProjection{
			ModelID:                    modelID,
			PriceVersion:               record["price_version_id"].Int64(),
			InputPer1K:                 record["input_price_per_1k_micro_yuan"].Int64(),
			OutputPer1K:                record["output_price_per_1k_micro_yuan"].Int64(),
			InputRequestPriceMicroYuan: record["input_request_price_micro_yuan"].Int64(),
			CacheCreationInputTokenPricePer1KMicroYuan:          record["cache_creation_input_token_price_per_1k_micro_yuan"].Int64(),
			CacheCreationInputTokenPriceAbove1hrPer1KMicroYuan:  record["cache_creation_input_token_price_above_1hr_per_1k_micro_yuan"].Int64(),
			CacheReadInputTokenPricePer1KMicroYuan:              record["cache_read_input_token_price_per_1k_micro_yuan"].Int64(),
			InputTokenPriceAbove200kPer1KMicroYuan:              record["input_token_price_above_200k_per_1k_micro_yuan"].Int64(),
			OutputTokenPriceAbove200kPer1KMicroYuan:             record["output_token_price_above_200k_per_1k_micro_yuan"].Int64(),
			CacheCreationInputTokenPriceAbove200kPer1KMicroYuan: record["cache_creation_input_token_price_above_200k_per_1k_micro_yuan"].Int64(),
			CacheReadInputTokenPriceAbove200kPer1KMicroYuan:     record["cache_read_input_token_price_above_200k_per_1k_micro_yuan"].Int64(),
			OutputImagePriceMicroYuan:                           record["output_image_price_micro_yuan"].Int64(),
			OutputImageTokenPricePer1KMicroYuan:                 record["output_image_token_price_per_1k_micro_yuan"].Int64(),
			InputImagePriceMicroYuan:                            record["input_image_price_micro_yuan"].Int64(),
			InputImageTokenPricePer1KMicroYuan:                  record["input_image_token_price_per_1k_micro_yuan"].Int64(),
			SupportsPromptCaching:                               record["supports_prompt_caching"].Int64() > 0,
		})
	}
	return items, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func billingUsageEventInsertArgs(payload billingUsageEventPayload, streamID string) []any {
	return []any{
		payload.EventID,
		payload.RequestID,
		nullIfEmpty(payload.TraceID),
		payload.ConsumerName,
		nullIfEmpty(payload.APIKeyID),
		payload.RouteName,
		payload.RequestPath,
		payload.RequestKind,
		payload.ModelID,
		payload.RequestStatus,
		payload.UsageStatus,
		payload.HTTPStatus,
		payload.ErrorCode,
		payload.ErrorMessage,
		payload.InputTokens,
		payload.OutputTokens,
		payload.TotalTokens,
		payload.CacheCreationInputTokens,
		payload.CacheCreation5mInputTokens,
		payload.CacheCreation1hInputTokens,
		payload.CacheReadInputTokens,
		payload.InputImageTokens,
		payload.OutputImageTokens,
		payload.InputImageCount,
		payload.OutputImageCount,
		payload.RequestCount,
		payload.CacheTTL,
		nullIfEmpty(payload.InputTokenDetailsJSON),
		nullIfEmpty(payload.OutputTokenDetailsJSON),
		nullIfEmpty(payload.ProviderUsageJSON),
		payload.CostMicroYuan,
		nullIfZero(payload.PriceVersionID),
		nullIfZeroTime(payload.StartedAt),
		nullIfZeroTime(payload.FinishedAt),
		streamID,
		payload.OccurredAt,
	}
}

func ensureBillingUsageConsumerGroup(ctx context.Context, client *redis.Client, stream string) error {
	if err := client.XGroupCreateMkStream(ctx, stream, billingConsumerGroup, "0").Err(); err != nil &&
		!strings.Contains(strings.ToUpper(err.Error()), "BUSYGROUP") {
		return err
	}
	return nil
}

func isRedisNoGroupError(err error) bool {
	return err != nil && strings.Contains(strings.ToUpper(err.Error()), "NOGROUP")
}

func (s *Service) consumeBillingUsageEventsLoop(ctx context.Context, binding clientK8s.AIQuotaBinding) {
	client := newRedisClient(binding.Redis)
	defer client.Close()

	stream := binding.UsageEventStream
	if stream == "" {
		stream = billingDefaultUsageStream
	}
	if err := ensureBillingUsageConsumerGroup(ctx, client, stream); err != nil {
		s.logf(ctx, "create billing redis consumer group failed: %v", err)
		return
	}

	block := s.cfg.BillingConsumerBlock
	if block <= 0 {
		block = 5 * time.Second
	}
	count := int64(s.cfg.BillingConsumerBatchSize)
	if count <= 0 {
		count = 20
	}

	for {
		if ctx.Err() != nil {
			return
		}
		streams, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    billingConsumerGroup,
			Consumer: s.billingNodeName,
			Streams:  []string{stream, ">"},
			Count:    count,
			Block:    block,
		}).Result()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if isRedisNoGroupError(err) {
				if groupErr := ensureBillingUsageConsumerGroup(ctx, client, stream); groupErr != nil {
					s.logf(ctx, "recreate billing redis consumer group failed: %v", groupErr)
				} else {
					s.logf(ctx, "recreated billing redis consumer group: stream=%s group=%s", stream, billingConsumerGroup)
				}
				time.Sleep(time.Second)
				continue
			}
			s.logf(ctx, "billing usage event consume failed: %v", err)
			time.Sleep(time.Second)
			continue
		}
		for _, streamResult := range streams {
			for _, msg := range streamResult.Messages {
				if err = s.processBillingUsageEvent(ctx, msg.ID, msg.Values); err != nil {
					s.logf(ctx, "process billing usage event failed: stream=%s id=%s err=%v", stream, msg.ID, err)
					continue
				}
				if ackErr := client.XAck(ctx, stream, billingConsumerGroup, msg.ID).Err(); ackErr != nil {
					s.logf(ctx, "ack billing usage event failed: stream=%s id=%s err=%v", stream, msg.ID, ackErr)
				}
			}
		}
	}
}

func (s *Service) processBillingUsageEvent(ctx context.Context, streamID string, values map[string]any) error {
	payload, err := parseBillingUsageEventPayload(streamID, values)
	if err != nil {
		return err
	}
	if strings.EqualFold(payload.ConsumerName, builtinAdministratorUser) {
		return nil
	}

	err = s.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, txErr := tx.Exec(billingUsageEventInsertSQL, billingUsageEventInsertArgs(payload, streamID)...); txErr != nil {
			if isDuplicateEntryErr(txErr) {
				return nil
			}
			return gerror.Wrap(txErr, "insert billing usage event failed")
		}

		if payload.APIKeyID != "" && strings.EqualFold(payload.RequestStatus, "success") {
			if _, txErr := tx.Exec(`
				UPDATE portal_api_key
				SET total_calls = total_calls + 1,
					last_used_at = CASE
						WHEN last_used_at IS NULL OR last_used_at < ? THEN ?
						ELSE last_used_at
					END
				WHERE key_id = ? AND consumer_name = ?`,
				payload.OccurredAt,
				payload.OccurredAt,
				payload.APIKeyID,
				payload.ConsumerName,
			); txErr != nil {
				return gerror.Wrap(txErr, "update portal api key usage failed")
			}
		}

		if !strings.EqualFold(payload.RequestStatus, "success") || !strings.EqualFold(payload.UsageStatus, "parsed") {
			return nil
		}

		if _, txErr := tx.Exec(`
			INSERT INTO billing_transaction
			(tx_id, consumer_name, tx_type, amount_micro_yuan, currency, source_type, source_id,
			 request_id, api_key_id, model_id, price_version_id, input_tokens, output_tokens,
			 cache_creation_input_tokens, cache_creation_5m_input_tokens, cache_creation_1h_input_tokens,
			 cache_read_input_tokens, input_image_tokens, output_image_tokens, input_image_count,
			 output_image_count, request_count, occurred_at)
			VALUES (?, ?, 'consume', ?, 'CNY', 'billing_usage_event', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"c"+sha256Hex("billing_usage_event:" + payload.EventID)[:32],
			payload.ConsumerName,
			0-payload.CostMicroYuan,
			payload.EventID,
			payload.RequestID,
			nullIfEmpty(payload.APIKeyID),
			payload.ModelID,
			nullIfZero(payload.PriceVersionID),
			payload.InputTokens,
			payload.OutputTokens,
			payload.CacheCreationInputTokens,
			payload.CacheCreation5mInputTokens,
			payload.CacheCreation1hInputTokens,
			payload.CacheReadInputTokens,
			payload.InputImageTokens,
			payload.OutputImageTokens,
			payload.InputImageCount,
			payload.OutputImageCount,
			payload.RequestCount,
			payload.OccurredAt,
		); txErr != nil {
			return gerror.Wrap(txErr, "insert billing consume transaction failed")
		}

		if _, txErr := tx.Exec(`
			INSERT INTO billing_wallet
			(consumer_name, currency, available_micro_yuan, version)
			VALUES (?, 'CNY', ?, 1)
			ON DUPLICATE KEY UPDATE
				available_micro_yuan = available_micro_yuan + VALUES(available_micro_yuan),
				version = version + 1`,
			payload.ConsumerName,
			0-payload.CostMicroYuan,
		); txErr != nil {
			return gerror.Wrap(txErr, "update billing wallet failed")
		}

		if _, txErr := tx.Exec(`
			INSERT INTO portal_usage_daily
			(billing_date, consumer_name, model_name, request_count, input_tokens, output_tokens, total_tokens,
			 cache_creation_input_tokens, cache_creation_5m_input_tokens, cache_creation_1h_input_tokens,
			 cache_read_input_tokens, input_image_tokens, output_image_tokens, input_image_count, output_image_count,
			 cost_amount, source_from, source_to)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				request_count = request_count + VALUES(request_count),
				input_tokens = input_tokens + VALUES(input_tokens),
				output_tokens = output_tokens + VALUES(output_tokens),
				total_tokens = total_tokens + VALUES(total_tokens),
				cache_creation_input_tokens = cache_creation_input_tokens + VALUES(cache_creation_input_tokens),
				cache_creation_5m_input_tokens = cache_creation_5m_input_tokens + VALUES(cache_creation_5m_input_tokens),
				cache_creation_1h_input_tokens = cache_creation_1h_input_tokens + VALUES(cache_creation_1h_input_tokens),
				cache_read_input_tokens = cache_read_input_tokens + VALUES(cache_read_input_tokens),
				input_image_tokens = input_image_tokens + VALUES(input_image_tokens),
				output_image_tokens = output_image_tokens + VALUES(output_image_tokens),
				input_image_count = input_image_count + VALUES(input_image_count),
				output_image_count = output_image_count + VALUES(output_image_count),
				cost_amount = cost_amount + VALUES(cost_amount),
				source_from = IF(source_from IS NULL OR VALUES(source_from) < source_from, VALUES(source_from), source_from),
				source_to = IF(source_to IS NULL OR VALUES(source_to) > source_to, VALUES(source_to), source_to)`,
			model.DayText(payload.OccurredAt),
			payload.ConsumerName,
			payload.ModelID,
			maxInt64(payload.RequestCount, 1),
			payload.InputTokens,
			payload.OutputTokens,
			payload.TotalTokens,
			payload.CacheCreationInputTokens,
			payload.CacheCreation5mInputTokens,
			payload.CacheCreation1hInputTokens,
			payload.CacheReadInputTokens,
			payload.InputImageTokens,
			payload.OutputImageTokens,
			payload.InputImageCount,
			payload.OutputImageCount,
			microYuanToRMB(payload.CostMicroYuan),
			payload.OccurredAt,
			payload.OccurredAt,
		); txErr != nil {
			return gerror.Wrap(txErr, "upsert portal usage daily failed")
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func parseBillingUsageEventPayload(streamID string, values map[string]any) (billingUsageEventPayload, error) {
	payload := billingUsageEventPayload{
		EventID:                    strings.TrimSpace(stringifyAny(values["event_id"])),
		RequestID:                  strings.TrimSpace(stringifyAny(values["request_id"])),
		TraceID:                    strings.TrimSpace(stringifyAny(values["trace_id"])),
		ConsumerName:               model.NormalizeUsername(stringifyAny(values["consumer_name"])),
		RouteName:                  strings.TrimSpace(stringifyAny(values["route_name"])),
		RequestPath:                strings.TrimSpace(stringifyAny(values["request_path"])),
		RequestKind:                strings.TrimSpace(stringifyAny(values["request_kind"])),
		APIKeyID:                   strings.TrimSpace(stringifyAny(values["api_key_id"])),
		ModelID:                    strings.TrimSpace(stringifyAny(values["model_id"])),
		RequestStatus:              strings.TrimSpace(stringifyAny(values["request_status"])),
		UsageStatus:                strings.TrimSpace(stringifyAny(values["usage_status"])),
		HTTPStatus:                 int(parseInt64Any(values["http_status"])),
		ErrorCode:                  strings.TrimSpace(stringifyAny(values["error_code"])),
		ErrorMessage:               strings.TrimSpace(stringifyAny(values["error_message"])),
		InputTokens:                parseInt64Any(values["input_tokens"]),
		OutputTokens:               parseInt64Any(values["output_tokens"]),
		TotalTokens:                parseInt64Any(values["total_tokens"]),
		CacheCreationInputTokens:   parseInt64Any(values["cache_creation_input_tokens"]),
		CacheCreation5mInputTokens: parseInt64Any(values["cache_creation_5m_input_tokens"]),
		CacheCreation1hInputTokens: parseInt64Any(values["cache_creation_1h_input_tokens"]),
		CacheReadInputTokens:       parseInt64Any(values["cache_read_input_tokens"]),
		InputImageTokens:           parseInt64Any(values["input_image_tokens"]),
		OutputImageTokens:          parseInt64Any(values["output_image_tokens"]),
		InputImageCount:            parseInt64Any(values["input_image_count"]),
		OutputImageCount:           parseInt64Any(values["output_image_count"]),
		RequestCount:               parseInt64Any(values["request_count"]),
		CacheTTL:                   strings.TrimSpace(stringifyAny(values["cache_ttl"])),
		InputTokenDetailsJSON:      strings.TrimSpace(stringifyAny(values["input_token_details_json"])),
		OutputTokenDetailsJSON:     strings.TrimSpace(stringifyAny(values["output_token_details_json"])),
		ProviderUsageJSON:          strings.TrimSpace(stringifyAny(values["provider_usage_json"])),
		CostMicroYuan:              parseInt64Any(values["cost_micro_yuan"]),
		PriceVersionID:             parseInt64Any(values["price_version_id"]),
		StartedAt:                  parseTimeAny(values["started_at"]),
		FinishedAt:                 parseTimeAny(values["finished_at"]),
		OccurredAt:                 parseTimeAny(values["occurred_at"]),
	}
	if payload.EventID == "" {
		payload.EventID = strings.TrimSpace(streamID)
	}
	if payload.RequestStatus == "" {
		payload.RequestStatus = "success"
	}
	if payload.UsageStatus == "" {
		payload.UsageStatus = "parsed"
	}
	if payload.HTTPStatus == 0 {
		if strings.EqualFold(payload.RequestStatus, "success") {
			payload.HTTPStatus = 200
		} else {
			payload.HTTPStatus = 500
		}
	}
	if payload.TotalTokens == 0 {
		payload.TotalTokens = payload.InputTokens + payload.OutputTokens + maxInt64(payload.CacheCreationInputTokens,
			payload.CacheCreation5mInputTokens+payload.CacheCreation1hInputTokens) + payload.CacheReadInputTokens +
			payload.InputImageTokens + payload.OutputImageTokens
	}
	if payload.RequestCount == 0 && strings.EqualFold(payload.RequestStatus, "success") &&
		strings.EqualFold(payload.UsageStatus, "parsed") {
		payload.RequestCount = 1
	}
	if payload.TraceID == "" {
		payload.TraceID = payload.RequestID
	}
	if payload.OccurredAt.IsZero() {
		payload.OccurredAt = time.Now()
	}
	if payload.StartedAt.IsZero() {
		payload.StartedAt = payload.OccurredAt
	}
	if payload.FinishedAt.IsZero() {
		payload.FinishedAt = payload.OccurredAt
	}
	if payload.ConsumerName == "" {
		return billingUsageEventPayload{}, gerror.New("billing usage event consumer_name is required")
	}
	if payload.ModelID == "" {
		return billingUsageEventPayload{}, gerror.New("billing usage event model_id is required")
	}
	return payload, nil
}

func parseInt64Any(value any) int64 {
	raw := strings.TrimSpace(stringifyAny(value))
	if raw == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err == nil {
		return parsed
	}
	floatValue, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return int64(math.Round(floatValue))
}

func parseTimeAny(value any) time.Time {
	raw := strings.TrimSpace(stringifyAny(value))
	if raw == "" {
		return time.Time{}
	}
	if unixMillis, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if unixMillis > 0 {
			if unixMillis > 9999999999 {
				return time.UnixMilli(unixMillis)
			}
			return time.Unix(unixMillis, 0)
		}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func stringifyAny(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return fmt.Sprint(typed)
	}
}

func isDuplicateEntryErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate entry")
}

func nullIfZero(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}

func nullIfZeroTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func maxInt64(left int64, right int64) int64 {
	if left >= right {
		return left
	}
	return right
}

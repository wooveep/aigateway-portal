package portal

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	clientK8s "higress-portal-backend/internal/client/k8s"
	"higress-portal-backend/internal/model"
)

type activeModelPrice struct {
	ModelID    string
	Name       string
	Vendor     string
	ModelType  string
	Capability string
	Pricing    model.ModelPricing
	Endpoint   string
	SDK        string
	Summary    string
}

type modelBindingPricingAlias struct {
	Currency                                                   string  `json:"currency,omitempty"`
	InputCostPerMillionTokens                                  float64 `json:"inputCostPerMillionTokens,omitempty"`
	OutputCostPerMillionTokens                                 float64 `json:"outputCostPerMillionTokens,omitempty"`
	PricePerImage                                              float64 `json:"pricePerImage,omitempty"`
	PricePerSecond                                             float64 `json:"pricePerSecond,omitempty"`
	PricePerSecond720p                                         float64 `json:"pricePerSecond720p,omitempty"`
	PricePerSecond1080p                                        float64 `json:"pricePerSecond1080p,omitempty"`
	PricePer10kChars                                           float64 `json:"pricePer10kChars,omitempty"`
	InputCostPerRequest                                        float64 `json:"inputCostPerRequest,omitempty"`
	CacheCreationInputTokenCostPerMillionTokens                float64 `json:"cacheCreationInputTokenCostPerMillionTokens,omitempty"`
	CacheCreationInputTokenCostAbove1hrPerMillionTokens        float64 `json:"cacheCreationInputTokenCostAbove1hrPerMillionTokens,omitempty"`
	CacheReadInputTokenCostPerMillionTokens                    float64 `json:"cacheReadInputTokenCostPerMillionTokens,omitempty"`
	InputCostPerMillionTokensAbove200kTokens                   float64 `json:"inputCostPerMillionTokensAbove200kTokens,omitempty"`
	OutputCostPerMillionTokensAbove200kTokens                  float64 `json:"outputCostPerMillionTokensAbove200kTokens,omitempty"`
	CacheCreationInputTokenCostPerMillionTokensAbove200kTokens float64 `json:"cacheCreationInputTokenCostPerMillionTokensAbove200kTokens,omitempty"`
	CacheReadInputTokenCostPerMillionTokensAbove200kTokens     float64 `json:"cacheReadInputTokenCostPerMillionTokensAbove200kTokens,omitempty"`
	OutputCostPerImage                                         float64 `json:"outputCostPerImage,omitempty"`
	OutputImageTokenCostPerMillionTokens                       float64 `json:"outputImageTokenCostPerMillionTokens,omitempty"`
	InputCostPerImage                                          float64 `json:"inputCostPerImage,omitempty"`
	InputImageTokenCostPerMillionTokens                        float64 `json:"inputImageTokenCostPerMillionTokens,omitempty"`
	SupportsPromptCaching                                      bool    `json:"supportsPromptCaching,omitempty"`

	InputCostPerToken                          float64 `json:"inputCostPerToken,omitempty"`
	OutputCostPerToken                         float64 `json:"outputCostPerToken,omitempty"`
	CacheCreationInputTokenCost                float64 `json:"cacheCreationInputTokenCost,omitempty"`
	CacheCreationInputTokenCostAbove1hr        float64 `json:"cacheCreationInputTokenCostAbove1hr,omitempty"`
	CacheReadInputTokenCost                    float64 `json:"cacheReadInputTokenCost,omitempty"`
	InputCostPerTokenAbove200kTokens           float64 `json:"inputCostPerTokenAbove200kTokens,omitempty"`
	OutputCostPerTokenAbove200kTokens          float64 `json:"outputCostPerTokenAbove200kTokens,omitempty"`
	CacheCreationInputTokenCostAbove200kTokens float64 `json:"cacheCreationInputTokenCostAbove200kTokens,omitempty"`
	CacheReadInputTokenCostAbove200kTokens     float64 `json:"cacheReadInputTokenCostAbove200kTokens,omitempty"`
	OutputCostPerImageToken                    float64 `json:"outputCostPerImageToken,omitempty"`
	InputCostPerImageToken                     float64 `json:"inputCostPerImageToken,omitempty"`
}

func (s *Service) loadActiveModelPrices(ctx context.Context) ([]activeModelPrice, error) {
	entries, err := s.loadActiveModelPricesFromDB(ctx)
	if err == nil && len(entries) > 0 {
		return entries, nil
	}
	if err != nil {
		s.logf(ctx, "load active model prices from db failed, fallback to gateway: %v", err)
	}

	entries, err = s.loadActiveModelPricesFromGateway(ctx)
	if err != nil {
		return nil, gerror.Wrap(err, "load active model prices from gateway failed")
	}
	if len(entries) > 0 {
		if syncErr := s.syncModelCatalogFromGateway(ctx, entries); syncErr != nil {
			s.logf(ctx, "sync model catalog from gateway failed: %v", syncErr)
		}
	}
	return entries, nil
}

func (s *Service) syncBillingModelCatalog(ctx context.Context) error {
	entries, err := s.loadBillingCatalogSeedEntries(ctx)
	if err != nil {
		return err
	}
	return s.upsertBillingModels(ctx, entries)
}

func (s *Service) loadBillingCatalogSeedEntries(ctx context.Context) ([]activeModelPrice, error) {
	baseEntries, err := s.loadActiveModelPricesFromGateway(ctx)
	if err != nil {
		s.logf(ctx, "load active model prices from gateway failed, fallback to portal model catalog: %v", err)
		baseEntries, err = s.loadBootstrapModelPricesFromCatalog(ctx)
		if err != nil {
			return nil, gerror.Wrap(err, "load portal model catalog failed")
		}
	} else if len(baseEntries) == 0 {
		baseEntries, err = s.loadBootstrapModelPricesFromCatalog(ctx)
		if err != nil {
			return nil, gerror.Wrap(err, "load portal model catalog failed")
		}
	}

	publishedEntries, err := s.loadPublishedBindingModelPrices(ctx)
	if err != nil {
		return nil, err
	}
	return mergeActiveModelPrices(baseEntries, publishedEntries), nil
}

func (s *Service) loadActiveModelPricesFromGateway(ctx context.Context) ([]activeModelPrice, error) {
	models, err := s.modelK8s.ListEnabledModels(ctx)
	if err != nil {
		return nil, gerror.Wrap(err, "list enabled models from gateway failed")
	}
	if len(models) == 0 {
		return []activeModelPrice{}, nil
	}

	entries := make([]activeModelPrice, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, item := range models {
		modelID := strings.TrimSpace(item.ID)
		if modelID == "" {
			continue
		}
		key := normalizeModelPriceKey(modelID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		vendor := strings.TrimSpace(item.Type)
		if vendor == "" {
			vendor = "unknown"
		}
		summary := strings.TrimSpace(item.Meta.Intro)
		if summary == "" {
			summary = "-"
		}
		capability := summary
		if capability == "-" {
			capability = vendor
		}
		endpoint := strings.TrimSpace(item.Endpoint)
		if endpoint == "" {
			endpoint = "-"
		}
		sdk := strings.TrimSpace(item.Protocol)
		if sdk == "" {
			sdk = "openai/v1"
		}

		entries = append(entries, activeModelPrice{
			ModelID:    modelID,
			Name:       modelID,
			Vendor:     vendor,
			Capability: capability,
			Pricing:    materializeModelPricing(toPortalModelPricing(item.Meta.Pricing)),
			Endpoint:   endpoint,
			SDK:        sdk,
			Summary:    summary,
		})
	}

	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].ModelID < entries[j].ModelID
	})
	return entries, nil
}

func (s *Service) loadActiveModelPricesFromDB(ctx context.Context) ([]activeModelPrice, error) {
	records, err := s.db.GetAll(ctx, `
		SELECT
			c.model_id,
			c.name,
			c.vendor,
			c.capability,
			p.input_price_per_1k_micro_yuan,
			p.output_price_per_1k_micro_yuan,
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
			p.supports_prompt_caching,
			c.endpoint,
			c.sdk,
			c.summary
		FROM billing_model_catalog c
		INNER JOIN billing_model_price_version p
			ON p.model_id = c.model_id
		WHERE c.status = 'active'
		  AND p.status = 'active'
		  AND p.effective_to IS NULL`)
	if err != nil {
		return nil, gerror.Wrap(err, "query billing model prices failed")
	}
	if len(records) > 0 {
		entries := make([]activeModelPrice, 0, len(records))
		for _, record := range records {
			modelID := strings.TrimSpace(record["model_id"].String())
			if modelID == "" {
				continue
			}
			name := strings.TrimSpace(record["name"].String())
			if name == "" {
				name = modelID
			}
			entries = append(entries, activeModelPrice{
				ModelID:    modelID,
				Name:       name,
				Vendor:     strings.TrimSpace(record["vendor"].String()),
				Capability: strings.TrimSpace(record["capability"].String()),
				Pricing:    modelPricingFromPriceVersionRecord(record),
				Endpoint:   strings.TrimSpace(record["endpoint"].String()),
				SDK:        strings.TrimSpace(record["sdk"].String()),
				Summary:    strings.TrimSpace(record["summary"].String()),
			})
		}
		return entries, nil
	}
	return []activeModelPrice{}, nil
}

func (s *Service) loadBootstrapModelPricesFromCatalog(ctx context.Context) ([]activeModelPrice, error) {
	records, err := s.db.GetAll(ctx, `
		SELECT model_id, name, vendor, capability, input_token_price, output_token_price, endpoint, sdk, summary
		FROM portal_model_catalog
		WHERE status = 'active'`)
	if err != nil {
		return nil, err
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
			Pricing:    bootstrapCatalogPricing(record["input_token_price"].Float64(), record["output_token_price"].Float64()),
			Endpoint:   strings.TrimSpace(record["endpoint"].String()),
			SDK:        strings.TrimSpace(record["sdk"].String()),
			Summary:    strings.TrimSpace(record["summary"].String()),
		})
	}
	return entries, nil
}

func (s *Service) syncModelCatalogFromGateway(ctx context.Context, entries []activeModelPrice) error {
	if len(entries) == 0 {
		return nil
	}
	if err := s.upsertBillingModels(ctx, entries); err != nil {
		return err
	}

	return s.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		for _, item := range entries {
			status := "disabled"
			if hasValidModelPrice(item) {
				status = "active"
			}
			if _, err := tx.Exec(`
				INSERT INTO portal_model_catalog
				(model_id, name, vendor, capability, input_token_price, output_token_price, endpoint, sdk, summary, status)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				`+s.upsertClause([]string{"model_id"},
				s.assignExcluded("name"),
				s.assignExcluded("vendor"),
				s.assignExcluded("capability"),
				s.assignExcluded("input_token_price"),
				s.assignExcluded("output_token_price"),
				s.assignExcluded("endpoint"),
				s.assignExcluded("sdk"),
				s.assignExcluded("summary"),
				s.assignExcluded("status"))+``,
				item.ModelID,
				item.Name,
				item.Vendor,
				item.Capability,
				item.Pricing.InputCostPerMillionTokens,
				item.Pricing.OutputCostPerMillionTokens,
				item.Endpoint,
				item.SDK,
				item.Summary,
				status,
			); err != nil {
				return gerror.Wrapf(err, "upsert active model catalog failed: %s", item.ModelID)
			}
		}

		placeholders := make([]string, 0, len(entries))
		args := make([]any, 0, len(entries))
		for _, item := range entries {
			placeholders = append(placeholders, "?")
			args = append(args, item.ModelID)
		}
		disableSQL := fmt.Sprintf(
			"UPDATE portal_model_catalog SET status = 'disabled' WHERE model_id NOT IN (%s)",
			strings.Join(placeholders, ","),
		)
		if _, err := tx.Exec(disableSQL, args...); err != nil {
			return gerror.Wrap(err, "disable stale model catalog entries failed")
		}
		return nil
	})
}

func (s *Service) upsertBillingModels(ctx context.Context, entries []activeModelPrice) error {
	now := time.Now()
	activeModelIDs := make([]string, 0, len(entries))
	for _, item := range entries {
		modelID := strings.TrimSpace(item.ModelID)
		if modelID == "" {
			continue
		}
		status := "disabled"
		if hasValidModelPrice(item) {
			status = "active"
			activeModelIDs = append(activeModelIDs, modelID)
		}
		if _, err := s.db.Exec(ctx, `
			INSERT INTO billing_model_catalog
			(model_id, name, vendor, capability, endpoint, sdk, summary, status)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			`+s.upsertClause([]string{"model_id"},
			s.assignExcluded("name"),
			s.assignExcluded("vendor"),
			s.assignExcluded("capability"),
			s.assignExcluded("endpoint"),
			s.assignExcluded("sdk"),
			s.assignExcluded("summary"),
			s.assignExcluded("status"))+``,
			modelID,
			strings.TrimSpace(item.Name),
			strings.TrimSpace(item.Vendor),
			strings.TrimSpace(item.Capability),
			strings.TrimSpace(item.Endpoint),
			strings.TrimSpace(item.SDK),
			strings.TrimSpace(item.Summary),
			status,
		); err != nil {
			return gerror.Wrapf(err, "upsert billing model catalog failed: %s", modelID)
		}

		if !hasValidModelPrice(item) {
			if _, err := s.db.Exec(ctx, `
				UPDATE billing_model_price_version
				SET status = 'inactive', effective_to = ?
				WHERE model_id = ? AND status = 'active' AND effective_to IS NULL`,
				now,
				modelID,
			); err != nil {
				return gerror.Wrapf(err, "close invalid billing model price failed: %s", modelID)
			}
			continue
		}

		pricing := materializeModelPricingForType(item.ModelType, item.Pricing)
		inputMicro := rmbPerMillionToMicroYuanPerToken(pricing.InputCostPerMillionTokens)
		outputMicro := rmbPerMillionToMicroYuanPerToken(pricing.OutputCostPerMillionTokens)
		inputRequestMicro := rmbToMicroYuan(pricing.InputCostPerRequest)
		cacheCreationMicro := rmbPerMillionToMicroYuanPerToken(pricing.CacheCreationInputTokenCostPerMillionTokens)
		cacheCreationAbove1hrMicro := rmbPerMillionToMicroYuanPerToken(pricing.CacheCreationInputTokenCostAbove1hrPerMillionTokens)
		cacheReadMicro := rmbPerMillionToMicroYuanPerToken(pricing.CacheReadInputTokenCostPerMillionTokens)
		inputAbove200kMicro := rmbPerMillionToMicroYuanPerToken(pricing.InputCostPerMillionTokensAbove200kTokens)
		outputAbove200kMicro := rmbPerMillionToMicroYuanPerToken(pricing.OutputCostPerMillionTokensAbove200kTokens)
		cacheCreationAbove200kMicro := rmbPerMillionToMicroYuanPerToken(pricing.CacheCreationInputTokenCostPerMillionTokensAbove200kTokens)
		cacheReadAbove200kMicro := rmbPerMillionToMicroYuanPerToken(pricing.CacheReadInputTokenCostPerMillionTokensAbove200kTokens)
		outputImageMicro := rmbToMicroYuan(pricing.OutputCostPerImage)
		outputImageTokenMicro := rmbPerMillionToMicroYuanPerToken(pricing.OutputImageTokenCostPerMillionTokens)
		inputImageMicro := rmbToMicroYuan(pricing.InputCostPerImage)
		inputImageTokenMicro := rmbPerMillionToMicroYuanPerToken(pricing.InputImageTokenCostPerMillionTokens)
		inputLegacyPer1KMicro := microYuanPerTokenToPer1KMicroYuan(inputMicro)
		outputLegacyPer1KMicro := microYuanPerTokenToPer1KMicroYuan(outputMicro)
		cacheCreationLegacyPer1KMicro := microYuanPerTokenToPer1KMicroYuan(cacheCreationMicro)
		cacheCreationAbove1hrLegacyPer1KMicro := microYuanPerTokenToPer1KMicroYuan(cacheCreationAbove1hrMicro)
		cacheReadLegacyPer1KMicro := microYuanPerTokenToPer1KMicroYuan(cacheReadMicro)
		inputAbove200kLegacyPer1KMicro := microYuanPerTokenToPer1KMicroYuan(inputAbove200kMicro)
		outputAbove200kLegacyPer1KMicro := microYuanPerTokenToPer1KMicroYuan(outputAbove200kMicro)
		cacheCreationAbove200kLegacyPer1KMicro := microYuanPerTokenToPer1KMicroYuan(cacheCreationAbove200kMicro)
		cacheReadAbove200kLegacyPer1KMicro := microYuanPerTokenToPer1KMicroYuan(cacheReadAbove200kMicro)
		outputImageTokenLegacyPer1KMicro := microYuanPerTokenToPer1KMicroYuan(outputImageTokenMicro)
		inputImageTokenLegacyPer1KMicro := microYuanPerTokenToPer1KMicroYuan(inputImageTokenMicro)
		current, err := s.db.GetOne(ctx, `
			SELECT
				id,
				input_price_micro_yuan_per_token,
				output_price_micro_yuan_per_token,
				input_price_per_1k_micro_yuan,
				output_price_per_1k_micro_yuan,
				input_request_price_micro_yuan,
				cache_creation_input_token_price_micro_yuan_per_token,
				cache_creation_input_token_price_above_1hr_micro_yuan_per_token,
				cache_read_input_token_price_micro_yuan_per_token,
				input_token_price_above_200k_micro_yuan_per_token,
				output_token_price_above_200k_micro_yuan_per_token,
				cache_creation_input_token_price_above_200k_micro_yuan_per_token,
				cache_read_input_token_price_above_200k_micro_yuan_per_token,
				cache_creation_input_token_price_per_1k_micro_yuan,
				cache_creation_input_token_price_above_1hr_per_1k_micro_yuan,
				cache_read_input_token_price_per_1k_micro_yuan,
				input_token_price_above_200k_per_1k_micro_yuan,
				output_token_price_above_200k_per_1k_micro_yuan,
				cache_creation_input_token_price_above_200k_per_1k_micro_yuan,
				cache_read_input_token_price_above_200k_per_1k_micro_yuan,
				output_image_price_micro_yuan,
				output_image_token_price_micro_yuan_per_token,
				output_image_token_price_per_1k_micro_yuan,
				input_image_price_micro_yuan,
				input_image_token_price_micro_yuan_per_token,
				input_image_token_price_per_1k_micro_yuan,
				supports_prompt_caching
			FROM billing_model_price_version
			WHERE model_id = ? AND status = 'active' AND effective_to IS NULL
			ORDER BY id DESC
			LIMIT 1`, modelID)
		if err != nil {
			return gerror.Wrapf(err, "query billing model price failed: %s", modelID)
		}

		if len(current) > 0 &&
			current["input_price_micro_yuan_per_token"].Int64() == inputMicro &&
			current["output_price_micro_yuan_per_token"].Int64() == outputMicro &&
			current["input_price_per_1k_micro_yuan"].Int64() == inputLegacyPer1KMicro &&
			current["output_price_per_1k_micro_yuan"].Int64() == outputLegacyPer1KMicro &&
			current["input_request_price_micro_yuan"].Int64() == inputRequestMicro &&
			current["cache_creation_input_token_price_micro_yuan_per_token"].Int64() == cacheCreationMicro &&
			current["cache_creation_input_token_price_above_1hr_micro_yuan_per_token"].Int64() == cacheCreationAbove1hrMicro &&
			current["cache_read_input_token_price_micro_yuan_per_token"].Int64() == cacheReadMicro &&
			current["input_token_price_above_200k_micro_yuan_per_token"].Int64() == inputAbove200kMicro &&
			current["output_token_price_above_200k_micro_yuan_per_token"].Int64() == outputAbove200kMicro &&
			current["cache_creation_input_token_price_above_200k_micro_yuan_per_token"].Int64() == cacheCreationAbove200kMicro &&
			current["cache_read_input_token_price_above_200k_micro_yuan_per_token"].Int64() == cacheReadAbove200kMicro &&
			current["cache_creation_input_token_price_per_1k_micro_yuan"].Int64() == cacheCreationLegacyPer1KMicro &&
			current["cache_creation_input_token_price_above_1hr_per_1k_micro_yuan"].Int64() == cacheCreationAbove1hrLegacyPer1KMicro &&
			current["cache_read_input_token_price_per_1k_micro_yuan"].Int64() == cacheReadLegacyPer1KMicro &&
			current["input_token_price_above_200k_per_1k_micro_yuan"].Int64() == inputAbove200kLegacyPer1KMicro &&
			current["output_token_price_above_200k_per_1k_micro_yuan"].Int64() == outputAbove200kLegacyPer1KMicro &&
			current["cache_creation_input_token_price_above_200k_per_1k_micro_yuan"].Int64() == cacheCreationAbove200kLegacyPer1KMicro &&
			current["cache_read_input_token_price_above_200k_per_1k_micro_yuan"].Int64() == cacheReadAbove200kLegacyPer1KMicro &&
			current["output_image_price_micro_yuan"].Int64() == outputImageMicro &&
			current["output_image_token_price_micro_yuan_per_token"].Int64() == outputImageTokenMicro &&
			current["output_image_token_price_per_1k_micro_yuan"].Int64() == outputImageTokenLegacyPer1KMicro &&
			current["input_image_price_micro_yuan"].Int64() == inputImageMicro &&
			current["input_image_token_price_micro_yuan_per_token"].Int64() == inputImageTokenMicro &&
			current["input_image_token_price_per_1k_micro_yuan"].Int64() == inputImageTokenLegacyPer1KMicro &&
			current["supports_prompt_caching"].Int64() == int64(boolToInt(pricing.SupportsPromptCaching)) {
			continue
		}

		if _, err := s.db.Exec(ctx, `
			UPDATE billing_model_price_version
			SET status = 'inactive', effective_to = ?
			WHERE model_id = ? AND status = 'active' AND effective_to IS NULL`,
			now,
			modelID,
		); err != nil {
			return gerror.Wrapf(err, "close active billing model price failed: %s", modelID)
		}
		if _, err = s.db.Exec(ctx, `
			INSERT INTO billing_model_price_version
			(model_id, currency,
			 input_price_micro_yuan_per_token, output_price_micro_yuan_per_token,
			 input_price_per_1k_micro_yuan, output_price_per_1k_micro_yuan,
			 input_request_price_micro_yuan,
			 cache_creation_input_token_price_micro_yuan_per_token,
			 cache_creation_input_token_price_above_1hr_micro_yuan_per_token,
			 cache_read_input_token_price_micro_yuan_per_token,
			 input_token_price_above_200k_micro_yuan_per_token, output_token_price_above_200k_micro_yuan_per_token,
			 cache_creation_input_token_price_above_200k_micro_yuan_per_token,
			 cache_read_input_token_price_above_200k_micro_yuan_per_token,
			 cache_creation_input_token_price_per_1k_micro_yuan,
			 cache_creation_input_token_price_above_1hr_per_1k_micro_yuan, cache_read_input_token_price_per_1k_micro_yuan,
			 input_token_price_above_200k_per_1k_micro_yuan, output_token_price_above_200k_per_1k_micro_yuan,
			 cache_creation_input_token_price_above_200k_per_1k_micro_yuan, cache_read_input_token_price_above_200k_per_1k_micro_yuan,
			 output_image_price_micro_yuan, output_image_token_price_micro_yuan_per_token, output_image_token_price_per_1k_micro_yuan,
			 input_image_price_micro_yuan, input_image_token_price_micro_yuan_per_token, input_image_token_price_per_1k_micro_yuan, supports_prompt_caching,
			 effective_from, status)
			VALUES (?, 'CNY', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active')`,
			modelID,
			inputMicro,
			outputMicro,
			inputLegacyPer1KMicro,
			outputLegacyPer1KMicro,
			inputRequestMicro,
			cacheCreationMicro,
			cacheCreationAbove1hrMicro,
			cacheReadMicro,
			inputAbove200kMicro,
			outputAbove200kMicro,
			cacheCreationAbove200kMicro,
			cacheReadAbove200kMicro,
			cacheCreationLegacyPer1KMicro,
			cacheCreationAbove1hrLegacyPer1KMicro,
			cacheReadLegacyPer1KMicro,
			inputAbove200kLegacyPer1KMicro,
			outputAbove200kLegacyPer1KMicro,
			cacheCreationAbove200kLegacyPer1KMicro,
			cacheReadAbove200kLegacyPer1KMicro,
			outputImageMicro,
			outputImageTokenMicro,
			outputImageTokenLegacyPer1KMicro,
			inputImageMicro,
			inputImageTokenMicro,
			inputImageTokenLegacyPer1KMicro,
			boolToInt(pricing.SupportsPromptCaching),
			now,
		); err != nil {
			return gerror.Wrapf(err, "insert billing model price failed: %s", modelID)
		}
	}

	if len(activeModelIDs) == 0 {
		if _, err := s.db.Exec(ctx, `UPDATE billing_model_catalog SET status = 'disabled'`); err != nil {
			return gerror.Wrap(err, "disable stale billing model catalog entries failed")
		}
		if _, err := s.db.Exec(ctx, `
			UPDATE billing_model_price_version
			SET status = 'inactive', effective_to = ?
			WHERE effective_to IS NULL`, now); err != nil {
			return gerror.Wrap(err, "disable stale billing model price entries failed")
		}
		return nil
	}
	placeholders := make([]string, 0, len(activeModelIDs))
	args := make([]any, 0, len(activeModelIDs))
	for _, modelID := range activeModelIDs {
		placeholders = append(placeholders, "?")
		args = append(args, modelID)
	}
	disableCatalogSQL := fmt.Sprintf(
		"UPDATE billing_model_catalog SET status = 'disabled' WHERE model_id NOT IN (%s)",
		strings.Join(placeholders, ","),
	)
	if _, err := s.db.Exec(ctx, disableCatalogSQL, args...); err != nil {
		return gerror.Wrap(err, "disable stale billing model catalog entries failed")
	}
	disablePriceSQL := fmt.Sprintf(
		"UPDATE billing_model_price_version SET status = 'inactive', effective_to = ? WHERE effective_to IS NULL AND model_id NOT IN (%s)",
		strings.Join(placeholders, ","),
	)
	args = append([]any{now}, args...)
	if _, err := s.db.Exec(ctx, disablePriceSQL, args...); err != nil {
		return gerror.Wrap(err, "disable stale billing model price entries failed")
	}
	return nil
}

func (s *Service) loadPublishedBindingModelPrices(ctx context.Context) ([]activeModelPrice, error) {
	records, err := s.db.GetAll(ctx, `
		SELECT
			a.asset_id,
			b.binding_id,
			b.model_id,
			a.display_name,
			a.intro,
			a.model_type,
			a.input_modalities_json,
			a.output_modalities_json,
			a.feature_flags_json,
			a.modalities_json,
			a.features_json,
			b.provider_name,
			b.endpoint,
			b.protocol,
			b.pricing_json AS binding_pricing_json,
			v.version_id,
			v.pricing_json AS version_pricing_json
		FROM portal_model_binding b
		INNER JOIN portal_model_asset a
			ON a.asset_id = b.asset_id
		LEFT JOIN portal_model_binding_price_version v
			ON v.asset_id = b.asset_id
		   AND v.binding_id = b.binding_id
		   AND v.active = TRUE
		   AND v.effective_to IS NULL
		WHERE b.status = 'published'
		ORDER BY b.model_id ASC, b.binding_id ASC`)
	if err != nil {
		return nil, gerror.Wrap(err, "query published binding pricing failed")
	}

	entries := make([]activeModelPrice, 0, len(records))
	for _, record := range records {
		modelID := strings.TrimSpace(record["model_id"].String())
		if modelID == "" {
			continue
		}
		modelType := normalizeModelType(record["model_type"].String())

		bindingPricing, bindingCanonical, err := parseModelBindingPricingJSON(record["binding_pricing_json"].String(), modelType)
		if err != nil {
			return nil, gerror.Wrapf(err, "parse binding pricing failed: %s", modelID)
		}
		if err := s.canonicalizeBindingPricingJSON(
			ctx,
			record["asset_id"].String(),
			record["binding_id"].String(),
			record["binding_pricing_json"].String(),
			bindingCanonical,
		); err != nil {
			return nil, err
		}

		pricing := bindingPricing
		versionID := record["version_id"].Int64()
		if strings.TrimSpace(record["version_pricing_json"].String()) != "" {
			versionPricing, versionCanonical, err := parseModelBindingPricingJSON(record["version_pricing_json"].String(), modelType)
			if err != nil {
				return nil, gerror.Wrapf(err, "parse binding price version failed: %s", modelID)
			}
			if err := s.canonicalizeBindingPriceVersionJSON(
				ctx,
				versionID,
				record["version_pricing_json"].String(),
				versionCanonical,
			); err != nil {
				return nil, err
			}
			pricing = versionPricing
		}

		vendor := strings.TrimSpace(record["provider_name"].String())
		if vendor == "" {
			vendor = "unknown"
		}
		summary := strings.TrimSpace(record["intro"].String())
		if summary == "" {
			summary = "-"
		}
		capability := buildCapabilitySummary(model.ModelCapabilities{
			InputModalities:  firstNonEmptyStringSlice(parseStringList(record["input_modalities_json"].String()), parseStringList(record["modalities_json"].String())),
			OutputModalities: firstNonEmptyStringSlice(parseStringList(record["output_modalities_json"].String()), parseStringList(record["modalities_json"].String())),
			FeatureFlags:     firstNonEmptyStringSlice(parseStringList(record["feature_flags_json"].String()), parseStringList(record["features_json"].String())),
			Modalities:       parseStringList(record["modalities_json"].String()),
			Features:         parseStringList(record["features_json"].String()),
		})
		if capability == "" {
			capability = vendor
		}
		name := strings.TrimSpace(record["display_name"].String())
		if name == "" {
			name = modelID
		}
		endpoint := strings.TrimSpace(record["endpoint"].String())
		sdk := normalizePublishedBindingProtocol(record["protocol"].String())
		endpoint = normalizePublishedBindingEndpoint(endpoint, record["protocol"].String())

		entries = append(entries, activeModelPrice{
			ModelID:    modelID,
			Name:       name,
			Vendor:     vendor,
			ModelType:  modelType,
			Capability: capability,
			Pricing:    pricing,
			Endpoint:   endpoint,
			SDK:        sdk,
			Summary:    summary,
		})
	}
	return entries, nil
}

func hasValidModelPrice(item activeModelPrice) bool {
	pricing := materializeModelPricingForType(item.ModelType, item.Pricing)
	switch normalizeModelType(item.ModelType) {
	case "embedding":
		return pricing.InputCostPerMillionTokens > 0
	case "image_generation":
		return pricing.PricePerImage > 0
	case "video_generation":
		return pricing.PricePerSecond720p > 0 || pricing.PricePerSecond1080p > 0
	case "speech_recognition":
		return pricing.PricePerSecond > 0
	case "speech_synthesis":
		return pricing.PricePer10kChars > 0
	default:
		return pricing.InputCostPerMillionTokens > 0 || pricing.OutputCostPerMillionTokens > 0
	}
}

func buildModelPriceIndex(entries []activeModelPrice) map[string]activeModelPrice {
	index := make(map[string]activeModelPrice, len(entries)*2)
	for _, item := range entries {
		if key := normalizeModelPriceKey(item.ModelID); key != "" {
			index[key] = item
		}
		if key := normalizeModelPriceKey(item.Name); key != "" {
			if _, ok := index[key]; !ok {
				index[key] = item
			}
		}
	}
	return index
}

func lookupModelPrice(modelName string, index map[string]activeModelPrice) (float64, float64, bool) {
	key := normalizeModelPriceKey(modelName)
	if key == "" {
		return 0, 0, false
	}
	item, ok := index[key]
	if !ok {
		return 0, 0, false
	}
	pricing := materializeModelPricing(item.Pricing)
	return pricing.InputCostPerMillionTokens, pricing.OutputCostPerMillionTokens, true
}

func normalizeModelPriceKey(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func normalizeModelType(v string) string {
	normalized := strings.ToLower(strings.TrimSpace(v))
	switch normalized {
	case "text", "multimodal", "image_generation", "video_generation", "speech_recognition", "speech_synthesis", "embedding":
		return normalized
	default:
		return ""
	}
}

func buildCapabilitySummary(capabilities model.ModelCapabilities) string {
	parts := append([]string{}, capabilities.FeatureFlags...)
	if len(parts) == 0 {
		parts = append(parts, capabilities.Features...)
	}
	if len(parts) == 0 {
		parts = append(parts, capabilities.Modalities...)
	}
	return strings.Join(parts, " / ")
}

func firstNonEmptyStringSlice(values ...[]string) []string {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return []string{}
}

func mergeActiveModelPrices(base []activeModelPrice, overrides []activeModelPrice) []activeModelPrice {
	merged := make([]activeModelPrice, 0, len(base)+len(overrides))
	index := make(map[string]int, len(base)+len(overrides))
	for _, item := range base {
		key := normalizeModelPriceKey(item.ModelID)
		if key == "" {
			continue
		}
		index[key] = len(merged)
		merged = append(merged, item)
	}
	for _, item := range overrides {
		key := normalizeModelPriceKey(item.ModelID)
		if key == "" {
			continue
		}
		if existing, ok := index[key]; ok {
			merged[existing] = item
			continue
		}
		index[key] = len(merged)
		merged = append(merged, item)
	}
	sort.Slice(merged, func(i int, j int) bool {
		return merged[i].ModelID < merged[j].ModelID
	})
	return merged
}

func toPortalModelPricing(src clientK8s.ProviderPricing) model.ModelPricing {
	return model.ModelPricing{
		Currency:                                                   src.Currency,
		InputCostPerMillionTokens:                                  src.InputCostPerMillionTokens,
		OutputCostPerMillionTokens:                                 src.OutputCostPerMillionTokens,
		InputCostPerRequest:                                        src.InputCostPerRequest,
		CacheCreationInputTokenCostPerMillionTokens:                src.CacheCreationInputTokenCostPerMillionTokens,
		CacheCreationInputTokenCostAbove1hrPerMillionTokens:        src.CacheCreationInputTokenCostAbove1hrPerMillionTokens,
		CacheReadInputTokenCostPerMillionTokens:                    src.CacheReadInputTokenCostPerMillionTokens,
		InputCostPerMillionTokensAbove200kTokens:                   src.InputCostPerMillionTokensAbove200kTokens,
		OutputCostPerMillionTokensAbove200kTokens:                  src.OutputCostPerMillionTokensAbove200kTokens,
		CacheCreationInputTokenCostPerMillionTokensAbove200kTokens: src.CacheCreationInputTokenCostPerMillionTokensAbove200kTokens,
		CacheReadInputTokenCostPerMillionTokensAbove200kTokens:     src.CacheReadInputTokenCostPerMillionTokensAbove200kTokens,
		OutputCostPerImage:                                         src.OutputCostPerImage,
		OutputImageTokenCostPerMillionTokens:                       src.OutputImageTokenCostPerMillionTokens,
		InputCostPerImage:                                          src.InputCostPerImage,
		InputImageTokenCostPerMillionTokens:                        src.InputImageTokenCostPerMillionTokens,
		SupportsPromptCaching:                                      src.SupportsPromptCaching,
	}
}

func modelPricingFromPriceVersionRecord(record gdb.Record) model.ModelPricing {
	return modelPricingFromPriceVersionRecordForType(record, "")
}

func modelPricingFromPriceVersionRecordForType(record gdb.Record, modelType string) model.ModelPricing {
	inputPerMillion := readPerMillionPriceFromRecord(record, "input_price_micro_yuan_per_token", "input_price_per_1k_micro_yuan")
	outputPerMillion := readPerMillionPriceFromRecord(record, "output_price_micro_yuan_per_token", "output_price_per_1k_micro_yuan")
	return materializeModelPricingForType(modelType, model.ModelPricing{
		Currency:                                                   billingCurrencyCNY,
		InputCostPerMillionTokens:                                  inputPerMillion,
		OutputCostPerMillionTokens:                                 outputPerMillion,
		InputCostPerRequest:                                        microYuanToRMB(record["input_request_price_micro_yuan"].Int64()),
		CacheCreationInputTokenCostPerMillionTokens:                readPerMillionPriceFromRecord(record, "cache_creation_input_token_price_micro_yuan_per_token", "cache_creation_input_token_price_per_1k_micro_yuan"),
		CacheCreationInputTokenCostAbove1hrPerMillionTokens:        readPerMillionPriceFromRecord(record, "cache_creation_input_token_price_above_1hr_micro_yuan_per_token", "cache_creation_input_token_price_above_1hr_per_1k_micro_yuan"),
		CacheReadInputTokenCostPerMillionTokens:                    readPerMillionPriceFromRecord(record, "cache_read_input_token_price_micro_yuan_per_token", "cache_read_input_token_price_per_1k_micro_yuan"),
		InputCostPerMillionTokensAbove200kTokens:                   readPerMillionPriceFromRecord(record, "input_token_price_above_200k_micro_yuan_per_token", "input_token_price_above_200k_per_1k_micro_yuan"),
		OutputCostPerMillionTokensAbove200kTokens:                  readPerMillionPriceFromRecord(record, "output_token_price_above_200k_micro_yuan_per_token", "output_token_price_above_200k_per_1k_micro_yuan"),
		CacheCreationInputTokenCostPerMillionTokensAbove200kTokens: readPerMillionPriceFromRecord(record, "cache_creation_input_token_price_above_200k_micro_yuan_per_token", "cache_creation_input_token_price_above_200k_per_1k_micro_yuan"),
		CacheReadInputTokenCostPerMillionTokensAbove200kTokens:     readPerMillionPriceFromRecord(record, "cache_read_input_token_price_above_200k_micro_yuan_per_token", "cache_read_input_token_price_above_200k_per_1k_micro_yuan"),
		OutputCostPerImage:                                         microYuanToRMB(record["output_image_price_micro_yuan"].Int64()),
		OutputImageTokenCostPerMillionTokens:                       readPerMillionPriceFromRecord(record, "output_image_token_price_micro_yuan_per_token", "output_image_token_price_per_1k_micro_yuan"),
		InputCostPerImage:                                          microYuanToRMB(record["input_image_price_micro_yuan"].Int64()),
		InputImageTokenCostPerMillionTokens:                        readPerMillionPriceFromRecord(record, "input_image_token_price_micro_yuan_per_token", "input_image_token_price_per_1k_micro_yuan"),
		SupportsPromptCaching:                                      record["supports_prompt_caching"].Int64() > 0,
	})
}

func materializeModelPricing(raw model.ModelPricing) model.ModelPricing {
	return materializeModelPricingForType("", raw)
}

func materializeModelPricingForType(modelType string, raw model.ModelPricing) model.ModelPricing {
	pricing := raw
	if strings.TrimSpace(pricing.Currency) == "" {
		pricing.Currency = billingCurrencyCNY
	}
	switch normalizeModelType(modelType) {
	case "text", "multimodal":
		return model.ModelPricing{
			Currency:                                    pricing.Currency,
			InputCostPerMillionTokens:                   pricing.InputCostPerMillionTokens,
			OutputCostPerMillionTokens:                  pricing.OutputCostPerMillionTokens,
			CacheCreationInputTokenCostPerMillionTokens: pricing.CacheCreationInputTokenCostPerMillionTokens,
			CacheReadInputTokenCostPerMillionTokens:     pricing.CacheReadInputTokenCostPerMillionTokens,
			SupportsPromptCaching:                       pricing.SupportsPromptCaching,
		}
	case "embedding":
		return model.ModelPricing{
			Currency:                  pricing.Currency,
			InputCostPerMillionTokens: pricing.InputCostPerMillionTokens,
		}
	case "image_generation":
		outputCostPerImage := firstPositiveFloat(pricing.OutputCostPerImage, pricing.PricePerImage)
		return model.ModelPricing{
			Currency:           pricing.Currency,
			PricePerImage:      outputCostPerImage,
			OutputCostPerImage: outputCostPerImage,
		}
	case "video_generation":
		return model.ModelPricing{
			Currency:            pricing.Currency,
			PricePerSecond720p:  pricing.PricePerSecond720p,
			PricePerSecond1080p: pricing.PricePerSecond1080p,
		}
	case "speech_recognition":
		return model.ModelPricing{
			Currency:       pricing.Currency,
			PricePerSecond: pricing.PricePerSecond,
		}
	case "speech_synthesis":
		return model.ModelPricing{
			Currency:         pricing.Currency,
			PricePer10kChars: pricing.PricePer10kChars,
		}
	default:
		return pricing
	}
}

func readPerMillionPriceFromRecord(record gdb.Record, modernKey string, legacyKey string) float64 {
	if value := record[modernKey].Int64(); value > 0 {
		return microYuanPerTokenToRMBPerMillion(value)
	}
	if value := record[legacyKey].Int64(); value > 0 {
		return microYuanPerTokenToRMBPerMillion(per1KMicroYuanToMicroYuanPerToken(value))
	}
	return 0
}

func firstPositiveFloat(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func parseModelBindingPricingJSON(raw string, modelType string) (model.ModelPricing, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		pricing := materializeModelPricingForType(modelType, model.ModelPricing{Currency: billingCurrencyCNY})
		canonical, err := json.Marshal(pricing)
		return pricing, string(canonical), err
	}

	var alias modelBindingPricingAlias
	if err := json.Unmarshal([]byte(trimmed), &alias); err != nil {
		return model.ModelPricing{}, "", err
	}

	pricing := model.ModelPricing{
		Currency:                   canonicalPricingCurrency(alias.Currency),
		InputCostPerMillionTokens:  firstPositiveFloat(alias.InputCostPerMillionTokens, alias.InputCostPerToken),
		OutputCostPerMillionTokens: firstPositiveFloat(alias.OutputCostPerMillionTokens, alias.OutputCostPerToken),
		PricePerImage:              alias.PricePerImage,
		PricePerSecond:             alias.PricePerSecond,
		PricePerSecond720p:         alias.PricePerSecond720p,
		PricePerSecond1080p:        alias.PricePerSecond1080p,
		PricePer10kChars:           alias.PricePer10kChars,
		InputCostPerRequest:        alias.InputCostPerRequest,
		CacheCreationInputTokenCostPerMillionTokens:                firstPositiveFloat(alias.CacheCreationInputTokenCostPerMillionTokens, alias.CacheCreationInputTokenCost),
		CacheCreationInputTokenCostAbove1hrPerMillionTokens:        firstPositiveFloat(alias.CacheCreationInputTokenCostAbove1hrPerMillionTokens, alias.CacheCreationInputTokenCostAbove1hr),
		CacheReadInputTokenCostPerMillionTokens:                    firstPositiveFloat(alias.CacheReadInputTokenCostPerMillionTokens, alias.CacheReadInputTokenCost),
		InputCostPerMillionTokensAbove200kTokens:                   firstPositiveFloat(alias.InputCostPerMillionTokensAbove200kTokens, alias.InputCostPerTokenAbove200kTokens),
		OutputCostPerMillionTokensAbove200kTokens:                  firstPositiveFloat(alias.OutputCostPerMillionTokensAbove200kTokens, alias.OutputCostPerTokenAbove200kTokens),
		CacheCreationInputTokenCostPerMillionTokensAbove200kTokens: firstPositiveFloat(alias.CacheCreationInputTokenCostPerMillionTokensAbove200kTokens, alias.CacheCreationInputTokenCostAbove200kTokens),
		CacheReadInputTokenCostPerMillionTokensAbove200kTokens:     firstPositiveFloat(alias.CacheReadInputTokenCostPerMillionTokensAbove200kTokens, alias.CacheReadInputTokenCostAbove200kTokens),
		OutputCostPerImage:                   alias.OutputCostPerImage,
		OutputImageTokenCostPerMillionTokens: firstPositiveFloat(alias.OutputImageTokenCostPerMillionTokens, alias.OutputCostPerImageToken),
		InputCostPerImage:                    alias.InputCostPerImage,
		InputImageTokenCostPerMillionTokens:  firstPositiveFloat(alias.InputImageTokenCostPerMillionTokens, alias.InputCostPerImageToken),
		SupportsPromptCaching:                alias.SupportsPromptCaching,
	}
	pricing = materializeModelPricingForType(modelType, pricing)
	canonical, err := json.Marshal(pricing)
	if err != nil {
		return model.ModelPricing{}, "", err
	}
	return pricing, string(canonical), nil
}

func canonicalPricingCurrency(currency string) string {
	trimmed := strings.TrimSpace(currency)
	if trimmed == "" {
		return billingCurrencyCNY
	}
	return trimmed
}

func (s *Service) canonicalizeBindingPricingJSON(ctx context.Context, assetID, bindingID, raw, canonical string) error {
	if strings.TrimSpace(raw) == "" || raw == canonical {
		return nil
	}
	if _, err := s.db.Exec(ctx, `
		UPDATE portal_model_binding
		SET pricing_json = ?, updated_at = CURRENT_TIMESTAMP
		WHERE asset_id = ? AND binding_id = ?`,
		canonical,
		strings.TrimSpace(assetID),
		strings.TrimSpace(bindingID),
	); err != nil {
		return gerror.Wrapf(err, "canonicalize binding pricing failed: %s/%s", strings.TrimSpace(assetID), strings.TrimSpace(bindingID))
	}
	return nil
}

func (s *Service) canonicalizeBindingPriceVersionJSON(ctx context.Context, versionID int64, raw, canonical string) error {
	if versionID <= 0 || strings.TrimSpace(raw) == "" || raw == canonical {
		return nil
	}
	if _, err := s.db.Exec(ctx, `
		UPDATE portal_model_binding_price_version
		SET pricing_json = ?, updated_at = CURRENT_TIMESTAMP
		WHERE version_id = ?`,
		canonical,
		versionID,
	); err != nil {
		return gerror.Wrapf(err, "canonicalize binding price version failed: %d", versionID)
	}
	return nil
}

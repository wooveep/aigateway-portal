package portal

import (
	"context"
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
	Capability string
	Pricing    model.ModelPricing
	Endpoint   string
	SDK        string
	Summary    string
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
				ON DUPLICATE KEY UPDATE
				name = VALUES(name),
				vendor = VALUES(vendor),
				capability = VALUES(capability),
				input_token_price = VALUES(input_token_price),
				output_token_price = VALUES(output_token_price),
				endpoint = VALUES(endpoint),
				sdk = VALUES(sdk),
				summary = VALUES(summary),
				status = VALUES(status)`,
				item.ModelID,
				item.Name,
				item.Vendor,
				item.Capability,
				item.Pricing.InputPer1K,
				item.Pricing.OutputPer1K,
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
			ON DUPLICATE KEY UPDATE
				name = VALUES(name),
				vendor = VALUES(vendor),
				capability = VALUES(capability),
				endpoint = VALUES(endpoint),
				sdk = VALUES(sdk),
				summary = VALUES(summary),
				status = VALUES(status)`,
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

		pricing := materializeModelPricing(item.Pricing)
		inputMicro := rmbToMicroYuan(pricing.InputPer1K)
		outputMicro := rmbToMicroYuan(pricing.OutputPer1K)
		inputRequestMicro := rmbToMicroYuan(pricing.InputCostPerRequest)
		cacheCreationMicro := rmbToMicroYuan(pricing.CacheCreationInputTokenCost * 1000)
		cacheCreationAbove1hrMicro := rmbToMicroYuan(pricing.CacheCreationInputTokenCostAbove1hr * 1000)
		cacheReadMicro := rmbToMicroYuan(pricing.CacheReadInputTokenCost * 1000)
		inputAbove200kMicro := rmbToMicroYuan(pricing.InputCostPerTokenAbove200kTokens * 1000)
		outputAbove200kMicro := rmbToMicroYuan(pricing.OutputCostPerTokenAbove200kTokens * 1000)
		cacheCreationAbove200kMicro := rmbToMicroYuan(pricing.CacheCreationInputTokenCostAbove200kTokens * 1000)
		cacheReadAbove200kMicro := rmbToMicroYuan(pricing.CacheReadInputTokenCostAbove200kTokens * 1000)
		outputImageMicro := rmbToMicroYuan(pricing.OutputCostPerImage)
		outputImageTokenMicro := rmbToMicroYuan(pricing.OutputCostPerImageToken * 1000)
		inputImageMicro := rmbToMicroYuan(pricing.InputCostPerImage)
		inputImageTokenMicro := rmbToMicroYuan(pricing.InputCostPerImageToken * 1000)
		current, err := s.db.GetOne(ctx, `
			SELECT
				id,
				input_price_per_1k_micro_yuan,
				output_price_per_1k_micro_yuan,
				input_request_price_micro_yuan,
				cache_creation_input_token_price_per_1k_micro_yuan,
				cache_creation_input_token_price_above_1hr_per_1k_micro_yuan,
				cache_read_input_token_price_per_1k_micro_yuan,
				input_token_price_above_200k_per_1k_micro_yuan,
				output_token_price_above_200k_per_1k_micro_yuan,
				cache_creation_input_token_price_above_200k_per_1k_micro_yuan,
				cache_read_input_token_price_above_200k_per_1k_micro_yuan,
				output_image_price_micro_yuan,
				output_image_token_price_per_1k_micro_yuan,
				input_image_price_micro_yuan,
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
			current["input_price_per_1k_micro_yuan"].Int64() == inputMicro &&
			current["output_price_per_1k_micro_yuan"].Int64() == outputMicro &&
			current["input_request_price_micro_yuan"].Int64() == inputRequestMicro &&
			current["cache_creation_input_token_price_per_1k_micro_yuan"].Int64() == cacheCreationMicro &&
			current["cache_creation_input_token_price_above_1hr_per_1k_micro_yuan"].Int64() == cacheCreationAbove1hrMicro &&
			current["cache_read_input_token_price_per_1k_micro_yuan"].Int64() == cacheReadMicro &&
			current["input_token_price_above_200k_per_1k_micro_yuan"].Int64() == inputAbove200kMicro &&
			current["output_token_price_above_200k_per_1k_micro_yuan"].Int64() == outputAbove200kMicro &&
			current["cache_creation_input_token_price_above_200k_per_1k_micro_yuan"].Int64() == cacheCreationAbove200kMicro &&
			current["cache_read_input_token_price_above_200k_per_1k_micro_yuan"].Int64() == cacheReadAbove200kMicro &&
			current["output_image_price_micro_yuan"].Int64() == outputImageMicro &&
			current["output_image_token_price_per_1k_micro_yuan"].Int64() == outputImageTokenMicro &&
			current["input_image_price_micro_yuan"].Int64() == inputImageMicro &&
			current["input_image_token_price_per_1k_micro_yuan"].Int64() == inputImageTokenMicro &&
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
			(model_id, currency, input_price_per_1k_micro_yuan, output_price_per_1k_micro_yuan,
			 input_request_price_micro_yuan, cache_creation_input_token_price_per_1k_micro_yuan,
			 cache_creation_input_token_price_above_1hr_per_1k_micro_yuan, cache_read_input_token_price_per_1k_micro_yuan,
			 input_token_price_above_200k_per_1k_micro_yuan, output_token_price_above_200k_per_1k_micro_yuan,
			 cache_creation_input_token_price_above_200k_per_1k_micro_yuan, cache_read_input_token_price_above_200k_per_1k_micro_yuan,
			 output_image_price_micro_yuan, output_image_token_price_per_1k_micro_yuan,
			 input_image_price_micro_yuan, input_image_token_price_per_1k_micro_yuan, supports_prompt_caching,
			 effective_from, status)
			VALUES (?, 'CNY', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active')`,
			modelID,
			inputMicro,
			outputMicro,
			inputRequestMicro,
			cacheCreationMicro,
			cacheCreationAbove1hrMicro,
			cacheReadMicro,
			inputAbove200kMicro,
			outputAbove200kMicro,
			cacheCreationAbove200kMicro,
			cacheReadAbove200kMicro,
			outputImageMicro,
			outputImageTokenMicro,
			inputImageMicro,
			inputImageTokenMicro,
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

func hasValidModelPrice(item activeModelPrice) bool {
	pricing := materializeModelPricing(item.Pricing)
	return pricing.InputPer1K > 0 || pricing.OutputPer1K > 0
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
	return pricing.InputPer1K, pricing.OutputPer1K, true
}

func normalizeModelPriceKey(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func toPortalModelPricing(src clientK8s.ProviderPricing) model.ModelPricing {
	return model.ModelPricing{
		Currency:                                   src.Currency,
		InputPer1K:                                 src.InputPer1K,
		OutputPer1K:                                src.OutputPer1K,
		InputCostPerToken:                          src.InputCostPerToken,
		OutputCostPerToken:                         src.OutputCostPerToken,
		InputCostPerRequest:                        src.InputCostPerRequest,
		CacheCreationInputTokenCost:                src.CacheCreationInputTokenCost,
		CacheCreationInputTokenCostAbove1hr:        src.CacheCreationInputTokenCostAbove1hr,
		CacheReadInputTokenCost:                    src.CacheReadInputTokenCost,
		InputCostPerTokenAbove200kTokens:           src.InputCostPerTokenAbove200kTokens,
		OutputCostPerTokenAbove200kTokens:          src.OutputCostPerTokenAbove200kTokens,
		CacheCreationInputTokenCostAbove200kTokens: src.CacheCreationInputTokenCostAbove200kTokens,
		CacheReadInputTokenCostAbove200kTokens:     src.CacheReadInputTokenCostAbove200kTokens,
		OutputCostPerImage:                         src.OutputCostPerImage,
		OutputCostPerImageToken:                    src.OutputCostPerImageToken,
		InputCostPerImage:                          src.InputCostPerImage,
		InputCostPerImageToken:                     src.InputCostPerImageToken,
		SupportsPromptCaching:                      src.SupportsPromptCaching,
	}
}

func modelPricingFromPriceVersionRecord(record gdb.Record) model.ModelPricing {
	inputPer1K := microYuanToRMB(record["input_price_per_1k_micro_yuan"].Int64())
	outputPer1K := microYuanToRMB(record["output_price_per_1k_micro_yuan"].Int64())
	return materializeModelPricing(model.ModelPricing{
		Currency:                                   billingCurrencyCNY,
		InputPer1K:                                 inputPer1K,
		OutputPer1K:                                outputPer1K,
		InputCostPerToken:                          inputPer1K / 1000,
		OutputCostPerToken:                         outputPer1K / 1000,
		InputCostPerRequest:                        microYuanToRMB(record["input_request_price_micro_yuan"].Int64()),
		CacheCreationInputTokenCost:                microYuanToRMB(record["cache_creation_input_token_price_per_1k_micro_yuan"].Int64()) / 1000,
		CacheCreationInputTokenCostAbove1hr:        microYuanToRMB(record["cache_creation_input_token_price_above_1hr_per_1k_micro_yuan"].Int64()) / 1000,
		CacheReadInputTokenCost:                    microYuanToRMB(record["cache_read_input_token_price_per_1k_micro_yuan"].Int64()) / 1000,
		InputCostPerTokenAbove200kTokens:           microYuanToRMB(record["input_token_price_above_200k_per_1k_micro_yuan"].Int64()) / 1000,
		OutputCostPerTokenAbove200kTokens:          microYuanToRMB(record["output_token_price_above_200k_per_1k_micro_yuan"].Int64()) / 1000,
		CacheCreationInputTokenCostAbove200kTokens: microYuanToRMB(record["cache_creation_input_token_price_above_200k_per_1k_micro_yuan"].Int64()) / 1000,
		CacheReadInputTokenCostAbove200kTokens:     microYuanToRMB(record["cache_read_input_token_price_above_200k_per_1k_micro_yuan"].Int64()) / 1000,
		OutputCostPerImage:                         microYuanToRMB(record["output_image_price_micro_yuan"].Int64()),
		OutputCostPerImageToken:                    microYuanToRMB(record["output_image_token_price_per_1k_micro_yuan"].Int64()) / 1000,
		InputCostPerImage:                          microYuanToRMB(record["input_image_price_micro_yuan"].Int64()),
		InputCostPerImageToken:                     microYuanToRMB(record["input_image_token_price_per_1k_micro_yuan"].Int64()) / 1000,
		SupportsPromptCaching:                      record["supports_prompt_caching"].Int64() > 0,
	})
}

func materializeModelPricing(raw model.ModelPricing) model.ModelPricing {
	pricing := raw
	if strings.TrimSpace(pricing.Currency) == "" {
		pricing.Currency = billingCurrencyCNY
	}
	if pricing.InputCostPerToken == 0 && pricing.InputPer1K > 0 {
		pricing.InputCostPerToken = pricing.InputPer1K / 1000
	}
	if pricing.OutputCostPerToken == 0 && pricing.OutputPer1K > 0 {
		pricing.OutputCostPerToken = pricing.OutputPer1K / 1000
	}
	if pricing.InputPer1K == 0 && pricing.InputCostPerToken > 0 {
		pricing.InputPer1K = pricing.InputCostPerToken * 1000
	}
	if pricing.OutputPer1K == 0 && pricing.OutputCostPerToken > 0 {
		pricing.OutputPer1K = pricing.OutputCostPerToken * 1000
	}
	pricing.CacheCreationInputTokenCost = firstPositiveFloat(
		pricing.CacheCreationInputTokenCost,
		pricing.InputCostPerToken*1.25,
	)
	pricing.CacheCreationInputTokenCostAbove1hr = firstPositiveFloat(
		pricing.CacheCreationInputTokenCostAbove1hr,
		pricing.InputCostPerToken*2,
		pricing.CacheCreationInputTokenCost,
	)
	pricing.CacheReadInputTokenCost = firstPositiveFloat(
		pricing.CacheReadInputTokenCost,
		pricing.InputCostPerToken*0.1,
		pricing.OutputCostPerToken*0.1,
	)
	pricing.InputCostPerTokenAbove200kTokens = firstPositiveFloat(
		pricing.InputCostPerTokenAbove200kTokens,
		pricing.InputCostPerToken,
	)
	pricing.OutputCostPerTokenAbove200kTokens = firstPositiveFloat(
		pricing.OutputCostPerTokenAbove200kTokens,
		pricing.OutputCostPerToken,
	)
	pricing.CacheCreationInputTokenCostAbove200kTokens = firstPositiveFloat(
		pricing.CacheCreationInputTokenCostAbove200kTokens,
		pricing.CacheCreationInputTokenCost,
	)
	pricing.CacheReadInputTokenCostAbove200kTokens = firstPositiveFloat(
		pricing.CacheReadInputTokenCostAbove200kTokens,
		pricing.CacheReadInputTokenCost,
	)
	pricing.OutputCostPerImageToken = firstPositiveFloat(
		pricing.OutputCostPerImageToken,
		pricing.OutputCostPerToken,
	)
	pricing.InputCostPerImageToken = firstPositiveFloat(
		pricing.InputCostPerImageToken,
		pricing.InputCostPerToken,
	)
	return pricing
}

func firstPositiveFloat(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

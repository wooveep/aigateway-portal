package portal

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"higress-portal-backend/internal/apperr"
	clientK8s "higress-portal-backend/internal/client/k8s"
	"higress-portal-backend/internal/model"
)

func (s *Service) ListModels(ctx context.Context) ([]model.ModelInfo, error) {
	items, err := s.listModelsFromCatalogDB(ctx)
	if err == nil && len(items) > 0 {
		return items, nil
	}
	records, err := s.modelK8s.ListEnabledModels(ctx)
	if err != nil {
		return nil, apperr.New(503, "model catalog unavailable", err.Error())
	}

	items = make([]model.ModelInfo, 0, len(records))
	for _, record := range records {
		items = append(items, toPortalModelInfo(record))
	}
	return items, nil
}

func (s *Service) GetModelDetail(ctx context.Context, id string) (model.ModelInfo, error) {
	targetID := strings.TrimSpace(id)
	if targetID == "" {
		return model.ModelInfo{}, apperr.New(404, "model not found")
	}

	item, err := s.getModelFromCatalogDB(ctx, targetID)
	if err == nil && strings.TrimSpace(item.ID) != "" {
		return item, nil
	}
	records, err := s.modelK8s.ListEnabledModels(ctx)
	if err != nil {
		return model.ModelInfo{}, apperr.New(503, "model catalog unavailable", err.Error())
	}
	for _, record := range records {
		if record.ID == targetID {
			return toPortalModelInfo(record), nil
		}
	}
	return model.ModelInfo{}, apperr.New(404, "model not found")
}

func toPortalModelInfo(src clientK8s.ProviderModel) model.ModelInfo {
	capabilities := model.ModelCapabilities{
		Modalities: src.Meta.Capabilities.Modalities,
		Features:   src.Meta.Capabilities.Features,
	}
	pricing := model.ModelPricing{
		Currency:                                   billingCurrencyCNY,
		InputPer1K:                                 src.Meta.Pricing.InputPer1K,
		OutputPer1K:                                src.Meta.Pricing.OutputPer1K,
		InputCostPerToken:                          src.Meta.Pricing.InputCostPerToken,
		OutputCostPerToken:                         src.Meta.Pricing.OutputCostPerToken,
		InputCostPerRequest:                        src.Meta.Pricing.InputCostPerRequest,
		CacheCreationInputTokenCost:                src.Meta.Pricing.CacheCreationInputTokenCost,
		CacheCreationInputTokenCostAbove1hr:        src.Meta.Pricing.CacheCreationInputTokenCostAbove1hr,
		CacheReadInputTokenCost:                    src.Meta.Pricing.CacheReadInputTokenCost,
		InputCostPerTokenAbove200kTokens:           src.Meta.Pricing.InputCostPerTokenAbove200kTokens,
		OutputCostPerTokenAbove200kTokens:          src.Meta.Pricing.OutputCostPerTokenAbove200kTokens,
		CacheCreationInputTokenCostAbove200kTokens: src.Meta.Pricing.CacheCreationInputTokenCostAbove200kTokens,
		CacheReadInputTokenCostAbove200kTokens:     src.Meta.Pricing.CacheReadInputTokenCostAbove200kTokens,
		OutputCostPerImage:                         src.Meta.Pricing.OutputCostPerImage,
		OutputCostPerImageToken:                    src.Meta.Pricing.OutputCostPerImageToken,
		InputCostPerImage:                          src.Meta.Pricing.InputCostPerImage,
		InputCostPerImageToken:                     src.Meta.Pricing.InputCostPerImageToken,
		SupportsPromptCaching:                      src.Meta.Pricing.SupportsPromptCaching,
	}
	limits := model.ModelLimits{
		RPM:           src.Meta.Limits.RPM,
		TPM:           src.Meta.Limits.TPM,
		ContextWindow: src.Meta.Limits.ContextWindow,
	}

	capabilitySummary := strings.Join(append(append([]string{}, capabilities.Modalities...), capabilities.Features...), " / ")
	if capabilitySummary == "" {
		capabilitySummary = src.Type
	}

	endpoint := src.Endpoint
	if strings.TrimSpace(endpoint) == "" {
		endpoint = "-"
	}

	return model.ModelInfo{
		ID:               src.ID,
		Name:             src.ID,
		Vendor:           src.Type,
		Capability:       capabilitySummary,
		InputTokenPrice:  pricing.InputPer1K,
		OutputTokenPrice: pricing.OutputPer1K,
		Endpoint:         endpoint,
		SDK:              src.Protocol,
		UpdatedAt:        model.DayText(model.NowInAppLocation()),
		Summary:          src.Meta.Intro,
		Tags:             src.Meta.Tags,
		Capabilities:     capabilities,
		Pricing:          pricing,
		Limits:           limits,
	}
}

func (s *Service) GetOpenStats(ctx context.Context, consumerName string) (model.OpenStats, error) {
	var (
		todayCalls     int64
		todayCost      int64
		last7DaysCalls int64
		activeKeys     int64
	)

	now := model.NowInAppLocation()
	today := model.DayText(now)
	startOfToday := model.StartOfAppDay(now)
	sevenDaysAgo := model.DayText(startOfToday.AddDate(0, 0, -6))

	todayRecord, err := s.db.GetOne(ctx, `
		SELECT COALESCE(SUM(request_count),0) AS calls
		FROM portal_usage_daily
		WHERE consumer_name = ?
		  AND billing_date = ?`, consumerName, today)
	if err != nil {
		return model.OpenStats{}, gerror.Wrap(err, "query today stats failed")
	}
	todayCalls = todayRecord["calls"].Int64()

	todayCostRecord, err := s.db.GetValue(ctx, `
		SELECT COALESCE(SUM(cost_amount),0)
		FROM portal_usage_daily
		WHERE consumer_name = ?
		  AND billing_date = ?`, consumerName, today)
	if err != nil {
		return model.OpenStats{}, gerror.Wrap(err, "query today cost failed")
	}
	todayCost = rmbToMicroYuan(todayCostRecord.Float64())

	last7Record, err := s.db.GetOne(ctx, `
		SELECT COALESCE(SUM(request_count),0) AS calls
		FROM portal_usage_daily
		WHERE consumer_name = ?
		  AND billing_date >= ?
		  AND billing_date <= ?`, consumerName, sevenDaysAgo, today)
	if err != nil {
		return model.OpenStats{}, gerror.Wrap(err, "query 7days stats failed")
	}
	last7DaysCalls = last7Record["calls"].Int64()

	keyCount, err := s.db.GetValue(ctx, `
		SELECT COUNT(1)
		FROM portal_api_key
		WHERE consumer_name = ?
		  AND deleted_at IS NULL
		  AND status = 'active'
		  AND (expires_at IS NULL OR expires_at > ?)`, consumerName, now)
	if err != nil {
		return model.OpenStats{}, gerror.Wrap(err, "query active key failed")
	}
	activeKeys = keyCount.Int64()

	return model.OpenStats{
		TodayCalls:     todayCalls,
		TodayCost:      microYuanToText(todayCost),
		Last7DaysCalls: last7DaysCalls,
		ActiveKeys:     activeKeys,
	}, nil
}

func (s *Service) ListCostDetails(ctx context.Context, consumerName string) ([]model.CostDetailRecord, error) {
	records, err := s.db.GetAll(ctx, `
		SELECT
			billing_date,
			model_name,
			request_count,
			total_tokens,
			cost_amount
		FROM portal_usage_daily
		WHERE consumer_name = ?
		ORDER BY billing_date DESC, model_name ASC`, consumerName)
	if err != nil {
		return nil, gerror.Wrap(err, "query cost details failed")
	}

	items := make([]model.CostDetailRecord, 0, len(records))
	for _, record := range records {
		billingDate := strings.TrimSpace(record["billing_date"].String())
		modelID := strings.TrimSpace(record["model_name"].String())
		items = append(items, model.CostDetailRecord{
			ID:     fmt.Sprintf("COST-%s-%s", billingDate, modelID),
			Date:   billingDate,
			Model:  modelID,
			Calls:  record["request_count"].Int64(),
			Tokens: record["total_tokens"].Int64(),
			Cost:   record["cost_amount"].Float64(),
		})
	}
	return items, nil
}

func (s *Service) listModelsFromCatalogDB(ctx context.Context) ([]model.ModelInfo, error) {
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
			c.summary,
			c.updated_at
		FROM billing_model_catalog c
		INNER JOIN billing_model_price_version p
			ON p.model_id = c.model_id
		WHERE c.status = 'active'
		  AND p.status = 'active'
		  AND p.effective_to IS NULL
		ORDER BY c.model_id ASC`)
	if err != nil {
		return nil, gerror.Wrap(err, "query billing model catalog failed")
	}
	items := make([]model.ModelInfo, 0, len(records))
	for _, record := range records {
		items = append(items, toPortalModelInfoFromRecord(record))
	}
	return items, nil
}

func (s *Service) getModelFromCatalogDB(ctx context.Context, id string) (model.ModelInfo, error) {
	record, err := s.db.GetOne(ctx, `
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
			c.summary,
			c.updated_at
		FROM billing_model_catalog c
		INNER JOIN billing_model_price_version p
			ON p.model_id = c.model_id
		WHERE c.model_id = ?
		  AND c.status = 'active'
		  AND p.status = 'active'
		  AND p.effective_to IS NULL
		ORDER BY p.id DESC
		LIMIT 1`, id)
	if err != nil {
		return model.ModelInfo{}, gerror.Wrap(err, "query billing model detail failed")
	}
	if len(record) == 0 {
		return model.ModelInfo{}, nil
	}
	return toPortalModelInfoFromRecord(record), nil
}

func toPortalModelInfoFromRecord(record gdb.Record) model.ModelInfo {
	modelID := strings.TrimSpace(record["model_id"].String())
	name := strings.TrimSpace(record["name"].String())
	if name == "" {
		name = modelID
	}
	inputPer1K := microYuanToRMB(record["input_price_per_1k_micro_yuan"].Int64())
	outputPer1K := microYuanToRMB(record["output_price_per_1k_micro_yuan"].Int64())
	inputPerToken := inputPer1K / 1000
	outputPerToken := outputPer1K / 1000
	updatedAt := model.DayText(model.NowInAppLocation())
	if updatedTime := record["updated_at"].Time(); !updatedTime.IsZero() {
		updatedAt = model.DayText(updatedTime)
	}
	return model.ModelInfo{
		ID:               modelID,
		Name:             name,
		Vendor:           strings.TrimSpace(record["vendor"].String()),
		Capability:       strings.TrimSpace(record["capability"].String()),
		InputTokenPrice:  inputPer1K,
		OutputTokenPrice: outputPer1K,
		Endpoint:         strings.TrimSpace(record["endpoint"].String()),
		SDK:              strings.TrimSpace(record["sdk"].String()),
		UpdatedAt:        updatedAt,
		Summary:          strings.TrimSpace(record["summary"].String()),
		Pricing: model.ModelPricing{
			Currency:                                   billingCurrencyCNY,
			InputPer1K:                                 inputPer1K,
			OutputPer1K:                                outputPer1K,
			InputCostPerToken:                          inputPerToken,
			OutputCostPerToken:                         outputPerToken,
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
		},
	}
}

package portal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"higress-portal-backend/internal/apperr"
	"higress-portal-backend/internal/consts"
	"higress-portal-backend/internal/model"
)

type publishedBindingModel struct {
	BindingID string
	ModelInfo model.ModelInfo
}

func (s *Service) ListModels(ctx context.Context, user model.AuthUser) ([]model.ModelInfo, error) {
	items, err := s.listVisibleModelsFromPublishedBindings(ctx, user)
	if err != nil {
		return nil, apperr.New(503, "model catalog unavailable", err.Error())
	}
	for index := range items {
		items[index] = s.applyModelRequestURL(items[index])
	}
	return items, nil
}

func (s *Service) GetModelDetail(ctx context.Context, id string, user model.AuthUser) (model.ModelInfo, error) {
	targetID := strings.TrimSpace(id)
	if targetID == "" {
		return model.ModelInfo{}, apperr.New(404, "model not found")
	}

	item, err := s.getVisibleModelFromPublishedBindings(ctx, targetID, user)
	if err == nil && strings.TrimSpace(item.ID) != "" {
		return s.applyModelRequestURL(item), nil
	}
	if err != nil {
		return model.ModelInfo{}, apperr.New(503, "model catalog unavailable", err.Error())
	}
	return model.ModelInfo{}, apperr.New(404, "model not found")
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

func (s *Service) listModelsFromPublishedBindings(ctx context.Context) ([]model.ModelInfo, error) {
	records, err := s.db.GetAll(ctx, `
		SELECT
			b.binding_id,
			b.model_id,
			a.display_name AS name,
			b.provider_name AS vendor,
			a.intro AS summary,
			a.tags_json,
			a.modalities_json,
			a.features_json,
			a.request_kinds_json,
			b.rpm,
			b.tpm,
			b.context_window,
			b.endpoint,
			b.protocol,
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
			GREATEST(a.updated_at, b.updated_at) AS updated_at
		FROM portal_model_binding b
		INNER JOIN portal_model_asset a
			ON a.asset_id = b.asset_id
		LEFT JOIN billing_model_price_version p
			ON p.model_id = b.model_id
		   AND p.status = 'active'
		   AND p.effective_to IS NULL
		WHERE b.status = 'published'
		ORDER BY a.canonical_name ASC, b.model_id ASC`)
	if err != nil {
		return nil, gerror.Wrap(err, "query published model bindings failed")
	}
	items := make([]model.ModelInfo, 0, len(records))
	for _, record := range records {
		items = append(items, toPortalModelInfoFromPublishedBinding(record))
	}
	return items, nil
}

func (s *Service) getModelFromPublishedBindings(ctx context.Context, id string) (model.ModelInfo, error) {
	record, err := s.db.GetOne(ctx, `
		SELECT
			b.binding_id,
			b.model_id,
			a.display_name AS name,
			b.provider_name AS vendor,
			a.intro AS summary,
			a.tags_json,
			a.modalities_json,
			a.features_json,
			a.request_kinds_json,
			b.rpm,
			b.tpm,
			b.context_window,
			b.endpoint,
			b.protocol,
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
			GREATEST(a.updated_at, b.updated_at) AS updated_at
		FROM portal_model_binding b
		INNER JOIN portal_model_asset a
			ON a.asset_id = b.asset_id
		LEFT JOIN billing_model_price_version p
			ON p.model_id = b.model_id
		   AND p.status = 'active'
		   AND p.effective_to IS NULL
		WHERE b.status = 'published'
		  AND b.model_id = ?
		LIMIT 1`, id)
	if err != nil {
		return model.ModelInfo{}, gerror.Wrap(err, "query published model binding detail failed")
	}
	if len(record) == 0 {
		return model.ModelInfo{}, nil
	}
	return toPortalModelInfoFromPublishedBinding(record), nil
}

func (s *Service) listVisibleModelsFromPublishedBindings(ctx context.Context, user model.AuthUser) ([]model.ModelInfo, error) {
	records, err := s.db.GetAll(ctx, `
		SELECT
			b.binding_id,
			b.model_id,
			a.display_name AS name,
			b.provider_name AS vendor,
			a.intro AS summary,
			a.tags_json,
			a.modalities_json,
			a.features_json,
			a.request_kinds_json,
			b.rpm,
			b.tpm,
			b.context_window,
			b.endpoint,
			b.protocol,
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
			GREATEST(a.updated_at, b.updated_at) AS updated_at
		FROM portal_model_binding b
		INNER JOIN portal_model_asset a
			ON a.asset_id = b.asset_id
		LEFT JOIN billing_model_price_version p
			ON p.model_id = b.model_id
		   AND p.status = 'active'
		   AND p.effective_to IS NULL
		WHERE b.status = 'published'
		ORDER BY a.canonical_name ASC, b.model_id ASC`)
	if err != nil {
		return nil, gerror.Wrap(err, "query visible published model bindings failed")
	}
	items := make([]publishedBindingModel, 0, len(records))
	for _, record := range records {
		items = append(items, publishedBindingModel{
			BindingID: strings.TrimSpace(record["binding_id"].String()),
			ModelInfo: toPortalModelInfoFromPublishedBinding(record),
		})
	}
	visible, err := s.filterVisiblePublishedBindings(ctx, user, items)
	if err != nil {
		return nil, err
	}
	result := make([]model.ModelInfo, 0, len(visible))
	for _, item := range visible {
		result = append(result, item.ModelInfo)
	}
	return result, nil
}

func (s *Service) getVisibleModelFromPublishedBindings(ctx context.Context, id string, user model.AuthUser) (model.ModelInfo, error) {
	record, err := s.db.GetOne(ctx, `
		SELECT
			b.binding_id,
			b.model_id,
			a.display_name AS name,
			b.provider_name AS vendor,
			a.intro AS summary,
			a.tags_json,
			a.modalities_json,
			a.features_json,
			a.request_kinds_json,
			b.rpm,
			b.tpm,
			b.context_window,
			b.endpoint,
			b.protocol,
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
			GREATEST(a.updated_at, b.updated_at) AS updated_at
		FROM portal_model_binding b
		INNER JOIN portal_model_asset a
			ON a.asset_id = b.asset_id
		LEFT JOIN billing_model_price_version p
			ON p.model_id = b.model_id
		   AND p.status = 'active'
		   AND p.effective_to IS NULL
		WHERE b.status = 'published'
		  AND b.model_id = ?
		LIMIT 1`, id)
	if err != nil {
		return model.ModelInfo{}, gerror.Wrap(err, "query visible published model binding detail failed")
	}
	if len(record) == 0 {
		return model.ModelInfo{}, nil
	}
	visible, filterErr := s.filterVisiblePublishedBindings(ctx, user, []publishedBindingModel{{
		BindingID: strings.TrimSpace(record["binding_id"].String()),
		ModelInfo: toPortalModelInfoFromPublishedBinding(record),
	}})
	if filterErr != nil {
		return model.ModelInfo{}, filterErr
	}
	if len(visible) == 0 {
		return model.ModelInfo{}, nil
	}
	return visible[0].ModelInfo, nil
}

func (s *Service) filterVisiblePublishedBindings(ctx context.Context, user model.AuthUser,
	items []publishedBindingModel,
) ([]publishedBindingModel, error) {
	if len(items) == 0 {
		return []publishedBindingModel{}, nil
	}
	grantsByBinding, err := s.loadBindingGrants(ctx, items)
	if err != nil {
		return nil, err
	}
	ancestorIDs, err := s.listDepartmentAncestorIds(ctx, user.DepartmentID)
	if err != nil {
		return nil, err
	}
	ancestorSet := make(map[string]struct{}, len(ancestorIDs))
	for _, item := range ancestorIDs {
		if item != "" {
			ancestorSet[item] = struct{}{}
		}
	}
	consumerName := model.NormalizeUsername(user.ConsumerName)
	result := make([]publishedBindingModel, 0, len(items))
	for _, item := range items {
		grants := grantsByBinding[item.BindingID]
		if len(grants) == 0 {
			result = append(result, item)
			continue
		}
		visible := false
		for _, grant := range grants {
			subjectType := strings.TrimSpace(grant["subject_type"].String())
			subjectID := strings.TrimSpace(grant["subject_id"].String())
			if subjectType == "consumer" && subjectID == consumerName {
				visible = true
				break
			}
			if subjectType == "department" {
				if _, ok := ancestorSet[subjectID]; ok {
					visible = true
					break
				}
			}
			if subjectType == "user_level" && userLevelRank(user.UserLevel) >= userLevelRank(subjectID) {
				visible = true
				break
			}
		}
		if visible {
			result = append(result, item)
		}
	}
	return result, nil
}

func userLevelRank(level string) int {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case consts.UserLevelUltra:
		return 4
	case consts.UserLevelPro:
		return 3
	case consts.UserLevelPlus:
		return 2
	default:
		return 1
	}
}

func (s *Service) loadBindingGrants(ctx context.Context, items []publishedBindingModel) (map[string]gdb.Result, error) {
	bindingIDs := make([]string, 0, len(items))
	for _, item := range items {
		if item.BindingID != "" {
			bindingIDs = append(bindingIDs, item.BindingID)
		}
	}
	query, args := buildStringInQuery(`
		SELECT asset_id, subject_type, subject_id
		FROM asset_grant
		WHERE asset_type = 'model_binding'
		  AND asset_id IN (%s)`, bindingIDs)
	records, err := s.db.GetAll(ctx, query, args...)
	if err != nil {
		return nil, gerror.Wrap(err, "query model binding grants failed")
	}
	result := make(map[string]gdb.Result, len(bindingIDs))
	for _, record := range records {
		assetID := strings.TrimSpace(record["asset_id"].String())
		result[assetID] = append(result[assetID], record)
	}
	return result, nil
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

func toPortalModelInfoFromPublishedBinding(record gdb.Record) model.ModelInfo {
	modelID := strings.TrimSpace(record["model_id"].String())
	name := strings.TrimSpace(record["name"].String())
	if name == "" {
		name = modelID
	}
	capabilities := model.ModelCapabilities{
		Modalities:   parseStringList(record["modalities_json"].String()),
		Features:     parseStringList(record["features_json"].String()),
		RequestKinds: parseStringList(record["request_kinds_json"].String()),
	}
	limits := model.ModelLimits{
		RPM:           record["rpm"].Int64(),
		TPM:           record["tpm"].Int64(),
		ContextWindow: record["context_window"].Int64(),
	}
	capabilitySummary := strings.Join(append(append([]string{}, capabilities.Modalities...), capabilities.Features...), " / ")
	if capabilitySummary == "" {
		capabilitySummary = strings.TrimSpace(record["vendor"].String())
	}
	endpoint := strings.TrimSpace(record["endpoint"].String())
	if endpoint == "" {
		endpoint = "-"
	}
	sdk := strings.TrimSpace(record["protocol"].String())
	if sdk == "" {
		sdk = "openai/v1"
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
		Capability:       capabilitySummary,
		InputTokenPrice:  inputPer1K,
		OutputTokenPrice: outputPer1K,
		Endpoint:         endpoint,
		SDK:              sdk,
		UpdatedAt:        updatedAt,
		Summary:          strings.TrimSpace(record["summary"].String()),
		Tags:             parseStringList(record["tags_json"].String()),
		Capabilities:     capabilities,
		Limits:           limits,
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

func parseStringList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	items := make([]string, 0)
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return []string{}
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

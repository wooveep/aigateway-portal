package portal

import (
	"context"
	"fmt"
	"strings"
	"time"

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
		Currency:    billingCurrencyCNY,
		InputPer1K:  src.Meta.Pricing.InputPer1K,
		OutputPer1K: src.Meta.Pricing.OutputPer1K,
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
		UpdatedAt:        time.Now().Format("2006-01-02"),
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

	todayRecord, err := s.db.GetOne(ctx, `
		SELECT COALESCE(COUNT(1),0) AS calls
		FROM billing_usage_event
		WHERE consumer_name = ?
		  AND request_status = 'success'
		  AND DATE(occurred_at) = CURDATE()`, consumerName)
	if err != nil {
		return model.OpenStats{}, gerror.Wrap(err, "query today stats failed")
	}
	todayCalls = todayRecord["calls"].Int64()

	todayCostRecord, err := s.db.GetValue(ctx, `
		SELECT COALESCE(SUM(0 - amount_micro_yuan),0)
		FROM billing_transaction
		WHERE consumer_name = ?
		  AND tx_type IN ('consume', 'reconcile')
		  AND DATE(occurred_at) = CURDATE()`, consumerName)
	if err != nil {
		return model.OpenStats{}, gerror.Wrap(err, "query today cost failed")
	}
	todayCost = todayCostRecord.Int64()

	last7Record, err := s.db.GetOne(ctx, `
		SELECT COALESCE(COUNT(1),0) AS calls
		FROM billing_usage_event
		WHERE consumer_name = ?
		  AND request_status = 'success'
		  AND occurred_at >= DATE_SUB(CURDATE(), INTERVAL 6 DAY)`, consumerName)
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
		  AND (expires_at IS NULL OR expires_at > NOW())`, consumerName)
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
			DATE(occurred_at) AS billing_date,
			model_id,
			COUNT(1) AS request_count,
			COALESCE(SUM(input_tokens + output_tokens), 0) AS total_tokens,
			COALESCE(SUM(0 - amount_micro_yuan), 0) AS total_cost_micro_yuan
		FROM billing_transaction
		WHERE consumer_name = ?
		  AND tx_type IN ('consume', 'reconcile')
		GROUP BY DATE(occurred_at), model_id
		ORDER BY billing_date DESC, model_id ASC`, consumerName)
	if err != nil {
		return nil, gerror.Wrap(err, "query cost details failed")
	}

	items := make([]model.CostDetailRecord, 0, len(records))
	for _, record := range records {
		billingDate := record["billing_date"].Time().Format("2006-01-02")
		modelID := record["model_id"].String()
		items = append(items, model.CostDetailRecord{
			ID:     fmt.Sprintf("COST-%s-%s", billingDate, modelID),
			Date:   billingDate,
			Model:  modelID,
			Calls:  record["request_count"].Int64(),
			Tokens: record["total_tokens"].Int64(),
			Cost:   microYuanToRMB(record["total_cost_micro_yuan"].Int64()),
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
	updatedAt := time.Now().Format("2006-01-02")
	if updatedTime := record["updated_at"].Time(); !updatedTime.IsZero() {
		updatedAt = updatedTime.Format("2006-01-02")
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
			Currency:    billingCurrencyCNY,
			InputPer1K:  inputPer1K,
			OutputPer1K: outputPer1K,
		},
	}
}

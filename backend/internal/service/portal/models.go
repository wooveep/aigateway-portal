package portal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"higress-portal-backend/internal/apperr"
	clientK8s "higress-portal-backend/internal/client/k8s"
	"higress-portal-backend/internal/model"
)

func (s *Service) ListModels(ctx context.Context) ([]model.ModelInfo, error) {
	records, err := s.modelK8s.ListEnabledModels(ctx)
	if err != nil {
		return nil, apperr.New(503, "model catalog unavailable", err.Error())
	}

	items := make([]model.ModelInfo, 0, len(records))
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
		Currency:    src.Meta.Pricing.Currency,
		InputPer1K:  src.Meta.Pricing.InputPer1K,
		OutputPer1K: src.Meta.Pricing.OutputPer1K,
	}
	if strings.TrimSpace(pricing.Currency) == "" {
		pricing.Currency = "USD"
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
		todayCost      float64
		last7DaysCalls int64
		activeKeys     int64
	)

	todayRecord, err := s.db.GetOne(ctx, `
		SELECT COALESCE(SUM(request_count),0) AS calls, COALESCE(SUM(cost_amount),0) AS cost
		FROM portal_usage_daily
		WHERE consumer_name = ? AND billing_date = CURDATE()`, consumerName)
	if err != nil {
		return model.OpenStats{}, gerror.Wrap(err, "query today stats failed")
	}
	todayCalls = todayRecord["calls"].Int64()
	todayCost = todayRecord["cost"].Float64()

	if todayCalls == 0 {
		fallback, fbErr := s.db.GetValue(ctx, `
			SELECT COALESCE(SUM(total_tokens),0)
			FROM portal_usage_daily
			WHERE consumer_name = ? AND billing_date = CURDATE()`, consumerName)
		if fbErr == nil {
			todayCalls = fallback.Int64()
		}
	}

	last7Record, err := s.db.GetOne(ctx, `
		SELECT COALESCE(SUM(request_count),0) AS calls
		FROM portal_usage_daily
		WHERE consumer_name = ? AND billing_date >= DATE_SUB(CURDATE(), INTERVAL 6 DAY)`, consumerName)
	if err != nil {
		return model.OpenStats{}, gerror.Wrap(err, "query 7days stats failed")
	}
	last7DaysCalls = last7Record["calls"].Int64()

	if last7DaysCalls == 0 {
		fallback, fbErr := s.db.GetValue(ctx, `
			SELECT COALESCE(SUM(total_tokens),0)
			FROM portal_usage_daily
			WHERE consumer_name = ? AND billing_date >= DATE_SUB(CURDATE(), INTERVAL 6 DAY)`, consumerName)
		if fbErr == nil {
			last7DaysCalls = fallback.Int64()
		}
	}

	keyCount, err := s.db.GetValue(ctx, `
		SELECT COUNT(1)
		FROM portal_api_key
		WHERE consumer_name = ? AND status = 'active'`, consumerName)
	if err != nil {
		return model.OpenStats{}, gerror.Wrap(err, "query active key failed")
	}
	activeKeys = keyCount.Int64()

	return model.OpenStats{
		TodayCalls:     todayCalls,
		TodayCost:      fmt.Sprintf("%.2f", todayCost),
		Last7DaysCalls: last7DaysCalls,
		ActiveKeys:     activeKeys,
	}, nil
}

func (s *Service) ListCostDetails(ctx context.Context, consumerName string) ([]model.CostDetailRecord, error) {
	records, err := s.db.GetAll(ctx, `
		SELECT id, billing_date, model_name, request_count, total_tokens, cost_amount
		FROM portal_usage_daily
		WHERE consumer_name = ?
		ORDER BY billing_date DESC, id DESC`, consumerName)
	if err != nil {
		return nil, gerror.Wrap(err, "query cost details failed")
	}

	items := make([]model.CostDetailRecord, 0, len(records))
	for _, record := range records {
		items = append(items, model.CostDetailRecord{
			ID:     fmt.Sprintf("COST%d", record["id"].Int64()),
			Date:   record["billing_date"].Time().Format("2006-01-02"),
			Model:  record["model_name"].String(),
			Calls:  record["request_count"].Int64(),
			Tokens: record["total_tokens"].Int64(),
			Cost:   record["cost_amount"].Float64(),
		})
	}
	return items, nil
}

package portal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"higress-portal-backend/internal/apperr"
	"higress-portal-backend/internal/model"
)

func (s *Service) ListRequestDetails(ctx context.Context, consumerName string,
	apiKeyID string, modelID string, routeName string, requestStatus string, usageStatus string,
	startAt string, endAt string, pageNum int, pageSize int,
) ([]model.RequestDetailRecord, error) {
	normalizedConsumer := model.NormalizeUsername(consumerName)
	if normalizedConsumer == "" {
		return []model.RequestDetailRecord{}, nil
	}
	startTime, endTime, err := normalizeRequestDetailTimeRange(startAt, endAt)
	if err != nil {
		return nil, err
	}
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	offset := (pageNum - 1) * pageSize

	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`
		SELECT
			event_id, request_id, trace_id, consumer_name, department_id, department_path,
			api_key_id, model_id, price_version_id, route_name, request_kind,
			request_status, usage_status, http_status,
			input_tokens, output_tokens, total_tokens,
			cache_creation_input_tokens, cache_creation_5m_input_tokens, cache_creation_1h_input_tokens,
			cache_read_input_tokens, input_image_tokens, output_image_tokens,
			input_image_count, output_image_count, request_count, cost_micro_yuan, occurred_at
		FROM billing_usage_event
		WHERE consumer_name = ?`)
	args := []any{normalizedConsumer}

	appendUsageFilter(&queryBuilder, &args, "api_key_id", apiKeyID)
	appendUsageFilter(&queryBuilder, &args, "model_id", modelID)
	appendUsageFilter(&queryBuilder, &args, "route_name", routeName)
	appendUsageFilter(&queryBuilder, &args, "request_status", requestStatus)
	appendUsageFilter(&queryBuilder, &args, "usage_status", usageStatus)
	queryBuilder.WriteString(" AND occurred_at >= ? AND occurred_at <= ?")
	args = append(args, *startTime, *endTime)
	queryBuilder.WriteString(" ORDER BY occurred_at DESC, id DESC LIMIT ? OFFSET ?")
	args = append(args, pageSize, offset)

	records, err := s.db.GetAll(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, gerror.Wrap(err, "query request details failed")
	}
	items := make([]model.RequestDetailRecord, 0, len(records))
	for _, record := range records {
		items = append(items, model.RequestDetailRecord{
			EventID:                    strings.TrimSpace(record["event_id"].String()),
			RequestID:                  strings.TrimSpace(record["request_id"].String()),
			TraceID:                    strings.TrimSpace(record["trace_id"].String()),
			ConsumerName:               strings.TrimSpace(record["consumer_name"].String()),
			APIKeyID:                   strings.TrimSpace(record["api_key_id"].String()),
			ModelID:                    strings.TrimSpace(record["model_id"].String()),
			PriceVersionID:             record["price_version_id"].Int64(),
			RouteName:                  strings.TrimSpace(record["route_name"].String()),
			RequestKind:                strings.TrimSpace(record["request_kind"].String()),
			RequestStatus:              strings.TrimSpace(record["request_status"].String()),
			UsageStatus:                strings.TrimSpace(record["usage_status"].String()),
			HTTPStatus:                 record["http_status"].Int(),
			InputTokens:                record["input_tokens"].Int64(),
			OutputTokens:               record["output_tokens"].Int64(),
			TotalTokens:                record["total_tokens"].Int64(),
			CacheCreationInputTokens:   record["cache_creation_input_tokens"].Int64(),
			CacheCreation5mInputTokens: record["cache_creation_5m_input_tokens"].Int64(),
			CacheCreation1hInputTokens: record["cache_creation_1h_input_tokens"].Int64(),
			CacheReadInputTokens:       record["cache_read_input_tokens"].Int64(),
			InputImageTokens:           record["input_image_tokens"].Int64(),
			OutputImageTokens:          record["output_image_tokens"].Int64(),
			InputImageCount:            record["input_image_count"].Int64(),
			OutputImageCount:           record["output_image_count"].Int64(),
			RequestCount:               record["request_count"].Int64(),
			CostMicroYuan:              record["cost_micro_yuan"].Int64(),
			DepartmentID:               strings.TrimSpace(record["department_id"].String()),
			DepartmentPath:             strings.TrimSpace(record["department_path"].String()),
			OccurredAt:                 model.NowText(record["occurred_at"].Time()),
		})
	}
	return items, nil
}

func (s *Service) ListDepartmentBillingSummaries(ctx context.Context, consumerName string,
	departmentID string, includeChildren bool, startDate string, endDate string,
) ([]model.DepartmentBillingSummary, error) {
	normalizedConsumer := model.NormalizeUsername(consumerName)
	if normalizedConsumer == "" {
		return []model.DepartmentBillingSummary{}, nil
	}
	orgContext, err := s.loadUserOrgContext(ctx, normalizedConsumer)
	if err != nil {
		return nil, err
	}
	if !orgContext.IsDepartmentAdmin || strings.TrimSpace(orgContext.DepartmentID) == "" {
		return nil, apperr.New(403, "department admin required")
	}
	rootDepartmentID := strings.TrimSpace(departmentID)
	if rootDepartmentID == "" {
		rootDepartmentID = strings.TrimSpace(orgContext.DepartmentID)
	}
	if !includeChildren {
		includeChildren = strings.TrimSpace(departmentID) == ""
	}

	departmentIDs := []string{rootDepartmentID}
	if includeChildren {
		departmentIDs, err = s.listDepartmentIdsInSubtree(ctx, rootDepartmentID)
		if err != nil {
			return nil, err
		}
		if len(departmentIDs) == 0 {
			departmentIDs = []string{rootDepartmentID}
		}
	}
	allowedDepartmentIDs, err := s.listDepartmentIdsInSubtree(ctx, orgContext.DepartmentID)
	if err != nil {
		return nil, err
	}
	allowedSet := make(map[string]struct{}, len(allowedDepartmentIDs))
	for _, item := range allowedDepartmentIDs {
		allowedSet[strings.TrimSpace(item)] = struct{}{}
	}
	for _, item := range departmentIDs {
		if _, ok := allowedSet[strings.TrimSpace(item)]; !ok {
			return nil, apperr.New(403, "department scope forbidden")
		}
	}

	startValue, endValue, err := normalizeBillingDateRange(startDate, endDate)
	if err != nil {
		return nil, err
	}
	query, args := buildStringInQuery(`
		SELECT
			d.department_id,
			COALESCE(o.name, '') AS department_name,
			COALESCE(d.department_path, '') AS department_path,
			COALESCE(SUM(d.request_count), 0) AS request_count,
			COALESCE(SUM(d.total_tokens), 0) AS total_tokens,
			COALESCE(SUM(d.cost_amount), 0) AS total_cost,
			COUNT(DISTINCT d.consumer_name) AS active_consumers
		FROM portal_usage_daily d
		LEFT JOIN org_department o ON o.department_id = d.department_id
		WHERE d.department_id IN (%s)
		  AND d.billing_date >= ?
		  AND d.billing_date <= ?
		GROUP BY d.department_id, o.name, d.department_path
		ORDER BY d.department_path ASC, d.department_id ASC`, departmentIDs)
	args = append(args, startValue, endValue)

	records, err := s.db.GetAll(ctx, query, args...)
	if err != nil {
		return nil, gerror.Wrap(err, "query department billing summary failed")
	}
	items := make([]model.DepartmentBillingSummary, 0, len(records))
	for _, record := range records {
		items = append(items, model.DepartmentBillingSummary{
			DepartmentID:    strings.TrimSpace(record["department_id"].String()),
			DepartmentName:  strings.TrimSpace(record["department_name"].String()),
			DepartmentPath:  strings.TrimSpace(record["department_path"].String()),
			RequestCount:    record["request_count"].Int64(),
			TotalTokens:     record["total_tokens"].Int64(),
			TotalCost:       record["total_cost"].Float64(),
			ActiveConsumers: record["active_consumers"].Int64(),
		})
	}
	return items, nil
}

func appendUsageFilter(builder *strings.Builder, args *[]any, column string, value string) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return
	}
	builder.WriteString(fmt.Sprintf(" AND %s = ?", column))
	*args = append(*args, normalized)
}

func normalizeRequestDetailTimeRange(startAt string, endAt string) (*time.Time, *time.Time, error) {
	now := model.NowInAppLocation()
	defaultStart := now.AddDate(0, 0, -7)
	startTime, err := model.ParseDateTime(startAt)
	if err != nil {
		return nil, nil, gerror.Wrap(err, "parse request detail startAt failed")
	}
	endTime, err := model.ParseDateTime(endAt)
	if err != nil {
		return nil, nil, gerror.Wrap(err, "parse request detail endAt failed")
	}
	if startTime == nil {
		startTime = &defaultStart
	}
	if endTime == nil {
		endTime = &now
	}
	if endTime.Before(*startTime) {
		return nil, nil, gerror.New("endAt must be later than startAt")
	}
	return startTime, endTime, nil
}

func normalizeBillingDateRange(startDate string, endDate string) (string, string, error) {
	now := model.NowInAppLocation()
	defaultStart := model.DayText(now.AddDate(0, 0, -29))
	defaultEnd := model.DayText(now)
	startTime, err := model.ParseDateTime(startDate)
	if err != nil {
		return "", "", gerror.Wrap(err, "parse billing startDate failed")
	}
	endTime, err := model.ParseDateTime(endDate)
	if err != nil {
		return "", "", gerror.Wrap(err, "parse billing endDate failed")
	}
	startValue := defaultStart
	if startTime != nil {
		startValue = model.DayText(*startTime)
	}
	endValue := defaultEnd
	if endTime != nil {
		endValue = model.DayText(*endTime)
	}
	if endValue < startValue {
		return "", "", gerror.New("endDate must be later than startDate")
	}
	return startValue, endValue, nil
}

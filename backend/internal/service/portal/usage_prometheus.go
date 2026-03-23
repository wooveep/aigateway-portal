package portal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"higress-portal-backend/internal/model"
)

const (
	usageMetricInputExpr   = "sum by(ai_consumer, ai_model) (increase(route_upstream_model_consumer_metric_input_token[%s]))"
	usageMetricOutputExpr  = "sum by(ai_consumer, ai_model) (increase(route_upstream_model_consumer_metric_output_token[%s]))"
	usageMetricRequestExpr = "sum by(ai_consumer, ai_model) (increase(route_upstream_model_consumer_metric_llm_stream_duration_count[%s]))"
)

type usageMetricKey struct {
	ConsumerName string
	ModelName    string
}

type prometheusQueryResponse struct {
	Status    string `json:"status"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
	Data      struct {
		Result []struct {
			Metric map[string]string `json:"metric"`
			Value  []any             `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

func (s *Service) fetchUsageStatsFromCore(ctx context.Context, from time.Time, to time.Time) ([]model.ConsumerUsageStat, error) {
	if strings.TrimSpace(s.cfg.CorePrometheusURL) == "" {
		return nil, nil
	}

	rangeSeconds := int64(to.Sub(from).Seconds())
	if rangeSeconds < 60 {
		rangeSeconds = 60
	}
	rangeLiteral := fmt.Sprintf("%ds", rangeSeconds)

	inputMap, err := s.queryUsageMetric(ctx, fmt.Sprintf(usageMetricInputExpr, rangeLiteral), to)
	if err != nil {
		return nil, err
	}
	outputMap, err := s.queryUsageMetric(ctx, fmt.Sprintf(usageMetricOutputExpr, rangeLiteral), to)
	if err != nil {
		return nil, err
	}

	requestMap, err := s.queryUsageMetric(ctx, fmt.Sprintf(usageMetricRequestExpr, rangeLiteral), to)
	if err != nil {
		s.logf(ctx, "query request count metric failed, use 0 fallback: %v", err)
		requestMap = make(map[usageMetricKey]int64)
	}

	merged := make(map[usageMetricKey]*model.ConsumerUsageStat)
	mergeUsageMetric(merged, inputMap, true, false, false)
	mergeUsageMetric(merged, outputMap, false, true, false)
	mergeUsageMetric(merged, requestMap, false, false, true)

	items := make([]model.ConsumerUsageStat, 0, len(merged))
	for _, item := range merged {
		item.TotalTokens = item.InputTokens + item.OutputTokens
		items = append(items, *item)
	}
	sort.Slice(items, func(i int, j int) bool {
		if items[i].ConsumerName == items[j].ConsumerName {
			return items[i].ModelName < items[j].ModelName
		}
		return items[i].ConsumerName < items[j].ConsumerName
	})
	return items, nil
}

func mergeUsageMetric(
	merged map[usageMetricKey]*model.ConsumerUsageStat,
	values map[usageMetricKey]int64,
	input bool,
	output bool,
	request bool,
) {
	for key, value := range values {
		item, ok := merged[key]
		if !ok {
			item = &model.ConsumerUsageStat{
				ConsumerName: key.ConsumerName,
				ModelName:    key.ModelName,
			}
			merged[key] = item
		}
		if input {
			item.InputTokens = value
		}
		if output {
			item.OutputTokens = value
		}
		if request {
			item.RequestCount = value
		}
	}
}

func (s *Service) queryUsageMetric(ctx context.Context, expression string, queryTime time.Time) (map[usageMetricKey]int64, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(s.cfg.CorePrometheusURL), "/")
	if baseURL == "" {
		return nil, nil
	}

	query := url.Values{}
	query.Set("query", expression)
	query.Set("time", strconv.FormatInt(queryTime.Unix(), 10))
	requestURL := baseURL + "/api/v1/query?" + query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, gerror.Wrap(err, "build prometheus request failed")
	}

	client := s.httpClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, gerror.Wrap(err, "query prometheus failed")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, gerror.Wrap(err, "read prometheus response failed")
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, gerror.Newf("prometheus query failed: %s, body=%s", resp.Status, truncateText(string(body), 512))
	}

	var parsed prometheusQueryResponse
	if err = json.Unmarshal(body, &parsed); err != nil {
		return nil, gerror.Wrap(err, "decode prometheus response failed")
	}
	if parsed.Status != "success" {
		return nil, gerror.Newf("prometheus query error: type=%s, error=%s", parsed.ErrorType, parsed.Error)
	}

	values := make(map[usageMetricKey]int64, len(parsed.Data.Result))
	for _, result := range parsed.Data.Result {
		consumerName := strings.TrimSpace(result.Metric["ai_consumer"])
		if consumerName == "" {
			continue
		}
		modelName := strings.TrimSpace(result.Metric["ai_model"])
		if modelName == "" {
			modelName = "unknown"
		}
		values[usageMetricKey{
			ConsumerName: consumerName,
			ModelName:    modelName,
		}] = parsePrometheusMetricValue(result.Value)
	}
	return values, nil
}

func parsePrometheusMetricValue(values []any) int64 {
	if len(values) < 2 {
		return 0
	}
	raw := strings.TrimSpace(fmt.Sprint(values[1]))
	if raw == "" || strings.EqualFold(raw, "nan") {
		return 0
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0
	}
	return int64(math.Round(parsed))
}

func truncateText(raw string, max int) string {
	if max <= 0 || len(raw) <= max {
		return raw
	}
	return raw[:max]
}

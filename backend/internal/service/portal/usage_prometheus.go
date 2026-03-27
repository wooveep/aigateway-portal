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
	usageMetricInputExpr           = "sum by(ai_consumer, ai_model) (increase(route_upstream_model_consumer_metric_input_token[%s]))"
	usageMetricOutputExpr          = "sum by(ai_consumer, ai_model) (increase(route_upstream_model_consumer_metric_output_token[%s]))"
	usageMetricRequestExpr         = "sum by(ai_consumer, ai_model) (increase(route_upstream_model_consumer_metric_llm_stream_duration_count[%s]))"
	usageMetricRequestFallbackExpr = "sum by(ai_consumer, ai_model) (increase(route_upstream_model_consumer_metric_llm_duration_count[%s]))"
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
	if requestMap == nil {
		requestMap = make(map[usageMetricKey]int64)
	}
	requestFallbackMap, fallbackErr := s.queryUsageMetric(ctx, fmt.Sprintf(usageMetricRequestFallbackExpr, rangeLiteral), to)
	if fallbackErr != nil {
		s.logf(ctx, "query request fallback metric failed: %v", fallbackErr)
	} else {
		for key, value := range requestFallbackMap {
			if value > requestMap[key] {
				requestMap[key] = value
			}
		}
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
	requestURLs := buildPrometheusQueryURLs(baseURL, query.Encode())

	client := s.httpClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	var lastErr error
	for idx, requestURL := range requestURLs {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			lastErr = gerror.Wrap(err, "build prometheus request failed")
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = gerror.Wrap(err, "query prometheus failed")
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = gerror.Wrap(readErr, "read prometheus response failed")
			continue
		}
		if resp.StatusCode >= http.StatusBadRequest {
			lastErr = gerror.Newf(
				"prometheus query failed: %s, url=%s, body=%s",
				resp.Status,
				requestURL,
				truncateText(string(body), 512),
			)
			// Some deployments expose Prometheus under "/prometheus".
			if resp.StatusCode == http.StatusNotFound && idx < len(requestURLs)-1 {
				continue
			}
			return nil, lastErr
		}

		var parsed prometheusQueryResponse
		if err = json.Unmarshal(body, &parsed); err != nil {
			lastErr = gerror.Wrap(err, "decode prometheus response failed")
			continue
		}
		if parsed.Status != "success" {
			lastErr = gerror.Newf("prometheus query error: type=%s, error=%s", parsed.ErrorType, parsed.Error)
			return nil, lastErr
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
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, gerror.New("prometheus query failed")
}

func buildPrometheusQueryURLs(baseURL string, encodedQuery string) []string {
	normalized := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if normalized == "" {
		return []string{}
	}

	urls := []string{
		normalized + "/api/v1/query?" + encodedQuery,
	}
	if !strings.HasSuffix(normalized, "/prometheus") {
		urls = append(urls, normalized+"/prometheus/api/v1/query?"+encodedQuery)
	}
	return urls
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

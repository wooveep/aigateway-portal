package portal

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"higress-portal-backend/internal/model"
)

const usageReadSyncMinInterval = 15 * time.Second

func (s *Service) StartUsageSync(ctx context.Context) {
	if !s.cfg.UsageSyncEnabled {
		return
	}
	if strings.TrimSpace(s.cfg.CorePrometheusURL) == "" {
		s.logf(ctx, "usage sync disabled: PORTAL_CORE_PROMETHEUS_URL is empty")
		return
	}
	if err := s.syncUsageOnce(ctx); err != nil {
		s.logf(ctx, "initial usage sync failed: %v", err)
	}

	ticker := time.NewTicker(s.cfg.UsageSyncInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.syncUsageOnce(ctx); err != nil {
					s.logf(ctx, "usage sync failed: %v", err)
				}
			}
		}
	}()
}

func (s *Service) syncUsageOnce(ctx context.Context) error {
	if !s.cfg.UsageSyncEnabled {
		return nil
	}

	now := time.Now()
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	stats, err := s.fetchUsageStatsFromCore(ctx, from, now)
	if err != nil {
		return gerror.Wrap(err, "fetch usage stats from core failed")
	}
	if len(stats) == 0 {
		return nil
	}

	ledgerStats, err := s.loadUsageStatsFromLedger(ctx, from, now)
	if err != nil {
		return gerror.Wrap(err, "load usage stats from ledger failed")
	}
	diffCount := s.logUsageReconcileDiff(ctx, stats, ledgerStats)
	if diffCount > 0 {
		s.logf(ctx, "usage reconcile found %d mismatched consumer/model pairs between Prometheus and billing ledger", diffCount)
	}
	return nil
}

func calculateCost(inputTokens int64, outputTokens int64, inputPrice float64, outputPrice float64) float64 {
	return (float64(inputTokens)/1000.0)*inputPrice + (float64(outputTokens)/1000.0)*outputPrice
}

func (s *Service) syncUsageForRead(ctx context.Context) {
	if !s.cfg.UsageSyncEnabled {
		return
	}
	if strings.TrimSpace(s.cfg.CorePrometheusURL) == "" {
		return
	}

	now := time.Now()
	s.usageReadSyncMu.Lock()
	if !s.usageReadSyncAt.IsZero() && now.Sub(s.usageReadSyncAt) < usageReadSyncMinInterval {
		s.usageReadSyncMu.Unlock()
		return
	}
	s.usageReadSyncAt = now
	s.usageReadSyncMu.Unlock()

	_ = s.syncUsageOnce(ctx)
}

func (s *Service) loadUsageStatsFromLedger(ctx context.Context, from time.Time, to time.Time) ([]model.ConsumerUsageStat, error) {
	records, err := s.db.GetAll(ctx, `
		SELECT
			consumer_name,
			model_id,
			COUNT(1) AS request_count,
			COALESCE(SUM(input_tokens), 0) AS input_tokens,
			COALESCE(SUM(output_tokens), 0) AS output_tokens
		FROM billing_usage_event
		WHERE request_status = 'success'
		  AND usage_status = 'parsed'
		  AND occurred_at >= ?
		  AND occurred_at < ?
		GROUP BY consumer_name, model_id`, from, to)
	if err != nil {
		return nil, err
	}
	items := make([]model.ConsumerUsageStat, 0, len(records))
	for _, record := range records {
		input := record["input_tokens"].Int64()
		output := record["output_tokens"].Int64()
		items = append(items, model.ConsumerUsageStat{
			ConsumerName: model.NormalizeUsername(record["consumer_name"].String()),
			ModelName:    strings.TrimSpace(record["model_id"].String()),
			RequestCount: record["request_count"].Int64(),
			InputTokens:  input,
			OutputTokens: output,
			TotalTokens:  input + output,
		})
	}
	return items, nil
}

func (s *Service) logUsageReconcileDiff(ctx context.Context, metrics []model.ConsumerUsageStat, ledger []model.ConsumerUsageStat) int {
	type usageKey struct {
		consumer string
		model    string
	}
	toMap := func(values []model.ConsumerUsageStat) map[usageKey]model.ConsumerUsageStat {
		index := make(map[usageKey]model.ConsumerUsageStat, len(values))
		for _, item := range values {
			key := usageKey{
				consumer: model.NormalizeUsername(item.ConsumerName),
				model:    strings.TrimSpace(item.ModelName),
			}
			index[key] = item
		}
		return index
	}

	metricMap := toMap(metrics)
	ledgerMap := toMap(ledger)
	diffCount := 0
	for key, metricItem := range metricMap {
		ledgerItem, ok := ledgerMap[key]
		if !ok {
			diffCount++
			s.logf(ctx, "usage reconcile missing ledger pair: consumer=%s model=%s metric_calls=%d metric_tokens=%d",
				key.consumer, key.model, metricItem.RequestCount, metricItem.TotalTokens)
			continue
		}
		if metricItem.RequestCount != ledgerItem.RequestCount ||
			metricItem.InputTokens != ledgerItem.InputTokens ||
			metricItem.OutputTokens != ledgerItem.OutputTokens {
			diffCount++
			s.logf(ctx,
				"usage reconcile mismatch: consumer=%s model=%s metric(calls=%d,input=%d,output=%d) ledger(calls=%d,input=%d,output=%d)",
				key.consumer, key.model,
				metricItem.RequestCount, metricItem.InputTokens, metricItem.OutputTokens,
				ledgerItem.RequestCount, ledgerItem.InputTokens, ledgerItem.OutputTokens,
			)
		}
	}
	for key, ledgerItem := range ledgerMap {
		if _, ok := metricMap[key]; ok {
			continue
		}
		diffCount++
		s.logf(ctx, "usage reconcile missing prometheus pair: consumer=%s model=%s ledger_calls=%d ledger_tokens=%d",
			key.consumer, key.model, ledgerItem.RequestCount, ledgerItem.TotalTokens)
	}
	return diffCount
}

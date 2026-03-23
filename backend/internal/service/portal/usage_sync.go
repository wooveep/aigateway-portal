package portal

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"higress-portal-backend/internal/model"
)

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

	return s.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		for _, item := range stats {
			consumerName := model.NormalizeUsername(item.ConsumerName)
			if consumerName == "" {
				continue
			}

			inputPrice, outputPrice, priceErr := s.getModelPrices(ctx, item.ModelName)
			if priceErr != nil {
				s.logf(ctx, "get model price failed: model=%s err=%v", item.ModelName, priceErr)
				inputPrice = 0
				outputPrice = 0
			}
			cost := calculateCost(item.InputTokens, item.OutputTokens, inputPrice, outputPrice)

			if _, txErr := tx.Exec(`
				INSERT INTO portal_usage_daily
				(billing_date, consumer_name, model_name, request_count, input_tokens, output_tokens, total_tokens, cost_amount, source_from, source_to)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON DUPLICATE KEY UPDATE
				request_count = VALUES(request_count),
				input_tokens = VALUES(input_tokens),
				output_tokens = VALUES(output_tokens),
				total_tokens = VALUES(total_tokens),
				cost_amount = VALUES(cost_amount),
				source_from = VALUES(source_from),
				source_to = VALUES(source_to)`,
				from.Format("2006-01-02"),
				consumerName,
				item.ModelName,
				item.RequestCount,
				item.InputTokens,
				item.OutputTokens,
				item.TotalTokens,
				cost,
				from,
				now,
			); txErr != nil {
				return gerror.Wrap(txErr, "upsert usage daily failed")
			}
		}
		return nil
	})
}

func (s *Service) getModelPrices(ctx context.Context, modelName string) (float64, float64, error) {
	record, err := s.db.GetOne(ctx, `
		SELECT input_token_price, output_token_price
		FROM portal_model_catalog
		WHERE name = ? OR model_id = ?
		LIMIT 1`, modelName, modelName)
	if err != nil {
		return 0, 0, gerror.Wrap(err, "query model prices failed")
	}
	if record.IsEmpty() {
		return 0, 0, nil
	}
	return record["input_token_price"].Float64(), record["output_token_price"].Float64(), nil
}

func calculateCost(inputTokens int64, outputTokens int64, inputPrice float64, outputPrice float64) float64 {
	return (float64(inputTokens)/1000.0)*inputPrice + (float64(outputTokens)/1000.0)*outputPrice
}

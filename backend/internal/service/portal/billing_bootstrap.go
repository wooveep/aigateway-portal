package portal

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

type billingBackfillSummary struct {
	LegacyModelCount              int64
	MatchedBillingModelCount      int64
	LegacyPricedModelCount        int64
	MatchedBillingPriceModelCount int64
	LegacyRechargeCount           int64
	LedgerRechargeCount           int64
	LegacyRechargeTotalMicroYuan  int64
	LedgerRechargeTotalMicroYuan  int64
	LegacyUsageCount              int64
	LedgerUsageCount              int64
	LegacyUsageTotalMicroYuan     int64
	LedgerUsageTotalMicroYuan     int64
	LedgerNetTotalMicroYuan       int64
	WalletNetTotalMicroYuan       int64
}

func (s *Service) collectBillingBackfillSummary(ctx context.Context) (billingBackfillSummary, error) {
	var (
		summary billingBackfillSummary
		err     error
	)

	if summary.LegacyModelCount, err = s.queryInt64(ctx, `
		SELECT COUNT(1)
		FROM portal_model_catalog
		WHERE status = 'active'`); err != nil {
		return summary, gerror.Wrap(err, "query legacy model count failed")
	}
	if summary.MatchedBillingModelCount, err = s.queryInt64(ctx, `
		SELECT COUNT(1)
		FROM portal_model_catalog p
		INNER JOIN billing_model_catalog b
			ON b.model_id = p.model_id
		WHERE p.status = 'active'`); err != nil {
		return summary, gerror.Wrap(err, "query matched billing model count failed")
	}
	if summary.LegacyPricedModelCount, err = s.queryInt64(ctx, `
		SELECT COUNT(1)
		FROM portal_model_catalog
		WHERE status = 'active'
		  AND (input_token_price > 0 OR output_token_price > 0)`); err != nil {
		return summary, gerror.Wrap(err, "query legacy priced model count failed")
	}
	if summary.MatchedBillingPriceModelCount, err = s.queryInt64(ctx, `
		SELECT COUNT(DISTINCT p.model_id)
		FROM portal_model_catalog p
		INNER JOIN billing_model_price_version b
			ON b.model_id = p.model_id
		WHERE p.status = 'active'
		  AND (p.input_token_price > 0 OR p.output_token_price > 0)
		  AND b.status = 'active'
		  AND b.effective_to IS NULL`); err != nil {
		return summary, gerror.Wrap(err, "query matched billing price model count failed")
	}
	if summary.LegacyRechargeCount, err = s.queryInt64(ctx, `
		SELECT COUNT(1)
		FROM portal_recharge_order
		WHERE status = 'success'
		  AND LOWER(TRIM(consumer_name)) <> 'administrator'`); err != nil {
		return summary, gerror.Wrap(err, "query legacy recharge count failed")
	}
	if summary.LedgerRechargeCount, err = s.queryInt64(ctx, `
		SELECT COUNT(1)
		FROM billing_transaction
		WHERE source_type = 'portal_recharge_order'
		  AND tx_type = 'recharge'
		  AND LOWER(TRIM(consumer_name)) <> 'administrator'`); err != nil {
		return summary, gerror.Wrap(err, "query ledger recharge count failed")
	}
	if summary.LegacyRechargeTotalMicroYuan, err = s.queryInt64(ctx, `
		SELECT COALESCE(SUM(ROUND(amount * 1000000)), 0)
		FROM portal_recharge_order
		WHERE status = 'success'
		  AND LOWER(TRIM(consumer_name)) <> 'administrator'`); err != nil {
		return summary, gerror.Wrap(err, "query legacy recharge total failed")
	}
	if summary.LedgerRechargeTotalMicroYuan, err = s.queryInt64(ctx, `
		SELECT COALESCE(SUM(amount_micro_yuan), 0)
		FROM billing_transaction
		WHERE source_type = 'portal_recharge_order'
		  AND tx_type = 'recharge'
		  AND LOWER(TRIM(consumer_name)) <> 'administrator'`); err != nil {
		return summary, gerror.Wrap(err, "query ledger recharge total failed")
	}
	if summary.LegacyUsageCount, err = s.queryInt64(ctx, `
		SELECT COUNT(1)
		FROM portal_usage_daily
		WHERE LOWER(TRIM(consumer_name)) <> 'administrator'`); err != nil {
		return summary, gerror.Wrap(err, "query legacy usage count failed")
	}
	if summary.LedgerUsageCount, err = s.queryInt64(ctx, `
		SELECT COUNT(1)
		FROM billing_transaction
		WHERE source_type = 'portal_usage_daily'
		  AND tx_type = 'consume'
		  AND LOWER(TRIM(consumer_name)) <> 'administrator'`); err != nil {
		return summary, gerror.Wrap(err, "query ledger usage count failed")
	}
	if summary.LegacyUsageTotalMicroYuan, err = s.queryInt64(ctx, `
		SELECT COALESCE(SUM(ROUND(cost_amount * 1000000)), 0)
		FROM portal_usage_daily
		WHERE LOWER(TRIM(consumer_name)) <> 'administrator'`); err != nil {
		return summary, gerror.Wrap(err, "query legacy usage total failed")
	}
	if summary.LedgerUsageTotalMicroYuan, err = s.queryInt64(ctx, `
		SELECT COALESCE(SUM(0 - amount_micro_yuan), 0)
		FROM billing_transaction
		WHERE source_type = 'portal_usage_daily'
		  AND tx_type = 'consume'
		  AND LOWER(TRIM(consumer_name)) <> 'administrator'`); err != nil {
		return summary, gerror.Wrap(err, "query ledger usage total failed")
	}
	if summary.LedgerNetTotalMicroYuan, err = s.queryInt64(ctx, `
		SELECT COALESCE(SUM(amount_micro_yuan), 0)
		FROM billing_transaction
		WHERE LOWER(TRIM(consumer_name)) <> 'administrator'`); err != nil {
		return summary, gerror.Wrap(err, "query ledger net total failed")
	}
	if summary.WalletNetTotalMicroYuan, err = s.queryInt64(ctx, `
		SELECT COALESCE(SUM(available_micro_yuan), 0)
		FROM billing_wallet
		WHERE LOWER(TRIM(consumer_name)) <> 'administrator'`); err != nil {
		return summary, gerror.Wrap(err, "query wallet net total failed")
	}
	return summary, nil
}

func (s *Service) queryInt64(ctx context.Context, sql string, args ...any) (int64, error) {
	value, err := s.db.GetValue(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	return value.Int64(), nil
}

func validateBillingBackfillSummary(summary billingBackfillSummary) error {
	var mismatches []string

	if summary.LegacyModelCount != summary.MatchedBillingModelCount {
		mismatches = append(mismatches, fmt.Sprintf("legacy models=%d matched billing models=%d",
			summary.LegacyModelCount, summary.MatchedBillingModelCount))
	}
	if summary.LegacyRechargeCount != summary.LedgerRechargeCount {
		mismatches = append(mismatches, fmt.Sprintf("legacy recharge count=%d ledger recharge count=%d",
			summary.LegacyRechargeCount, summary.LedgerRechargeCount))
	}
	if summary.LegacyRechargeTotalMicroYuan != summary.LedgerRechargeTotalMicroYuan {
		mismatches = append(mismatches, fmt.Sprintf("legacy recharge total=%d ledger recharge total=%d",
			summary.LegacyRechargeTotalMicroYuan, summary.LedgerRechargeTotalMicroYuan))
	}
	if summary.LegacyUsageCount != summary.LedgerUsageCount {
		mismatches = append(mismatches, fmt.Sprintf("legacy usage count=%d ledger usage count=%d",
			summary.LegacyUsageCount, summary.LedgerUsageCount))
	}
	if summary.LegacyUsageTotalMicroYuan != summary.LedgerUsageTotalMicroYuan {
		mismatches = append(mismatches, fmt.Sprintf("legacy usage total=%d ledger usage total=%d",
			summary.LegacyUsageTotalMicroYuan, summary.LedgerUsageTotalMicroYuan))
	}
	if summary.LedgerNetTotalMicroYuan != summary.WalletNetTotalMicroYuan {
		mismatches = append(mismatches, fmt.Sprintf("ledger net total=%d wallet net total=%d",
			summary.LedgerNetTotalMicroYuan, summary.WalletNetTotalMicroYuan))
	}
	if len(mismatches) > 0 {
		return gerror.New("billing backfill validation failed: " + strings.Join(mismatches, "; "))
	}
	return nil
}

func (s *Service) logBillingBackfillSummary(ctx context.Context, summary billingBackfillSummary) {
	s.logf(ctx,
		"billing backfill summary: models=%d/%d priced=%d/%d recharge=%d/%d recharge_total=%d/%d usage=%d/%d usage_total=%d/%d ledger_net=%d wallet_net=%d",
		summary.MatchedBillingModelCount,
		summary.LegacyModelCount,
		summary.MatchedBillingPriceModelCount,
		summary.LegacyPricedModelCount,
		summary.LedgerRechargeCount,
		summary.LegacyRechargeCount,
		summary.LedgerRechargeTotalMicroYuan,
		summary.LegacyRechargeTotalMicroYuan,
		summary.LedgerUsageCount,
		summary.LegacyUsageCount,
		summary.LedgerUsageTotalMicroYuan,
		summary.LegacyUsageTotalMicroYuan,
		summary.LedgerNetTotalMicroYuan,
		summary.WalletNetTotalMicroYuan,
	)
}

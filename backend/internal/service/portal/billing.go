package portal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"higress-portal-backend/internal/apperr"
	"higress-portal-backend/internal/model"
	"higress-portal-backend/internal/model/do"
)

func (s *Service) GetBillingOverview(ctx context.Context, consumerName string) (model.BillingOverview, error) {
	totalRecharge, err := s.db.GetValue(ctx, `
		SELECT COALESCE(SUM(amount_micro_yuan), 0)
		FROM billing_transaction
		WHERE consumer_name = ? AND tx_type = 'recharge'`, consumerName)
	if err != nil {
		return model.BillingOverview{}, gerror.Wrap(err, "query recharge failed")
	}

	totalConsumption, err := s.db.GetValue(ctx, `
		SELECT COALESCE(SUM(0 - amount_micro_yuan), 0)
		FROM billing_transaction
		WHERE consumer_name = ?
		  AND tx_type IN ('consume', 'reconcile')
		  AND amount_micro_yuan < 0`, consumerName)
	if err != nil {
		return model.BillingOverview{}, gerror.Wrap(err, "query consumption failed")
	}
	currentBalance, err := s.db.GetValue(ctx, `
		SELECT COALESCE(available_micro_yuan, 0)
		FROM billing_wallet
		WHERE consumer_name = ?`, consumerName)
	if err != nil {
		return model.BillingOverview{}, gerror.Wrap(err, "query balance failed")
	}

	recharge := totalRecharge.Int64()
	consumption := totalConsumption.Int64()
	balance := currentBalance.Int64()

	return model.BillingOverview{
		Balance:          microYuanToText(balance),
		TotalRecharge:    microYuanToText(recharge),
		TotalConsumption: microYuanToText(consumption),
	}, nil
}

func (s *Service) ListConsumptions(ctx context.Context, consumerName string) ([]model.ConsumptionRecord, error) {
	records, err := s.db.GetAll(ctx, `
		SELECT tx_id, model_id, input_tokens, output_tokens, amount_micro_yuan, occurred_at
		FROM billing_transaction
		WHERE consumer_name = ?
		  AND tx_type IN ('consume', 'reconcile')
		ORDER BY occurred_at DESC, id DESC`, consumerName)
	if err != nil {
		return nil, gerror.Wrap(err, "query consumptions failed")
	}

	items := make([]model.ConsumptionRecord, 0, len(records))
	for _, record := range records {
		inputTokens := record["input_tokens"].Int64()
		outputTokens := record["output_tokens"].Int64()
		items = append(items, model.ConsumptionRecord{
			ID:        record["tx_id"].String(),
			Model:     record["model_id"].String(),
			Tokens:    inputTokens + outputTokens,
			Cost:      microYuanToRMB(0 - record["amount_micro_yuan"].Int64()),
			CreatedAt: model.NowText(record["occurred_at"].Time()),
		})
	}
	return items, nil
}

func (s *Service) ListRecharges(ctx context.Context, consumerName string) ([]model.RechargeRecord, error) {
	records, err := s.db.GetAll(ctx, `
		SELECT order_id, amount, channel, status, created_at
		FROM portal_recharge_order
		WHERE consumer_name = ?
		ORDER BY id DESC`, consumerName)
	if err != nil {
		return nil, gerror.Wrap(err, "query recharges failed")
	}

	items := make([]model.RechargeRecord, 0, len(records))
	for _, record := range records {
		items = append(items, model.RechargeRecord{
			ID:        record["order_id"].String(),
			Amount:    record["amount"].Float64(),
			Channel:   record["channel"].String(),
			Status:    record["status"].String(),
			CreatedAt: model.NowText(record["created_at"].Time()),
		})
	}
	return items, nil
}

func (s *Service) CreateRecharge(ctx context.Context, consumerName string, req model.CreateRechargeRequest) (model.RechargeRecord, error) {
	normalizedConsumer := model.NormalizeUsername(consumerName)
	if req.Amount <= 0 || strings.TrimSpace(req.Channel) == "" {
		return model.RechargeRecord{}, apperr.New(400, "amount and channel are required")
	}
	if strings.EqualFold(normalizedConsumer, builtinAdministratorUser) {
		return model.RechargeRecord{}, apperr.New(403, "administrator wallet is not supported")
	}

	now := time.Now()
	orderID := fmt.Sprintf("RC%d", time.Now().UnixMilli())
	amountMicroYuan := rmbToMicroYuan(req.Amount)
	err := s.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, txErr := tx.Model("portal_recharge_order").Ctx(ctx).Data(do.PortalRechargeOrder{
			OrderId:      orderID,
			ConsumerName: normalizedConsumer,
			Amount:       req.Amount,
			Channel:      strings.TrimSpace(req.Channel),
			Status:       "success",
		}).Insert(); txErr != nil {
			return gerror.Wrap(txErr, "insert recharge order failed")
		}
		if _, txErr := tx.Exec(`
			INSERT INTO billing_transaction
			(tx_id, consumer_name, tx_type, amount_micro_yuan, currency, source_type, source_id, occurred_at, created_at)
			VALUES (?, ?, 'recharge', ?, 'CNY', 'portal_recharge_order', ?, ?, ?)`,
			"r"+sha256Hex("portal_recharge_order:" + orderID)[:32],
			normalizedConsumer,
			amountMicroYuan,
			orderID,
			now,
			now,
		); txErr != nil {
			return gerror.Wrap(txErr, "insert billing recharge transaction failed")
		}
		if _, txErr := tx.Exec(`
			INSERT INTO billing_wallet
			(consumer_name, currency, available_micro_yuan, version)
			VALUES (?, 'CNY', ?, 1)
			ON DUPLICATE KEY UPDATE
				available_micro_yuan = available_micro_yuan + VALUES(available_micro_yuan),
				version = version + 1`,
			normalizedConsumer,
			amountMicroYuan,
		); txErr != nil {
			return gerror.Wrap(txErr, "update billing wallet failed")
		}
		return nil
	})
	if err != nil {
		return model.RechargeRecord{}, gerror.Wrap(err, "create recharge failed")
	}
	if err = s.syncConsumerBalanceToRedis(ctx, normalizedConsumer); err != nil {
		s.logf(ctx, "sync recharge balance to redis failed: consumer=%s err=%v", normalizedConsumer, err)
	}

	return model.RechargeRecord{
		ID:        orderID,
		Amount:    req.Amount,
		Channel:   strings.TrimSpace(req.Channel),
		Status:    "success",
		CreatedAt: model.NowText(now),
	}, nil
}

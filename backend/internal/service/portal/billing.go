package portal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"higress-portal-backend/internal/apperr"
	"higress-portal-backend/internal/model"
	"higress-portal-backend/internal/model/do"
)

func (s *Service) GetBillingOverview(ctx context.Context, consumerName string) (model.BillingOverview, error) {
	totalRecharge, err := s.db.GetValue(ctx, `
		SELECT COALESCE(SUM(amount), 0)
		FROM portal_recharge_order
		WHERE consumer_name = ? AND status = 'success'`, consumerName)
	if err != nil {
		return model.BillingOverview{}, gerror.Wrap(err, "query recharge failed")
	}

	totalConsumption, err := s.db.GetValue(ctx, `
		SELECT COALESCE(SUM(cost_amount), 0)
		FROM portal_usage_daily
		WHERE consumer_name = ?`, consumerName)
	if err != nil {
		return model.BillingOverview{}, gerror.Wrap(err, "query consumption failed")
	}

	recharge := totalRecharge.Float64()
	consumption := totalConsumption.Float64()
	balance := recharge - consumption

	return model.BillingOverview{
		Balance:          fmt.Sprintf("%.2f", balance),
		TotalRecharge:    fmt.Sprintf("%.2f", recharge),
		TotalConsumption: fmt.Sprintf("%.2f", consumption),
	}, nil
}

func (s *Service) ListConsumptions(ctx context.Context, consumerName string) ([]model.ConsumptionRecord, error) {
	records, err := s.db.GetAll(ctx, `
		SELECT id, model_name, total_tokens, cost_amount, updated_at
		FROM portal_usage_daily
		WHERE consumer_name = ?
		ORDER BY billing_date DESC, id DESC`, consumerName)
	if err != nil {
		return nil, gerror.Wrap(err, "query consumptions failed")
	}

	items := make([]model.ConsumptionRecord, 0, len(records))
	for _, record := range records {
		items = append(items, model.ConsumptionRecord{
			ID:        fmt.Sprintf("CS%d", record["id"].Int64()),
			Model:     record["model_name"].String(),
			Tokens:    record["total_tokens"].Int64(),
			Cost:      record["cost_amount"].Float64(),
			CreatedAt: model.NowText(record["updated_at"].Time()),
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
	if req.Amount <= 0 || strings.TrimSpace(req.Channel) == "" {
		return model.RechargeRecord{}, apperr.New(400, "amount and channel are required")
	}

	orderID := fmt.Sprintf("RC%d", time.Now().UnixMilli())
	if _, err := s.db.Model("portal_recharge_order").Ctx(ctx).Data(do.PortalRechargeOrder{
		OrderId:      orderID,
		ConsumerName: consumerName,
		Amount:       req.Amount,
		Channel:      strings.TrimSpace(req.Channel),
		Status:       "success",
	}).Insert(); err != nil {
		return model.RechargeRecord{}, gerror.Wrap(err, "create recharge failed")
	}

	return model.RechargeRecord{
		ID:        orderID,
		Amount:    req.Amount,
		Channel:   strings.TrimSpace(req.Channel),
		Status:    "success",
		CreatedAt: model.NowText(time.Now()),
	}, nil
}

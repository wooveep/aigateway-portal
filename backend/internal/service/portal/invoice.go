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

func (s *Service) GetInvoiceProfile(ctx context.Context, consumerName string) (model.InvoiceProfile, error) {
	record, err := s.db.GetOne(ctx, `
		SELECT company_name, tax_no, address, bank_account, receiver, email
		FROM portal_invoice_profile
		WHERE consumer_name = ?`, consumerName)
	if err != nil {
		return model.InvoiceProfile{}, gerror.Wrap(err, "query invoice profile failed")
	}
	if record.IsEmpty() {
		return model.InvoiceProfile{}, nil
	}
	return model.InvoiceProfile{
		CompanyName: record["company_name"].String(),
		TaxNo:       record["tax_no"].String(),
		Address:     record["address"].String(),
		BankAccount: record["bank_account"].String(),
		Receiver:    record["receiver"].String(),
		Email:       record["email"].String(),
	}, nil
}

func (s *Service) UpdateInvoiceProfile(ctx context.Context, consumerName string, req model.InvoiceProfile) (model.InvoiceProfile, error) {
	_, err := s.db.Exec(ctx, `
		INSERT INTO portal_invoice_profile
		(consumer_name, company_name, tax_no, address, bank_account, receiver, email)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		company_name = VALUES(company_name),
		tax_no = VALUES(tax_no),
		address = VALUES(address),
		bank_account = VALUES(bank_account),
		receiver = VALUES(receiver),
		email = VALUES(email)`,
		consumerName,
		strings.TrimSpace(req.CompanyName),
		strings.TrimSpace(req.TaxNo),
		strings.TrimSpace(req.Address),
		strings.TrimSpace(req.BankAccount),
		strings.TrimSpace(req.Receiver),
		strings.TrimSpace(req.Email),
	)
	if err != nil {
		return model.InvoiceProfile{}, gerror.Wrap(err, "update invoice profile failed")
	}
	return model.InvoiceProfile{
		CompanyName: strings.TrimSpace(req.CompanyName),
		TaxNo:       strings.TrimSpace(req.TaxNo),
		Address:     strings.TrimSpace(req.Address),
		BankAccount: strings.TrimSpace(req.BankAccount),
		Receiver:    strings.TrimSpace(req.Receiver),
		Email:       strings.TrimSpace(req.Email),
	}, nil
}

func (s *Service) ListInvoiceRecords(ctx context.Context, consumerName string) ([]model.InvoiceRecord, error) {
	records, err := s.db.GetAll(ctx, `
		SELECT invoice_id, title, tax_no, amount, status, remark, created_at
		FROM portal_invoice_record
		WHERE consumer_name = ?
		ORDER BY id DESC`, consumerName)
	if err != nil {
		return nil, gerror.Wrap(err, "query invoice records failed")
	}

	items := make([]model.InvoiceRecord, 0, len(records))
	for _, record := range records {
		items = append(items, model.InvoiceRecord{
			ID:        record["invoice_id"].String(),
			Title:     record["title"].String(),
			TaxNo:     record["tax_no"].String(),
			Amount:    record["amount"].Float64(),
			Status:    record["status"].String(),
			Remark:    record["remark"].String(),
			CreatedAt: model.NowText(record["created_at"].Time()),
		})
	}
	return items, nil
}

func (s *Service) CreateInvoice(ctx context.Context, consumerName string, req model.CreateInvoiceRequest) (model.InvoiceRecord, error) {
	if req.Amount <= 0 {
		return model.InvoiceRecord{}, apperr.New(400, "amount must be greater than 0")
	}

	profileRecord, err := s.db.GetOne(ctx, `
		SELECT company_name, tax_no
		FROM portal_invoice_profile
		WHERE consumer_name = ?`, consumerName)
	if err != nil {
		return model.InvoiceRecord{}, gerror.Wrap(err, "query invoice profile failed")
	}
	if profileRecord.IsEmpty() {
		return model.InvoiceRecord{}, apperr.New(400, "please set invoice profile first")
	}

	title := profileRecord["company_name"].String()
	taxNo := profileRecord["tax_no"].String()
	invoiceID := fmt.Sprintf("INV%d", time.Now().UnixMilli())

	if _, err := s.db.Model("portal_invoice_record").Ctx(ctx).Data(do.PortalInvoiceRecord{
		InvoiceId:    invoiceID,
		ConsumerName: consumerName,
		Title:        title,
		TaxNo:        taxNo,
		Amount:       req.Amount,
		Status:       "pending",
		Remark:       strings.TrimSpace(req.Remark),
	}).Insert(); err != nil {
		return model.InvoiceRecord{}, gerror.Wrap(err, "create invoice failed")
	}

	return model.InvoiceRecord{
		ID:        invoiceID,
		Title:     title,
		TaxNo:     taxNo,
		Amount:    req.Amount,
		Status:    "pending",
		CreatedAt: model.NowText(time.Now()),
		Remark:    strings.TrimSpace(req.Remark),
	}, nil
}

// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PortalInvoiceRecordDao is the data access object for the table portal_invoice_record.
type PortalInvoiceRecordDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  PortalInvoiceRecordColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// PortalInvoiceRecordColumns defines and stores column names for the table portal_invoice_record.
type PortalInvoiceRecordColumns struct {
	Id           string //
	InvoiceId    string //
	ConsumerName string //
	Title        string //
	TaxNo        string //
	Amount       string //
	Status       string //
	Remark       string //
	CreatedAt    string //
}

// portalInvoiceRecordColumns holds the columns for the table portal_invoice_record.
var portalInvoiceRecordColumns = PortalInvoiceRecordColumns{
	Id:           "id",
	InvoiceId:    "invoice_id",
	ConsumerName: "consumer_name",
	Title:        "title",
	TaxNo:        "tax_no",
	Amount:       "amount",
	Status:       "status",
	Remark:       "remark",
	CreatedAt:    "created_at",
}

// NewPortalInvoiceRecordDao creates and returns a new DAO object for table data access.
func NewPortalInvoiceRecordDao(handlers ...gdb.ModelHandler) *PortalInvoiceRecordDao {
	return &PortalInvoiceRecordDao{
		group:    "default",
		table:    "portal_invoice_record",
		columns:  portalInvoiceRecordColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PortalInvoiceRecordDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PortalInvoiceRecordDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PortalInvoiceRecordDao) Columns() PortalInvoiceRecordColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PortalInvoiceRecordDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PortalInvoiceRecordDao) Ctx(ctx context.Context) *gdb.Model {
	model := dao.DB().Model(dao.table)
	for _, handler := range dao.handlers {
		model = handler(model)
	}
	return model.Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
// It rolls back the transaction and returns the error if function f returns a non-nil error.
// It commits the transaction and returns nil if function f returns nil.
//
// Note: Do not commit or roll back the transaction in function f,
// as it is automatically handled by this function.
func (dao *PortalInvoiceRecordDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}

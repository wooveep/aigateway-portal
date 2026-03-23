// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PortalInvoiceProfileDao is the data access object for the table portal_invoice_profile.
type PortalInvoiceProfileDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  PortalInvoiceProfileColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// PortalInvoiceProfileColumns defines and stores column names for the table portal_invoice_profile.
type PortalInvoiceProfileColumns struct {
	Id           string //
	ConsumerName string //
	CompanyName  string //
	TaxNo        string //
	Address      string //
	BankAccount  string //
	Receiver     string //
	Email        string //
	CreatedAt    string //
	UpdatedAt    string //
}

// portalInvoiceProfileColumns holds the columns for the table portal_invoice_profile.
var portalInvoiceProfileColumns = PortalInvoiceProfileColumns{
	Id:           "id",
	ConsumerName: "consumer_name",
	CompanyName:  "company_name",
	TaxNo:        "tax_no",
	Address:      "address",
	BankAccount:  "bank_account",
	Receiver:     "receiver",
	Email:        "email",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewPortalInvoiceProfileDao creates and returns a new DAO object for table data access.
func NewPortalInvoiceProfileDao(handlers ...gdb.ModelHandler) *PortalInvoiceProfileDao {
	return &PortalInvoiceProfileDao{
		group:    "default",
		table:    "portal_invoice_profile",
		columns:  portalInvoiceProfileColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PortalInvoiceProfileDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PortalInvoiceProfileDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PortalInvoiceProfileDao) Columns() PortalInvoiceProfileColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PortalInvoiceProfileDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PortalInvoiceProfileDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PortalInvoiceProfileDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}

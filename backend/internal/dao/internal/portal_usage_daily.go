// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PortalUsageDailyDao is the data access object for the table portal_usage_daily.
type PortalUsageDailyDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  PortalUsageDailyColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// PortalUsageDailyColumns defines and stores column names for the table portal_usage_daily.
type PortalUsageDailyColumns struct {
	Id           string //
	BillingDate  string //
	ConsumerName string //
	ModelName    string //
	RequestCount string //
	InputTokens  string //
	OutputTokens string //
	TotalTokens  string //
	CostAmount   string //
	SourceFrom   string //
	SourceTo     string //
	CreatedAt    string //
	UpdatedAt    string //
}

// portalUsageDailyColumns holds the columns for the table portal_usage_daily.
var portalUsageDailyColumns = PortalUsageDailyColumns{
	Id:           "id",
	BillingDate:  "billing_date",
	ConsumerName: "consumer_name",
	ModelName:    "model_name",
	RequestCount: "request_count",
	InputTokens:  "input_tokens",
	OutputTokens: "output_tokens",
	TotalTokens:  "total_tokens",
	CostAmount:   "cost_amount",
	SourceFrom:   "source_from",
	SourceTo:     "source_to",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewPortalUsageDailyDao creates and returns a new DAO object for table data access.
func NewPortalUsageDailyDao(handlers ...gdb.ModelHandler) *PortalUsageDailyDao {
	return &PortalUsageDailyDao{
		group:    "default",
		table:    "portal_usage_daily",
		columns:  portalUsageDailyColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PortalUsageDailyDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PortalUsageDailyDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PortalUsageDailyDao) Columns() PortalUsageDailyColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PortalUsageDailyDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PortalUsageDailyDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PortalUsageDailyDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}

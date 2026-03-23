// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PortalRechargeOrderDao is the data access object for the table portal_recharge_order.
type PortalRechargeOrderDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  PortalRechargeOrderColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// PortalRechargeOrderColumns defines and stores column names for the table portal_recharge_order.
type PortalRechargeOrderColumns struct {
	Id           string //
	OrderId      string //
	ConsumerName string //
	Amount       string //
	Channel      string //
	Status       string //
	CreatedAt    string //
}

// portalRechargeOrderColumns holds the columns for the table portal_recharge_order.
var portalRechargeOrderColumns = PortalRechargeOrderColumns{
	Id:           "id",
	OrderId:      "order_id",
	ConsumerName: "consumer_name",
	Amount:       "amount",
	Channel:      "channel",
	Status:       "status",
	CreatedAt:    "created_at",
}

// NewPortalRechargeOrderDao creates and returns a new DAO object for table data access.
func NewPortalRechargeOrderDao(handlers ...gdb.ModelHandler) *PortalRechargeOrderDao {
	return &PortalRechargeOrderDao{
		group:    "default",
		table:    "portal_recharge_order",
		columns:  portalRechargeOrderColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PortalRechargeOrderDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PortalRechargeOrderDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PortalRechargeOrderDao) Columns() PortalRechargeOrderColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PortalRechargeOrderDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PortalRechargeOrderDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PortalRechargeOrderDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}

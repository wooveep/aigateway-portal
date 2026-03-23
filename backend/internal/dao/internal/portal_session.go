// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PortalSessionDao is the data access object for the table portal_session.
type PortalSessionDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  PortalSessionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// PortalSessionColumns defines and stores column names for the table portal_session.
type PortalSessionColumns struct {
	SessionToken string //
	ConsumerName string //
	ExpiresAt    string //
	CreatedAt    string //
	LastSeenAt   string //
}

// portalSessionColumns holds the columns for the table portal_session.
var portalSessionColumns = PortalSessionColumns{
	SessionToken: "session_token",
	ConsumerName: "consumer_name",
	ExpiresAt:    "expires_at",
	CreatedAt:    "created_at",
	LastSeenAt:   "last_seen_at",
}

// NewPortalSessionDao creates and returns a new DAO object for table data access.
func NewPortalSessionDao(handlers ...gdb.ModelHandler) *PortalSessionDao {
	return &PortalSessionDao{
		group:    "default",
		table:    "portal_session",
		columns:  portalSessionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PortalSessionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PortalSessionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PortalSessionDao) Columns() PortalSessionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PortalSessionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PortalSessionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PortalSessionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}

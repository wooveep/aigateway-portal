// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PortalUserDao is the data access object for the table portal_user.
type PortalUserDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  PortalUserColumns  // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// PortalUserColumns defines and stores column names for the table portal_user.
type PortalUserColumns struct {
	Id           string //
	ConsumerName string //
	DisplayName  string //
	Email        string //
	Department   string //
	PasswordHash string //
	Status       string //
	Source       string //
	LastLoginAt  string //
	CreatedAt    string //
	UpdatedAt    string //
}

// portalUserColumns holds the columns for the table portal_user.
var portalUserColumns = PortalUserColumns{
	Id:           "id",
	ConsumerName: "consumer_name",
	DisplayName:  "display_name",
	Email:        "email",
	Department:   "department",
	PasswordHash: "password_hash",
	Status:       "status",
	Source:       "source",
	LastLoginAt:  "last_login_at",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewPortalUserDao creates and returns a new DAO object for table data access.
func NewPortalUserDao(handlers ...gdb.ModelHandler) *PortalUserDao {
	return &PortalUserDao{
		group:    "default",
		table:    "portal_user",
		columns:  portalUserColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PortalUserDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PortalUserDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PortalUserDao) Columns() PortalUserColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PortalUserDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PortalUserDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PortalUserDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}

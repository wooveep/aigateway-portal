// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PortalApiKeyDao is the data access object for the table portal_api_key.
type PortalApiKeyDao struct {
	table    string              // table is the underlying table name of the DAO.
	group    string              // group is the database configuration group name of the current DAO.
	columns  PortalApiKeyColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler  // handlers for customized model modification.
}

// PortalApiKeyColumns defines and stores column names for the table portal_api_key.
type PortalApiKeyColumns struct {
	Id           string //
	KeyId        string //
	ConsumerName string //
	Name         string //
	KeyMasked    string //
	KeyHash      string //
	RawKey       string //
	Status       string //
	TotalCalls   string //
	LastUsedAt   string //
	CreatedAt    string //
	UpdatedAt    string //
}

// portalApiKeyColumns holds the columns for the table portal_api_key.
var portalApiKeyColumns = PortalApiKeyColumns{
	Id:           "id",
	KeyId:        "key_id",
	ConsumerName: "consumer_name",
	Name:         "name",
	KeyMasked:    "key_masked",
	KeyHash:      "key_hash",
	RawKey:       "raw_key",
	Status:       "status",
	TotalCalls:   "total_calls",
	LastUsedAt:   "last_used_at",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewPortalApiKeyDao creates and returns a new DAO object for table data access.
func NewPortalApiKeyDao(handlers ...gdb.ModelHandler) *PortalApiKeyDao {
	return &PortalApiKeyDao{
		group:    "default",
		table:    "portal_api_key",
		columns:  portalApiKeyColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PortalApiKeyDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PortalApiKeyDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PortalApiKeyDao) Columns() PortalApiKeyColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PortalApiKeyDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PortalApiKeyDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PortalApiKeyDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}

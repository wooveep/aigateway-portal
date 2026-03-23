// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PortalModelCatalogDao is the data access object for the table portal_model_catalog.
type PortalModelCatalogDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  PortalModelCatalogColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// PortalModelCatalogColumns defines and stores column names for the table portal_model_catalog.
type PortalModelCatalogColumns struct {
	Id               string //
	ModelId          string //
	Name             string //
	Vendor           string //
	Capability       string //
	InputTokenPrice  string //
	OutputTokenPrice string //
	Endpoint         string //
	Sdk              string //
	Summary          string //
	Status           string //
	CreatedAt        string //
	UpdatedAt        string //
}

// portalModelCatalogColumns holds the columns for the table portal_model_catalog.
var portalModelCatalogColumns = PortalModelCatalogColumns{
	Id:               "id",
	ModelId:          "model_id",
	Name:             "name",
	Vendor:           "vendor",
	Capability:       "capability",
	InputTokenPrice:  "input_token_price",
	OutputTokenPrice: "output_token_price",
	Endpoint:         "endpoint",
	Sdk:              "sdk",
	Summary:          "summary",
	Status:           "status",
	CreatedAt:        "created_at",
	UpdatedAt:        "updated_at",
}

// NewPortalModelCatalogDao creates and returns a new DAO object for table data access.
func NewPortalModelCatalogDao(handlers ...gdb.ModelHandler) *PortalModelCatalogDao {
	return &PortalModelCatalogDao{
		group:    "default",
		table:    "portal_model_catalog",
		columns:  portalModelCatalogColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PortalModelCatalogDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PortalModelCatalogDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PortalModelCatalogDao) Columns() PortalModelCatalogColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PortalModelCatalogDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PortalModelCatalogDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PortalModelCatalogDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}

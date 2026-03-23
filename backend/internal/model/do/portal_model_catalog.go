// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// PortalModelCatalog is the golang structure of table portal_model_catalog for DAO operations like Where/Data.
type PortalModelCatalog struct {
	g.Meta           `orm:"table:portal_model_catalog, do:true"`
	Id               any         //
	ModelId          any         //
	Name             any         //
	Vendor           any         //
	Capability       any         //
	InputTokenPrice  any         //
	OutputTokenPrice any         //
	Endpoint         any         //
	Sdk              any         //
	Summary          any         //
	Status           any         //
	CreatedAt        *gtime.Time //
	UpdatedAt        *gtime.Time //
}

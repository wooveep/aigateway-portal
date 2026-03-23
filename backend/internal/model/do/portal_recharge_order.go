// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// PortalRechargeOrder is the golang structure of table portal_recharge_order for DAO operations like Where/Data.
type PortalRechargeOrder struct {
	g.Meta       `orm:"table:portal_recharge_order, do:true"`
	Id           any         //
	OrderId      any         //
	ConsumerName any         //
	Amount       any         //
	Channel      any         //
	Status       any         //
	CreatedAt    *gtime.Time //
}

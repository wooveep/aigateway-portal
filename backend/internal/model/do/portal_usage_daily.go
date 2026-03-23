// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// PortalUsageDaily is the golang structure of table portal_usage_daily for DAO operations like Where/Data.
type PortalUsageDaily struct {
	g.Meta       `orm:"table:portal_usage_daily, do:true"`
	Id           any         //
	BillingDate  *gtime.Time //
	ConsumerName any         //
	ModelName    any         //
	RequestCount any         //
	InputTokens  any         //
	OutputTokens any         //
	TotalTokens  any         //
	CostAmount   any         //
	SourceFrom   *gtime.Time //
	SourceTo     *gtime.Time //
	CreatedAt    *gtime.Time //
	UpdatedAt    *gtime.Time //
}

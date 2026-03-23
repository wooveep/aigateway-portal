// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// PortalInviteCode is the golang structure of table portal_invite_code for DAO operations like Where/Data.
type PortalInviteCode struct {
	g.Meta         `orm:"table:portal_invite_code, do:true"`
	Id             any         //
	InviteCode     any         //
	Status         any         //
	ExpiresAt      *gtime.Time //
	UsedByConsumer any         //
	UsedAt         *gtime.Time //
	CreatedAt      *gtime.Time //
	UpdatedAt      *gtime.Time //
}

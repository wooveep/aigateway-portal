// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// PortalUser is the golang structure of table portal_user for DAO operations like Where/Data.
type PortalUser struct {
	g.Meta       `orm:"table:portal_user, do:true"`
	Id           any         //
	ConsumerName any         //
	DisplayName  any         //
	Email        any         //
	Department   any         //
	PasswordHash any         //
	Status       any         //
	Source       any         //
	LastLoginAt  *gtime.Time //
	CreatedAt    *gtime.Time //
	UpdatedAt    *gtime.Time //
}

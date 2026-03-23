// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// PortalSession is the golang structure of table portal_session for DAO operations like Where/Data.
type PortalSession struct {
	g.Meta       `orm:"table:portal_session, do:true"`
	SessionToken any         //
	ConsumerName any         //
	ExpiresAt    *gtime.Time //
	CreatedAt    *gtime.Time //
	LastSeenAt   *gtime.Time //
}

// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// PortalApiKey is the golang structure of table portal_api_key for DAO operations like Where/Data.
type PortalApiKey struct {
	g.Meta       `orm:"table:portal_api_key, do:true"`
	Id           any         //
	KeyId        any         //
	ConsumerName any         //
	Name         any         //
	KeyMasked    any         //
	KeyHash      any         //
	RawKey       any         //
	Status       any         //
	TotalCalls   any         //
	LastUsedAt   *gtime.Time //
	CreatedAt    *gtime.Time //
	UpdatedAt    *gtime.Time //
}

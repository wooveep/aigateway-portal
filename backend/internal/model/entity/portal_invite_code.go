// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// PortalInviteCode is the golang structure for table portal_invite_code.
type PortalInviteCode struct {
	Id             int64       `json:"id"             orm:"id"               ` //
	InviteCode     string      `json:"inviteCode"     orm:"invite_code"      ` //
	Status         string      `json:"status"         orm:"status"           ` //
	ExpiresAt      *gtime.Time `json:"expiresAt"      orm:"expires_at"       ` //
	UsedByConsumer string      `json:"usedByConsumer" orm:"used_by_consumer" ` //
	UsedAt         *gtime.Time `json:"usedAt"         orm:"used_at"          ` //
	CreatedAt      *gtime.Time `json:"createdAt"      orm:"created_at"       ` //
	UpdatedAt      *gtime.Time `json:"updatedAt"      orm:"updated_at"       ` //
}

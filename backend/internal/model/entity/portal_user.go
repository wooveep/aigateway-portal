// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// PortalUser is the golang structure for table portal_user.
type PortalUser struct {
	Id           int64       `json:"id"           orm:"id"            ` //
	ConsumerName string      `json:"consumerName" orm:"consumer_name" ` //
	DisplayName  string      `json:"displayName"  orm:"display_name"  ` //
	Email        string      `json:"email"        orm:"email"         ` //
	Department   string      `json:"department"   orm:"department"    ` //
	PasswordHash string      `json:"passwordHash" orm:"password_hash" ` //
	Status       string      `json:"status"       orm:"status"        ` //
	Source       string      `json:"source"       orm:"source"        ` //
	LastLoginAt  *gtime.Time `json:"lastLoginAt"  orm:"last_login_at" ` //
	CreatedAt    *gtime.Time `json:"createdAt"    orm:"created_at"    ` //
	UpdatedAt    *gtime.Time `json:"updatedAt"    orm:"updated_at"    ` //
}

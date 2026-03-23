// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// PortalSession is the golang structure for table portal_session.
type PortalSession struct {
	SessionToken string      `json:"sessionToken" orm:"session_token" ` //
	ConsumerName string      `json:"consumerName" orm:"consumer_name" ` //
	ExpiresAt    *gtime.Time `json:"expiresAt"    orm:"expires_at"    ` //
	CreatedAt    *gtime.Time `json:"createdAt"    orm:"created_at"    ` //
	LastSeenAt   *gtime.Time `json:"lastSeenAt"   orm:"last_seen_at"  ` //
}

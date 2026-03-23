// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// PortalApiKey is the golang structure for table portal_api_key.
type PortalApiKey struct {
	Id           int64       `json:"id"           orm:"id"            ` //
	KeyId        string      `json:"keyId"        orm:"key_id"        ` //
	ConsumerName string      `json:"consumerName" orm:"consumer_name" ` //
	Name         string      `json:"name"         orm:"name"          ` //
	KeyMasked    string      `json:"keyMasked"    orm:"key_masked"    ` //
	KeyHash      string      `json:"keyHash"      orm:"key_hash"      ` //
	RawKey       string      `json:"rawKey"       orm:"raw_key"       ` //
	Status       string      `json:"status"       orm:"status"        ` //
	TotalCalls   int64       `json:"totalCalls"   orm:"total_calls"   ` //
	LastUsedAt   *gtime.Time `json:"lastUsedAt"   orm:"last_used_at"  ` //
	CreatedAt    *gtime.Time `json:"createdAt"    orm:"created_at"    ` //
	UpdatedAt    *gtime.Time `json:"updatedAt"    orm:"updated_at"    ` //
}

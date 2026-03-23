// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// PortalRechargeOrder is the golang structure for table portal_recharge_order.
type PortalRechargeOrder struct {
	Id           int64       `json:"id"           orm:"id"            ` //
	OrderId      string      `json:"orderId"      orm:"order_id"      ` //
	ConsumerName string      `json:"consumerName" orm:"consumer_name" ` //
	Amount       float64     `json:"amount"       orm:"amount"        ` //
	Channel      string      `json:"channel"      orm:"channel"       ` //
	Status       string      `json:"status"       orm:"status"        ` //
	CreatedAt    *gtime.Time `json:"createdAt"    orm:"created_at"    ` //
}

// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// PortalUsageDaily is the golang structure for table portal_usage_daily.
type PortalUsageDaily struct {
	Id           int64       `json:"id"           orm:"id"            ` //
	BillingDate  *gtime.Time `json:"billingDate"  orm:"billing_date"  ` //
	ConsumerName string      `json:"consumerName" orm:"consumer_name" ` //
	ModelName    string      `json:"modelName"    orm:"model_name"    ` //
	RequestCount int64       `json:"requestCount" orm:"request_count" ` //
	InputTokens  int64       `json:"inputTokens"  orm:"input_tokens"  ` //
	OutputTokens int64       `json:"outputTokens" orm:"output_tokens" ` //
	TotalTokens  int64       `json:"totalTokens"  orm:"total_tokens"  ` //
	CostAmount   float64     `json:"costAmount"   orm:"cost_amount"   ` //
	SourceFrom   *gtime.Time `json:"sourceFrom"   orm:"source_from"   ` //
	SourceTo     *gtime.Time `json:"sourceTo"     orm:"source_to"     ` //
	CreatedAt    *gtime.Time `json:"createdAt"    orm:"created_at"    ` //
	UpdatedAt    *gtime.Time `json:"updatedAt"    orm:"updated_at"    ` //
}

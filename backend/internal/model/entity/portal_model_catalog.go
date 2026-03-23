// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// PortalModelCatalog is the golang structure for table portal_model_catalog.
type PortalModelCatalog struct {
	Id               int64       `json:"id"               orm:"id"                 ` //
	ModelId          string      `json:"modelId"          orm:"model_id"           ` //
	Name             string      `json:"name"             orm:"name"               ` //
	Vendor           string      `json:"vendor"           orm:"vendor"             ` //
	Capability       string      `json:"capability"       orm:"capability"         ` //
	InputTokenPrice  float64     `json:"inputTokenPrice"  orm:"input_token_price"  ` //
	OutputTokenPrice float64     `json:"outputTokenPrice" orm:"output_token_price" ` //
	Endpoint         string      `json:"endpoint"         orm:"endpoint"           ` //
	Sdk              string      `json:"sdk"              orm:"sdk"                ` //
	Summary          string      `json:"summary"          orm:"summary"            ` //
	Status           string      `json:"status"           orm:"status"             ` //
	CreatedAt        *gtime.Time `json:"createdAt"        orm:"created_at"         ` //
	UpdatedAt        *gtime.Time `json:"updatedAt"        orm:"updated_at"         ` //
}

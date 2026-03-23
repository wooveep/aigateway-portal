// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// PortalInvoiceRecord is the golang structure for table portal_invoice_record.
type PortalInvoiceRecord struct {
	Id           int64       `json:"id"           orm:"id"            ` //
	InvoiceId    string      `json:"invoiceId"    orm:"invoice_id"    ` //
	ConsumerName string      `json:"consumerName" orm:"consumer_name" ` //
	Title        string      `json:"title"        orm:"title"         ` //
	TaxNo        string      `json:"taxNo"        orm:"tax_no"        ` //
	Amount       float64     `json:"amount"       orm:"amount"        ` //
	Status       string      `json:"status"       orm:"status"        ` //
	Remark       string      `json:"remark"       orm:"remark"        ` //
	CreatedAt    *gtime.Time `json:"createdAt"    orm:"created_at"    ` //
}

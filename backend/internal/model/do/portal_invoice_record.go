// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// PortalInvoiceRecord is the golang structure of table portal_invoice_record for DAO operations like Where/Data.
type PortalInvoiceRecord struct {
	g.Meta       `orm:"table:portal_invoice_record, do:true"`
	Id           any         //
	InvoiceId    any         //
	ConsumerName any         //
	Title        any         //
	TaxNo        any         //
	Amount       any         //
	Status       any         //
	Remark       any         //
	CreatedAt    *gtime.Time //
}

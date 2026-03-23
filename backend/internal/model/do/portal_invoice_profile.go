// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// PortalInvoiceProfile is the golang structure of table portal_invoice_profile for DAO operations like Where/Data.
type PortalInvoiceProfile struct {
	g.Meta       `orm:"table:portal_invoice_profile, do:true"`
	Id           any         //
	ConsumerName any         //
	CompanyName  any         //
	TaxNo        any         //
	Address      any         //
	BankAccount  any         //
	Receiver     any         //
	Email        any         //
	CreatedAt    *gtime.Time //
	UpdatedAt    *gtime.Time //
}

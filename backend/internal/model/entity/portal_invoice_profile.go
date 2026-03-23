// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// PortalInvoiceProfile is the golang structure for table portal_invoice_profile.
type PortalInvoiceProfile struct {
	Id           int64       `json:"id"           orm:"id"            ` //
	ConsumerName string      `json:"consumerName" orm:"consumer_name" ` //
	CompanyName  string      `json:"companyName"  orm:"company_name"  ` //
	TaxNo        string      `json:"taxNo"        orm:"tax_no"        ` //
	Address      string      `json:"address"      orm:"address"       ` //
	BankAccount  string      `json:"bankAccount"  orm:"bank_account"  ` //
	Receiver     string      `json:"receiver"     orm:"receiver"      ` //
	Email        string      `json:"email"        orm:"email"         ` //
	CreatedAt    *gtime.Time `json:"createdAt"    orm:"created_at"    ` //
	UpdatedAt    *gtime.Time `json:"updatedAt"    orm:"updated_at"    ` //
}

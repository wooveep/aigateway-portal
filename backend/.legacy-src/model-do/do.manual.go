package do

type PortalUser struct {
	ConsumerName interface{} `orm:"consumer_name"`
	DisplayName  interface{} `orm:"display_name"`
	Email        interface{} `orm:"email"`
	Department   interface{} `orm:"department"`
	PasswordHash interface{} `orm:"password_hash"`
	Status       interface{} `orm:"status"`
	Source       interface{} `orm:"source"`
	LastLoginAt  interface{} `orm:"last_login_at"`
}

type PortalInviteCode struct {
	Status         interface{} `orm:"status"`
	UsedByConsumer interface{} `orm:"used_by_consumer"`
	UsedAt         interface{} `orm:"used_at"`
	ExpiresAt      interface{} `orm:"expires_at"`
}

type PortalSession struct {
	SessionToken interface{} `orm:"session_token"`
	ConsumerName interface{} `orm:"consumer_name"`
	ExpiresAt    interface{} `orm:"expires_at"`
	LastSeenAt   interface{} `orm:"last_seen_at"`
}

type PortalAPIKey struct {
	KeyID        interface{} `orm:"key_id"`
	ConsumerName interface{} `orm:"consumer_name"`
	Name         interface{} `orm:"name"`
	KeyMasked    interface{} `orm:"key_masked"`
	KeyHash      interface{} `orm:"key_hash"`
	RawKey       interface{} `orm:"raw_key"`
	Status       interface{} `orm:"status"`
	TotalCalls   interface{} `orm:"total_calls"`
	LastUsedAt   interface{} `orm:"last_used_at"`
}

type PortalRechargeOrder struct {
	OrderID      interface{} `orm:"order_id"`
	ConsumerName interface{} `orm:"consumer_name"`
	Amount       interface{} `orm:"amount"`
	Channel      interface{} `orm:"channel"`
	Status       interface{} `orm:"status"`
}

type PortalInvoiceProfile struct {
	ConsumerName interface{} `orm:"consumer_name"`
	CompanyName  interface{} `orm:"company_name"`
	TaxNo        interface{} `orm:"tax_no"`
	Address      interface{} `orm:"address"`
	BankAccount  interface{} `orm:"bank_account"`
	Receiver     interface{} `orm:"receiver"`
	Email        interface{} `orm:"email"`
}

type PortalInvoiceRecord struct {
	InvoiceID    interface{} `orm:"invoice_id"`
	ConsumerName interface{} `orm:"consumer_name"`
	Title        interface{} `orm:"title"`
	TaxNo        interface{} `orm:"tax_no"`
	Amount       interface{} `orm:"amount"`
	Status       interface{} `orm:"status"`
	Remark       interface{} `orm:"remark"`
}

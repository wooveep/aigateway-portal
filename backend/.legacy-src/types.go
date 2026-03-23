package main

import (
	"fmt"
	"strings"
	"time"
)

const (
	userStatusActive   = "active"
	userStatusDisabled = "disabled"
	userStatusPending  = "pending"

	apiKeyStatusActive   = "active"
	apiKeyStatusDisabled = "disabled"

	inviteStatusActive = "active"
	inviteStatusUsed   = "used"
)

type PortalUser struct {
	ConsumerName string
	DisplayName  string
	Email        string
	Department   string
	Status       string
	Source       string
	LastLoginAt  *time.Time
}

type AuthUser struct {
	ConsumerName string `json:"consumerName"`
	DisplayName  string `json:"displayName"`
	Email        string `json:"email"`
	Department   string `json:"department"`
	Status       string `json:"status"`
}

type BillingOverview struct {
	Balance          string `json:"balance"`
	TotalRecharge    string `json:"totalRecharge"`
	TotalConsumption string `json:"totalConsumption"`
}

type RechargeRecord struct {
	ID        string  `json:"id"`
	Amount    float64 `json:"amount"`
	Channel   string  `json:"channel"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"createdAt"`
}

type ConsumptionRecord struct {
	ID        string  `json:"id"`
	Model     string  `json:"model"`
	Tokens    int64   `json:"tokens"`
	Cost      float64 `json:"cost"`
	CreatedAt string  `json:"createdAt"`
}

type ModelInfo struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Vendor           string  `json:"vendor"`
	Capability       string  `json:"capability"`
	InputTokenPrice  float64 `json:"inputTokenPrice"`
	OutputTokenPrice float64 `json:"outputTokenPrice"`
	Endpoint         string  `json:"endpoint"`
	SDK              string  `json:"sdk"`
	UpdatedAt        string  `json:"updatedAt"`
	Summary          string  `json:"summary"`
}

type ApiKeyRecord struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Key        string `json:"key"`
	Status     string `json:"status"`
	CreatedAt  string `json:"createdAt"`
	LastUsed   string `json:"lastUsed"`
	TotalCalls int64  `json:"totalCalls"`
}

type OpenStats struct {
	TodayCalls     int64  `json:"todayCalls"`
	TodayCost      string `json:"todayCost"`
	Last7DaysCalls int64  `json:"last7DaysCalls"`
	ActiveKeys     int64  `json:"activeKeys"`
}

type CostDetailRecord struct {
	ID     string  `json:"id"`
	Date   string  `json:"date"`
	Model  string  `json:"model"`
	Calls  int64   `json:"calls"`
	Tokens int64   `json:"tokens"`
	Cost   float64 `json:"cost"`
}

type InvoiceProfile struct {
	CompanyName string `json:"companyName"`
	TaxNo       string `json:"taxNo"`
	Address     string `json:"address"`
	BankAccount string `json:"bankAccount"`
	Receiver    string `json:"receiver"`
	Email       string `json:"email"`
}

type InvoiceRecord struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	TaxNo     string  `json:"taxNo"`
	Amount    float64 `json:"amount"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"createdAt"`
	Remark    string  `json:"remark"`
}

type RegisterRequest struct {
	InviteCode  string `json:"inviteCode"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Department  string `json:"department"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type CreateAPIKeyRequest struct {
	Name string `json:"name"`
}

type UpdateAPIKeyStatusRequest struct {
	Status string `json:"status"`
}

type CreateRechargeRequest struct {
	Amount  float64 `json:"amount"`
	Channel string  `json:"channel"`
}

type CreateInvoiceRequest struct {
	Amount float64 `json:"amount"`
	Remark string  `json:"remark"`
}

type ConsumerUsageStat struct {
	ConsumerName string `json:"consumerName"`
	ModelName    string `json:"modelName"`
	RequestCount int64  `json:"requestCount"`
	InputTokens  int64  `json:"inputTokens"`
	OutputTokens int64  `json:"outputTokens"`
	TotalTokens  int64  `json:"totalTokens"`
}

func nowText(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

func maskKey(raw string) string {
	if len(raw) <= 8 {
		return "****"
	}
	return fmt.Sprintf("%s****%s", raw[:4], raw[len(raw)-4:])
}

func normalizeUsername(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

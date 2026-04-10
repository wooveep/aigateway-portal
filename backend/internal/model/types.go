package model

import (
	"fmt"
	"strings"
	"time"
)

const AppTimeZone = "UTC"

var appLocation = time.UTC

type PortalUserRow struct {
	ConsumerName       string     `orm:"consumer_name"`
	DisplayName        string     `orm:"display_name"`
	Email              string     `orm:"email"`
	DepartmentID       string     `orm:"department_id"`
	UserLevel          string     `orm:"user_level"`
	Status             string     `orm:"status"`
	Source             string     `orm:"source"`
	ParentConsumerName string     `orm:"parent_consumer_name"`
	PasswordHash       string     `orm:"password_hash"`
	LastLoginAt        *time.Time `orm:"last_login_at"`
}

type APIKeyRow struct {
	KeyID                 string     `orm:"key_id"`
	Name                  string     `orm:"name"`
	RawKey                string     `orm:"raw_key"`
	Masked                string     `orm:"key_masked"`
	Status                string     `orm:"status"`
	TotalCalls            int64      `orm:"total_calls"`
	LastUsedAt            *time.Time `orm:"last_used_at"`
	ExpiresAt             *time.Time `orm:"expires_at"`
	DeletedAt             *time.Time `orm:"deleted_at"`
	LimitTotalMicroYuan   int64      `orm:"limit_total_micro_yuan"`
	Limit5hMicroYuan      int64      `orm:"limit_5h_micro_yuan"`
	LimitDailyMicroYuan   int64      `orm:"limit_daily_micro_yuan"`
	DailyResetMode        string     `orm:"daily_reset_mode"`
	DailyResetTime        string     `orm:"daily_reset_time"`
	LimitWeeklyMicroYuan  int64      `orm:"limit_weekly_micro_yuan"`
	LimitMonthlyMicroYuan int64      `orm:"limit_monthly_micro_yuan"`
	CreatedAt             time.Time  `orm:"created_at"`
}

type AuthUser struct {
	ConsumerName       string `json:"consumerName"`
	DisplayName        string `json:"displayName"`
	Email              string `json:"email"`
	DepartmentID       string `json:"departmentId"`
	DepartmentName     string `json:"departmentName"`
	DepartmentPath     string `json:"departmentPath"`
	ParentConsumerName string `json:"parentConsumerName"`
	AdminConsumerName  string `json:"adminConsumerName"`
	IsDepartmentAdmin  bool   `json:"isDepartmentAdmin"`
	UserLevel          string `json:"userLevel"`
	Status             string `json:"status"`
}

type RegisterResult struct {
	User          AuthUser `json:"user"`
	DefaultAPIKey string   `json:"defaultApiKey"`
}

type BillingOverview struct {
	Balance          string `json:"balance"`
	TotalRecharge    string `json:"totalRecharge"`
	TotalConsumption string `json:"totalConsumption"`
}

type ManagedAccountSummary struct {
	ConsumerName       string `json:"consumerName"`
	DisplayName        string `json:"displayName"`
	Email              string `json:"email"`
	DepartmentID       string `json:"departmentId"`
	DepartmentName     string `json:"departmentName"`
	DepartmentPath     string `json:"departmentPath"`
	ParentConsumerName string `json:"parentConsumerName"`
	AdminConsumerName  string `json:"adminConsumerName"`
	IsDepartmentAdmin  bool   `json:"isDepartmentAdmin"`
	UserLevel          string `json:"userLevel"`
	Status             string `json:"status"`
	Balance            string `json:"balance"`
	TotalConsumption   string `json:"totalConsumption"`
	ActiveKeys         int64  `json:"activeKeys"`
}

type ManagedDepartmentNode struct {
	DepartmentID       string                  `json:"departmentId"`
	Name               string                  `json:"name"`
	DepartmentPath     string                  `json:"departmentPath"`
	ParentDepartmentID string                  `json:"parentDepartmentId"`
	AdminConsumerName  string                  `json:"adminConsumerName"`
	MemberCount        int64                   `json:"memberCount"`
	Children           []ManagedDepartmentNode `json:"children,omitempty"`
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
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Vendor           string            `json:"vendor"`
	Capability       string            `json:"capability"`
	InputTokenPrice  float64           `json:"inputTokenPrice"`
	OutputTokenPrice float64           `json:"outputTokenPrice"`
	Endpoint         string            `json:"endpoint"`
	RequestURL       string            `json:"requestUrl,omitempty"`
	InternalEndpoint string            `json:"-"`
	InternalRouteURL string            `json:"-"`
	RouteModel       string            `json:"-"`
	SDK              string            `json:"sdk"`
	UpdatedAt        string            `json:"updatedAt"`
	Summary          string            `json:"summary"`
	Tags             []string          `json:"tags,omitempty"`
	Capabilities     ModelCapabilities `json:"capabilities,omitempty"`
	Pricing          ModelPricing      `json:"pricing,omitempty"`
	Limits           ModelLimits       `json:"limits,omitempty"`
}

type AgentToolSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type AgentInfo struct {
	ID              string             `json:"id"`
	CanonicalName   string             `json:"canonicalName"`
	DisplayName     string             `json:"displayName"`
	Intro           string             `json:"intro"`
	Description     string             `json:"description"`
	IconURL         string             `json:"iconUrl"`
	Tags            []string           `json:"tags,omitempty"`
	McpServerName   string             `json:"mcpServerName"`
	ToolCount       int64              `json:"toolCount"`
	TransportTypes  []string           `json:"transportTypes,omitempty"`
	ResourceSummary string             `json:"resourceSummary"`
	PromptSummary   string             `json:"promptSummary"`
	HTTPURL         string             `json:"httpUrl"`
	SSEURL          string             `json:"sseUrl"`
	Tools           []AgentToolSummary `json:"tools,omitempty"`
	PublishedAt     string             `json:"publishedAt"`
	UpdatedAt       string             `json:"updatedAt"`
}

type ChatSessionSummary struct {
	SessionID          string `json:"sessionId"`
	ConsumerName       string `json:"consumerName"`
	Title              string `json:"title"`
	DefaultModelID     string `json:"defaultModelId"`
	DefaultAPIKeyID    string `json:"defaultApiKeyId"`
	LastMessagePreview string `json:"lastMessagePreview"`
	LastMessageAt      string `json:"lastMessageAt"`
	CreatedAt          string `json:"createdAt"`
}

type ChatMessageRecord struct {
	MessageID    string `json:"messageId"`
	SessionID    string `json:"sessionId"`
	Role         string `json:"role"`
	Content      string `json:"content"`
	Status       string `json:"status"`
	ModelID      string `json:"modelId"`
	APIKeyID     string `json:"apiKeyId"`
	RequestID    string `json:"requestId"`
	TraceID      string `json:"traceId"`
	HTTPStatus   int    `json:"httpStatus"`
	ErrorMessage string `json:"errorMessage"`
	CreatedAt    string `json:"createdAt"`
	FinishedAt   string `json:"finishedAt"`
}

type ChatSessionDetail struct {
	Session  ChatSessionSummary  `json:"session"`
	Messages []ChatMessageRecord `json:"messages"`
}

type ModelCapabilities struct {
	Modalities   []string `json:"modalities,omitempty"`
	Features     []string `json:"features,omitempty"`
	RequestKinds []string `json:"requestKinds,omitempty"`
}

type ModelPricing struct {
	Currency                                   string  `json:"currency,omitempty"`
	InputPer1K                                 float64 `json:"inputPer1K,omitempty"`
	OutputPer1K                                float64 `json:"outputPer1K,omitempty"`
	InputCostPerToken                          float64 `json:"input_cost_per_token,omitempty"`
	OutputCostPerToken                         float64 `json:"output_cost_per_token,omitempty"`
	InputCostPerRequest                        float64 `json:"input_cost_per_request,omitempty"`
	CacheCreationInputTokenCost                float64 `json:"cache_creation_input_token_cost,omitempty"`
	CacheCreationInputTokenCostAbove1hr        float64 `json:"cache_creation_input_token_cost_above_1hr,omitempty"`
	CacheReadInputTokenCost                    float64 `json:"cache_read_input_token_cost,omitempty"`
	InputCostPerTokenAbove200kTokens           float64 `json:"input_cost_per_token_above_200k_tokens,omitempty"`
	OutputCostPerTokenAbove200kTokens          float64 `json:"output_cost_per_token_above_200k_tokens,omitempty"`
	CacheCreationInputTokenCostAbove200kTokens float64 `json:"cache_creation_input_token_cost_above_200k_tokens,omitempty"`
	CacheReadInputTokenCostAbove200kTokens     float64 `json:"cache_read_input_token_cost_above_200k_tokens,omitempty"`
	OutputCostPerImage                         float64 `json:"output_cost_per_image,omitempty"`
	OutputCostPerImageToken                    float64 `json:"output_cost_per_image_token,omitempty"`
	InputCostPerImage                          float64 `json:"input_cost_per_image,omitempty"`
	InputCostPerImageToken                     float64 `json:"input_cost_per_image_token,omitempty"`
	SupportsPromptCaching                      bool    `json:"supports_prompt_caching,omitempty"`
}

type ModelLimits struct {
	RPM           int64 `json:"rpm,omitempty"`
	TPM           int64 `json:"tpm,omitempty"`
	ContextWindow int64 `json:"contextWindow,omitempty"`
}

type APIKeyRecord struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Key            string  `json:"key"`
	Status         string  `json:"status"`
	CreatedAt      string  `json:"createdAt"`
	LastUsed       string  `json:"lastUsed"`
	ExpiresAt      string  `json:"expiresAt"`
	TotalCalls     int64   `json:"totalCalls"`
	LimitTotal     float64 `json:"limitTotal"`
	Limit5h        float64 `json:"limit5h"`
	LimitDaily     float64 `json:"limitDaily"`
	DailyResetMode string  `json:"dailyResetMode"`
	DailyResetTime string  `json:"dailyResetTime"`
	LimitWeekly    float64 `json:"limitWeekly"`
	LimitMonthly   float64 `json:"limitMonthly"`
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

type RequestDetailRecord struct {
	EventID                    string `json:"eventId"`
	RequestID                  string `json:"requestId"`
	TraceID                    string `json:"traceId"`
	ConsumerName               string `json:"consumerName"`
	APIKeyID                   string `json:"apiKeyId"`
	ModelID                    string `json:"modelId"`
	PriceVersionID             int64  `json:"priceVersionId"`
	RouteName                  string `json:"routeName"`
	RequestKind                string `json:"requestKind"`
	RequestStatus              string `json:"requestStatus"`
	UsageStatus                string `json:"usageStatus"`
	HTTPStatus                 int    `json:"httpStatus"`
	InputTokens                int64  `json:"inputTokens"`
	OutputTokens               int64  `json:"outputTokens"`
	TotalTokens                int64  `json:"totalTokens"`
	CacheCreationInputTokens   int64  `json:"cacheCreationInputTokens"`
	CacheCreation5mInputTokens int64  `json:"cacheCreation5mInputTokens"`
	CacheCreation1hInputTokens int64  `json:"cacheCreation1hInputTokens"`
	CacheReadInputTokens       int64  `json:"cacheReadInputTokens"`
	InputImageTokens           int64  `json:"inputImageTokens"`
	OutputImageTokens          int64  `json:"outputImageTokens"`
	InputImageCount            int64  `json:"inputImageCount"`
	OutputImageCount           int64  `json:"outputImageCount"`
	RequestCount               int64  `json:"requestCount"`
	CostMicroYuan              int64  `json:"costMicroYuan"`
	DepartmentID               string `json:"departmentId"`
	DepartmentPath             string `json:"departmentPath"`
	OccurredAt                 string `json:"occurredAt"`
}

type DepartmentBillingSummary struct {
	DepartmentID    string  `json:"departmentId"`
	DepartmentName  string  `json:"departmentName"`
	DepartmentPath  string  `json:"departmentPath"`
	RequestCount    int64   `json:"requestCount"`
	TotalTokens     int64   `json:"totalTokens"`
	TotalCost       float64 `json:"totalCost"`
	ActiveConsumers int64   `json:"activeConsumers"`
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
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

type CreateAPIKeyRequest struct {
	Name           string  `json:"name"`
	ExpiresAt      string  `json:"expiresAt"`
	LimitTotal     float64 `json:"limitTotal"`
	Limit5h        float64 `json:"limit5h"`
	LimitDaily     float64 `json:"limitDaily"`
	DailyResetMode string  `json:"dailyResetMode"`
	DailyResetTime string  `json:"dailyResetTime"`
	LimitWeekly    float64 `json:"limitWeekly"`
	LimitMonthly   float64 `json:"limitMonthly"`
}

type UpdateAPIKeyStatusRequest struct {
	Status string `json:"status"`
}

type UpdateAPIKeyRequest struct {
	Name           string  `json:"name"`
	ExpiresAt      string  `json:"expiresAt"`
	LimitTotal     float64 `json:"limitTotal"`
	Limit5h        float64 `json:"limit5h"`
	LimitDaily     float64 `json:"limitDaily"`
	DailyResetMode string  `json:"dailyResetMode"`
	DailyResetTime string  `json:"dailyResetTime"`
	LimitWeekly    float64 `json:"limitWeekly"`
	LimitMonthly   float64 `json:"limitMonthly"`
}

type CreateRechargeRequest struct {
	Amount  float64 `json:"amount"`
	Channel string  `json:"channel"`
}

type UpdateManagedAccountRequest struct {
	UserLevel string `json:"userLevel"`
	Status    string `json:"status"`
}

type AdjustManagedAccountBalanceRequest struct {
	Amount float64 `json:"amount"`
	Reason string  `json:"reason"`
}

type CreateInvoiceRequest struct {
	Amount float64 `json:"amount"`
	Remark string  `json:"remark"`
}

type CreateChatSessionRequest struct {
	Title           string `json:"title"`
	DefaultModelID  string `json:"defaultModelId"`
	DefaultAPIKeyID string `json:"defaultApiKeyId"`
}

type UpdateChatSessionRequest struct {
	Title           string `json:"title"`
	DefaultModelID  string `json:"defaultModelId"`
	DefaultAPIKeyID string `json:"defaultApiKeyId"`
}

type ChatSendMessageRequest struct {
	Content  string `json:"content"`
	ModelID  string `json:"modelId"`
	APIKeyID string `json:"apiKeyId"`
}

type ChatStreamAck struct {
	UserMessageID      string `json:"userMessageId"`
	AssistantMessageID string `json:"assistantMessageId"`
	SessionID          string `json:"sessionId"`
}

type ChatStreamDelta struct {
	AssistantMessageID string `json:"assistantMessageId"`
	Delta              string `json:"delta"`
	Text               string `json:"text"`
}

type ChatStreamDone struct {
	AssistantMessageID string `json:"assistantMessageId"`
	RequestID          string `json:"requestId"`
	TraceID            string `json:"traceId"`
	HTTPStatus         int    `json:"httpStatus"`
}

type ChatStreamError struct {
	AssistantMessageID string `json:"assistantMessageId"`
	Code               string `json:"code"`
	Message            string `json:"message"`
}

type ConsumerUsageStat struct {
	ConsumerName               string `json:"consumerName"`
	ModelName                  string `json:"modelName"`
	RequestCount               int64  `json:"requestCount"`
	InputTokens                int64  `json:"inputTokens"`
	OutputTokens               int64  `json:"outputTokens"`
	TotalTokens                int64  `json:"totalTokens"`
	CacheCreationInputTokens   int64  `json:"cacheCreationInputTokens"`
	CacheCreation5mInputTokens int64  `json:"cacheCreation5mInputTokens"`
	CacheCreation1hInputTokens int64  `json:"cacheCreation1hInputTokens"`
	CacheReadInputTokens       int64  `json:"cacheReadInputTokens"`
	InputImageTokens           int64  `json:"inputImageTokens"`
	OutputImageTokens          int64  `json:"outputImageTokens"`
	InputImageCount            int64  `json:"inputImageCount"`
	OutputImageCount           int64  `json:"outputImageCount"`
}

func NowText(t time.Time) string {
	return ToAppTime(t).Format(time.RFC3339Nano)
}

func DayText(t time.Time) string {
	return ToAppTime(t).Format("2006-01-02")
}

func AppLocation() *time.Location {
	return appLocation
}

func ToAppTime(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.In(appLocation)
}

func NowInAppLocation() time.Time {
	return time.Now().UTC()
}

func StartOfAppDay(t time.Time) time.Time {
	local := ToAppTime(t)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, appLocation)
}

func ParseDateTime(value string) (*time.Time, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return nil, nil
	}
	absoluteLayouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, layout := range absoluteLayouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return &parsed, nil
		}
	}
	localLayouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02",
	}
	for _, layout := range localLayouts {
		if parsed, err := time.ParseInLocation(layout, raw, appLocation); err == nil {
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("unsupported datetime format")
}

func MaskKey(raw string) string {
	if len(raw) <= 8 {
		return "****"
	}
	return fmt.Sprintf("%s****%s", raw[:4], raw[len(raw)-4:])
}

func NormalizeUsername(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

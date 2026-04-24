export interface BillingOverview {
  balance: string;
  totalRecharge: string;
  totalConsumption: string;
}

export interface AuthUser {
  consumerName: string;
  displayName: string;
  email: string;
  departmentId: string;
  departmentName: string;
  departmentPath: string;
  adminConsumerName: string;
  isDepartmentAdmin: boolean;
  userLevel: 'normal' | 'plus' | 'pro' | 'ultra' | string;
  status: 'active' | 'disabled' | 'pending';
}

export interface PublicSSOConfig {
  enabled: boolean;
  displayName: string;
}

export interface ManagedAccountSummary {
  consumerName: string;
  displayName: string;
  email: string;
  departmentId: string;
  departmentName: string;
  departmentPath: string;
  adminConsumerName: string;
  isDepartmentAdmin: boolean;
  userLevel: 'normal' | 'plus' | 'pro' | 'ultra' | string;
  status: 'active' | 'disabled' | 'pending';
  balance: string;
  totalConsumption: string;
  activeKeys: number;
}

export interface CreateManagedAccountRequest {
  consumerName: string;
  displayName: string;
  email?: string;
  password?: string;
}

export interface CreateManagedAccountResponse {
  account: ManagedAccountSummary;
  tempPassword?: string;
}

export interface ManagedDepartmentNode {
  departmentId: string;
  name: string;
  departmentPath: string;
  parentDepartmentId: string;
  adminConsumerName: string;
  memberCount: number;
  children?: ManagedDepartmentNode[];
}

export interface ChangePasswordPayload {
  oldPassword: string;
  newPassword: string;
}

export interface RechargeRecord {
  id: string;
  amount: number;
  channel: string;
  status: 'success' | 'pending' | 'failed';
  createdAt: string;
}

export interface ConsumptionRecord {
  id: string;
  model: string;
  tokens: number;
  cost: number;
  createdAt: string;
}

export interface ModelInfo {
  id: string;
  name: string;
  vendor: string;
  modelType?: string;
  capability: string;
  inputPricePerMillionTokens: number;
  outputPricePerMillionTokens: number;
  endpoint: string;
  requestUrl?: string;
  sdk: string;
  updatedAt: string;
  summary: string;
  tags?: string[];
  capabilities?: {
    inputModalities?: string[];
    outputModalities?: string[];
    featureFlags?: string[];
    modalities?: string[];
    features?: string[];
    requestKinds?: string[];
  };
  pricing?: {
    currency?: string;
    inputCostPerMillionTokens?: number;
    outputCostPerMillionTokens?: number;
    pricePerImage?: number;
    pricePerSecond?: number;
    pricePerSecond720p?: number;
    pricePerSecond1080p?: number;
    pricePer10kChars?: number;
    cacheCreationInputTokenCostPerMillionTokens?: number;
    cacheReadInputTokenCostPerMillionTokens?: number;
  };
  limits?: {
    maxInputTokens?: number;
    maxOutputTokens?: number;
    contextWindowTokens?: number;
    maxReasoningTokens?: number;
    maxInputTokensInReasoningMode?: number;
    maxOutputTokensInReasoningMode?: number;
    rpm?: number;
    tpm?: number;
    contextWindow?: number;
  };
}

export interface AgentToolSummary {
  name: string;
  description: string;
}

export interface AgentInfo {
  id: string;
  canonicalName: string;
  displayName: string;
  intro: string;
  description: string;
  iconUrl: string;
  tags?: string[];
  mcpServerName: string;
  toolCount: number;
  transportTypes?: string[];
  resourceSummary: string;
  promptSummary: string;
  httpUrl: string;
  sseUrl: string;
  tools?: AgentToolSummary[];
  publishedAt: string;
  updatedAt: string;
}

export interface ChatSessionSummary {
  sessionId: string;
  consumerName: string;
  title: string;
  defaultModelId: string;
  defaultApiKeyId: string;
  lastMessagePreview: string;
  lastMessageAt: string;
  createdAt: string;
}

export interface ChatMessageRecord {
  messageId: string;
  sessionId: string;
  role: 'user' | 'assistant' | string;
  content: string;
  status: 'streaming' | 'succeeded' | 'failed' | 'cancelled' | string;
  modelId: string;
  apiKeyId: string;
  requestId: string;
  traceId: string;
  httpStatus: number;
  errorMessage: string;
  createdAt: string;
  finishedAt: string;
}

export interface ChatSessionDetail {
  session: ChatSessionSummary;
  messages: ChatMessageRecord[];
}

export interface ApiKeyRecord {
  id: string;
  name: string;
  key: string;
  status: 'active' | 'disabled';
  createdAt: string;
  lastUsed: string;
  expiresAt: string;
  totalCalls: number;
  limitTotal: number;
  limit5h: number;
  limitDaily: number;
  dailyResetMode: string;
  dailyResetTime: string;
  limitWeekly: number;
  limitMonthly: number;
}

export interface OpenStats {
  todayCalls: number;
  todayCost: string;
  last7DaysCalls: number;
  activeKeys: number;
}

export interface CostDetailRecord {
  id: string;
  date: string;
  model: string;
  calls: number;
  tokens: number;
  cost: number;
}

export interface RequestDetailRecord {
  eventId: string;
  requestId: string;
  traceId: string;
  consumerName: string;
  apiKeyId: string;
  modelId: string;
  priceVersionId: number;
  routeName: string;
  requestKind: string;
  requestStatus: string;
  usageStatus: string;
  httpStatus: number;
  inputTokens: number;
  outputTokens: number;
  totalTokens: number;
  cacheCreationInputTokens: number;
  cacheCreation5mInputTokens: number;
  cacheCreation1hInputTokens: number;
  cacheReadInputTokens: number;
  inputImageTokens: number;
  outputImageTokens: number;
  inputImageCount: number;
  outputImageCount: number;
  requestCount: number;
  costMicroYuan: number;
  departmentId: string;
  departmentPath: string;
  occurredAt: string;
}

export interface DepartmentBillingSummary {
  departmentId: string;
  departmentName: string;
  departmentPath: string;
  requestCount: number;
  totalTokens: number;
  totalCost: number;
  activeConsumers: number;
}

export interface InvoiceProfile {
  companyName: string;
  taxNo: string;
  address: string;
  bankAccount: string;
  receiver: string;
  email: string;
}

export interface InvoiceRecord {
  id: string;
  title: string;
  taxNo: string;
  amount: number;
  status: 'pending' | 'approved' | 'rejected' | 'issued';
  createdAt: string;
  remark: string;
}

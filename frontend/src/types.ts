export interface BillingOverview {
  balance: string;
  totalRecharge: string;
  totalConsumption: string;
}

export interface AuthUser {
  consumerName: string;
  displayName: string;
  email: string;
  department: string;
  userLevel: 'normal' | 'plus' | 'pro' | 'ultra' | string;
  status: 'active' | 'disabled' | 'pending';
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
  capability: string;
  inputTokenPrice: number;
  outputTokenPrice: number;
  endpoint: string;
  sdk: string;
  updatedAt: string;
  summary: string;
  tags?: string[];
  capabilities?: {
    modalities?: string[];
    features?: string[];
  };
  pricing?: {
    currency?: string;
    inputPer1K?: number;
    outputPer1K?: number;
  };
  limits?: {
    rpm?: number;
    tpm?: number;
    contextWindow?: number;
  };
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

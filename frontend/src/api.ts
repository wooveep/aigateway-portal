import axios from 'axios';
import type {
  ApiKeyRecord,
  AuthUser,
  BillingOverview,
  ChangePasswordPayload,
  ConsumptionRecord,
  CostDetailRecord,
  DepartmentBillingSummary,
  InvoiceProfile,
  InvoiceRecord,
  ManagedAccountSummary,
  ManagedDepartmentNode,
  ModelInfo,
  OpenStats,
  RequestDetailRecord,
  RechargeRecord,
} from './types';

const client = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || '/api',
  timeout: 10000,
  withCredentials: true,
});

export async function register(payload: {
  inviteCode: string;
  username: string;
  password: string;
  displayName?: string;
  email?: string;
}) {
  const { data } = await client.post<{ user: AuthUser; defaultApiKey: string }>('/auth/register', payload);
  return data;
}

export async function login(payload: { username: string; password: string }) {
  const { data } = await client.post<AuthUser>('/auth/login', payload);
  return data;
}

export async function logout() {
  const { data } = await client.post('/auth/logout');
  return data;
}

export async function fetchMe() {
  const { data } = await client.get<AuthUser>('/auth/me');
  return data;
}

export async function changePassword(payload: ChangePasswordPayload) {
  const { data } = await client.post<{ success: boolean }>('/auth/change-password', payload);
  return data;
}

export async function fetchManagedAccounts() {
  const { data } = await client.get<ManagedAccountSummary[]>('/accounts/managed');
  return data;
}

export async function fetchManagedDepartments() {
  const { data } = await client.get<ManagedDepartmentNode[]>('/departments/managed');
  return data;
}

export async function updateManagedAccount(
  consumerName: string,
  payload: {
    userLevel: string;
    status: 'active' | 'disabled' | 'pending';
  },
) {
  const { data } = await client.patch<ManagedAccountSummary>(`/accounts/${consumerName}/profile`, payload);
  return data;
}

export async function adjustManagedAccountBalance(
  consumerName: string,
  payload: {
    amount: number;
    reason?: string;
  },
) {
  const { data } = await client.post<ManagedAccountSummary>(`/accounts/${consumerName}/balance-adjustments`, payload);
  return data;
}

export async function fetchBillingOverview(consumerName?: string) {
  const { data } = await client.get<BillingOverview>('/billing/overview', {
    params: {
      consumerName,
    },
  });
  return data;
}

export async function fetchConsumptions(consumerName?: string) {
  const { data } = await client.get<ConsumptionRecord[]>('/billing/consumptions', {
    params: {
      consumerName,
    },
  });
  return data;
}

export async function fetchRecharges(consumerName?: string) {
  const { data } = await client.get<RechargeRecord[]>('/billing/recharges', {
    params: {
      consumerName,
    },
  });
  return data;
}

export async function createRecharge(payload: { amount: number; channel: string }, consumerName?: string) {
  const { data } = await client.post<RechargeRecord>('/billing/recharges', payload, {
    params: {
      consumerName,
    },
  });
  return data;
}

export async function fetchModels() {
  const { data } = await client.get<ModelInfo[]>('/models');
  return data;
}

export async function fetchModelDetail(id: string) {
  const { data } = await client.get<ModelInfo>(`/models/${id}`);
  return data;
}

export async function fetchApiKeys(includeRaw = false, consumerName?: string) {
  const { data } = await client.get<ApiKeyRecord[]>('/open-platform/keys', {
    params: {
      includeRaw,
      consumerName,
    },
  });
  return data;
}

export async function createApiKey(payload: {
  name: string;
  expiresAt?: string;
  limitTotal?: number;
  limit5h?: number;
  limitDaily?: number;
  dailyResetMode?: string;
  dailyResetTime?: string;
  limitWeekly?: number;
  limitMonthly?: number;
}, consumerName?: string) {
  const { data } = await client.post<ApiKeyRecord>('/open-platform/keys', payload, {
    params: {
      consumerName,
    },
  });
  return data;
}

export async function updateApiKey(
  id: string,
  payload: {
    name: string;
    expiresAt?: string;
    limitTotal?: number;
    limit5h?: number;
    limitDaily?: number;
    dailyResetMode?: string;
    dailyResetTime?: string;
    limitWeekly?: number;
    limitMonthly?: number;
  },
  consumerName?: string,
) {
  const { data } = await client.put<ApiKeyRecord>(`/open-platform/keys/${id}`, payload, {
    params: {
      consumerName,
    },
  });
  return data;
}

export async function updateApiKeyStatus(id: string, status: 'active' | 'disabled', consumerName?: string) {
  const { data } = await client.patch<ApiKeyRecord>(`/open-platform/keys/${id}/status`, { status }, {
    params: {
      consumerName,
    },
  });
  return data;
}

export async function removeApiKey(id: string, consumerName?: string) {
  const { data } = await client.delete<ApiKeyRecord>(`/open-platform/keys/${id}`, {
    params: {
      consumerName,
    },
  });
  return data;
}

export async function fetchOpenStats(consumerName?: string) {
  const { data } = await client.get<OpenStats>('/open-platform/stats', {
    params: {
      consumerName,
    },
  });
  return data;
}

export async function fetchCostDetails(consumerName?: string) {
  const { data } = await client.get<CostDetailRecord[]>('/open-platform/cost-details', {
    params: {
      consumerName,
    },
  });
  return data;
}

export async function fetchRequestDetails(params?: {
  consumerName?: string;
  apiKeyId?: string;
  modelId?: string;
  routeName?: string;
  requestStatus?: string;
  usageStatus?: string;
  startAt?: string;
  endAt?: string;
  pageNum?: number;
  pageSize?: number;
}) {
  const { data } = await client.get<RequestDetailRecord[]>('/open-platform/request-details', {
    params,
  });
  return data;
}

export async function fetchDepartmentBillingSummary(params?: {
  consumerName?: string;
  departmentId?: string;
  includeChildren?: boolean;
  startDate?: string;
  endDate?: string;
}) {
  const { data } = await client.get<DepartmentBillingSummary[]>('/billing/departments/summary', {
    params,
  });
  return data;
}

export async function fetchInvoiceProfile() {
  const { data } = await client.get<InvoiceProfile>('/invoices/profile');
  return data;
}

export async function updateInvoiceProfile(payload: InvoiceProfile) {
  const { data } = await client.put<InvoiceProfile>('/invoices/profile', payload);
  return data;
}

export async function fetchInvoiceRecords() {
  const { data } = await client.get<InvoiceRecord[]>('/invoices/records');
  return data;
}

export async function createInvoice(payload: { amount: number; remark?: string }) {
  const { data } = await client.post<InvoiceRecord>('/invoices/records', payload);
  return data;
}

import axios from 'axios';
import type {
  ApiKeyRecord,
  AuthUser,
  BillingOverview,
  ChangePasswordPayload,
  ConsumptionRecord,
  CostDetailRecord,
  InvoiceProfile,
  InvoiceRecord,
  ModelInfo,
  OpenStats,
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
  department?: string;
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

export async function fetchBillingOverview() {
  const { data } = await client.get<BillingOverview>('/billing/overview');
  return data;
}

export async function fetchConsumptions() {
  const { data } = await client.get<ConsumptionRecord[]>('/billing/consumptions');
  return data;
}

export async function fetchRecharges() {
  const { data } = await client.get<RechargeRecord[]>('/billing/recharges');
  return data;
}

export async function createRecharge(payload: { amount: number; channel: string }) {
  const { data } = await client.post<RechargeRecord>('/billing/recharges', payload);
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

export async function fetchApiKeys(includeRaw = false) {
  const { data } = await client.get<ApiKeyRecord[]>('/open-platform/keys', {
    params: {
      includeRaw,
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
}) {
  const { data } = await client.post<ApiKeyRecord>('/open-platform/keys', payload);
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
) {
  const { data } = await client.put<ApiKeyRecord>(`/open-platform/keys/${id}`, payload);
  return data;
}

export async function updateApiKeyStatus(id: string, status: 'active' | 'disabled') {
  const { data } = await client.patch<ApiKeyRecord>(`/open-platform/keys/${id}/status`, { status });
  return data;
}

export async function removeApiKey(id: string) {
  const { data } = await client.delete<ApiKeyRecord>(`/open-platform/keys/${id}`);
  return data;
}

export async function fetchOpenStats() {
  const { data } = await client.get<OpenStats>('/open-platform/stats');
  return data;
}

export async function fetchCostDetails() {
  const { data } = await client.get<CostDetailRecord[]>('/open-platform/cost-details');
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

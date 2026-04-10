<template>
  <section class="portal-page">
    <div class="portal-metric-strip">
      <div class="portal-metric">
        <div class="portal-metric__label">当前余额</div>
        <div class="portal-metric__value">¥{{ Number(overview.balance || 0).toFixed(2) }}</div>
      </div>
      <div class="portal-metric">
        <div class="portal-metric__label">累计充值</div>
        <div class="portal-metric__value">¥{{ Number(overview.totalRecharge || 0).toFixed(2) }}</div>
      </div>
      <div class="portal-metric">
        <div class="portal-metric__label">累计消费</div>
        <div class="portal-metric__value">¥{{ Number(overview.totalConsumption || 0).toFixed(2) }}</div>
      </div>
    </div>

    <section class="portal-section">
      <div class="portal-section__header">
        <div>
          <div class="portal-section__eyebrow">Recharge</div>
          <h2 class="portal-section__title">余额充值</h2>
        </div>
      </div>

      <a-form ref="rechargeFormRef" :model="rechargeForm" layout="vertical">
        <div class="portal-grid portal-grid--two">
          <a-form-item
            label="金额"
            name="amount"
            :rules="rechargeAmountRules"
          >
            <a-input-number v-model:value="rechargeForm.amount" :min="0" :precision="2" placeholder="请输入金额" style="width: 100%" />
          </a-form-item>
          <a-form-item label="渠道" name="channel" :rules="rechargeChannelRules">
            <a-select v-model:value="rechargeForm.channel" placeholder="请选择渠道">
              <a-select-option value="alipay">支付宝</a-select-option>
              <a-select-option value="wechat">微信支付</a-select-option>
              <a-select-option value="bank_card">银行卡</a-select-option>
            </a-select>
          </a-form-item>
        </div>
        <a-button type="primary" :loading="rechargeSubmitting" @click="submitRecharge">充值</a-button>
      </a-form>
    </section>

    <section class="portal-section">
      <div class="portal-section__header">
        <div>
          <div class="portal-section__eyebrow">Consumption</div>
          <h2 class="portal-section__title">消费记录</h2>
        </div>
      </div>

      <div v-if="pageLoading" class="portal-stack">
        <a-skeleton active :paragraph="{ rows: 4 }" />
      </div>
      <div v-else-if="!consumptions.length" class="portal-empty">
        <div class="portal-empty__title">还没有消费记录</div>
      </div>
      <div v-else class="portal-stack">
        <article v-for="item in consumptions" :key="item.id" class="portal-record">
          <div class="portal-record__header">
            <div>
              <div class="portal-record__title">{{ item.model }}</div>
              <div class="portal-record__subtitle">{{ formatDateTimeDisplay(item.createdAt) }}</div>
            </div>
            <span class="portal-pill">记录 {{ item.id }}</span>
          </div>
          <div class="portal-data-grid">
            <div class="portal-data-item">
              <div class="portal-data-item__label">Token 数</div>
              <div class="portal-data-item__value">{{ item.tokens }}</div>
            </div>
            <div class="portal-data-item">
              <div class="portal-data-item__label">费用</div>
              <div class="portal-data-item__value">¥{{ Number(item.cost).toFixed(2) }}</div>
            </div>
          </div>
        </article>
      </div>
    </section>

    <section v-if="authState.user?.isDepartmentAdmin" class="portal-section">
      <div class="portal-section__header">
        <div>
          <div class="portal-section__eyebrow">Department Summary</div>
          <h2 class="portal-section__title">部门账单汇总</h2>
        </div>
      </div>

      <div v-if="pageLoading" class="portal-stack">
        <a-skeleton active :paragraph="{ rows: 4 }" />
      </div>
      <div v-else-if="!departmentSummaries.length" class="portal-empty">
        <div class="portal-empty__title">暂无部门账单汇总</div>
      </div>
      <div v-else class="portal-stack">
        <article v-for="item in departmentSummaries" :key="item.departmentId" class="portal-record">
          <div class="portal-record__header">
            <div>
              <div class="portal-record__title">{{ item.departmentName }}</div>
              <div class="portal-record__subtitle">{{ item.departmentPath }}</div>
            </div>
            <span class="portal-pill">{{ item.activeConsumers }} 个活跃账号</span>
          </div>
          <div class="portal-data-grid">
            <div class="portal-data-item">
              <div class="portal-data-item__label">请求数</div>
              <div class="portal-data-item__value">{{ item.requestCount }}</div>
            </div>
            <div class="portal-data-item">
              <div class="portal-data-item__label">总 Token</div>
              <div class="portal-data-item__value">{{ item.totalTokens }}</div>
            </div>
            <div class="portal-data-item">
              <div class="portal-data-item__label">总费用</div>
              <div class="portal-data-item__value">¥{{ Number(item.totalCost).toFixed(2) }}</div>
            </div>
          </div>
        </article>
      </div>
    </section>

    <section class="portal-section">
      <div class="portal-section__header">
        <div>
          <div class="portal-section__eyebrow">Recharge History</div>
          <h2 class="portal-section__title">充值记录</h2>
        </div>
      </div>

      <div v-if="pageLoading" class="portal-stack">
        <a-skeleton active :paragraph="{ rows: 4 }" />
      </div>
      <div v-else-if="!recharges.length" class="portal-empty">
        <div class="portal-empty__title">还没有充值记录</div>
      </div>
      <div v-else class="portal-stack">
        <article v-for="item in recharges" :key="item.id" class="portal-record">
          <div class="portal-record__header">
            <div>
              <div class="portal-record__title">¥{{ Number(item.amount).toFixed(2) }}</div>
              <div class="portal-record__subtitle">{{ formatDateTimeDisplay(item.createdAt) }}</div>
            </div>
            <span class="portal-status" :class="statusClass(item.status)">{{ statusText(item.status) }}</span>
          </div>
          <div class="portal-data-grid">
            <div class="portal-data-item">
              <div class="portal-data-item__label">渠道</div>
              <div class="portal-data-item__value">{{ channelText(item.channel) }}</div>
            </div>
            <div class="portal-data-item">
              <div class="portal-data-item__label">记录 ID</div>
              <div class="portal-data-item__value portal-data-item__value--nowrap">{{ item.id }}</div>
            </div>
          </div>
        </article>
      </div>
    </section>
  </section>
</template>

<script setup lang="ts">
import {
  createRecharge,
  fetchBillingOverview,
  fetchConsumptions,
  fetchDepartmentBillingSummary,
  fetchRecharges,
} from '../api';
import { authState } from '../auth';
import { useManagedAccountScope } from '../composables/useManagedAccountScope';
import type { BillingOverview, ConsumptionRecord, DepartmentBillingSummary, RechargeRecord } from '../types';
import { formatDateTimeDisplay } from '../utils/time';
import { message } from 'ant-design-vue';
import { onMounted, reactive, ref, watch } from 'vue';

const pageLoading = ref(false);
const rechargeSubmitting = ref(false);
const rechargeFormRef = ref();
const {
  activeConsumerName,
  loadManagedAccounts,
} = useManagedAccountScope();
const overview = reactive<BillingOverview>({
  balance: '0',
  totalRecharge: '0',
  totalConsumption: '0',
});
const consumptions = ref<ConsumptionRecord[]>([]);
const recharges = ref<RechargeRecord[]>([]);
const departmentSummaries = ref<DepartmentBillingSummary[]>([]);

const rechargeForm = reactive({
  amount: 100,
  channel: 'alipay',
});

const rechargeAmountRules = [
  { required: true, message: '请输入充值金额' },
  {
    validator: (_rule: unknown, value: number | null | undefined) => {
      if (value === null || value === undefined || Number(value) >= 0) {
        return Promise.resolve();
      }
      return Promise.reject(new Error('充值金额不能小于 0'));
    },
  },
];

const rechargeChannelRules = [{ required: true, message: '请选择充值渠道' }];

const getErrorMessage = (error: unknown, fallback: string) => {
  const maybeError = error as {
    response?: { data?: { message?: string; error?: string } };
    message?: string;
  };
  return maybeError?.response?.data?.message || maybeError?.response?.data?.error || maybeError?.message || fallback;
};

const channelText = (value: string) => {
  const map: Record<string, string> = {
    alipay: '支付宝',
    wechat: '微信支付',
    bank_card: '银行卡',
  };
  return map[value] || value;
};

const statusText = (value: string) => {
  if (value === 'success') return '成功';
  if (value === 'pending') return '处理中';
  return '失败';
};

const statusClass = (value: string) => {
  if (value === 'success') return 'portal-status--success';
  if (value === 'pending') return 'portal-status--warning';
  return 'portal-status--danger';
};

const loadData = async () => {
  pageLoading.value = true;
  try {
    const consumerName = activeConsumerName.value || undefined;
    const [overviewRes, consumptionRes, rechargeRes, departmentRes] = await Promise.all([
      fetchBillingOverview(consumerName),
      fetchConsumptions(consumerName),
      fetchRecharges(consumerName),
      authState.user?.isDepartmentAdmin
        ? fetchDepartmentBillingSummary({
            includeChildren: true,
          })
        : Promise.resolve([] as DepartmentBillingSummary[]),
    ]);
    departmentSummaries.value = departmentRes;
    overview.balance = overviewRes.balance;
    overview.totalRecharge = overviewRes.totalRecharge;
    overview.totalConsumption = overviewRes.totalConsumption;
    consumptions.value = consumptionRes;
    recharges.value = rechargeRes;
  } catch (error: unknown) {
    message.error(getErrorMessage(error, '账单数据加载失败'));
  } finally {
    pageLoading.value = false;
  }
};

const submitRecharge = async () => {
  try {
    await rechargeFormRef.value?.validateFields();
  } catch {
    return;
  }
  rechargeSubmitting.value = true;
  message.loading({ key: 'recharge-submit', content: '充值处理中...' });
  try {
    await createRecharge({ amount: rechargeForm.amount, channel: rechargeForm.channel }, activeConsumerName.value || undefined);
    message.success({ key: 'recharge-submit', content: '充值成功' });
    await loadData();
  } catch (error: unknown) {
    message.error({ key: 'recharge-submit', content: getErrorMessage(error, '充值失败') });
  } finally {
    rechargeSubmitting.value = false;
  }
};

onMounted(async () => {
  try {
    await loadManagedAccounts();
  } catch (error: unknown) {
    message.error(getErrorMessage(error, '子账号范围加载失败'));
  }
  await loadData();
});

watch(
  () => activeConsumerName.value,
  async () => {
    await loadData();
  },
);
</script>

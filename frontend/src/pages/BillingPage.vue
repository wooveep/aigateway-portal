<template>
  <section class="portal-page">
    <div class="portal-metric-strip">
      <div class="portal-metric">
        <div class="portal-metric__label">当前余额</div>
        <div class="portal-metric__value">¥{{ Number(overview.balance || 0).toFixed(2) }}</div>
      </div>
      <div class="portal-metric">
        <div class="portal-metric__label">累计消费</div>
        <div class="portal-metric__value">¥{{ Number(overview.totalConsumption || 0).toFixed(2) }}</div>
      </div>
    </div>

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
  </section>
</template>

<script setup lang="ts">
import {
  fetchBillingOverview,
  fetchConsumptions,
  fetchDepartmentBillingSummary,
} from '../api';
import { authState } from '../auth';
import { useManagedAccountScope } from '../composables/useManagedAccountScope';
import type { BillingOverview, ConsumptionRecord, DepartmentBillingSummary } from '../types';
import { formatDateTimeDisplay } from '../utils/time';
import { message } from 'ant-design-vue';
import { onMounted, reactive, ref, watch } from 'vue';

const pageLoading = ref(false);
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
const departmentSummaries = ref<DepartmentBillingSummary[]>([]);

const getErrorMessage = (error: unknown, fallback: string) => {
  const maybeError = error as {
    response?: { data?: { message?: string; error?: string } };
    message?: string;
  };
  return maybeError?.response?.data?.message || maybeError?.response?.data?.error || maybeError?.message || fallback;
};

const loadData = async () => {
  pageLoading.value = true;
  try {
    const consumerName = activeConsumerName.value || undefined;
    const [overviewRes, consumptionRes, departmentRes] = await Promise.all([
      fetchBillingOverview(consumerName),
      fetchConsumptions(consumerName),
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
  } catch (error: unknown) {
    message.error(getErrorMessage(error, '账单数据加载失败'));
  } finally {
    pageLoading.value = false;
  }
};

onMounted(async () => {
  try {
    await loadManagedAccounts();
  } catch (error: unknown) {
    message.error(getErrorMessage(error, '部门成员范围加载失败'));
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

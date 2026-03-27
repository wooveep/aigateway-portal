<template>
  <div>
    <a-row :gutter="16">
      <a-col :xs="24" :md="8">
        <a-card>
          <a-statistic title="当前余额" :value="Number(overview.balance || 0)" :precision="2" prefix="¥" />
        </a-card>
      </a-col>
      <a-col :xs="24" :md="8">
        <a-card>
          <a-statistic title="累计充值" :value="Number(overview.totalRecharge || 0)" :precision="2" prefix="¥" />
        </a-card>
      </a-col>
      <a-col :xs="24" :md="8">
        <a-card>
          <a-statistic title="累计消费" :value="Number(overview.totalConsumption || 0)" :precision="2" prefix="¥" />
        </a-card>
      </a-col>
    </a-row>

    <a-card title="余额充值" class="portal-card">
      <a-form ref="rechargeFormRef" :model="rechargeForm" layout="inline">
        <a-form-item
          label="金额"
          name="amount"
          :rules="rechargeAmountRules"
        >
          <a-input-number v-model:value="rechargeForm.amount" :min="0" :precision="2" placeholder="请输入金额" />
        </a-form-item>
        <a-form-item label="渠道" name="channel" :rules="rechargeChannelRules">
          <a-select v-model:value="rechargeForm.channel" placeholder="请选择渠道" style="width: 160px">
            <a-select-option value="alipay">支付宝</a-select-option>
            <a-select-option value="wechat">微信支付</a-select-option>
            <a-select-option value="bank_card">银行卡</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item>
          <a-button type="primary" :loading="rechargeSubmitting" @click="submitRecharge">充值</a-button>
        </a-form-item>
      </a-form>
    </a-card>

    <a-card title="消费记录" class="portal-card">
      <a-table
        :columns="consumptionColumns"
        :data-source="consumptions"
        :pagination="{ pageSize: 5 }"
        :loading="pageLoading"
        row-key="id"
      />
    </a-card>

    <a-card title="充值记录" class="portal-card">
      <a-table
        :columns="rechargeColumns"
        :data-source="recharges"
        :pagination="{ pageSize: 5 }"
        :loading="pageLoading"
        row-key="id"
      />
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { createRecharge, fetchBillingOverview, fetchConsumptions, fetchRecharges } from '../api';
import type { BillingOverview, ConsumptionRecord, RechargeRecord } from '../types';
import { message, Tag } from 'ant-design-vue';
import type { TableColumnsType } from 'ant-design-vue';
import { h, onMounted, reactive, ref } from 'vue';

const pageLoading = ref(false);
const rechargeSubmitting = ref(false);
const rechargeFormRef = ref();
const overview = reactive<BillingOverview>({
  balance: '0',
  totalRecharge: '0',
  totalConsumption: '0',
});
const consumptions = ref<ConsumptionRecord[]>([]);
const recharges = ref<RechargeRecord[]>([]);

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

const consumptionColumns: TableColumnsType<ConsumptionRecord> = [
  { title: '记录 ID', dataIndex: 'id' },
  { title: '模型', dataIndex: 'model' },
  { title: 'Token 数', dataIndex: 'tokens' },
  {
    title: '费用',
    dataIndex: 'cost',
    customRender: ({ value }) => `¥${Number(value).toFixed(2)}`,
  },
  { title: '时间', dataIndex: 'createdAt' },
];

const rechargeColumns: TableColumnsType<RechargeRecord> = [
  { title: '记录 ID', dataIndex: 'id' },
  {
    title: '充值金额',
    dataIndex: 'amount',
    customRender: ({ value }) => `¥${Number(value).toFixed(2)}`,
  },
  {
    title: '渠道',
    dataIndex: 'channel',
    customRender: ({ value }) => {
      const map: Record<string, string> = {
        alipay: '支付宝',
        wechat: '微信支付',
        bank_card: '银行卡',
      };
      return map[value] || value;
    },
  },
  {
    title: '状态',
    dataIndex: 'status',
    customRender: ({ value }) => {
      const color = value === 'success' ? 'green' : value === 'pending' ? 'gold' : 'red';
      const text = value === 'success' ? '成功' : value === 'pending' ? '处理中' : '失败';
      return h(Tag, { color }, () => text);
    },
  },
  { title: '时间', dataIndex: 'createdAt' },
];

const loadData = async () => {
  pageLoading.value = true;
  try {
    const [overviewRes, consumptionRes, rechargeRes] = await Promise.all([
      fetchBillingOverview(),
      fetchConsumptions(),
      fetchRecharges(),
    ]);
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
    await createRecharge({ amount: rechargeForm.amount, channel: rechargeForm.channel });
    message.success({ key: 'recharge-submit', content: '充值成功' });
    await loadData();
  } catch (error: unknown) {
    message.error({ key: 'recharge-submit', content: getErrorMessage(error, '充值失败') });
  } finally {
    rechargeSubmitting.value = false;
  }
};

onMounted(loadData);
</script>

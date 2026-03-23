<template>
  <div>
    <a-row :gutter="16">
      <a-col :xs="24" :md="8">
        <a-card>
          <a-statistic title="当前余额" :value="Number(overview.balance || 0)" :precision="2" prefix="$" />
        </a-card>
      </a-col>
      <a-col :xs="24" :md="8">
        <a-card>
          <a-statistic title="累计充值" :value="Number(overview.totalRecharge || 0)" :precision="2" prefix="$" />
        </a-card>
      </a-col>
      <a-col :xs="24" :md="8">
        <a-card>
          <a-statistic title="累计消费" :value="Number(overview.totalConsumption || 0)" :precision="2" prefix="$" />
        </a-card>
      </a-col>
    </a-row>

    <a-card title="余额充值" class="portal-card">
      <a-form layout="inline" @finish="submitRecharge">
        <a-form-item label="金额">
          <a-input-number v-model:value="rechargeForm.amount" :min="1" :precision="2" placeholder="请输入金额" />
        </a-form-item>
        <a-form-item label="渠道">
          <a-select v-model:value="rechargeForm.channel" placeholder="请选择渠道" style="width: 160px">
            <a-select-option value="alipay">支付宝</a-select-option>
            <a-select-option value="wechat">微信支付</a-select-option>
            <a-select-option value="bank_card">银行卡</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item>
          <a-button type="primary" html-type="submit" :loading="loading">充值</a-button>
        </a-form-item>
      </a-form>
    </a-card>

    <a-card title="消费记录" class="portal-card">
      <a-table :columns="consumptionColumns" :data-source="consumptions" :pagination="{ pageSize: 5 }" row-key="id" />
    </a-card>

    <a-card title="充值记录" class="portal-card">
      <a-table :columns="rechargeColumns" :data-source="recharges" :pagination="{ pageSize: 5 }" row-key="id" />
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { createRecharge, fetchBillingOverview, fetchConsumptions, fetchRecharges } from '../api';
import type { BillingOverview, ConsumptionRecord, RechargeRecord } from '../types';
import { message, Tag } from 'ant-design-vue';
import type { TableColumnsType } from 'ant-design-vue';
import { h, onMounted, reactive, ref } from 'vue';

const loading = ref(false);
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

const consumptionColumns: TableColumnsType<ConsumptionRecord> = [
  { title: '记录 ID', dataIndex: 'id' },
  { title: '模型', dataIndex: 'model' },
  { title: 'Token 数', dataIndex: 'tokens' },
  {
    title: '费用',
    dataIndex: 'cost',
    customRender: ({ value }) => `$${Number(value).toFixed(2)}`,
  },
  { title: '时间', dataIndex: 'createdAt' },
];

const rechargeColumns: TableColumnsType<RechargeRecord> = [
  { title: '记录 ID', dataIndex: 'id' },
  {
    title: '充值金额',
    dataIndex: 'amount',
    customRender: ({ value }) => `$${Number(value).toFixed(2)}`,
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
  loading.value = true;
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
  } catch {
    message.error('账单数据加载失败');
  } finally {
    loading.value = false;
  }
};

const submitRecharge = async () => {
  if (!rechargeForm.amount || rechargeForm.amount <= 0) {
    message.warning('请输入正确金额');
    return;
  }
  try {
    await createRecharge({ amount: rechargeForm.amount, channel: rechargeForm.channel });
    message.success('充值成功');
    await loadData();
  } catch {
    message.error('充值失败');
  }
};

onMounted(loadData);
</script>

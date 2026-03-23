<template>
  <div>
    <a-card title="开票信息管理">
      <a-form layout="vertical">
        <a-row :gutter="16">
          <a-col :xs="24" :md="12">
            <a-form-item label="公司名称" required>
              <a-input v-model:value="profile.companyName" />
            </a-form-item>
          </a-col>
          <a-col :xs="24" :md="12">
            <a-form-item label="税号" required>
              <a-input v-model:value="profile.taxNo" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :xs="24" :md="12">
            <a-form-item label="地址" required>
              <a-input v-model:value="profile.address" />
            </a-form-item>
          </a-col>
          <a-col :xs="24" :md="12">
            <a-form-item label="开户行及账号" required>
              <a-input v-model:value="profile.bankAccount" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :xs="24" :md="12">
            <a-form-item label="收票人" required>
              <a-input v-model:value="profile.receiver" />
            </a-form-item>
          </a-col>
          <a-col :xs="24" :md="12">
            <a-form-item label="邮箱" required>
              <a-input v-model:value="profile.email" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-button type="primary" :loading="loading" @click="saveProfile">保存开票信息</a-button>
      </a-form>
    </a-card>

    <a-card title="申请开票" class="portal-card">
      <a-form layout="inline" @finish="submitInvoice">
        <a-form-item label="开票金额">
          <a-input-number v-model:value="invoiceAmount" :min="1" :precision="2" />
        </a-form-item>
        <a-form-item label="备注">
          <a-input v-model:value="invoiceRemark" placeholder="例如：3 月份账单" style="width: 280px" />
        </a-form-item>
        <a-form-item>
          <a-button type="primary" html-type="submit" :loading="loading">提交开票申请</a-button>
        </a-form-item>
      </a-form>
    </a-card>

    <a-card title="开票记录" class="portal-card">
      <a-table :columns="invoiceColumns" :data-source="records" row-key="id" :pagination="{ pageSize: 5 }">
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'status'">
            <a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag>
          </template>
        </template>
      </a-table>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import {
  createInvoice,
  fetchInvoiceProfile,
  fetchInvoiceRecords,
  updateInvoiceProfile,
} from '../api';
import type { InvoiceProfile, InvoiceRecord } from '../types';
import { message } from 'ant-design-vue';
import type { TableColumnsType } from 'ant-design-vue';
import { onMounted, reactive, ref } from 'vue';

const loading = ref(false);

const profile = reactive<InvoiceProfile>({
  companyName: '',
  taxNo: '',
  address: '',
  bankAccount: '',
  receiver: '',
  email: '',
});

const records = ref<InvoiceRecord[]>([]);
const invoiceAmount = ref(100);
const invoiceRemark = ref('');

const invoiceColumns: TableColumnsType<InvoiceRecord> = [
  { title: '申请编号', dataIndex: 'id' },
  { title: '抬头', dataIndex: 'title' },
  { title: '税号', dataIndex: 'taxNo' },
  {
    title: '金额',
    dataIndex: 'amount',
    customRender: ({ value }) => `$${Number(value).toFixed(2)}`,
  },
  { title: '状态', dataIndex: 'status' },
  { title: '申请时间', dataIndex: 'createdAt' },
  { title: '备注', dataIndex: 'remark' },
];

const statusColor = (status: string) => {
  if (status === 'issued') return 'green';
  if (status === 'approved') return 'blue';
  if (status === 'rejected') return 'red';
  return 'gold';
};

const statusText = (status: string) => {
  if (status === 'issued') return '已开票';
  if (status === 'approved') return '已通过';
  if (status === 'rejected') return '已拒绝';
  return '审核中';
};

const loadData = async () => {
  loading.value = true;
  try {
    const [profileRes, recordRes] = await Promise.all([fetchInvoiceProfile(), fetchInvoiceRecords()]);
    Object.assign(profile, profileRes);
    records.value = recordRes;
  } catch {
    message.error('发票数据加载失败');
  } finally {
    loading.value = false;
  }
};

const saveProfile = async () => {
  loading.value = true;
  try {
    await updateInvoiceProfile({ ...profile });
    message.success('开票信息已更新');
  } catch {
    message.error('开票信息保存失败');
  } finally {
    loading.value = false;
  }
};

const submitInvoice = async () => {
  if (!invoiceAmount.value || invoiceAmount.value <= 0) {
    message.warning('请输入正确的开票金额');
    return;
  }
  loading.value = true;
  try {
    await createInvoice({ amount: invoiceAmount.value, remark: invoiceRemark.value });
    message.success('开票申请已提交');
    invoiceRemark.value = '';
    await loadData();
  } catch {
    message.error('提交开票申请失败');
  } finally {
    loading.value = false;
  }
};

onMounted(loadData);
</script>

<template>
  <section class="portal-page">
    <section class="portal-section">
      <div class="portal-section__header">
        <div>
          <div class="portal-section__eyebrow">Profile</div>
          <h2 class="portal-section__title">开票信息管理</h2>
        </div>
      </div>

      <a-form layout="vertical">
        <div class="portal-grid portal-grid--two">
          <a-form-item label="公司名称" required>
            <a-input v-model:value="profile.companyName" />
          </a-form-item>
          <a-form-item label="税号" required>
            <a-input v-model:value="profile.taxNo" />
          </a-form-item>
          <a-form-item label="地址" required>
            <a-input v-model:value="profile.address" />
          </a-form-item>
          <a-form-item label="开户行及账号" required>
            <a-input v-model:value="profile.bankAccount" />
          </a-form-item>
          <a-form-item label="收票人" required>
            <a-input v-model:value="profile.receiver" />
          </a-form-item>
          <a-form-item label="邮箱" required>
            <a-input v-model:value="profile.email" />
          </a-form-item>
        </div>
        <a-button type="primary" :loading="loading" @click="saveProfile">保存开票信息</a-button>
      </a-form>
    </section>

    <section class="portal-section">
      <div class="portal-section__header">
        <div>
          <div class="portal-section__eyebrow">Create Invoice</div>
          <h2 class="portal-section__title">申请开票</h2>
        </div>
      </div>

      <a-form layout="vertical" @finish="submitInvoice">
        <div class="portal-grid portal-grid--two">
          <a-form-item label="开票金额">
            <a-input-number v-model:value="invoiceAmount" :min="1" :precision="2" style="width: 100%" />
          </a-form-item>
          <a-form-item label="备注">
            <a-input v-model:value="invoiceRemark" placeholder="例如：3 月份账单" />
          </a-form-item>
        </div>
        <a-button type="primary" html-type="submit" :loading="loading">提交开票申请</a-button>
      </a-form>
    </section>

    <section class="portal-section">
      <div class="portal-section__header">
        <div>
          <div class="portal-section__eyebrow">Records</div>
          <h2 class="portal-section__title">开票记录</h2>
        </div>
      </div>

      <div v-if="loading" class="portal-stack">
        <a-skeleton active :paragraph="{ rows: 4 }" />
      </div>
      <div v-else-if="!records.length" class="portal-empty">
        <div class="portal-empty__title">还没有开票记录</div>
      </div>
      <div v-else class="portal-stack">
        <article v-for="record in records" :key="record.id" class="portal-record">
          <div class="portal-record__header">
            <div>
              <div class="portal-record__title">{{ record.title }}</div>
              <div class="portal-record__subtitle">{{ formatDateTimeDisplay(record.createdAt) }}</div>
            </div>
            <span class="portal-status" :class="statusClass(record.status)">{{ statusText(record.status) }}</span>
          </div>
          <div class="portal-data-grid">
            <div class="portal-data-item">
              <div class="portal-data-item__label">申请编号</div>
              <div class="portal-data-item__value portal-data-item__value--nowrap">{{ record.id }}</div>
            </div>
            <div class="portal-data-item">
              <div class="portal-data-item__label">税号</div>
              <div class="portal-data-item__value portal-data-item__value--nowrap">{{ record.taxNo }}</div>
            </div>
            <div class="portal-data-item">
              <div class="portal-data-item__label">金额</div>
              <div class="portal-data-item__value">¥{{ Number(record.amount).toFixed(2) }}</div>
            </div>
            <div class="portal-data-item">
              <div class="portal-data-item__label">备注</div>
              <div class="portal-data-item__value">{{ record.remark || '-' }}</div>
            </div>
          </div>
        </article>
      </div>
    </section>
  </section>
</template>

<script setup lang="ts">
import {
  createInvoice,
  fetchInvoiceProfile,
  fetchInvoiceRecords,
  updateInvoiceProfile,
} from '../api';
import type { InvoiceProfile, InvoiceRecord } from '../types';
import { formatDateTimeDisplay } from '../utils/time';
import { message } from 'ant-design-vue';
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

const statusClass = (status: string) => {
  if (status === 'issued') return 'portal-status--success';
  if (status === 'approved') return 'portal-status--success';
  if (status === 'rejected') return 'portal-status--danger';
  return 'portal-status--warning';
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

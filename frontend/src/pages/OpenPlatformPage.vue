<template>
  <div>
    <a-row :gutter="16">
      <a-col :xs="24" :md="6">
        <a-card><a-statistic title="今日调用量" :value="stats.todayCalls" /></a-card>
      </a-col>
      <a-col :xs="24" :md="6">
        <a-card><a-statistic title="今日费用" :value="Number(stats.todayCost || 0)" :precision="2" prefix="¥" /></a-card>
      </a-col>
      <a-col :xs="24" :md="6">
        <a-card><a-statistic title="近 7 天调用量" :value="stats.last7DaysCalls" /></a-card>
      </a-col>
      <a-col :xs="24" :md="6">
        <a-card><a-statistic title="启用 Key 数" :value="stats.activeKeys" /></a-card>
      </a-col>
    </a-row>

    <a-card title="API Key 管理" class="portal-card">
      <template #extra>
        <a-space>
          <a-button @click="toggleShowAllKeys">
            {{ showRawKeys ? '隐藏完整 Key' : '展示所有完整 API Key' }}
          </a-button>
          <a-button :disabled="!apiKeys.length" @click="copyAllKeys">复制全部 API Key</a-button>
          <a-button type="primary" @click="openCreateModal">新建 API Key</a-button>
        </a-space>
      </template>
      <a-table :columns="keyColumns" :data-source="apiKeys" row-key="id" :pagination="{ pageSize: 5 }">
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'status'">
            <a-switch :checked="record.status === 'active'" @change="(checked: boolean) => toggleStatus(record.id, checked)" />
          </template>
          <template v-if="column.dataIndex === 'operation'">
            <a-space>
              <a-button type="link" @click="openEditModal(record)">编辑</a-button>
              <a-popconfirm title="确认删除这个 Key 吗？" @confirm="deleteKey(record.id)">
                <a-button danger type="link">删除</a-button>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>
    </a-card>

    <a-card title="费用明细" class="portal-card">
      <a-table :columns="costColumns" :data-source="costDetails" row-key="id" :pagination="{ pageSize: 5 }" />
    </a-card>

    <a-modal v-model:open="showCreateModal" :title="modalTitle" @ok="submitKey" :confirm-loading="loading">
      <a-form layout="vertical">
        <a-form-item label="Key 名称" required>
          <a-input v-model:value="keyForm.name" placeholder="例如：生产环境 Key" />
        </a-form-item>
        <a-form-item label="过期时间">
          <a-input v-model:value="keyForm.expiresAt" type="datetime-local" />
        </a-form-item>
        <a-form-item label="5 小时限额（RMB）">
          <a-input-number v-model:value="keyForm.limit5h" :min="0" style="width: 100%" />
        </a-form-item>
        <a-form-item label="日限额（RMB）">
          <a-input-number v-model:value="keyForm.limitDaily" :min="0" style="width: 100%" />
        </a-form-item>
        <a-form-item label="日重置模式">
          <a-input :value="keyForm.dailyResetMode" disabled />
        </a-form-item>
        <a-form-item label="日重置时间">
          <a-input v-model:value="keyForm.dailyResetTime" placeholder="00:00" />
        </a-form-item>
        <a-form-item label="周限额（RMB）">
          <a-input-number v-model:value="keyForm.limitWeekly" :min="0" style="width: 100%" />
        </a-form-item>
        <a-form-item label="月限额（RMB）">
          <a-input-number v-model:value="keyForm.limitMonthly" :min="0" style="width: 100%" />
        </a-form-item>
        <a-form-item label="总限额（RMB）">
          <a-input-number v-model:value="keyForm.limitTotal" :min="0" style="width: 100%" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import {
  createApiKey,
  fetchApiKeys,
  fetchCostDetails,
  fetchOpenStats,
  removeApiKey,
  updateApiKey,
  updateApiKeyStatus,
} from '../api';
import type { ApiKeyRecord, CostDetailRecord, OpenStats } from '../types';
import { message } from 'ant-design-vue';
import type { TableColumnsType } from 'ant-design-vue';
import { computed, onMounted, reactive, ref } from 'vue';

const stats = reactive<OpenStats>({
  todayCalls: 0,
  todayCost: '0',
  last7DaysCalls: 0,
  activeKeys: 0,
});

const apiKeys = ref<ApiKeyRecord[]>([]);
const costDetails = ref<CostDetailRecord[]>([]);
const showCreateModal = ref(false);
const editingKeyId = ref('');
const loading = ref(false);
const showRawKeys = ref(false);
const keyForm = reactive({
  name: '',
  expiresAt: '',
  limitTotal: 0,
  limit5h: 0,
  limitDaily: 0,
  dailyResetMode: 'fixed',
  dailyResetTime: '00:00',
  limitWeekly: 0,
  limitMonthly: 0,
});

const modalTitle = computed(() => (editingKeyId.value ? '编辑 API Key' : '新建 API Key'));

const getErrorMessage = (error: unknown, fallback: string) => {
  const maybeError = error as {
    response?: { data?: { message?: string; error?: string } };
    message?: string;
  };
  return maybeError?.response?.data?.message || maybeError?.response?.data?.error || maybeError?.message || fallback;
};

const keyColumns: TableColumnsType<ApiKeyRecord> = [
  { title: 'Key 名称', dataIndex: 'name' },
  { title: 'API Key', dataIndex: 'key' },
  { title: '创建时间', dataIndex: 'createdAt' },
  { title: '过期时间', dataIndex: 'expiresAt' },
  { title: '最近调用', dataIndex: 'lastUsed' },
  { title: '调用次数', dataIndex: 'totalCalls' },
  {
    title: '金额限额',
    dataIndex: 'limitSummary',
    customRender: ({ record }) =>
      `5h ¥${record.limit5h.toFixed(2)} / 日 ¥${record.limitDaily.toFixed(2)} / 周 ¥${record.limitWeekly.toFixed(2)} / 月 ¥${record.limitMonthly.toFixed(2)} / 总 ¥${record.limitTotal.toFixed(2)}`,
  },
  { title: '启用状态', dataIndex: 'status' },
  { title: '操作', dataIndex: 'operation' },
];

const costColumns: TableColumnsType<CostDetailRecord> = [
  { title: '日期', dataIndex: 'date' },
  { title: '模型', dataIndex: 'model' },
  { title: '调用次数', dataIndex: 'calls' },
  { title: 'Tokens', dataIndex: 'tokens' },
  {
    title: '费用',
    dataIndex: 'cost',
    customRender: ({ value }) => `¥${Number(value).toFixed(2)}`,
  },
];

const loadData = async (includeRaw = showRawKeys.value) => {
  loading.value = true;
  try {
    const [statsRes, keyRes, costRes] = await Promise.all([
      fetchOpenStats(),
      fetchApiKeys(includeRaw),
      fetchCostDetails(),
    ]);
    showRawKeys.value = includeRaw;
    stats.todayCalls = statsRes.todayCalls;
    stats.todayCost = statsRes.todayCost;
    stats.last7DaysCalls = statsRes.last7DaysCalls;
    stats.activeKeys = statsRes.activeKeys;
    apiKeys.value = keyRes;
    costDetails.value = costRes;
  } catch (error: unknown) {
    message.error(getErrorMessage(error, '开放平台数据加载失败'));
  } finally {
    loading.value = false;
  }
};

const toggleShowAllKeys = async () => {
  await loadData(!showRawKeys.value);
};

const resetKeyForm = () => {
  editingKeyId.value = '';
  keyForm.name = '';
  keyForm.expiresAt = '';
  keyForm.limitTotal = 0;
  keyForm.limit5h = 0;
  keyForm.limitDaily = 0;
  keyForm.dailyResetMode = 'fixed';
  keyForm.dailyResetTime = '00:00';
  keyForm.limitWeekly = 0;
  keyForm.limitMonthly = 0;
};

const copyAllKeys = async () => {
  try {
    if (!showRawKeys.value) {
      await loadData(true);
    }
    const lines = apiKeys.value.map((item) => `${item.name}: ${item.key}`);
    if (!lines.length) {
      message.warning('暂无可复制的 API Key');
      return;
    }
    if (!navigator?.clipboard?.writeText) {
      message.error('当前浏览器不支持剪贴板复制');
      return;
    }
    await navigator.clipboard.writeText(lines.join('\n'));
    message.success('API Key 已复制到剪贴板');
  } catch {
    message.error('复制 API Key 失败');
  }
};

const openEditModal = (record: ApiKeyRecord) => {
  editingKeyId.value = record.id;
  keyForm.name = record.name;
  keyForm.expiresAt = record.expiresAt === '-' ? '' : record.expiresAt.replace(' ', 'T').slice(0, 16);
  keyForm.limitTotal = record.limitTotal;
  keyForm.limit5h = record.limit5h;
  keyForm.limitDaily = record.limitDaily;
  keyForm.dailyResetMode = record.dailyResetMode || 'fixed';
  keyForm.dailyResetTime = record.dailyResetTime || '00:00';
  keyForm.limitWeekly = record.limitWeekly;
  keyForm.limitMonthly = record.limitMonthly;
  showCreateModal.value = true;
};

const submitKey = async () => {
  if (!keyForm.name.trim()) {
    message.warning('请填写 Key 名称');
    return;
  }
  loading.value = true;
  try {
    const payload = {
      name: keyForm.name.trim(),
      expiresAt: keyForm.expiresAt || undefined,
      limitTotal: keyForm.limitTotal,
      limit5h: keyForm.limit5h,
      limitDaily: keyForm.limitDaily,
      dailyResetMode: keyForm.dailyResetMode,
      dailyResetTime: keyForm.dailyResetTime,
      limitWeekly: keyForm.limitWeekly,
      limitMonthly: keyForm.limitMonthly,
    };
    if (editingKeyId.value) {
      await updateApiKey(editingKeyId.value, payload);
      message.success('API Key 更新成功');
    } else {
      await createApiKey(payload);
      message.success('API Key 创建成功');
    }
    resetKeyForm();
    showCreateModal.value = false;
    await loadData();
  } catch (error: unknown) {
    message.error(getErrorMessage(error, editingKeyId.value ? 'API Key 更新失败' : 'API Key 创建失败'));
  } finally {
    loading.value = false;
  }
};

const toggleStatus = async (id: string, checked: boolean) => {
  try {
    await updateApiKeyStatus(id, checked ? 'active' : 'disabled');
    message.success('状态更新成功');
    await loadData();
  } catch (error: unknown) {
    message.error(getErrorMessage(error, '状态更新失败'));
  }
};

const deleteKey = async (id: string) => {
  try {
    await removeApiKey(id);
    message.success('删除成功');
    await loadData();
  } catch (error: unknown) {
    message.error(getErrorMessage(error, '删除失败'));
  }
};

const openCreateModal = () => {
  resetKeyForm();
  showCreateModal.value = true;
};

onMounted(loadData);
</script>

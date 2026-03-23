<template>
  <div>
    <a-row :gutter="16">
      <a-col :xs="24" :md="6">
        <a-card><a-statistic title="今日调用量" :value="stats.todayCalls" /></a-card>
      </a-col>
      <a-col :xs="24" :md="6">
        <a-card><a-statistic title="今日费用" :value="Number(stats.todayCost || 0)" :precision="2" prefix="$" /></a-card>
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
          <a-button type="primary" @click="showCreateModal = true">新建 API Key</a-button>
        </a-space>
      </template>
      <a-table :columns="keyColumns" :data-source="apiKeys" row-key="id" :pagination="{ pageSize: 5 }">
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'status'">
            <a-switch :checked="record.status === 'active'" @change="(checked: boolean) => toggleStatus(record.id, checked)" />
          </template>
          <template v-if="column.dataIndex === 'operation'">
            <a-popconfirm title="确认删除这个 Key 吗？" @confirm="deleteKey(record.id)">
              <a-button danger type="link">删除</a-button>
            </a-popconfirm>
          </template>
        </template>
      </a-table>
    </a-card>

    <a-card title="费用明细" class="portal-card">
      <a-table :columns="costColumns" :data-source="costDetails" row-key="id" :pagination="{ pageSize: 5 }" />
    </a-card>

    <a-modal v-model:open="showCreateModal" title="新建 API Key" @ok="createKey" :confirm-loading="loading">
      <a-form layout="vertical">
        <a-form-item label="Key 名称" required>
          <a-input v-model:value="newKeyName" placeholder="例如：生产环境 Key" />
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
  updateApiKeyStatus,
} from '../api';
import type { ApiKeyRecord, CostDetailRecord, OpenStats } from '../types';
import { message } from 'ant-design-vue';
import type { TableColumnsType } from 'ant-design-vue';
import { onMounted, reactive, ref } from 'vue';

const stats = reactive<OpenStats>({
  todayCalls: 0,
  todayCost: '0',
  last7DaysCalls: 0,
  activeKeys: 0,
});

const apiKeys = ref<ApiKeyRecord[]>([]);
const costDetails = ref<CostDetailRecord[]>([]);
const showCreateModal = ref(false);
const newKeyName = ref('');
const loading = ref(false);
const showRawKeys = ref(false);

const keyColumns: TableColumnsType<ApiKeyRecord> = [
  { title: 'Key 名称', dataIndex: 'name' },
  { title: 'API Key', dataIndex: 'key' },
  { title: '创建时间', dataIndex: 'createdAt' },
  { title: '最近调用', dataIndex: 'lastUsed' },
  { title: '调用次数', dataIndex: 'totalCalls' },
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
    customRender: ({ value }) => `$${Number(value).toFixed(2)}`,
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
  } catch {
    message.error('开放平台数据加载失败');
  } finally {
    loading.value = false;
  }
};

const toggleShowAllKeys = async () => {
  await loadData(!showRawKeys.value);
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

const createKey = async () => {
  if (!newKeyName.value.trim()) {
    message.warning('请填写 Key 名称');
    return;
  }
  loading.value = true;
  try {
    await createApiKey({ name: newKeyName.value.trim() });
    message.success('API Key 创建成功');
    newKeyName.value = '';
    showCreateModal.value = false;
    await loadData();
  } catch {
    message.error('API Key 创建失败');
  } finally {
    loading.value = false;
  }
};

const toggleStatus = async (id: string, checked: boolean) => {
  try {
    await updateApiKeyStatus(id, checked ? 'active' : 'disabled');
    message.success('状态更新成功');
    await loadData();
  } catch {
    message.error('状态更新失败');
  }
};

const deleteKey = async (id: string) => {
  try {
    await removeApiKey(id);
    message.success('删除成功');
    await loadData();
  } catch {
    message.error('删除失败');
  }
};

onMounted(loadData);
</script>

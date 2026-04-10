<template>
  <section class="portal-page">
    <div class="portal-metric-strip">
      <div class="portal-metric">
        <div class="portal-metric__label">今日调用量</div>
        <div class="portal-metric__value">{{ stats.todayCalls }}</div>
      </div>
      <div class="portal-metric">
        <div class="portal-metric__label">今日费用</div>
        <div class="portal-metric__value">¥{{ Number(stats.todayCost || 0).toFixed(2) }}</div>
      </div>
      <div class="portal-metric">
        <div class="portal-metric__label">近 7 天调用量</div>
        <div class="portal-metric__value">{{ stats.last7DaysCalls }}</div>
      </div>
      <div class="portal-metric">
        <div class="portal-metric__label">启用 Key 数</div>
        <div class="portal-metric__value">{{ stats.activeKeys }}</div>
      </div>
    </div>

    <section class="portal-section">
      <div class="portal-section__header">
        <div>
          <div class="portal-section__eyebrow">API Keys</div>
          <h2 class="portal-section__title">API Key 管理</h2>
        </div>
        <div style="display: flex; flex-wrap: wrap; gap: 10px;">
          <a-button @click="toggleShowAllKeys">
            {{ showRawKeys ? '隐藏完整 Key' : '展示完整 API Key' }}
          </a-button>
          <a-button :disabled="!apiKeys.length" @click="copyAllKeys">复制全部 API Key</a-button>
          <a-button type="primary" @click="openCreateModal">新建 API Key</a-button>
        </div>
      </div>

      <div v-if="loading" class="portal-stack">
        <a-skeleton active :paragraph="{ rows: 4 }" />
      </div>
      <div v-else-if="!apiKeys.length" class="portal-empty">
        <div class="portal-empty__title">当前范围还没有 API Key</div>
      </div>
      <div v-else class="portal-stack">
        <article v-for="record in apiKeys" :key="record.id" class="portal-record">
          <div class="portal-record__header">
            <div>
              <div class="portal-record__title">{{ record.name || record.id }}</div>
              <div class="portal-record__subtitle">{{ record.id }}</div>
            </div>
            <span class="portal-status" :class="record.status === 'active' ? 'portal-status--success' : 'portal-status--danger'">
              {{ record.status === 'active' ? '启用中' : '已禁用' }}
            </span>
          </div>

          <div class="portal-copy-row">
            <code class="portal-inline-code">{{ record.key }}</code>
            <a-button @click="copyKey(record.key)">复制</a-button>
          </div>

          <div class="portal-data-grid">
            <div class="portal-data-item">
              <div class="portal-data-item__label">创建时间</div>
              <div class="portal-data-item__value">{{ formatDateTimeDisplay(record.createdAt) }}</div>
            </div>
            <div class="portal-data-item">
              <div class="portal-data-item__label">过期时间</div>
              <div class="portal-data-item__value">{{ formatDateTimeDisplay(record.expiresAt) }}</div>
            </div>
            <div class="portal-data-item">
              <div class="portal-data-item__label">最近调用</div>
              <div class="portal-data-item__value">{{ formatDateTimeDisplay(record.lastUsed) }}</div>
            </div>
            <div class="portal-data-item">
              <div class="portal-data-item__label">调用次数</div>
              <div class="portal-data-item__value">{{ record.totalCalls }}</div>
            </div>
            <div class="portal-data-item">
              <div class="portal-data-item__label">限额策略</div>
              <div class="portal-data-item__value">5h ¥{{ record.limit5h.toFixed(2) }} / 日 ¥{{ record.limitDaily.toFixed(2) }} / 周 ¥{{ record.limitWeekly.toFixed(2) }} / 月 ¥{{ record.limitMonthly.toFixed(2) }} / 总 ¥{{ record.limitTotal.toFixed(2) }}</div>
            </div>
            <div class="portal-data-item">
              <div class="portal-data-item__label">日重置</div>
              <div class="portal-data-item__value">{{ record.dailyResetMode || 'fixed' }} {{ record.dailyResetTime || '00:00' }}</div>
            </div>
          </div>

          <div style="display: flex; flex-wrap: wrap; gap: 10px; margin-top: 16px;">
            <a-button @click="openEditModal(record)">编辑</a-button>
            <a-button @click="toggleStatus(record.id, record.status !== 'active')">{{ record.status === 'active' ? '禁用' : '启用' }}</a-button>
            <a-popconfirm title="确认删除这个 Key 吗？" @confirm="deleteKey(record.id)">
              <a-button danger>删除</a-button>
            </a-popconfirm>
          </div>
        </article>
      </div>
    </section>

    <section class="portal-section">
      <div class="portal-section__header">
        <div>
          <div class="portal-section__eyebrow">Cost</div>
          <h2 class="portal-section__title">费用明细</h2>
        </div>
      </div>
      <div v-if="loading" class="portal-stack">
        <a-skeleton active :paragraph="{ rows: 4 }" />
      </div>
      <div v-else-if="!costDetails.length" class="portal-empty">
        <div class="portal-empty__title">还没有费用明细</div>
      </div>
      <div v-else class="portal-stack">
        <article v-for="record in costDetails" :key="record.id" class="portal-record">
          <div class="portal-record__header">
            <div>
              <div class="portal-record__title">{{ record.model }}</div>
              <div class="portal-record__subtitle">{{ record.date }}</div>
            </div>
            <span class="portal-pill">¥{{ Number(record.cost).toFixed(2) }}</span>
          </div>
          <div class="portal-data-grid">
            <div class="portal-data-item">
              <div class="portal-data-item__label">调用次数</div>
              <div class="portal-data-item__value">{{ record.calls }}</div>
            </div>
            <div class="portal-data-item">
              <div class="portal-data-item__label">总 Tokens</div>
              <div class="portal-data-item__value">{{ record.tokens }}</div>
            </div>
            <div class="portal-data-item">
              <div class="portal-data-item__label">记录 ID</div>
              <div class="portal-data-item__value portal-data-item__value--nowrap">{{ record.id }}</div>
            </div>
          </div>
        </article>
      </div>
    </section>

    <section class="portal-section">
      <div class="portal-section__header">
        <div>
          <div class="portal-section__eyebrow">Requests</div>
          <h2 class="portal-section__title">请求明细</h2>
        </div>
      </div>
      <div v-if="loading" class="portal-stack">
        <a-skeleton active :paragraph="{ rows: 4 }" />
      </div>
      <div v-else-if="!requestDetails.length" class="portal-empty">
        <div class="portal-empty__title">还没有请求明细</div>
      </div>
      <div v-else class="portal-stack">
        <article v-for="record in requestDetails" :key="record.eventId" class="portal-record">
          <div class="portal-record__header">
            <div>
              <div class="portal-record__title">{{ record.modelId || '未知模型' }}</div>
              <div class="portal-record__subtitle">{{ formatDateTimeDisplay(record.occurredAt) }}</div>
            </div>
            <span class="portal-pill">HTTP {{ record.httpStatus || 0 }}</span>
          </div>
          <PortalRecordScroller :items="buildRequestScrollerItems(record)" />
        </article>
      </div>
    </section>

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
  </section>
</template>

<script setup lang="ts">
import {
  createApiKey,
  fetchApiKeys,
  fetchCostDetails,
  fetchOpenStats,
  fetchRequestDetails,
  removeApiKey,
  updateApiKey,
  updateApiKeyStatus,
} from '../api';
import PortalRecordScroller, { type PortalRecordScrollerItem } from '../components/shell/PortalRecordScroller.vue';
import { useManagedAccountScope } from '../composables/useManagedAccountScope';
import type { ApiKeyRecord, CostDetailRecord, OpenStats, RequestDetailRecord } from '../types';
import { dateTimeLocalInputToISOString, formatDateTimeDisplay, toDateTimeLocalInputValue } from '../utils/time';
import { message } from 'ant-design-vue';
import { computed, onMounted, reactive, ref, watch } from 'vue';

const stats = reactive<OpenStats>({
  todayCalls: 0,
  todayCost: '0',
  last7DaysCalls: 0,
  activeKeys: 0,
});

const {
  activeConsumerName,
  loadManagedAccounts,
} = useManagedAccountScope();
const apiKeys = ref<ApiKeyRecord[]>([]);
const costDetails = ref<CostDetailRecord[]>([]);
const requestDetails = ref<RequestDetailRecord[]>([]);
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

const buildRequestScrollerItems = (record: RequestDetailRecord): PortalRecordScrollerItem[] => [
  { label: 'Request ID', value: record.requestId || '-', copyable: true, nowrap: true },
  { label: 'Trace ID', value: record.traceId || '-', copyable: true, nowrap: true },
  { label: 'API Key', value: record.apiKeyId || '-', copyable: true, nowrap: true },
  { label: 'Route', value: record.routeName || '-', nowrap: true },
  {
    label: '请求状态',
    value: record.requestStatus || '-',
    tone: record.requestStatus === 'success' ? 'success' : record.requestStatus === 'rejected' ? 'danger' : 'warning',
  },
  {
    label: '计量状态',
    value: record.usageStatus || '-',
    tone: record.usageStatus === 'parsed' ? 'success' : record.usageStatus === 'missing' ? 'warning' : 'default',
  },
  { label: '总 Tokens', value: record.totalTokens },
  { label: '费用', value: `¥${(Number(record.costMicroYuan) / 1_000_000).toFixed(4)}` },
  { label: 'Department Path', value: record.departmentPath || '-', nowrap: true },
  { label: 'Consumer', value: record.consumerName || '-', nowrap: true },
];

const getErrorMessage = (error: unknown, fallback: string) => {
  const maybeError = error as {
    response?: { data?: { message?: string; error?: string } };
    message?: string;
  };
  return maybeError?.response?.data?.message || maybeError?.response?.data?.error || maybeError?.message || fallback;
};

const copyKey = async (value: string) => {
  try {
    await navigator.clipboard.writeText(value);
    message.success('API Key 已复制');
  } catch {
    message.error('复制 API Key 失败');
  }
};

const loadData = async (includeRaw = showRawKeys.value) => {
  loading.value = true;
  try {
    const consumerName = activeConsumerName.value || undefined;
    const [statsRes, keyRes, costRes, requestRes] = await Promise.all([
      fetchOpenStats(consumerName),
      fetchApiKeys(includeRaw, consumerName),
      fetchCostDetails(consumerName),
      fetchRequestDetails({
        consumerName,
        pageNum: 1,
        pageSize: 50,
      }),
    ]);
    showRawKeys.value = includeRaw;
    stats.todayCalls = statsRes.todayCalls;
    stats.todayCost = statsRes.todayCost;
    stats.last7DaysCalls = statsRes.last7DaysCalls;
    stats.activeKeys = statsRes.activeKeys;
    apiKeys.value = keyRes;
    costDetails.value = costRes;
    requestDetails.value = requestRes;
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
    await navigator.clipboard.writeText(lines.join('\n'));
    message.success('API Key 已复制到剪贴板');
  } catch {
    message.error('复制 API Key 失败');
  }
};

const openEditModal = (record: ApiKeyRecord) => {
  editingKeyId.value = record.id;
  keyForm.name = record.name;
  keyForm.expiresAt = toDateTimeLocalInputValue(record.expiresAt);
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
      expiresAt: dateTimeLocalInputToISOString(keyForm.expiresAt),
      limitTotal: keyForm.limitTotal,
      limit5h: keyForm.limit5h,
      limitDaily: keyForm.limitDaily,
      dailyResetMode: keyForm.dailyResetMode,
      dailyResetTime: keyForm.dailyResetTime,
      limitWeekly: keyForm.limitWeekly,
      limitMonthly: keyForm.limitMonthly,
    };
    if (editingKeyId.value) {
      await updateApiKey(editingKeyId.value, payload, activeConsumerName.value || undefined);
      message.success('API Key 更新成功');
    } else {
      await createApiKey(payload, activeConsumerName.value || undefined);
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
    await updateApiKeyStatus(id, checked ? 'active' : 'disabled', activeConsumerName.value || undefined);
    message.success('状态更新成功');
    await loadData();
  } catch (error: unknown) {
    message.error(getErrorMessage(error, '状态更新失败'));
  }
};

const deleteKey = async (id: string) => {
  try {
    await removeApiKey(id, activeConsumerName.value || undefined);
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

<template>
  <div>
    <a-alert
      type="info"
      show-icon
      class="portal-card"
      message="当前页面仅部门管理员可见"
      description="这里展示当前管理员负责部门及其所有子部门的成员、余额和账单操作范围。"
    />

    <a-row :gutter="16">
      <a-col :xs="24" :md="8">
        <a-card>
          <a-statistic title="可管理账号" :value="managedAccounts.length" />
        </a-card>
      </a-col>
      <a-col :xs="24" :md="8">
        <a-card>
          <a-statistic title="账号总余额" :value="Number(totalBalance)" :precision="2" prefix="¥" />
        </a-card>
      </a-col>
      <a-col :xs="24" :md="8">
        <a-card>
          <a-statistic title="账号累计消费" :value="Number(totalConsumption)" :precision="2" prefix="¥" />
        </a-card>
      </a-col>
    </a-row>

    <a-card title="部门树" class="portal-card">
      <a-empty v-if="!managedDepartments.length && !pageLoading" description="暂无可管理部门" />
      <a-tree
        v-else
        :tree-data="departmentTreeData"
        default-expand-all
      />
    </a-card>

    <a-card title="部门成员列表" class="portal-card">
      <template #extra>
        <a-button @click="loadData" :loading="pageLoading">刷新</a-button>
      </template>

      <a-empty v-if="!managedAccounts.length && !pageLoading" description="当前部门下暂无可管理成员" />

      <a-table
        v-else
        :columns="columns"
        :data-source="managedAccounts"
        row-key="consumerName"
        :loading="pageLoading"
        :pagination="{ pageSize: 8 }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'account'">
            <div class="account-name">{{ record.displayName || record.consumerName }}</div>
            <div class="account-subtitle">{{ record.consumerName }}</div>
          </template>

          <template v-if="column.dataIndex === 'department'">
            <span>{{ record.departmentPath || '未分配部门' }}</span>
          </template>

          <template v-if="column.dataIndex === 'userLevel'">
            <a-tag color="blue">{{ formatUserLevel(record.userLevel) }}</a-tag>
          </template>

          <template v-if="column.dataIndex === 'status'">
            <a-tag :color="statusColor(record.status)">{{ formatStatus(record.status) }}</a-tag>
          </template>

          <template v-if="column.dataIndex === 'balance'">
            <span>¥{{ Number(record.balance || 0).toFixed(2) }}</span>
          </template>

          <template v-if="column.dataIndex === 'totalConsumption'">
            <span>¥{{ Number(record.totalConsumption || 0).toFixed(2) }}</span>
          </template>

          <template v-if="column.dataIndex === 'operation'">
            <a-space wrap>
              <a-button type="link" @click="openProfileModal(record)">账号设置</a-button>
              <a-button type="link" @click="openBalanceModal(record)">调整余额</a-button>
              <a-button type="link" @click="goBilling(record.consumerName)">账单</a-button>
              <a-button type="link" @click="goOpenPlatform(record.consumerName)">API Key</a-button>
            </a-space>
          </template>
        </template>
      </a-table>
    </a-card>

    <a-modal
      v-model:open="showProfileModal"
      title="更新账号设置"
      :confirm-loading="submittingProfile"
      @ok="submitProfile"
    >
      <a-form layout="vertical">
        <a-form-item label="账号">
          <a-input :value="currentAccountLabel" disabled />
        </a-form-item>
        <a-form-item label="用户等级" required>
          <a-select v-model:value="profileForm.userLevel" :options="userLevelOptions" />
        </a-form-item>
        <a-form-item label="账号状态" required>
          <a-select v-model:value="profileForm.status" :options="statusOptions" />
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal
      v-model:open="showBalanceModal"
      title="调整账号余额"
      :confirm-loading="submittingBalance"
      @ok="submitBalanceAdjustment"
    >
      <a-alert
        type="info"
        show-icon
        class="balance-alert"
        message="支持正负金额"
        description="输入正数表示增加余额，输入负数表示扣减余额。"
      />
      <a-form layout="vertical">
        <a-form-item label="账号">
          <a-input :value="currentAccountLabel" disabled />
        </a-form-item>
        <a-form-item label="调整金额（元）" required>
          <a-input-number v-model:value="balanceForm.amount" :precision="2" style="width: 100%" />
        </a-form-item>
        <a-form-item label="备注">
          <a-textarea v-model:value="balanceForm.reason" :rows="3" placeholder="例如：父账号补贴、人工扣减、月度激励" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { adjustManagedAccountBalance, fetchManagedDepartments, updateManagedAccount } from '../api';
import { useManagedAccountScope } from '../composables/useManagedAccountScope';
import type { ManagedAccountSummary, ManagedDepartmentNode } from '../types';
import { message } from 'ant-design-vue';
import type { TableColumnsType } from 'ant-design-vue';
import { computed, onMounted, reactive, ref } from 'vue';
import { useRouter } from 'vue-router';

const router = useRouter();
const {
  managedAccounts,
  loadManagedAccounts,
} = useManagedAccountScope();
const managedDepartments = ref<ManagedDepartmentNode[]>([]);

const pageLoading = ref(false);
const showProfileModal = ref(false);
const showBalanceModal = ref(false);
const submittingProfile = ref(false);
const submittingBalance = ref(false);
const currentAccount = ref<ManagedAccountSummary | null>(null);
const profileForm = reactive({
  userLevel: 'normal',
  status: 'active' as 'active' | 'disabled' | 'pending',
});
const balanceForm = reactive({
  amount: 0,
  reason: '',
});

const totalBalance = computed(() =>
  managedAccounts.value.reduce((sum, item) => sum + Number(item.balance || 0), 0).toFixed(2),
);
const totalConsumption = computed(() =>
  managedAccounts.value.reduce((sum, item) => sum + Number(item.totalConsumption || 0), 0).toFixed(2),
);
const currentAccountLabel = computed(() => {
  if (!currentAccount.value) {
    return '';
  }
  return `${currentAccount.value.displayName || currentAccount.value.consumerName}（${currentAccount.value.consumerName}）`;
});
const departmentTreeData = computed(() => {
  const build = (nodes: ManagedDepartmentNode[] = []) =>
    nodes.map((node) => ({
      key: node.departmentId,
      title: `${node.name} / 管理员：${node.adminConsumerName || '未设置'} / 成员：${node.memberCount}`,
      children: build(node.children || []),
    }));
  return build(managedDepartments.value);
});

const userLevelOptions = [
  { label: 'Normal', value: 'normal' },
  { label: 'Plus', value: 'plus' },
  { label: 'Pro', value: 'pro' },
  { label: 'Ultra', value: 'ultra' },
];

const statusOptions = [
  { label: '启用', value: 'active' },
  { label: '禁用', value: 'disabled' },
  { label: '待激活', value: 'pending' },
];

const columns: TableColumnsType<ManagedAccountSummary> = [
  { title: '成员账号', dataIndex: 'account' },
  { title: '邮箱', dataIndex: 'email' },
  { title: '所属部门', dataIndex: 'department' },
  { title: '用户等级', dataIndex: 'userLevel' },
  { title: '状态', dataIndex: 'status' },
  { title: '当前余额', dataIndex: 'balance' },
  { title: '累计消费', dataIndex: 'totalConsumption' },
  { title: '启用 Key', dataIndex: 'activeKeys' },
  { title: '操作', dataIndex: 'operation', width: 280 },
];

const getErrorMessage = (error: unknown, fallback: string) => {
  const maybeError = error as {
    response?: { data?: { message?: string; error?: string } };
    message?: string;
  };
  return maybeError?.response?.data?.message || maybeError?.response?.data?.error || maybeError?.message || fallback;
};

const formatUserLevel = (value: string) => {
  const normalized = String(value || '').toLowerCase();
  if (normalized === 'ultra') return 'Ultra';
  if (normalized === 'pro') return 'Pro';
  if (normalized === 'plus') return 'Plus';
  return 'Normal';
};

const formatStatus = (value: string) => {
  const normalized = String(value || '').toLowerCase();
  if (normalized === 'disabled') return '禁用';
  if (normalized === 'pending') return '待激活';
  return '启用';
};

const statusColor = (value: string) => {
  const normalized = String(value || '').toLowerCase();
  if (normalized === 'disabled') return 'red';
  if (normalized === 'pending') return 'gold';
  return 'green';
};

const loadData = async () => {
  pageLoading.value = true;
  try {
    const [, departments] = await Promise.all([
      loadManagedAccounts(),
      fetchManagedDepartments(),
    ]);
    managedDepartments.value = departments;
  } catch (error: unknown) {
    message.error(getErrorMessage(error, '部门管理数据加载失败'));
  } finally {
    pageLoading.value = false;
  }
};

const openProfileModal = (account: ManagedAccountSummary) => {
  currentAccount.value = account;
  profileForm.userLevel = account.userLevel;
  profileForm.status = account.status;
  showProfileModal.value = true;
};

const openBalanceModal = (account: ManagedAccountSummary) => {
  currentAccount.value = account;
  balanceForm.amount = 0;
  balanceForm.reason = '';
  showBalanceModal.value = true;
};

const submitProfile = async () => {
  if (!currentAccount.value) {
    return;
  }
  submittingProfile.value = true;
  try {
    await updateManagedAccount(currentAccount.value.consumerName, {
      userLevel: profileForm.userLevel,
      status: profileForm.status,
    });
    message.success('账号设置已更新');
    showProfileModal.value = false;
    await loadData();
  } catch (error: unknown) {
    message.error(getErrorMessage(error, '更新账号设置失败'));
  } finally {
    submittingProfile.value = false;
  }
};

const submitBalanceAdjustment = async () => {
  if (!currentAccount.value) {
    return;
  }
  if (!balanceForm.amount) {
    message.warning('请输入非 0 的调整金额');
    return;
  }
  submittingBalance.value = true;
  try {
    await adjustManagedAccountBalance(currentAccount.value.consumerName, {
      amount: balanceForm.amount,
      reason: balanceForm.reason.trim(),
    });
    message.success('余额调整成功');
    showBalanceModal.value = false;
    await loadData();
  } catch (error: unknown) {
    message.error(getErrorMessage(error, '余额调整失败'));
  } finally {
    submittingBalance.value = false;
  }
};

const goBilling = (consumerName: string) => {
  router.push({ path: '/billing', query: { consumerName } });
};

const goOpenPlatform = (consumerName: string) => {
  router.push({ path: '/open-platform', query: { consumerName } });
};

onMounted(loadData);
</script>

<style scoped>
.account-name {
  font-weight: 600;
}

.account-subtitle {
  color: #667085;
  font-size: 12px;
}

.balance-alert {
  margin-bottom: 16px;
}
</style>

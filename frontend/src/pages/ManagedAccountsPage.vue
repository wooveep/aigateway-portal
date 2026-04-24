<template>
  <section class="portal-page">
    <div class="portal-metric-strip">
      <div class="portal-metric">
        <div class="portal-metric__label">可管理账号</div>
        <div class="portal-metric__value">{{ managedAccounts.length }}</div>
      </div>
      <div class="portal-metric">
        <div class="portal-metric__label">账号总余额</div>
        <div class="portal-metric__value">¥{{ Number(totalBalance).toFixed(2) }}</div>
      </div>
      <div class="portal-metric">
        <div class="portal-metric__label">账号累计消费</div>
        <div class="portal-metric__value">¥{{ Number(totalConsumption).toFixed(2) }}</div>
      </div>
    </div>

    <section class="portal-section">
      <div class="portal-section__header">
        <div>
          <div class="portal-section__eyebrow">Department Tree</div>
          <h2 class="portal-section__title">部门树</h2>
        </div>
      </div>
      <a-empty v-if="!managedDepartments.length && !pageLoading" description="暂无可管理部门" />
      <a-tree
        v-else
        :tree-data="departmentTreeData"
        default-expand-all
      />
    </section>

    <section class="portal-section">
      <div class="portal-section__header">
        <div>
          <div class="portal-section__eyebrow">Members</div>
          <h2 class="portal-section__title">部门成员列表</h2>
        </div>
        <div style="display: flex; flex-wrap: wrap; gap: 8px;">
          <a-button type="primary" @click="openCreateModal">新建成员</a-button>
          <a-button @click="loadData" :loading="pageLoading">刷新</a-button>
        </div>
      </div>

      <a-empty v-if="!managedAccounts.length && !pageLoading" description="当前部门下暂无可管理成员" />
      <div v-else class="portal-stack">
        <article v-for="record in managedAccounts" :key="record.consumerName" class="portal-record">
          <div class="portal-record__header">
            <div>
              <div class="portal-record__title">{{ record.displayName || record.consumerName }}</div>
              <div class="portal-record__subtitle">{{ record.consumerName }} · {{ record.email || '未设置邮箱' }}</div>
            </div>
            <div style="display: flex; flex-wrap: wrap; gap: 8px;">
              <span class="portal-status portal-status--success">{{ formatUserLevel(record.userLevel) }}</span>
              <span class="portal-status" :class="statusClass(record.status)">{{ formatStatus(record.status) }}</span>
            </div>
          </div>

          <div class="portal-data-grid">
            <div class="portal-data-item">
              <div class="portal-data-item__label">所属部门</div>
              <div class="portal-data-item__value">{{ record.departmentName || '未分配部门' }}</div>
            </div>
            <div class="portal-data-item">
              <div class="portal-data-item__label">Department Path</div>
              <div class="portal-data-item__value portal-data-item__value--nowrap">{{ record.departmentPath || '未分配部门' }}</div>
            </div>
            <div class="portal-data-item">
              <div class="portal-data-item__label">当前余额</div>
              <div class="portal-data-item__value">¥{{ Number(record.balance || 0).toFixed(2) }}</div>
            </div>
            <div class="portal-data-item">
              <div class="portal-data-item__label">累计消费</div>
              <div class="portal-data-item__value">¥{{ Number(record.totalConsumption || 0).toFixed(2) }}</div>
            </div>
            <div class="portal-data-item">
              <div class="portal-data-item__label">启用 Key</div>
              <div class="portal-data-item__value">{{ record.activeKeys }}</div>
            </div>
          </div>

          <div style="display: flex; flex-wrap: wrap; gap: 10px; margin-top: 16px;">
            <a-button @click="openProfileModal(record)">账号设置</a-button>
            <a-button @click="openBalanceModal(record)">转账余额</a-button>
            <a-button @click="goBilling(record.consumerName)">账单</a-button>
            <a-button @click="goOpenPlatform(record.consumerName)">API Key</a-button>
          </div>
        </article>
      </div>
    </section>

    <a-modal
      v-model:open="showCreateModal"
      title="新建部门成员"
      :confirm-loading="submittingCreate"
      @ok="submitCreate"
    >
      <div class="portal-callout" style="margin-bottom: 16px;">
        新账号会直接加入你当前所在部门。密码留空时，系统会生成临时密码。
      </div>
      <a-form layout="vertical">
        <a-form-item label="账号" required>
          <a-input v-model:value="createForm.consumerName" />
        </a-form-item>
        <a-form-item label="显示名" required>
          <a-input v-model:value="createForm.displayName" />
        </a-form-item>
        <a-form-item label="邮箱">
          <a-input v-model:value="createForm.email" />
        </a-form-item>
        <a-form-item label="初始密码">
          <a-input-password v-model:value="createForm.password" placeholder="留空则自动生成临时密码" />
        </a-form-item>
      </a-form>
    </a-modal>

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
      title="成员余额转账"
      :confirm-loading="submittingBalance"
      @ok="submitBalanceAdjustment"
    >
      <div class="portal-callout" style="margin-bottom: 16px;">
        管理员当前可用余额：¥{{ adminBalance.toFixed(2) }}。输入正数会从管理员余额转给成员，输入负数会从成员余额退回管理员。
      </div>
      <a-form layout="vertical">
        <a-form-item label="账号">
          <a-input :value="currentAccountLabel" disabled />
        </a-form-item>
        <a-form-item label="转账金额（元）" required>
          <a-input-number v-model:value="balanceForm.amount" :precision="2" style="width: 100%" />
        </a-form-item>
        <a-form-item label="备注">
          <a-textarea v-model:value="balanceForm.reason" :rows="3" placeholder="例如：部门预算划拨、余额回收、月度激励" />
        </a-form-item>
      </a-form>
    </a-modal>
  </section>
</template>

<script setup lang="ts">
import { adjustManagedAccountBalance, createManagedAccount, fetchBillingOverview, fetchManagedDepartments, updateManagedAccount } from '../api';
import { useManagedAccountScope } from '../composables/useManagedAccountScope';
import type { ManagedAccountSummary, ManagedDepartmentNode } from '../types';
import { copyTextToClipboard } from '../utils/clipboard';
import { message } from 'ant-design-vue';
import { computed, onMounted, reactive, ref } from 'vue';
import { useRouter } from 'vue-router';

const router = useRouter();
const {
  managedAccounts,
  loadManagedAccounts,
} = useManagedAccountScope();
const managedDepartments = ref<ManagedDepartmentNode[]>([]);

const pageLoading = ref(false);
const showCreateModal = ref(false);
const showProfileModal = ref(false);
const showBalanceModal = ref(false);
const submittingCreate = ref(false);
const submittingProfile = ref(false);
const submittingBalance = ref(false);
const currentAccount = ref<ManagedAccountSummary | null>(null);
const adminBalance = ref(0);
const createForm = reactive({
  consumerName: '',
  displayName: '',
  email: '',
  password: '',
});
const profileForm = reactive({
  userLevel: 'normal',
  status: 'active' as 'active' | 'disabled' | 'pending',
});
const balanceForm = reactive({
  amount: 0,
  reason: '',
});

type DepartmentTreeItem = {
  key: string;
  title: string;
  children: DepartmentTreeItem[];
};

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
  const build = (nodes: ManagedDepartmentNode[] = []): DepartmentTreeItem[] =>
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

const statusClass = (value: string) => {
  const normalized = String(value || '').toLowerCase();
  if (normalized === 'disabled') return 'portal-status--danger';
  if (normalized === 'pending') return 'portal-status--warning';
  return 'portal-status--success';
};

const loadData = async () => {
  pageLoading.value = true;
  try {
    const [, departments, overview] = await Promise.all([
      loadManagedAccounts(),
      fetchManagedDepartments(),
      fetchBillingOverview(),
    ]);
    managedDepartments.value = departments;
    adminBalance.value = Number(overview.balance || 0);
  } catch (error: unknown) {
    message.error(getErrorMessage(error, '部门管理数据加载失败'));
  } finally {
    pageLoading.value = false;
  }
};

const openCreateModal = () => {
  createForm.consumerName = '';
  createForm.displayName = '';
  createForm.email = '';
  createForm.password = '';
  showCreateModal.value = true;
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

const submitCreate = async () => {
  const consumerName = createForm.consumerName.trim();
  const displayName = createForm.displayName.trim();
  if (!consumerName || !displayName) {
    message.warning('请填写账号和显示名');
    return;
  }

  submittingCreate.value = true;
  try {
    const response = await createManagedAccount({
      consumerName,
      displayName,
      email: createForm.email.trim() || undefined,
      password: createForm.password || undefined,
    });
    if (response.tempPassword) {
      try {
        await copyTextToClipboard(response.tempPassword);
        message.success(`成员已创建，临时密码已复制：${response.tempPassword}`);
      } catch {
        message.success(`成员已创建，临时密码：${response.tempPassword}`);
      }
    } else {
      message.success('成员已创建');
    }
    showCreateModal.value = false;
    await loadData();
  } catch (error: unknown) {
    message.error(getErrorMessage(error, '创建成员失败'));
  } finally {
    submittingCreate.value = false;
  }
};

const submitBalanceAdjustment = async () => {
  if (!currentAccount.value) {
    return;
  }
  if (!balanceForm.amount) {
    message.warning('请输入非 0 的转账金额');
    return;
  }
  submittingBalance.value = true;
  try {
    await adjustManagedAccountBalance(currentAccount.value.consumerName, {
      amount: balanceForm.amount,
      reason: balanceForm.reason.trim(),
    });
    message.success('余额转账成功');
    showBalanceModal.value = false;
    await loadData();
  } catch (error: unknown) {
    message.error(getErrorMessage(error, '余额转账失败'));
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

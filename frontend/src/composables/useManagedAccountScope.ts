import { computed, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { fetchManagedAccounts } from '../api';
import { authState } from '../auth';
import type { ManagedAccountSummary } from '../types';

export function useManagedAccountScope() {
  const route = useRoute();
  const router = useRouter();
  const managedAccounts = ref<ManagedAccountSummary[]>([]);
  const loadingManagedAccounts = ref(false);

  const selectedConsumerName = computed(() => {
    const value = route.query.consumerName;
    return typeof value === 'string' ? value : '';
  });

  const currentManagedAccount = computed(() => {
    if (!selectedConsumerName.value) {
      return null;
    }
    return managedAccounts.value.find((item) => item.consumerName === selectedConsumerName.value) ?? null;
  });

  const activeConsumerName = computed(() => currentManagedAccount.value?.consumerName || '');
  const hasManagedAccounts = computed(() => managedAccounts.value.length > 0);
  const canManageDepartment = computed(() => !!authState.user?.isDepartmentAdmin);
  const currentScopeTitle = computed(() => {
    if (currentManagedAccount.value) {
      return `${currentManagedAccount.value.displayName || currentManagedAccount.value.consumerName}（${currentManagedAccount.value.consumerName}）`;
    }
    const currentUser = authState.user;
    if (!currentUser) {
      return '当前账号';
    }
    return `${currentUser.displayName || currentUser.consumerName}（${currentUser.consumerName}）`;
  });

  const scopeOptions = computed(() => {
    const currentUser = authState.user;
    const options: Array<{ label: string; value: string }> = [];
    if (currentUser) {
      options.push({
        label: `当前账号：${currentUser.displayName || currentUser.consumerName}`,
        value: '',
      });
    }
    for (const item of managedAccounts.value) {
      options.push({
        label: `${item.displayName || item.consumerName} / ${item.consumerName}`,
        value: item.consumerName,
      });
    }
    return options;
  });

  const updateScopeConsumerName = async (consumerName: string) => {
    const query = { ...route.query };
    if (consumerName) {
      query.consumerName = consumerName;
    } else {
      delete query.consumerName;
    }
    await router.replace({ path: route.path, query });
  };

  const loadManagedAccounts = async () => {
    if (!canManageDepartment.value) {
      managedAccounts.value = [];
      return;
    }
    loadingManagedAccounts.value = true;
    try {
      const accounts = await fetchManagedAccounts();
      managedAccounts.value = accounts;
      if (
        selectedConsumerName.value &&
        !accounts.some((item) => item.consumerName === selectedConsumerName.value)
      ) {
        await updateScopeConsumerName('');
      }
    } finally {
      loadingManagedAccounts.value = false;
    }
  };

  return {
    managedAccounts,
    loadingManagedAccounts,
    selectedConsumerName,
    currentManagedAccount,
    activeConsumerName,
    hasManagedAccounts,
    canManageDepartment,
    currentScopeTitle,
    scopeOptions,
    loadManagedAccounts,
    updateScopeConsumerName,
  };
}

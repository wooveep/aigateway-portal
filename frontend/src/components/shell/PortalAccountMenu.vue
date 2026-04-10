<script setup lang="ts">
import {
  DownOutlined,
  LockOutlined,
  LogoutOutlined,
  UserOutlined,
} from '@ant-design/icons-vue';
import { ref } from 'vue';

const props = withDefaults(defineProps<{
  displayName: string;
  metaLine: string;
  initial?: string;
  disableChangePassword?: boolean;
}>(), {
  initial: '',
  disableChangePassword: false,
});

const emit = defineEmits<{
  changePassword: [];
  logout: [];
}>();

const open = ref(false);

const handleOpenChange = (value: boolean) => {
  open.value = value;
};

const handleAction = (action: 'changePassword' | 'logout') => {
  open.value = false;
  if (action === 'changePassword') {
    emit('changePassword');
    return;
  }
  emit('logout');
};
</script>

<template>
  <a-dropdown
    :open="open"
    :trigger="['click']"
    placement="bottomRight"
    @open-change="handleOpenChange"
  >
    <button class="portal-account-trigger" type="button">
      <span class="portal-account-trigger__avatar">
        <UserOutlined v-if="!initial" />
        <template v-else>{{ initial }}</template>
      </span>
      <span class="portal-account-trigger__name">{{ displayName }}</span>
      <DownOutlined class="portal-account-trigger__arrow" />
    </button>

    <template #overlay>
      <div class="portal-account-panel">
        <div class="portal-account-panel__header">
          <div class="portal-account-panel__name">{{ displayName }}</div>
          <div class="portal-account-panel__meta">{{ metaLine }}</div>
        </div>

        <button
          class="portal-account-panel__action"
          type="button"
          :disabled="disableChangePassword"
          @click="handleAction('changePassword')"
        >
          <LockOutlined />
          <span>修改密码</span>
        </button>

        <button
          class="portal-account-panel__action"
          type="button"
          @click="handleAction('logout')"
        >
          <LogoutOutlined />
          <span>退出登录</span>
        </button>
      </div>
    </template>
  </a-dropdown>
</template>

<style scoped>
.portal-account-trigger {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  height: 40px;
  padding: 0 12px 0 8px;
  border: 1px solid var(--portal-border);
  border-radius: 999px;
  background: var(--portal-surface);
  color: var(--portal-text-primary);
  cursor: pointer;
  transition: border-color 0.18s ease, background-color 0.18s ease, box-shadow 0.18s ease;
}

.portal-account-trigger:hover {
  border-color: var(--portal-border-strong);
  background: var(--portal-surface-raised);
  box-shadow: 0 8px 18px rgba(15, 23, 42, 0.06);
}

.portal-account-trigger__avatar {
  width: 28px;
  height: 28px;
  display: grid;
  place-items: center;
  border-radius: 999px;
  background: linear-gradient(135deg, #0f172a 0%, #1d4f72 100%);
  color: #f8fafc;
  font-size: 12px;
  font-weight: 700;
  flex-shrink: 0;
}

.portal-account-trigger__name {
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
  font-weight: 600;
}

.portal-account-trigger__arrow {
  color: var(--portal-text-muted);
  font-size: 12px;
}

.portal-account-panel {
  width: 260px;
  padding: 8px;
  border: 1px solid var(--portal-border);
  border-radius: 16px;
  background: var(--portal-surface);
  box-shadow: var(--portal-shadow-deep);
}

.portal-account-panel__header {
  padding: 10px 12px 12px;
  border-bottom: 1px solid var(--portal-border);
  margin-bottom: 6px;
}

.portal-account-panel__name {
  font-size: 14px;
  font-weight: 600;
  color: var(--portal-text-primary);
}

.portal-account-panel__meta {
  margin-top: 4px;
  color: var(--portal-text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.portal-account-panel__action {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  height: 40px;
  padding: 0 12px;
  border: 0;
  border-radius: 10px;
  background: transparent;
  color: var(--portal-text-primary);
  cursor: pointer;
  transition: background-color 0.18s ease, color 0.18s ease;
}

.portal-account-panel__action:hover:not(:disabled) {
  background: var(--portal-surface-raised);
}

.portal-account-panel__action:disabled {
  color: var(--portal-text-muted);
  cursor: not-allowed;
}

@media (max-width: 767px) {
  .portal-account-trigger {
    width: 40px;
    justify-content: center;
    padding: 0;
  }

  .portal-account-trigger__name,
  .portal-account-trigger__arrow {
    display: none;
  }
}
</style>

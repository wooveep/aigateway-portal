<script setup lang="ts">
import {
  AppstoreOutlined,
  CommentOutlined,
  FileTextOutlined,
  KeyOutlined,
  LockOutlined,
  RobotOutlined,
  TeamOutlined,
  WalletOutlined,
} from '@ant-design/icons-vue';
import type { MenuProps } from 'ant-design-vue';
import { message } from 'ant-design-vue';
import { computed, h } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { logout } from './api';
import { authState, clearAuth } from './auth';

const route = useRoute();
const router = useRouter();

const isPublicPage = computed(() => route.path === '/login' || route.path === '/register');
const selectedKeys = computed(() => [route.path]);
const isChangePasswordPage = computed(() => route.path === '/change-password');

const menuItems = computed<MenuProps['items']>(() => {
  const items: MenuProps['items'] = [
    {
      key: '/billing',
      icon: () => h(WalletOutlined),
      label: '个人账单',
    },
    {
      key: '/ai-chat',
      icon: () => h(CommentOutlined),
      label: 'AI 对话',
    },
    {
      key: '/models',
      icon: () => h(AppstoreOutlined),
      label: '模型广场',
    },
    {
      key: '/agents',
      icon: () => h(RobotOutlined),
      label: '智能体广场',
    },
    {
      key: '/open-platform',
      icon: () => h(KeyOutlined),
      label: '开放平台',
    },
    {
      key: '/invoices',
      icon: () => h(FileTextOutlined),
      label: '发票管理',
    },
  ];
  if (authState.user?.isDepartmentAdmin) {
    items.push({
      key: '/accounts',
      icon: () => h(TeamOutlined),
      label: '部门管理',
    });
  }
  return items;
});

const formatUserLevel = (value?: string) => {
  const level = String(value || '').toLowerCase();
  if (level === 'ultra') return 'Ultra';
  if (level === 'pro') return 'Pro';
  if (level === 'plus') return 'Plus';
  return 'Normal';
};

const handleMenuClick: MenuProps['onClick'] = ({ key }) => {
  router.push(String(key));
};

const goChangePassword = () => {
  router.push('/change-password');
};

const onLogout = async () => {
  try {
    await logout();
  } catch {
    // ignored
  }
  clearAuth();
  message.success('已退出登录');
  router.push('/login');
};
</script>

<template>
  <router-view v-if="isPublicPage" />

  <div v-else class="portal-shell">
    <aside class="portal-shell__sidebar">
      <div class="portal-shell__brand">
        <div class="portal-shell__brand-mark">HG</div>
        <div>
          <div class="portal-shell__brand-title">AIGateway Portal</div>
          <div class="portal-shell__brand-subtitle">Warm terminal for AI access</div>
        </div>
      </div>

      <a-menu
        mode="inline"
        :selected-keys="selectedKeys"
        :items="menuItems"
        class="portal-shell__menu"
        @click="handleMenuClick"
      />
    </aside>

    <main class="portal-shell__main">
      <header class="portal-shell__header">
        <div>
          <div class="portal-shell__eyebrow">Enterprise AI Open Platform</div>
          <div class="portal-shell__header-title">统一管理模型、智能体、会话与 API Key</div>
        </div>

        <div class="portal-shell__header-actions">
          <span class="portal-shell__pill">{{ authState.user?.displayName || authState.user?.consumerName }}</span>
          <span class="portal-shell__pill">Level {{ formatUserLevel(authState.user?.userLevel) }}</span>
          <a-button type="text" :disabled="isChangePasswordPage" @click="goChangePassword">
            <LockOutlined />
            修改密码
          </a-button>
          <a-button type="primary" @click="onLogout">退出登录</a-button>
        </div>
      </header>

      <section class="portal-shell__content">
        <router-view />
      </section>
    </main>
  </div>
</template>

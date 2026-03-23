<template>
  <router-view v-if="isPublicPage" />

  <a-layout v-else class="portal-layout">
    <a-layout-sider breakpoint="lg" collapsed-width="0">
      <div class="logo">AIGateway 用户门户</div>
      <a-menu
        theme="dark"
        mode="inline"
        :selected-keys="selectedKeys"
        :items="menuItems"
        @click="handleMenuClick"
      />
    </a-layout-sider>

    <a-layout>
      <a-layout-header class="header">
        <div class="header-title">企业 AI 开放平台</div>
        <div class="header-user">
          <a-tag color="blue">{{ authState.user?.displayName || authState.user?.consumerName }}</a-tag>
          <a-button type="link" @click="onLogout">退出登录</a-button>
        </div>
      </a-layout-header>
      <a-layout-content class="content">
        <router-view />
      </a-layout-content>
    </a-layout>
  </a-layout>
</template>

<script setup lang="ts">
import { AppstoreOutlined, FileTextOutlined, KeyOutlined, WalletOutlined } from '@ant-design/icons-vue';
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

const menuItems: MenuProps['items'] = [
  {
    key: '/billing',
    icon: () => h(WalletOutlined),
    label: '个人账单',
  },
  {
    key: '/models',
    icon: () => h(AppstoreOutlined),
    label: '模型广场',
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

const handleMenuClick: MenuProps['onClick'] = ({ key }) => {
  router.push(String(key));
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

<script setup lang="ts">
import {
  AppstoreOutlined,
  CommentOutlined,
  FileTextOutlined,
  KeyOutlined,
  MenuOutlined,
  RobotOutlined,
  TeamOutlined,
  WalletOutlined,
} from '@ant-design/icons-vue';
import type { MenuProps } from 'ant-design-vue';
import { message } from 'ant-design-vue';
import { computed, h, onMounted, onUnmounted, shallowRef, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { logout } from './api';
import { authState, clearAuth } from './auth';
import ChangePasswordForm from './components/shell/ChangePasswordForm.vue';
import PortalAccountMenu from './components/shell/PortalAccountMenu.vue';
import PortalNav from './components/shell/PortalNav.vue';

interface NavigationItem {
  key: string;
  label: string;
  icon: () => ReturnType<typeof h>;
  adminOnly?: boolean;
}

const route = useRoute();
const router = useRouter();

const viewportWidth = shallowRef(typeof window === 'undefined' ? 1440 : window.innerWidth);
const mobileNavOpen = shallowRef(false);
const passwordModalOpen = shallowRef(false);

const isPublicPage = computed(() => route.path === '/login' || route.path === '/register');
const isMobile = computed(() => viewportWidth.value < 768);
const isTablet = computed(() => viewportWidth.value >= 768 && viewportWidth.value < 1200);
const selectedKeys = computed(() => [route.path]);

const navigationItems = computed<NavigationItem[]>(() => {
  const items: NavigationItem[] = [
    { key: '/billing', label: '个人账单', icon: () => h(WalletOutlined) },
    { key: '/ai-chat', label: 'AI 对话', icon: () => h(CommentOutlined) },
    { key: '/models', label: '模型广场', icon: () => h(AppstoreOutlined) },
    { key: '/agents', label: '智能体广场', icon: () => h(RobotOutlined) },
    { key: '/open-platform', label: '开放平台', icon: () => h(KeyOutlined) },
    { key: '/invoices', label: '发票管理', icon: () => h(FileTextOutlined) },
    { key: '/accounts', label: '部门管理', icon: () => h(TeamOutlined), adminOnly: true },
  ];

  return items.filter((item) => !item.adminOnly || authState.user?.isDepartmentAdmin);
});

const menuItems = computed<MenuProps['items']>(() =>
  navigationItems.value.map((item) => ({
    key: item.key,
    icon: item.icon,
    label: item.label,
  })),
);

const currentRouteLabel = computed(() => {
  if (route.path === '/change-password') {
    return '修改密码';
  }
  return navigationItems.value.find((item) => item.key === route.path)?.label || 'AIGateway';
});

const formatUserLevel = (value?: string) => {
  const level = String(value || '').toLowerCase();
  if (level === 'ultra') return 'Ultra';
  if (level === 'pro') return 'Pro';
  if (level === 'plus') return 'Plus';
  return 'Normal';
};

const userDisplayName = computed(() => authState.user?.displayName || authState.user?.consumerName || 'Portal User');
const userMetaLine = computed(() => {
  const consumerName = authState.user?.consumerName || '-';
  return `${consumerName} · Level ${formatUserLevel(authState.user?.userLevel)}`;
});
const userInitial = computed(() => userDisplayName.value.slice(0, 1).toUpperCase());

const updateViewport = () => {
  viewportWidth.value = window.innerWidth;
  if (window.innerWidth >= 768) {
    mobileNavOpen.value = false;
  }
};

const handleNavigate = (key: string) => {
  router.push(key);
};

const openPasswordModal = () => {
  passwordModalOpen.value = true;
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

const handlePasswordSuccess = async () => {
  passwordModalOpen.value = false;
  clearAuth();
  await router.replace('/login');
};

watch(() => route.path, () => {
  mobileNavOpen.value = false;
});

onMounted(() => {
  updateViewport();
  window.addEventListener('resize', updateViewport);
});

onUnmounted(() => {
  window.removeEventListener('resize', updateViewport);
});
</script>

<template>
  <router-view v-if="isPublicPage" />

  <div
    v-else
    class="portal-shell"
    :class="{
      'portal-shell--tablet': isTablet,
      'portal-shell--mobile': isMobile,
    }"
  >
    <PortalNav
      v-if="!isMobile"
      :items="menuItems"
      :selected-keys="selectedKeys"
      :collapsed="isTablet"
      @navigate="handleNavigate"
    />

    <PortalNav
      v-else
      mobile
      :open="mobileNavOpen"
      :items="menuItems"
      :selected-keys="selectedKeys"
      @navigate="handleNavigate"
      @update:open="mobileNavOpen = $event"
    />

    <main class="portal-shell__main">
      <header class="portal-shell__header">
        <div class="portal-shell__header-main">
          <button
            v-if="isMobile"
            class="portal-shell__nav-trigger"
            type="button"
            @click="mobileNavOpen = true"
          >
            <MenuOutlined />
          </button>

          <div class="portal-shell__header-copy">
            <div class="portal-shell__header-title">{{ currentRouteLabel }}</div>
          </div>
        </div>

        <PortalAccountMenu
          :display-name="userDisplayName"
          :meta-line="userMetaLine"
          :initial="userInitial"
          @change-password="openPasswordModal"
          @logout="onLogout"
        />
      </header>

      <section class="portal-shell__content">
        <router-view />
      </section>
    </main>

    <a-modal
      v-model:open="passwordModalOpen"
      title="修改密码"
      :footer="null"
      :width="480"
      destroy-on-close
    >
      <ChangePasswordForm
        :username="authState.user?.consumerName"
        @cancel="passwordModalOpen = false"
        @success="handlePasswordSuccess"
      />
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { CloseOutlined } from '@ant-design/icons-vue';
import type { MenuProps } from 'ant-design-vue';

const props = withDefaults(defineProps<{
  items: MenuProps['items'];
  selectedKeys: string[];
  collapsed?: boolean;
  mobile?: boolean;
  open?: boolean;
}>(), {
  collapsed: false,
  mobile: false,
  open: false,
});

const emit = defineEmits<{
  navigate: [key: string];
  'update:open': [value: boolean];
}>();

const handleMenuClick: MenuProps['onClick'] = ({ key }) => {
  emit('navigate', String(key));
  if (props.mobile) {
    emit('update:open', false);
  }
};

const closeDrawer = () => {
  emit('update:open', false);
};
</script>

<template>
  <a-drawer
    v-if="mobile"
    :open="open"
    placement="left"
    :closable="false"
    width="280"
    class="portal-nav-drawer"
    @close="closeDrawer"
  >
    <div class="portal-nav__drawer-header">
      <div class="portal-nav__brand">
        <div class="portal-nav__brand-mark">AG</div>
        <div class="portal-nav__brand-copy">
          <div class="portal-nav__brand-title">AIGateway</div>
        </div>
      </div>

      <button class="portal-nav__drawer-close" type="button" @click="closeDrawer">
        <CloseOutlined />
      </button>
    </div>

    <a-menu
      mode="inline"
      :selected-keys="selectedKeys"
      :items="items"
      class="portal-nav__menu"
      @click="handleMenuClick"
    />
  </a-drawer>

  <aside v-else class="portal-nav" :class="{ 'portal-nav--collapsed': collapsed }">
    <div class="portal-nav__brand">
      <div class="portal-nav__brand-mark">AG</div>
      <div v-if="!collapsed" class="portal-nav__brand-copy">
        <div class="portal-nav__brand-title">AIGateway</div>
      </div>
    </div>

    <a-menu
      mode="inline"
      :inline-collapsed="collapsed"
      :selected-keys="selectedKeys"
      :items="items"
      class="portal-nav__menu"
      @click="handleMenuClick"
    />
  </aside>
</template>

<style scoped>
.portal-nav {
  display: flex;
  flex-direction: column;
  gap: 18px;
  height: 100%;
  min-height: calc(100vh - 32px);
  padding: 18px 14px;
  border: 1px solid var(--portal-border);
  border-radius: 20px;
  background: var(--portal-surface);
  box-shadow: var(--portal-shadow-standard);
}

.portal-nav--collapsed {
  align-items: center;
  padding-inline: 10px;
}

.portal-nav__brand,
.portal-nav__drawer-header {
  display: flex;
  align-items: center;
}

.portal-nav__brand {
  gap: 12px;
  min-height: 48px;
}

.portal-nav__drawer-header {
  justify-content: space-between;
  margin-bottom: 20px;
}

.portal-nav__brand-mark {
  width: 40px;
  height: 40px;
  display: grid;
  place-items: center;
  border-radius: 12px;
  background: linear-gradient(135deg, #0f172a 0%, #1e3a5f 100%);
  color: #f8fafc;
  font-size: 15px;
  font-weight: 700;
  letter-spacing: 0.08em;
  flex-shrink: 0;
}

.portal-nav__brand-copy {
  min-width: 0;
}

.portal-nav__brand-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--portal-text-primary);
  letter-spacing: -0.02em;
}

.portal-nav__menu {
  flex: 1;
  width: 100%;
}

.portal-nav__drawer-close {
  width: 36px;
  height: 36px;
  display: grid;
  place-items: center;
  border: 1px solid var(--portal-border);
  border-radius: 10px;
  background: var(--portal-surface);
  color: var(--portal-text-secondary);
  cursor: pointer;
  transition: border-color 0.18s ease, color 0.18s ease, background-color 0.18s ease;
}

.portal-nav__drawer-close:hover {
  border-color: var(--portal-border-strong);
  color: var(--portal-text-primary);
  background: var(--portal-surface-raised);
}
</style>

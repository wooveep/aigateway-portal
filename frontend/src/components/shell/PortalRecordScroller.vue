<script setup lang="ts">
import { CopyOutlined } from '@ant-design/icons-vue';
import { message } from 'ant-design-vue';

export interface PortalRecordScrollerItem {
  label: string;
  value: string | number;
  copyable?: boolean;
  nowrap?: boolean;
  tone?: 'default' | 'success' | 'warning' | 'danger';
}

const props = defineProps<{
  items: PortalRecordScrollerItem[];
}>();

const copyValue = async (value: string | number) => {
  try {
    await navigator.clipboard.writeText(String(value ?? ''));
    message.success('已复制');
  } catch {
    message.error('复制失败');
  }
};

const toneClass = (tone?: PortalRecordScrollerItem['tone']) => {
  if (!tone || tone === 'default') {
    return '';
  }
  return `portal-record-scroller__value--${tone}`;
};
</script>

<template>
  <div class="portal-record-scroller">
    <article
      v-for="item in props.items"
      :key="item.label"
      class="portal-record-scroller__item"
    >
      <div class="portal-data-item__label">{{ item.label }}</div>

      <div class="portal-record-scroller__value" :class="[toneClass(item.tone), { 'portal-record-scroller__value--nowrap': item.nowrap }]">
        <span class="portal-record-scroller__text" :title="String(item.value ?? '-')">
          {{ item.value ?? '-' }}
        </span>

        <button
          v-if="item.copyable"
          class="portal-record-scroller__copy"
          type="button"
          @click="copyValue(item.value)"
        >
          <CopyOutlined />
        </button>
      </div>
    </article>
  </div>
</template>

<style scoped>
.portal-record-scroller {
  display: grid;
  width: 100%;
  max-width: 100%;
  min-width: 0;
  grid-auto-flow: column;
  grid-auto-columns: minmax(156px, 192px);
  gap: 10px;
  overflow-x: auto;
  overflow-y: hidden;
  padding-bottom: 4px;
  scrollbar-width: thin;
}

.portal-record-scroller__item {
  min-width: 0;
  padding: 12px;
  border: 1px solid var(--portal-border);
  border-radius: 12px;
  background: var(--portal-surface-subtle);
}

.portal-record-scroller__value {
  margin-top: 6px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  min-width: 0;
  font-size: 12px;
  color: var(--portal-text-primary);
}

.portal-record-scroller__value--nowrap .portal-record-scroller__text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.portal-record-scroller__value--success {
  color: var(--portal-success-text);
}

.portal-record-scroller__value--warning {
  color: var(--portal-warning);
}

.portal-record-scroller__value--danger {
  color: var(--portal-danger);
}

.portal-record-scroller__text {
  flex: 1;
  min-width: 0;
  line-height: 1.4;
  word-break: break-word;
}

.portal-record-scroller__copy {
  width: 24px;
  height: 24px;
  display: grid;
  place-items: center;
  border: 1px solid var(--portal-border);
  border-radius: 7px;
  background: var(--portal-surface);
  color: var(--portal-text-secondary);
  cursor: pointer;
  flex-shrink: 0;
  transition: border-color 0.18s ease, color 0.18s ease, background-color 0.18s ease;
}

.portal-record-scroller__copy:hover {
  border-color: var(--portal-border-strong);
  color: var(--portal-text-primary);
  background: var(--portal-surface-raised);
}

@media (max-width: 767px) {
  .portal-record-scroller {
    grid-auto-flow: row;
    grid-auto-columns: unset;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    overflow: visible;
  }
}

@media (max-width: 520px) {
  .portal-record-scroller {
    grid-template-columns: 1fr;
  }
}
</style>

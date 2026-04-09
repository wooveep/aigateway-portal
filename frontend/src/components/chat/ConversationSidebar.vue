<script setup lang="ts">
import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons-vue';
import { computed, shallowRef } from 'vue';
import type { ChatSessionSummary } from '../../types';
import { formatDateDisplay } from '../../utils/time';

const props = defineProps<{
  sessions: ChatSessionSummary[];
  activeSessionId: string;
  loading?: boolean;
}>();

const emit = defineEmits<{
  (event: 'select', sessionId: string): void;
  (event: 'create'): void;
  (event: 'rename', payload: { sessionId: string; title: string }): void;
  (event: 'delete', sessionId: string): void;
}>();

const editingSessionId = shallowRef('');
const editingTitle = shallowRef('');

const emptyState = computed(() => !props.loading && props.sessions.length === 0);

const startRename = (item: ChatSessionSummary) => {
  editingSessionId.value = item.sessionId;
  editingTitle.value = item.title;
};

const cancelRename = () => {
  editingSessionId.value = '';
  editingTitle.value = '';
};

const submitRename = (sessionId: string) => {
  const title = editingTitle.value.trim();
  if (!title) {
    cancelRename();
    return;
  }
  emit('rename', { sessionId, title });
  cancelRename();
};
</script>

<template>
  <aside class="chat-sidebar">
    <div class="chat-sidebar__header">
      <div>
        <div class="chat-sidebar__eyebrow">History</div>
        <div class="chat-sidebar__title">历史会话</div>
      </div>
      <a-button type="text" class="chat-sidebar__create" @click="emit('create')">
        <PlusOutlined />
        新建
      </a-button>
    </div>

    <div v-if="loading" class="chat-sidebar__state">
      <a-skeleton active :paragraph="{ rows: 6 }" :title="false" />
    </div>

    <div v-else-if="emptyState" class="chat-sidebar__state chat-sidebar__state--empty">
      <div>还没有历史会话</div>
      <a-button type="primary" size="small" @click="emit('create')">新建第一个会话</a-button>
    </div>

    <div v-else class="chat-sidebar__list">
      <button
        v-for="item in sessions"
        :key="item.sessionId"
        type="button"
        class="chat-sidebar__item"
        :class="{ 'chat-sidebar__item--active': item.sessionId === activeSessionId }"
        @click="emit('select', item.sessionId)"
      >
        <div class="chat-sidebar__item-main">
          <template v-if="editingSessionId === item.sessionId">
            <input
              v-model="editingTitle"
              class="chat-sidebar__rename-input"
              maxlength="64"
              @click.stop
              @keydown.enter.stop.prevent="submitRename(item.sessionId)"
              @keydown.esc.stop.prevent="cancelRename"
              @blur="submitRename(item.sessionId)"
            >
          </template>
          <template v-else>
            <div class="chat-sidebar__item-title">{{ item.title }}</div>
            <div class="chat-sidebar__item-preview">{{ item.lastMessagePreview || '等待第一条消息' }}</div>
          </template>
        </div>
        <div class="chat-sidebar__item-meta">
          <span>{{ formatDateDisplay(item.lastMessageAt || item.createdAt) }}</span>
          <div class="chat-sidebar__actions">
            <a-button
              type="text"
              size="small"
              class="chat-sidebar__icon"
              @click.stop="startRename(item)"
            >
              <EditOutlined />
            </a-button>
            <a-button
              type="text"
              size="small"
              danger
              class="chat-sidebar__icon"
              @click.stop="emit('delete', item.sessionId)"
            >
              <DeleteOutlined />
            </a-button>
          </div>
        </div>
      </button>
    </div>
  </aside>
</template>

<style scoped>
.chat-sidebar {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  border-left: 1px solid var(--portal-border-strong);
  background: rgba(48, 44, 44, 0.48);
}

.chat-sidebar__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 18px 14px;
  border-bottom: 1px solid var(--portal-border);
}

.chat-sidebar__eyebrow {
  color: var(--portal-text-muted);
  font-size: 11px;
  letter-spacing: 0.16em;
  text-transform: uppercase;
}

.chat-sidebar__title {
  margin-top: 4px;
  font-size: 16px;
  font-weight: 700;
}

.chat-sidebar__create {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.chat-sidebar__list {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 8px;
}

.chat-sidebar__item {
  width: 100%;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  border: 1px solid transparent;
  border-radius: 4px;
  background: transparent;
  color: inherit;
  text-align: left;
  padding: 12px;
  transition: border-color 120ms ease, background-color 120ms ease;
}

.chat-sidebar__item:hover,
.chat-sidebar__item--active {
  border-color: var(--portal-border-strong);
  background: rgba(253, 252, 252, 0.03);
}

.chat-sidebar__item + .chat-sidebar__item {
  margin-top: 8px;
}

.chat-sidebar__item-main {
  min-width: 0;
  flex: 1;
}

.chat-sidebar__item-title {
  font-size: 13px;
  font-weight: 600;
  line-height: 1.4;
  color: var(--portal-text-primary);
}

.chat-sidebar__item-preview {
  margin-top: 6px;
  color: var(--portal-text-secondary);
  font-size: 12px;
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.chat-sidebar__item-meta {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
  color: var(--portal-text-muted);
  font-size: 11px;
}

.chat-sidebar__actions {
  display: flex;
  align-items: center;
  gap: 2px;
}

.chat-sidebar__icon {
  color: var(--portal-text-secondary);
}

.chat-sidebar__rename-input {
  width: 100%;
  border: 1px solid var(--portal-border-strong);
  border-radius: 4px;
  background: rgba(253, 252, 252, 0.04);
  color: var(--portal-text-primary);
  font: inherit;
  padding: 8px 10px;
}

.chat-sidebar__state {
  padding: 18px;
}

.chat-sidebar__state--empty {
  display: flex;
  flex-direction: column;
  gap: 12px;
  color: var(--portal-text-secondary);
}
</style>

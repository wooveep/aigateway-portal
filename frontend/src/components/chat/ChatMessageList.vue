<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';
import type { ChatMessageRecord } from '../../types';
import ChatMessageBubble from './ChatMessageBubble.vue';

const props = defineProps<{
  messages: ChatMessageRecord[];
  loading?: boolean;
  emptyTitle?: string;
  emptyText?: string;
}>();

const hasMessages = computed(() => props.messages.length > 0);
const scrollContainer = ref<HTMLElement | null>(null);

const scrollToBottom = async () => {
  await nextTick();
  const element = scrollContainer.value;
  if (!element) {
    return;
  }
  element.scrollTop = element.scrollHeight;
};

watch(
  () => props.messages.map((item) => `${item.messageId}:${item.content}:${item.status}`).join('|'),
  async () => {
    await scrollToBottom();
  },
  { flush: 'post' },
);
</script>

<template>
  <div ref="scrollContainer" class="chat-thread">
    <div v-if="loading" class="chat-thread__state">
      <a-skeleton active :paragraph="{ rows: 8 }" />
    </div>

    <div v-else-if="!hasMessages" class="chat-thread__empty">
      <div class="chat-thread__empty-badge">AI Chat</div>
      <div class="chat-thread__empty-title">{{ emptyTitle || '开始新会话' }}</div>
      <div class="chat-thread__empty-text">{{ emptyText || '选择模型后即可发送消息。' }}</div>
    </div>

    <div v-else class="chat-thread__messages">
      <ChatMessageBubble
        v-for="item in messages"
        :key="item.messageId"
        :item="item"
      />
    </div>
  </div>
</template>

<style scoped>
.chat-thread {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 24px clamp(18px, 3vw, 32px);
  background:
    radial-gradient(circle at top left, rgba(15, 118, 110, 0.05), transparent 22%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.72) 0%, transparent 100%);
}

.chat-thread__messages {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.chat-thread__empty {
  max-width: 760px;
  margin: 10vh auto 0;
  text-align: center;
}

.chat-thread__empty-badge {
  display: inline-flex;
  align-items: center;
  padding: 6px 10px;
  border-radius: 999px;
  border: 1px solid rgba(15, 118, 110, 0.14);
  background: rgba(15, 118, 110, 0.06);
  color: var(--portal-accent);
  font-size: 12px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.chat-thread__empty-title {
  margin-top: 18px;
  font-size: clamp(28px, 4vw, 40px);
  font-weight: 650;
  letter-spacing: -0.03em;
  line-height: 1.04;
  color: var(--portal-text-primary);
}

.chat-thread__empty-text {
  margin-top: 16px;
  color: var(--portal-text-secondary);
  line-height: 1.8;
}

.chat-thread__state {
  max-width: 880px;
  margin: 0 auto;
}
</style>

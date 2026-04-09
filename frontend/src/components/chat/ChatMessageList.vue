<script setup lang="ts">
import { computed } from 'vue';
import type { ChatMessageRecord } from '../../types';
import ChatMessageBubble from './ChatMessageBubble.vue';

const props = defineProps<{
  messages: ChatMessageRecord[];
  loading?: boolean;
  emptyTitle?: string;
  emptyText?: string;
}>();

const hasMessages = computed(() => props.messages.length > 0);
</script>

<template>
  <div class="chat-thread">
    <div v-if="loading" class="chat-thread__state">
      <a-skeleton active :paragraph="{ rows: 8 }" />
    </div>

    <div v-else-if="!hasMessages" class="chat-thread__empty">
      <div class="chat-thread__empty-title">{{ emptyTitle || '您好！' }}</div>
      <div class="chat-thread__empty-text">{{ emptyText || '我是您的 AI 创意助手，选择模型和 API Key 后就可以开始对话。' }}</div>
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
  padding: 28px clamp(18px, 3vw, 36px);
}

.chat-thread__messages {
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.chat-thread__empty {
  max-width: 720px;
  margin: 12vh auto 0;
  text-align: center;
}

.chat-thread__empty-title {
  font-size: clamp(28px, 4vw, 42px);
  font-weight: 700;
  line-height: 1.35;
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

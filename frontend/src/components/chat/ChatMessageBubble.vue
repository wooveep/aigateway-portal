<script setup lang="ts">
import { CopyOutlined } from '@ant-design/icons-vue';
import { message } from 'ant-design-vue';
import { computed } from 'vue';
import type { ChatMessageRecord } from '../../types';
import { formatDateDisplay } from '../../utils/time';

type MessageSegment =
  | { type: 'text'; content: string }
  | { type: 'code'; content: string };

const props = defineProps<{
  item: ChatMessageRecord;
}>();

const segments = computed<MessageSegment[]>(() => {
  const content = props.item.content || '';
  if (!content.includes('```')) {
    return [{ type: 'text', content }];
  }
  const parts = content.split('```');
  return parts
    .map((part, index) => ({
      type: index % 2 === 0 ? 'text' : 'code',
      content: index % 2 === 0 ? part : part.replace(/^[a-zA-Z0-9_-]+\n/, ''),
    }))
    .filter((part) => part.content.trim().length > 0);
});

const isUser = computed(() => props.item.role === 'user');
const isErrored = computed(() => props.item.status === 'failed');
const isCancelled = computed(() => props.item.status === 'cancelled');
const isStreaming = computed(() => props.item.status === 'streaming');

const copyBlock = async (value: string) => {
  try {
    await navigator.clipboard.writeText(value);
    message.success('代码已复制');
  } catch {
    message.error('复制失败');
  }
};
</script>

<template>
  <article class="chat-message" :class="{ 'chat-message--user': isUser }">
    <div class="chat-message__meta">
      <span>{{ isUser ? 'You' : 'AI' }}</span>
      <span>{{ formatDateDisplay(item.finishedAt || item.createdAt) }}</span>
      <span v-if="item.modelId">{{ item.modelId }}</span>
      <span v-if="isStreaming">生成中</span>
      <span v-else-if="isErrored" class="chat-message__status chat-message__status--error">失败</span>
      <span v-else-if="isCancelled" class="chat-message__status">已取消</span>
    </div>

    <div class="chat-message__bubble" :class="{ 'chat-message__bubble--error': isErrored }">
      <template v-if="segments.length">
        <template v-for="(segment, index) in segments" :key="`${item.messageId}-${index}`">
          <pre v-if="segment.type === 'text'" class="chat-message__text">{{ segment.content }}</pre>
          <div v-else class="chat-message__code-wrap">
            <button type="button" class="chat-message__copy" @click="copyBlock(segment.content)">
              <CopyOutlined />
              Copy
            </button>
            <pre class="chat-message__code">{{ segment.content }}</pre>
          </div>
        </template>
      </template>
      <div v-else class="chat-message__placeholder">等待内容...</div>
    </div>

    <div v-if="item.requestId || item.traceId || item.errorMessage" class="chat-message__footer">
      <span v-if="item.requestId">req: {{ item.requestId }}</span>
      <span v-if="item.traceId">trace: {{ item.traceId }}</span>
      <span v-if="item.errorMessage" class="chat-message__error-text">{{ item.errorMessage }}</span>
    </div>
  </article>
</template>

<style scoped>
.chat-message {
  display: flex;
  flex-direction: column;
  gap: 10px;
  align-items: flex-start;
}

.chat-message--user {
  align-items: flex-end;
}

.chat-message__meta,
.chat-message__footer {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  font-size: 11px;
  color: var(--portal-text-muted);
}

.chat-message__bubble {
  width: min(100%, 860px);
  border: 1px solid var(--portal-border);
  border-radius: 4px;
  background: rgba(253, 252, 252, 0.02);
  padding: 16px;
}

.chat-message--user .chat-message__bubble {
  background: rgba(253, 252, 252, 0.06);
}

.chat-message__bubble--error {
  border-color: rgba(255, 59, 48, 0.42);
}

.chat-message__text,
.chat-message__code {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font: inherit;
  line-height: 1.7;
  color: var(--portal-text-primary);
}

.chat-message__code-wrap {
  position: relative;
  border: 1px solid var(--portal-border-strong);
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.24);
  padding: 14px;
}

.chat-message__code-wrap + .chat-message__code-wrap,
.chat-message__text + .chat-message__code-wrap,
.chat-message__code-wrap + .chat-message__text {
  margin-top: 12px;
}

.chat-message__copy {
  position: absolute;
  top: 8px;
  right: 8px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: 1px solid var(--portal-border);
  border-radius: 4px;
  background: rgba(253, 252, 252, 0.04);
  color: var(--portal-text-secondary);
  font: inherit;
  padding: 4px 8px;
  cursor: pointer;
}

.chat-message__placeholder {
  color: var(--portal-text-muted);
}

.chat-message__status--error,
.chat-message__error-text {
  color: var(--portal-danger);
}
</style>

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
    .map<MessageSegment>((part, index) => ({
      type: index % 2 === 0 ? 'text' : 'code',
      content: index % 2 === 0 ? part : part.replace(/^[a-zA-Z0-9_-]+\n/, ''),
    }))
    .filter((part) => part.content.trim().length > 0);
});

const isUser = computed(() => props.item.role === 'user');
const isErrored = computed(() => props.item.status === 'failed');
const isCancelled = computed(() => props.item.status === 'cancelled');
const isStreaming = computed(() => props.item.status === 'streaming');

const stateText = computed(() => {
  if (isStreaming.value) return '生成中';
  if (isErrored.value) return '失败';
  if (isCancelled.value) return '已取消';
  return '已完成';
});

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
    <div class="chat-message__avatar">{{ isUser ? 'U' : 'AI' }}</div>

    <div class="chat-message__body">
      <div class="chat-message__meta">
        <span class="chat-message__author">{{ isUser ? '当前用户' : 'AI 助手' }}</span>
        <span>{{ formatDateDisplay(item.finishedAt || item.createdAt) }}</span>
        <span v-if="item.modelId">{{ item.modelId }}</span>
        <span class="portal-status" :class="{
          'portal-status--danger': isErrored,
          'portal-status--warning': isStreaming || isCancelled,
          'portal-status--success': !isErrored && !isStreaming && !isCancelled,
        }">
          {{ stateText }}
        </span>
      </div>

      <div class="chat-message__bubble" :class="{ 'chat-message__bubble--user': isUser, 'chat-message__bubble--error': isErrored }">
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

      <div v-if="item.requestId || item.traceId || item.errorMessage || item.apiKeyId" class="chat-message__footer">
        <div v-if="item.apiKeyId" class="chat-message__footer-item">
          <span class="portal-data-item__label">API Key</span>
          <span class="chat-message__footer-value chat-message__footer-value--nowrap">{{ item.apiKeyId }}</span>
        </div>
        <div v-if="item.requestId" class="chat-message__footer-item">
          <span class="portal-data-item__label">Request ID</span>
          <span class="chat-message__footer-value chat-message__footer-value--nowrap">{{ item.requestId }}</span>
        </div>
        <div v-if="item.traceId" class="chat-message__footer-item">
          <span class="portal-data-item__label">Trace</span>
          <span class="chat-message__footer-value chat-message__footer-value--nowrap">{{ item.traceId }}</span>
        </div>
        <div v-if="item.errorMessage" class="chat-message__footer-item chat-message__footer-item--error">
          <span class="portal-data-item__label">错误信息</span>
          <span class="chat-message__footer-value">{{ item.errorMessage }}</span>
        </div>
      </div>
    </div>
  </article>
</template>

<style scoped>
.chat-message {
  display: grid;
  grid-template-columns: 40px minmax(0, 1fr);
  gap: 14px;
  align-items: start;
}

.chat-message--user {
  grid-template-columns: minmax(0, 1fr) 40px;
}

.chat-message__avatar {
  width: 40px;
  height: 40px;
  display: grid;
  place-items: center;
  border-radius: 12px;
  background: linear-gradient(135deg, rgba(15, 118, 110, 0.14), rgba(15, 118, 110, 0.04));
  color: var(--portal-accent);
  font-size: 12px;
  font-weight: 600;
}

.chat-message--user .chat-message__avatar {
  order: 2;
  background: linear-gradient(135deg, rgba(6, 27, 49, 0.1), rgba(6, 27, 49, 0.03));
  color: var(--portal-text-primary);
}

.chat-message__body {
  min-width: 0;
}

.chat-message__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
  color: var(--portal-text-muted);
  font-size: 12px;
}

.chat-message__author {
  color: var(--portal-text-primary);
  font-weight: 500;
}

.chat-message__bubble {
  width: min(100%, 920px);
  margin-top: 10px;
  border: 1px solid var(--portal-border);
  border-radius: 16px;
  background: linear-gradient(180deg, #ffffff 0%, #f8fafc 100%);
  box-shadow: var(--portal-shadow-ambient);
  padding: 16px 18px;
}

.chat-message__bubble--user {
  margin-left: auto;
  background: linear-gradient(180deg, rgba(15, 118, 110, 0.08) 0%, rgba(15, 118, 110, 0.04) 100%);
  border-color: rgba(15, 118, 110, 0.18);
}

.chat-message__bubble--error {
  border-color: rgba(215, 45, 89, 0.18);
}

.chat-message__text,
.chat-message__code {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font: inherit;
  line-height: 1.8;
  color: var(--portal-text-primary);
}

.chat-message__code-wrap {
  position: relative;
  border: 1px solid var(--portal-border);
  border-radius: 14px;
  background: #f7fafc;
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
  border-radius: 10px;
  background: #ffffff;
  color: var(--portal-text-secondary);
  padding: 4px 8px;
  cursor: pointer;
}

.chat-message__placeholder {
  color: var(--portal-text-muted);
}

.chat-message__footer {
  display: grid;
  gap: 10px;
  margin-top: 12px;
}

.chat-message__footer-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.chat-message__footer-item--error .chat-message__footer-value {
  color: var(--portal-danger);
}

.chat-message__footer-value {
  color: var(--portal-text-secondary);
  font-size: 13px;
  line-height: 1.6;
  word-break: break-word;
}

.chat-message__footer-value--nowrap {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 768px) {
  .chat-message,
  .chat-message--user {
    grid-template-columns: 1fr;
  }

  .chat-message--user .chat-message__avatar {
    order: 0;
  }

  .chat-message__bubble {
    width: 100%;
  }
}
</style>

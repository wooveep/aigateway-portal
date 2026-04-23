<script setup lang="ts">
import { message } from 'ant-design-vue';
import { computed } from 'vue';
import ChatComposer from '../components/chat/ChatComposer.vue';
import ChatMessageList from '../components/chat/ChatMessageList.vue';
import ConversationSidebar from '../components/chat/ConversationSidebar.vue';
import { useAIChat } from '../composables/useAIChat';
import { useManagedAccountScope } from '../composables/useManagedAccountScope';

const {
  loadingManagedAccounts,
  loadManagedAccounts,
  activeConsumerName,
} = useManagedAccountScope();

loadManagedAccounts();

const {
  sessions,
  modelOptions,
  apiKeyOptions,
  activeSessionId,
  activeMessages,
  selectedModelId,
  selectedApiKeyId,
  loading,
  loadingMessages,
  sending,
  errorMessage,
  selectSession,
  createSessionNow,
  renameSession,
  deleteSession,
  sendMessage,
  cancelStreaming,
} = useAIChat(activeConsumerName);

const pageLoading = computed(() => loading.value || loadingManagedAccounts.value);

const updateModel = (value: string) => {
  selectedModelId.value = value;
};

const updateApiKey = (value: string) => {
  selectedApiKeyId.value = value;
};

const handleCreateSession = async () => {
  try {
    await createSessionNow();
  } catch {
    message.error('创建会话失败');
  }
};

const handleRenameSession = async (payload: { sessionId: string; title: string }) => {
  try {
    await renameSession(payload.sessionId, payload.title);
    message.success('会话名称已更新');
  } catch {
    message.error('重命名失败');
  }
};

const handleDeleteSession = async (sessionId: string) => {
  try {
    await deleteSession(sessionId);
    message.success('会话已删除');
  } catch {
    message.error('删除会话失败');
  }
};

const handleSend = async (content: string) => {
  try {
    await sendMessage(content);
  } catch {
    // inline error state is already rendered in the page and message bubble
  }
};

const statusLabel = computed(() => {
  if (sending.value) {
    return '流式响应中';
  }
  if (errorMessage.value) {
    return '需要处理错误';
  }
  return '准备就绪';
});
</script>

<template>
  <section class="portal-page portal-chat-page">
    <div class="portal-metric-strip">
      <div class="portal-metric">
        <div class="portal-metric__label">历史会话</div>
        <div class="portal-metric__value">{{ sessions.length }}</div>
      </div>
      <div class="portal-metric">
        <div class="portal-metric__label">可选模型 / API Key</div>
        <div class="portal-metric__value">{{ modelOptions.length }} / {{ apiKeyOptions.length }}</div>
      </div>
      <div class="portal-metric">
        <div class="portal-metric__label">当前状态</div>
        <div class="portal-metric__value">{{ statusLabel }}</div>
      </div>
    </div>

    <div v-if="errorMessage" class="portal-callout">
      {{ errorMessage }}
    </div>

    <div class="portal-chat-shell">
      <div class="portal-chat-shell__main">
        <ChatMessageList
          :messages="activeMessages"
          :loading="pageLoading || loadingMessages"
        />
        <ChatComposer
          :model-options="modelOptions"
          :api-key-options="apiKeyOptions"
          :selected-model-id="selectedModelId"
          :selected-api-key-id="selectedApiKeyId"
          :sending="sending"
          :disabled="!modelOptions.length || !apiKeyOptions.length"
          @update:model-id="updateModel"
          @update:api-key-id="updateApiKey"
          @create="handleCreateSession"
          @send="handleSend"
          @cancel="cancelStreaming"
        />
      </div>

      <ConversationSidebar
        class="portal-chat-shell__sidebar"
        :sessions="sessions"
        :active-session-id="activeSessionId"
        :loading="pageLoading"
        @create="handleCreateSession"
        @select="selectSession"
        @rename="handleRenameSession"
        @delete="handleDeleteSession"
      />
    </div>
  </section>
</template>

<style scoped>
.portal-chat-page {
  gap: 16px;
}

.portal-chat-shell {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 328px;
  min-height: calc(100vh - 250px);
  border: 1px solid var(--portal-border);
  border-radius: 18px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: var(--portal-shadow-standard);
}

.portal-chat-shell__main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.portal-chat-shell__sidebar {
  min-width: 0;
}

@media (max-width: 1180px) {
  .portal-chat-shell {
    grid-template-columns: 1fr;
  }
}
</style>

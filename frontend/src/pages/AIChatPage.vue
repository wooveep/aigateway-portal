<script setup lang="ts">
import { message } from 'ant-design-vue';
import { computed } from 'vue';
import ChatComposer from '../components/chat/ChatComposer.vue';
import ChatMessageList from '../components/chat/ChatMessageList.vue';
import ConversationSidebar from '../components/chat/ConversationSidebar.vue';
import { useAIChat } from '../composables/useAIChat';
import { useManagedAccountScope } from '../composables/useManagedAccountScope';

const {
  hasManagedAccounts,
  loadingManagedAccounts,
  loadManagedAccounts,
  scopeOptions,
  activeConsumerName,
  currentScopeTitle,
  updateScopeConsumerName,
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

const handleSend = async (value: string) => {
  try {
    await sendMessage(value);
  } catch (error: any) {
    message.error(error?.message || '发送失败');
  }
};
</script>

<template>
  <section class="portal-page portal-chat-page">
    <div class="portal-page__header">
      <div>
        <div class="portal-page__eyebrow">AI Chat</div>
        <h1 class="portal-page__title">AI 对话</h1>
        <p class="portal-page__description">直接通过您自己的 API Key 调用已发布模型，会话历史持久化保存在 Portal 中。</p>
      </div>

      <div v-if="hasManagedAccounts" class="portal-page__scope">
        <span>当前范围</span>
        <a-select
          :value="activeConsumerName"
          :options="scopeOptions"
          :loading="loadingManagedAccounts"
          style="min-width: 260px"
          @update:value="updateScopeConsumerName(($event as string) || '')"
        />
      </div>
    </div>

    <div class="portal-hero-card portal-chat-page__status">
      <div>
        <div class="portal-kv__label">当前会话范围</div>
        <div class="portal-kv__value">{{ currentScopeTitle }}</div>
      </div>
      <div>
        <div class="portal-kv__label">可选模型 / API Key</div>
        <div class="portal-kv__value">{{ modelOptions.length }} / {{ apiKeyOptions.length }}</div>
      </div>
      <div v-if="errorMessage" class="portal-kv portal-kv--danger">
        <div class="portal-kv__label">错误</div>
        <div class="portal-kv__value">{{ errorMessage }}</div>
      </div>
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
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.portal-chat-page__status {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 18px;
}

.portal-chat-shell {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  min-height: calc(100vh - 250px);
  border: 1px solid var(--portal-border-strong);
  border-radius: 4px;
  overflow: hidden;
  background: rgba(32, 29, 29, 0.92);
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

.portal-kv__label {
  color: var(--portal-text-muted);
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.12em;
}

.portal-kv__value {
  margin-top: 6px;
  color: var(--portal-text-primary);
  font-size: 16px;
}

.portal-kv--danger .portal-kv__value {
  color: var(--portal-danger);
}

@media (max-width: 1180px) {
  .portal-chat-page__status,
  .portal-chat-shell {
    grid-template-columns: 1fr;
  }
}
</style>

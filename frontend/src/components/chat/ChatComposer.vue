<script setup lang="ts">
import { LoadingOutlined, PlusOutlined, SendOutlined, StopOutlined } from '@ant-design/icons-vue';
import { computed, shallowRef } from 'vue';

const props = defineProps<{
  modelOptions: Array<{ label: string; value: string }>;
  apiKeyOptions: Array<{ label: string; value: string }>;
  selectedModelId: string;
  selectedApiKeyId: string;
  disabled?: boolean;
  sending?: boolean;
}>();

const emit = defineEmits<{
  (event: 'update:modelId', value: string): void;
  (event: 'update:apiKeyId', value: string): void;
  (event: 'send', value: string): void;
  (event: 'cancel'): void;
  (event: 'create'): void;
}>();

const localInput = shallowRef('');

const canSend = computed(() => {
  return !props.disabled && !props.sending && !!localInput.value.trim() && !!props.selectedModelId && !!props.selectedApiKeyId;
});

const promptSuggestions = ['文案优化', '翻译', '代码编写', '产品方案'];

const sendNow = () => {
  const value = localInput.value.trim();
  if (!value) {
    return;
  }
  emit('send', value);
  localInput.value = '';
};

const appendPrompt = (value: string) => {
  localInput.value = localInput.value ? `${localInput.value}\n${value}` : value;
};
</script>

<template>
  <div class="chat-composer">
    <div class="chat-composer__toolbar">
      <div class="chat-composer__selectors">
        <a-select
          class="chat-composer__select"
          :value="selectedModelId"
          :options="modelOptions"
          placeholder="选择模型"
          @update:value="emit('update:modelId', $event as string)"
        />
        <a-select
          class="chat-composer__select"
          :value="selectedApiKeyId"
          :options="apiKeyOptions"
          placeholder="选择 API Key"
          @update:value="emit('update:apiKeyId', $event as string)"
        />
      </div>
      <a-button class="chat-composer__new" @click="emit('create')">
        <PlusOutlined />
        新建会话
      </a-button>
    </div>

    <div class="chat-composer__input-wrap">
      <textarea
        v-model="localInput"
        class="chat-composer__textarea"
        placeholder="输入您的问题或指令..."
        :disabled="disabled"
        @keydown.enter.exact.prevent="sendNow"
      />

      <div class="chat-composer__footer">
        <div class="chat-composer__chips">
          <span class="chat-composer__chips-label">推荐指令</span>
          <button
            v-for="item in promptSuggestions"
            :key="item"
            type="button"
            class="chat-composer__chip"
            @click="appendPrompt(item)"
          >
            {{ item }}
          </button>
        </div>

        <div class="chat-composer__buttons">
          <a-button v-if="sending" danger @click="emit('cancel')">
            <StopOutlined />
            停止
          </a-button>
          <a-button v-else type="primary" :disabled="!canSend" @click="sendNow">
            <template #icon>
              <LoadingOutlined v-if="sending" />
              <SendOutlined v-else />
            </template>
            发送
          </a-button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.chat-composer {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  border-top: 1px solid var(--portal-border);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.94) 0%, #ffffff 100%);
}

.chat-composer__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.chat-composer__selectors {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.chat-composer__select {
  min-width: 220px;
}

.chat-composer__new {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.chat-composer__input-wrap {
  border: 1px solid var(--portal-border);
  border-radius: 16px;
  background: linear-gradient(180deg, #ffffff 0%, #f8fafc 100%);
  box-shadow: var(--portal-shadow-ambient);
  padding: 14px;
}

.chat-composer__textarea {
  width: 100%;
  min-height: 120px;
  resize: vertical;
  border: none;
  outline: none;
  background: transparent;
  color: var(--portal-text-primary);
  line-height: 1.7;
}

.chat-composer__footer {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  margin-top: 12px;
}

.chat-composer__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.chat-composer__chips-label {
  color: var(--portal-text-muted);
  font-size: 12px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.chat-composer__chip {
  border: 1px solid var(--portal-border);
  border-radius: 999px;
  background: var(--portal-surface);
  color: var(--portal-text-secondary);
  padding: 6px 12px;
  cursor: pointer;
  transition: border-color 0.18s ease, color 0.18s ease, background-color 0.18s ease;
}

.chat-composer__chip:hover {
  border-color: var(--portal-border-strong);
  color: var(--portal-accent);
  background: var(--portal-surface-raised);
}

.chat-composer__buttons {
  flex-shrink: 0;
}

@media (max-width: 960px) {
  .chat-composer__toolbar,
  .chat-composer__footer {
    flex-direction: column;
    align-items: stretch;
  }

  .chat-composer__selectors {
    width: 100%;
  }

  .chat-composer__select {
    min-width: 0;
    width: 100%;
  }
}
</style>

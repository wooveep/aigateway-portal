import { computed, reactive, ref, shallowRef, watch } from 'vue';
import {
  apiBaseURL,
  createChatSession,
  fetchChatSessionDetail,
  fetchChatSessions,
  fetchModels,
  fetchApiKeys,
  removeChatSession,
  updateChatSession,
} from '../api';
import type {
  ApiKeyRecord,
  ChatMessageRecord,
  ChatSessionDetail,
  ChatSessionSummary,
  ModelInfo,
} from '../types';

type SSEEvent = {
  event: string;
  data: string;
};

export function useAIChat(scopeConsumerName: { value: string }) {
  const sessions = ref<ChatSessionSummary[]>([]);
  const sessionDetails = reactive<Record<string, ChatSessionDetail>>({});
  const models = ref<ModelInfo[]>([]);
  const apiKeys = ref<ApiKeyRecord[]>([]);
  const activeSessionId = shallowRef('');
  const selectedModelId = shallowRef('');
  const selectedApiKeyId = shallowRef('');
  const loading = shallowRef(false);
  const loadingMessages = shallowRef(false);
  const sending = shallowRef(false);
  const errorMessage = shallowRef('');
  const abortController = shallowRef<AbortController | null>(null);

  const usableApiKeys = computed(() => {
    const now = Date.now();
    return apiKeys.value.filter((item) => {
      if (item.status !== 'active') {
        return false;
      }
      if (!item.expiresAt) {
        return true;
      }
      const expiresAt = Date.parse(item.expiresAt);
      return Number.isNaN(expiresAt) || expiresAt > now;
    });
  });

  const modelOptions = computed(() => {
    return models.value.map((item) => ({
      label: item.name,
      value: item.id,
    }));
  });

  const apiKeyOptions = computed(() => {
    return usableApiKeys.value.map((item) => ({
      label: item.name ? `${item.name} / ${item.id}` : item.id,
      value: item.id,
    }));
  });

  const activeSession = computed(() => {
    return sessions.value.find((item) => item.sessionId === activeSessionId.value) || null;
  });

  const activeSessionDetail = computed(() => {
    return activeSessionId.value ? sessionDetails[activeSessionId.value] || null : null;
  });

  const activeMessages = computed(() => activeSessionDetail.value?.messages || []);

  const syncSelectionWithSession = () => {
    const preferredModelId = activeSession.value?.defaultModelId || '';
    const preferredApiKeyId = activeSession.value?.defaultApiKeyId || '';
    const availableModelIds = new Set(models.value.map((item) => item.id));
    const availableKeyIds = new Set(usableApiKeys.value.map((item) => item.id));

    if (!selectedModelId.value || !availableModelIds.has(selectedModelId.value)) {
      selectedModelId.value = availableModelIds.has(preferredModelId)
        ? preferredModelId
        : (models.value[0]?.id || '');
    }
    if (!selectedApiKeyId.value || !availableKeyIds.has(selectedApiKeyId.value)) {
      selectedApiKeyId.value = availableKeyIds.has(preferredApiKeyId)
        ? preferredApiKeyId
        : (usableApiKeys.value[0]?.id || '');
    }
  };

  const applySessionSummary = (nextSession: ChatSessionSummary) => {
    const index = sessions.value.findIndex((item) => item.sessionId === nextSession.sessionId);
    if (index >= 0) {
      sessions.value.splice(index, 1, nextSession);
      return;
    }
    sessions.value.unshift(nextSession);
  };

  const loadSessionDetail = async (sessionId: string) => {
    if (!sessionId) {
      return;
    }
    loadingMessages.value = true;
    try {
      const detail = await fetchChatSessionDetail(sessionId, scopeConsumerName.value || undefined);
      sessionDetails[sessionId] = detail;
      applySessionSummary(detail.session);
    } finally {
      loadingMessages.value = false;
    }
  };

  const loadBootstrap = async () => {
    loading.value = true;
    errorMessage.value = '';
    try {
      const [nextSessions, nextModels, nextKeys] = await Promise.all([
        fetchChatSessions(scopeConsumerName.value || undefined),
        fetchModels(scopeConsumerName.value || undefined),
        fetchApiKeys(false, scopeConsumerName.value || undefined),
      ]);
      sessions.value = nextSessions;
      models.value = nextModels;
      apiKeys.value = nextKeys;
      const availableIds = new Set(nextSessions.map((item) => item.sessionId));
      if (!availableIds.has(activeSessionId.value)) {
        activeSessionId.value = nextSessions[0]?.sessionId || '';
      }
      if (activeSessionId.value) {
        await loadSessionDetail(activeSessionId.value);
      }
      syncSelectionWithSession();
    } catch (error: any) {
      errorMessage.value = error?.response?.data?.message || error?.message || 'AI 会话初始化失败';
    } finally {
      loading.value = false;
    }
  };

  const createSessionNow = async () => {
    const created = await createChatSession({
      defaultModelId: selectedModelId.value,
      defaultApiKeyId: selectedApiKeyId.value,
    }, scopeConsumerName.value || undefined);
    applySessionSummary(created);
    sessionDetails[created.sessionId] = {
      session: created,
      messages: [],
    };
    activeSessionId.value = created.sessionId;
    syncSelectionWithSession();
    return created;
  };

  const selectSession = async (sessionId: string) => {
    activeSessionId.value = sessionId;
    if (!sessionDetails[sessionId]) {
      await loadSessionDetail(sessionId);
    }
    syncSelectionWithSession();
  };

  const renameSession = async (sessionId: string, title: string) => {
    const updated = await updateChatSession(sessionId, {
      title,
      defaultModelId: sessionDetails[sessionId]?.session.defaultModelId || '',
      defaultApiKeyId: sessionDetails[sessionId]?.session.defaultApiKeyId || '',
    }, scopeConsumerName.value || undefined);
    applySessionSummary(updated);
    if (sessionDetails[sessionId]) {
      sessionDetails[sessionId] = {
        ...sessionDetails[sessionId],
        session: updated,
      };
    }
  };

  const deleteSession = async (sessionId: string) => {
    await removeChatSession(sessionId, scopeConsumerName.value || undefined);
    sessions.value = sessions.value.filter((item) => item.sessionId !== sessionId);
    delete sessionDetails[sessionId];
    if (activeSessionId.value === sessionId) {
      activeSessionId.value = sessions.value[0]?.sessionId || '';
      if (activeSessionId.value) {
        await loadSessionDetail(activeSessionId.value);
      }
    }
    syncSelectionWithSession();
  };

  const cancelStreaming = () => {
    abortController.value?.abort();
    abortController.value = null;
    sending.value = false;
    const lastAssistant = [...activeMessages.value].reverse().find((item) => item.role === 'assistant' && item.status === 'streaming');
    if (lastAssistant) {
      lastAssistant.status = 'cancelled';
      lastAssistant.errorMessage = '会话已取消';
    }
  };

  const patchMessage = (sessionId: string, messageId: string, patch: Partial<ChatMessageRecord>) => {
    const detail = sessionDetails[sessionId];
    if (!detail) {
      return;
    }
    const index = detail.messages.findIndex((item) => item.messageId === messageId);
    if (index < 0) {
      return;
    }
    detail.messages.splice(index, 1, {
      ...detail.messages[index],
      ...patch,
    });
  };

  const replaceMessageIds = (sessionId: string, userMessageId: string, assistantMessageId: string) => {
    const detail = sessionDetails[sessionId];
    if (!detail) {
      return;
    }
    const currentUser = detail.messages.find((item) => item.messageId.startsWith('temp-user-'));
    const currentAssistant = detail.messages.find((item) => item.messageId.startsWith('temp-assistant-'));
    if (currentUser) {
      currentUser.messageId = userMessageId;
    }
    if (currentAssistant) {
      currentAssistant.messageId = assistantMessageId;
    }
  };

  const updateSessionSummaryFromMessage = (sessionId: string, preview: string, titleFallback: string) => {
    const session = sessions.value.find((item) => item.sessionId === sessionId);
    if (!session) {
      return;
    }
    session.lastMessagePreview = preview;
    session.lastMessageAt = new Date().toISOString();
    if (!session.title || session.title === '新对话') {
      session.title = titleFallback;
    }
  };

  const ensureActiveSession = async () => {
    if (activeSessionId.value) {
      return activeSessionId.value;
    }
    const created = await createSessionNow();
    return created.sessionId;
  };

  const sendMessage = async (content: string) => {
    const trimmedContent = content.trim();
    if (!trimmedContent || sending.value) {
      return;
    }
    errorMessage.value = '';
    const sessionId = await ensureActiveSession();
    if (!sessionDetails[sessionId]) {
      await loadSessionDetail(sessionId);
    }
    const detail = sessionDetails[sessionId];
    const now = new Date().toISOString();
    const tempUserId = `temp-user-${Date.now()}`;
    const tempAssistantId = `temp-assistant-${Date.now()}`;
    detail.messages.push({
      messageId: tempUserId,
      sessionId,
      role: 'user',
      content: trimmedContent,
      status: 'succeeded',
      modelId: selectedModelId.value,
      apiKeyId: selectedApiKeyId.value,
      requestId: '',
      traceId: '',
      httpStatus: 0,
      errorMessage: '',
      createdAt: now,
      finishedAt: now,
    });
    detail.messages.push({
      messageId: tempAssistantId,
      sessionId,
      role: 'assistant',
      content: '',
      status: 'streaming',
      modelId: selectedModelId.value,
      apiKeyId: selectedApiKeyId.value,
      requestId: '',
      traceId: '',
      httpStatus: 0,
      errorMessage: '',
      createdAt: now,
      finishedAt: '',
    });
    updateSessionSummaryFromMessage(sessionId, trimmedContent.slice(0, 120), trimmedContent.slice(0, 24));

    sending.value = true;
    abortController.value = new AbortController();

    try {
      const query = new URLSearchParams();
      if (scopeConsumerName.value) {
        query.set('consumerName', scopeConsumerName.value);
      }
      const response = await fetch(`${apiBaseURL}/ai-chat/sessions/${sessionId}/messages/stream${query.toString() ? `?${query}` : ''}`, {
        method: 'POST',
        credentials: 'include',
        signal: abortController.value.signal,
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          content: trimmedContent,
          modelId: selectedModelId.value,
          apiKeyId: selectedApiKeyId.value,
        }),
      });

      if (!response.ok || !response.body) {
        const raw = await response.text();
        let message = raw || '发送失败';
        try {
          const parsed = JSON.parse(raw);
          message = parsed.message || message;
        } catch {
          // ignored
        }
        patchMessage(sessionId, tempAssistantId, {
          status: 'failed',
          errorMessage: message,
        });
        errorMessage.value = message;
        throw new Error(message);
      }

      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';
      let serverUserId = tempUserId;
      let serverAssistantId = tempAssistantId;

      while (true) {
        const { done, value } = await reader.read();
        if (done) {
          break;
        }
        buffer += decoder.decode(value, { stream: true });
        const parsed = consumeSSEBuffer(buffer);
        buffer = parsed.buffer;

        for (const event of parsed.events) {
          const payload = JSON.parse(event.data);
          if (event.event === 'ack') {
            serverUserId = payload.userMessageId || serverUserId;
            serverAssistantId = payload.assistantMessageId || serverAssistantId;
            replaceMessageIds(sessionId, serverUserId, serverAssistantId);
            continue;
          }
          if (event.event === 'delta') {
            patchMessage(sessionId, serverAssistantId, {
              content: payload.text || '',
              status: 'streaming',
            });
            continue;
          }
          if (event.event === 'done') {
            patchMessage(sessionId, serverAssistantId, {
              status: 'succeeded',
              requestId: payload.requestId || '',
              traceId: payload.traceId || '',
              httpStatus: payload.httpStatus || 0,
              finishedAt: new Date().toISOString(),
            });
            const assistant = sessionDetails[sessionId]?.messages.find((item) => item.messageId === serverAssistantId);
            updateSessionSummaryFromMessage(
              sessionId,
              (assistant?.content || trimmedContent).slice(0, 120),
              trimmedContent.slice(0, 24),
            );
            continue;
          }
          if (event.event === 'error') {
            patchMessage(sessionId, serverAssistantId, {
              status: 'failed',
              errorMessage: payload.message || '发送失败',
              finishedAt: new Date().toISOString(),
            });
            errorMessage.value = payload.message || '发送失败';
            throw new Error(payload.message || '发送失败');
          }
        }
      }
    } catch (error: any) {
      if (error?.name === 'AbortError') {
        patchMessage(sessionId, tempAssistantId, {
          status: 'cancelled',
          errorMessage: '会话已取消',
        });
        errorMessage.value = '';
      } else {
        const detailMessage = error?.message || '发送失败';
        patchMessage(sessionId, tempAssistantId, {
          status: 'failed',
          errorMessage: detailMessage,
        });
        errorMessage.value = detailMessage;
        throw error;
      }
    } finally {
      abortController.value = null;
      sending.value = false;
      const currentSession = sessions.value.find((item) => item.sessionId === sessionId);
      if (currentSession) {
        sessionDetails[sessionId].session = { ...currentSession };
      }
    }
  };

  watch(() => scopeConsumerName.value, async () => {
    activeSessionId.value = '';
    selectedModelId.value = '';
    selectedApiKeyId.value = '';
    Object.keys(sessionDetails).forEach((key) => {
      delete sessionDetails[key];
    });
    await loadBootstrap();
  }, { immediate: true });

  watch([activeSession, models, usableApiKeys], () => {
    syncSelectionWithSession();
  });

  return {
    sessions,
    models,
    apiKeys: usableApiKeys,
    modelOptions,
    apiKeyOptions,
    activeSessionId,
    activeSession,
    activeMessages,
    selectedModelId,
    selectedApiKeyId,
    loading,
    loadingMessages,
    sending,
    errorMessage,
    loadBootstrap,
    selectSession,
    createSessionNow,
    renameSession,
    deleteSession,
    sendMessage,
    cancelStreaming,
  };
}

function consumeSSEBuffer(buffer: string): { events: SSEEvent[]; buffer: string } {
  const normalized = buffer.replace(/\r\n/g, '\n');
  const blocks = normalized.split('\n\n');
  const events: SSEEvent[] = [];
  const rest = blocks.pop() || '';

  for (const block of blocks) {
    const lines = block.split('\n');
    let event = '';
    const dataLines: string[] = [];
    for (const line of lines) {
      if (line.startsWith('event:')) {
        event = line.slice(6).trim();
      } else if (line.startsWith('data:')) {
        dataLines.push(line.slice(5).trim());
      }
    }
    if (event && dataLines.length) {
      events.push({
        event,
        data: dataLines.join('\n'),
      });
    }
  }

  return {
    events,
    buffer: rest,
  };
}

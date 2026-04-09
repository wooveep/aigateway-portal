<script setup lang="ts">
import { CopyOutlined, LinkOutlined } from '@ant-design/icons-vue';
import { message } from 'ant-design-vue';
import { computed, onMounted, ref, watch } from 'vue';
import { fetchAgentDetail, fetchAgents } from '../api';
import { useManagedAccountScope } from '../composables/useManagedAccountScope';
import type { AgentInfo } from '../types';
import { formatDateDisplay } from '../utils/time';

const agents = ref<AgentInfo[]>([]);
const selectedAgent = ref<AgentInfo | null>(null);
const detailVisible = ref(false);
const loading = ref(false);

const {
  hasManagedAccounts,
  loadingManagedAccounts,
  loadManagedAccounts,
  scopeOptions,
  activeConsumerName,
  currentScopeTitle,
  updateScopeConsumerName,
} = useManagedAccountScope();

const emptyState = computed(() => !loading.value && agents.value.length === 0);

const copyText = async (value: string) => {
  try {
    await navigator.clipboard.writeText(value);
    message.success('地址已复制');
  } catch {
    message.error('复制失败');
  }
};

const loadAgents = async () => {
  loading.value = true;
  try {
    agents.value = await fetchAgents(activeConsumerName.value || undefined);
  } catch {
    message.error('智能体列表加载失败');
  } finally {
    loading.value = false;
  }
};

const showDetail = async (agentId: string) => {
  try {
    selectedAgent.value = await fetchAgentDetail(agentId, activeConsumerName.value || undefined);
    detailVisible.value = true;
  } catch {
    message.error('加载智能体详情失败');
  }
};

onMounted(async () => {
  await loadManagedAccounts();
  await loadAgents();
});

watch(() => activeConsumerName.value, () => {
  loadAgents();
});
</script>

<template>
  <section class="portal-page">
    <div class="portal-page__header">
      <div>
        <div class="portal-page__eyebrow">Agents</div>
        <h1 class="portal-page__title">智能体广场</h1>
        <p class="portal-page__description">首版只提供 MCP-backed agent 的展示、说明和接入复制能力，不在 Portal 内编辑资源或提示词。</p>
      </div>

      <div class="portal-page__scope">
        <span>当前范围</span>
        <a-select
          v-if="hasManagedAccounts"
          :value="activeConsumerName"
          :options="scopeOptions"
          :loading="loadingManagedAccounts"
          style="min-width: 260px"
          @update:value="updateScopeConsumerName(($event as string) || ''); loadAgents()"
        />
        <span v-else class="portal-page__scope-text">{{ currentScopeTitle }}</span>
      </div>
    </div>

    <div class="portal-hero-card portal-page__summary">
      <div>
        <div class="portal-kv__label">可见智能体</div>
        <div class="portal-kv__value">{{ agents.length }}</div>
      </div>
      <div>
        <div class="portal-kv__label">当前作用域</div>
        <div class="portal-kv__value">{{ currentScopeTitle }}</div>
      </div>
    </div>

    <div v-if="loading" class="portal-hero-card">
      <a-skeleton active :paragraph="{ rows: 6 }" />
    </div>

    <div v-else-if="emptyState" class="portal-hero-card portal-empty">
      <div class="portal-empty__title">当前范围下还没有可见智能体</div>
      <div class="portal-empty__text">可以去 Console 发布 agent_catalog，或者检查当前账号是否满足 consumer / department / user_level 授权。</div>
    </div>

    <div v-else class="portal-card-grid">
      <article v-for="item in agents" :key="item.id" class="portal-list-card">
        <div class="portal-list-card__header">
          <div>
            <div class="portal-list-card__title">{{ item.displayName }}</div>
            <div class="portal-list-card__subtitle">{{ item.canonicalName }}</div>
          </div>
          <div class="portal-list-card__badge">{{ item.toolCount }} tools</div>
        </div>

        <p class="portal-list-card__intro">{{ item.intro || item.description }}</p>

        <div class="portal-list-card__meta">
          <span>传输：{{ item.transportTypes?.join(' / ') || 'http / sse' }}</span>
          <span>更新：{{ formatDateDisplay(item.updatedAt) }}</span>
        </div>

        <div v-if="item.tags?.length" class="portal-list-card__tags">
          <span v-for="tag in item.tags" :key="tag" class="portal-tag">{{ tag }}</span>
        </div>

        <div class="portal-list-card__actions">
          <a-button type="default" @click="showDetail(item.id)">
            <LinkOutlined />
            查看详情
          </a-button>
          <a-button type="text" @click="copyText(item.httpUrl)">
            <CopyOutlined />
            复制 HTTP
          </a-button>
        </div>
      </article>
    </div>

    <a-drawer v-model:open="detailVisible" :width="640" title="智能体详情" destroy-on-close>
      <template v-if="selectedAgent">
        <div class="portal-detail-stack">
          <div class="portal-hero-card">
            <div class="portal-kv__label">名称</div>
            <div class="portal-kv__value">{{ selectedAgent.displayName }}</div>
            <p class="portal-page__description">{{ selectedAgent.description || selectedAgent.intro }}</p>
          </div>

          <div class="portal-hero-card">
            <div class="portal-kv__label">MCP Server</div>
            <div class="portal-kv__value">{{ selectedAgent.mcpServerName }}</div>
            <div class="portal-copy-row">
              <span class="portal-copy-row__label">HTTP</span>
              <code class="portal-inline-code">{{ selectedAgent.httpUrl }}</code>
              <a-button type="text" @click="copyText(selectedAgent.httpUrl)">
                <CopyOutlined />
              </a-button>
            </div>
            <div class="portal-copy-row">
              <span class="portal-copy-row__label">SSE</span>
              <code class="portal-inline-code">{{ selectedAgent.sseUrl }}</code>
              <a-button type="text" @click="copyText(selectedAgent.sseUrl)">
                <CopyOutlined />
              </a-button>
            </div>
          </div>

          <div class="portal-hero-card">
            <div class="portal-kv__label">Resource 摘要</div>
            <pre class="portal-code-block">{{ selectedAgent.resourceSummary || '当前未声明 resource。' }}</pre>
          </div>

          <div class="portal-hero-card">
            <div class="portal-kv__label">Prompt 摘要</div>
            <pre class="portal-code-block">{{ selectedAgent.promptSummary || '当前未声明 prompt。' }}</pre>
          </div>
        </div>
      </template>
    </a-drawer>
  </section>
</template>

<style scoped>
.portal-page__summary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px;
}

.portal-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 16px;
}

.portal-list-card {
  border: 1px solid var(--portal-border-strong);
  border-radius: 4px;
  background: rgba(48, 44, 44, 0.44);
  padding: 18px;
}

.portal-list-card__header,
.portal-list-card__meta,
.portal-list-card__actions,
.portal-copy-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.portal-list-card__title {
  font-size: 18px;
  font-weight: 700;
}

.portal-list-card__subtitle,
.portal-list-card__meta,
.portal-empty__text,
.portal-copy-row__label {
  color: var(--portal-text-secondary);
  font-size: 13px;
}

.portal-list-card__badge,
.portal-tag {
  border: 1px solid var(--portal-border);
  border-radius: 999px;
  padding: 5px 10px;
  color: var(--portal-text-secondary);
  font-size: 12px;
}

.portal-list-card__intro {
  color: var(--portal-text-primary);
  line-height: 1.7;
  margin: 18px 0 14px;
}

.portal-list-card__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 16px;
}

.portal-empty {
  text-align: center;
  padding: 48px 24px;
}

.portal-empty__title {
  font-size: 20px;
  font-weight: 700;
}

.portal-detail-stack {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.portal-inline-code {
  flex: 1;
  min-width: 0;
  overflow: auto;
}

.portal-code-block {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
}

@media (max-width: 960px) {
  .portal-page__summary {
    grid-template-columns: 1fr;
  }
}
</style>

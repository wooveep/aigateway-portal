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
  loadManagedAccounts,
  activeConsumerName,
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
    <div class="portal-metric-strip">
      <div class="portal-metric">
        <div class="portal-metric__label">可见智能体</div>
        <div class="portal-metric__value">{{ agents.length }}</div>
      </div>
    </div>

    <section v-if="loading" class="portal-section">
      <a-skeleton active :paragraph="{ rows: 6 }" />
    </section>

    <section v-else-if="emptyState" class="portal-section portal-empty">
      <div class="portal-empty__title">当前范围下还没有可见智能体</div>
    </section>

    <div v-else class="portal-grid portal-grid--cards">
      <article v-for="item in agents" :key="item.id" class="portal-record portal-agent-card">
        <div class="portal-record__header">
          <div>
            <div class="portal-record__title">{{ item.displayName }}</div>
            <div class="portal-record__subtitle">{{ item.canonicalName }}</div>
          </div>
          <span class="portal-pill">{{ item.toolCount }} tools</span>
        </div>

        <div class="portal-record__summary portal-agent-card__summary">{{ item.intro || item.description }}</div>

        <div class="portal-data-grid">
          <div class="portal-data-item">
            <div class="portal-data-item__label">传输</div>
            <div class="portal-data-item__value">{{ item.transportTypes?.join(' / ') || 'http / sse' }}</div>
          </div>
          <div class="portal-data-item">
            <div class="portal-data-item__label">MCP Server</div>
            <div class="portal-data-item__value">{{ item.mcpServerName }}</div>
          </div>
          <div class="portal-data-item">
            <div class="portal-data-item__label">标签</div>
            <div class="portal-data-item__value">{{ item.tags?.join(' / ') || '-' }}</div>
          </div>
          <div class="portal-data-item">
            <div class="portal-data-item__label">更新时间</div>
            <div class="portal-data-item__value">{{ formatDateDisplay(item.updatedAt) }}</div>
          </div>
        </div>

        <div class="portal-agent-card__actions">
          <a-button @click="showDetail(item.id)">
            <LinkOutlined />
            查看详情
          </a-button>
          <a-button @click="copyText(item.httpUrl)">
            <CopyOutlined />
            复制 HTTP
          </a-button>
        </div>
      </article>
    </div>

    <a-drawer v-model:open="detailVisible" :width="760" title="智能体详情" destroy-on-close>
      <template v-if="selectedAgent">
        <div class="portal-stack">
          <section class="portal-section">
            <div class="portal-section__header">
              <div>
                <div class="portal-section__eyebrow">Agent</div>
                <h2 class="portal-section__title">{{ selectedAgent.displayName }}</h2>
              </div>
            </div>

            <div v-if="selectedAgent.description || selectedAgent.intro" class="portal-callout">
              {{ selectedAgent.description || selectedAgent.intro }}
            </div>

            <div class="portal-data-grid">
              <div class="portal-data-item">
                <div class="portal-data-item__label">Canonical Name</div>
                <div class="portal-data-item__value">{{ selectedAgent.canonicalName }}</div>
              </div>
              <div class="portal-data-item">
                <div class="portal-data-item__label">MCP Server</div>
                <div class="portal-data-item__value">{{ selectedAgent.mcpServerName }}</div>
              </div>
              <div class="portal-data-item">
                <div class="portal-data-item__label">Transport</div>
                <div class="portal-data-item__value">{{ selectedAgent.transportTypes?.join(' / ') || 'http / sse' }}</div>
              </div>
              <div class="portal-data-item">
                <div class="portal-data-item__label">Published</div>
                <div class="portal-data-item__value">{{ formatDateDisplay(selectedAgent.publishedAt) }}</div>
              </div>
            </div>
          </section>

          <section class="portal-section">
            <div class="portal-section__header">
              <div>
                <div class="portal-section__eyebrow">Endpoints</div>
                <h2 class="portal-section__title">接入地址</h2>
              </div>
            </div>
            <div class="portal-copy-row">
              <code class="portal-inline-code">{{ selectedAgent.httpUrl }}</code>
              <a-button @click="copyText(selectedAgent.httpUrl)">
                <CopyOutlined />
              </a-button>
            </div>
            <div class="portal-copy-row">
              <code class="portal-inline-code">{{ selectedAgent.sseUrl }}</code>
              <a-button @click="copyText(selectedAgent.sseUrl)">
                <CopyOutlined />
              </a-button>
            </div>
          </section>

          <section class="portal-section">
            <div class="portal-section__header">
              <div>
                <div class="portal-section__eyebrow">Resource</div>
                <h2 class="portal-section__title">只读摘要</h2>
              </div>
            </div>
            <pre class="portal-code-block">{{ selectedAgent.resourceSummary || '当前未声明 resource。' }}</pre>
          </section>

          <section class="portal-section">
            <div class="portal-section__header">
              <div>
                <div class="portal-section__eyebrow">Prompt</div>
                <h2 class="portal-section__title">接入说明</h2>
              </div>
            </div>
            <pre class="portal-code-block">{{ selectedAgent.promptSummary || '当前未声明 prompt。' }}</pre>
          </section>
        </div>
      </template>
    </a-drawer>
  </section>
</template>

<style scoped>
.portal-agent-card__summary {
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 84px;
}

.portal-agent-card__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 18px;
}
</style>

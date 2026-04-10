<script setup lang="ts">
import { CopyOutlined, LinkOutlined } from '@ant-design/icons-vue';
import { message } from 'ant-design-vue';
import { computed, onMounted, ref, watch } from 'vue';
import { fetchModelDetail, fetchModels } from '../api';
import { useManagedAccountScope } from '../composables/useManagedAccountScope';
import type { ModelInfo } from '../types';
import { formatDateDisplay } from '../utils/time';

const models = ref<ModelInfo[]>([]);
const selectedModel = ref<ModelInfo | null>(null);
const detailVisible = ref(false);
const loading = ref(false);

const {
  loadManagedAccounts,
  activeConsumerName,
} = useManagedAccountScope();

const emptyState = computed(() => !loading.value && models.value.length === 0);

const buildCapabilityText = (model: ModelInfo) => {
  const groups = [
    model.capabilities?.modalities?.join(' / '),
    model.capabilities?.features?.join(' / '),
    model.capabilities?.requestKinds?.join(' / '),
  ].filter(Boolean);
  return groups.join(' | ') || model.capability || '-';
};

const buildRequestURL = (model: ModelInfo) => {
  return model.requestUrl || model.endpoint || '/v1/chat/completions';
};

const loadModels = async () => {
  loading.value = true;
  try {
    models.value = await fetchModels(activeConsumerName.value || undefined);
  } catch {
    message.error('模型数据加载失败');
  } finally {
    loading.value = false;
  }
};

const showDetail = async (id: string) => {
  try {
    selectedModel.value = await fetchModelDetail(id, activeConsumerName.value || undefined);
    detailVisible.value = true;
  } catch {
    message.error('加载模型详情失败');
  }
};

const copyText = async (value: string) => {
  try {
    await navigator.clipboard.writeText(value);
    message.success('地址已复制');
  } catch {
    message.error('复制失败');
  }
};

onMounted(async () => {
  await loadManagedAccounts();
  await loadModels();
});

watch(() => activeConsumerName.value, () => {
  loadModels();
});
</script>

<template>
  <section class="portal-page">
    <div class="portal-metric-strip">
      <div class="portal-metric">
        <div class="portal-metric__label">可见模型</div>
        <div class="portal-metric__value">{{ models.length }}</div>
      </div>
    </div>

    <section v-if="loading" class="portal-section">
      <a-skeleton active :paragraph="{ rows: 6 }" />
    </section>

    <section v-else-if="emptyState" class="portal-section portal-empty">
      <div class="portal-empty__title">当前范围下没有可见模型</div>
    </section>

    <div v-else class="portal-grid portal-grid--cards">
      <article v-for="item in models" :key="item.id" class="portal-record portal-model-card">
        <div class="portal-record__header">
          <div>
            <div class="portal-record__title">{{ item.name }}</div>
            <div class="portal-record__subtitle">{{ item.vendor }}</div>
          </div>
          <span class="portal-pill">{{ item.sdk || 'OpenAI-compatible' }}</span>
        </div>

        <div class="portal-record__summary portal-model-card__summary">{{ item.summary || buildCapabilityText(item) }}</div>

        <div class="portal-data-grid">
          <div class="portal-data-item">
            <div class="portal-data-item__label">能力</div>
            <div class="portal-data-item__value">{{ buildCapabilityText(item) }}</div>
          </div>
          <div class="portal-data-item">
            <div class="portal-data-item__label">价格</div>
            <div class="portal-data-item__value">输入 {{ item.pricing?.inputPer1K ?? item.inputTokenPrice }} / 1K，输出 {{ item.pricing?.outputPer1K ?? item.outputTokenPrice }} / 1K</div>
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

        <div class="portal-model-card__actions">
          <a-button @click="showDetail(item.id)">
            <LinkOutlined />
            查看详情
          </a-button>
          <a-button @click="copyText(buildRequestURL(item))">
            <CopyOutlined />
            复制地址
          </a-button>
        </div>
      </article>
    </div>

    <a-drawer v-model:open="detailVisible" :width="720" title="模型详情" destroy-on-close>
      <template v-if="selectedModel">
        <div class="portal-stack">
          <section class="portal-section">
            <div class="portal-section__header">
              <div>
                <div class="portal-section__eyebrow">Model</div>
                <h2 class="portal-section__title">{{ selectedModel.name }}</h2>
              </div>
            </div>

            <div v-if="selectedModel.summary" class="portal-callout">
              {{ selectedModel.summary }}
            </div>

            <div class="portal-data-grid">
              <div class="portal-data-item">
                <div class="portal-data-item__label">供应商</div>
                <div class="portal-data-item__value">{{ selectedModel.vendor }}</div>
              </div>
              <div class="portal-data-item">
                <div class="portal-data-item__label">SDK / 协议</div>
                <div class="portal-data-item__value">{{ selectedModel.sdk || 'openai/v1' }}</div>
              </div>
              <div class="portal-data-item">
                <div class="portal-data-item__label">能力</div>
                <div class="portal-data-item__value">{{ buildCapabilityText(selectedModel) }}</div>
              </div>
              <div class="portal-data-item">
                <div class="portal-data-item__label">更新时间</div>
                <div class="portal-data-item__value">{{ formatDateDisplay(selectedModel.updatedAt) }}</div>
              </div>
            </div>
          </section>

          <section class="portal-section">
            <div class="portal-section__header">
              <div>
                <div class="portal-section__eyebrow">Request URL</div>
                <h2 class="portal-section__title">调用地址</h2>
              </div>
            </div>
            <div class="portal-copy-row">
              <code class="portal-inline-code">{{ buildRequestURL(selectedModel) }}</code>
              <a-button @click="copyText(buildRequestURL(selectedModel))">
                <CopyOutlined />
              </a-button>
            </div>
          </section>

          <section class="portal-section">
            <div class="portal-section__header">
              <div>
                <div class="portal-section__eyebrow">cURL</div>
                <h2 class="portal-section__title">接入示例</h2>
              </div>
            </div>
            <pre class="portal-code-block">curl -X POST {{ buildRequestURL(selectedModel) }} \
  -H "x-api-key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"{{ selectedModel.id }}","messages":[{"role":"user","content":"Hello"}]}'</pre>
          </section>
        </div>
      </template>
    </a-drawer>
  </section>
</template>

<style scoped>
.portal-model-card__summary {
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 84px;
}

.portal-model-card__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 18px;
}
</style>

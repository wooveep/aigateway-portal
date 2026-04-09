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
  hasManagedAccounts,
  loadingManagedAccounts,
  loadManagedAccounts,
  scopeOptions,
  activeConsumerName,
  currentScopeTitle,
  updateScopeConsumerName,
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
    <div class="portal-page__header">
      <div>
        <div class="portal-page__eyebrow">Models</div>
        <h1 class="portal-page__title">模型广场</h1>
        <p class="portal-page__description">这里只展示当前作用域下可见且已发布的模型，调用地址直接使用网关公开地址，不再展示 placeholder 域名。</p>
      </div>

      <div class="portal-page__scope">
        <span>当前范围</span>
        <a-select
          v-if="hasManagedAccounts"
          :value="activeConsumerName"
          :options="scopeOptions"
          :loading="loadingManagedAccounts"
          style="min-width: 260px"
          @update:value="updateScopeConsumerName(($event as string) || '')"
        />
        <span v-else class="portal-page__scope-text">{{ currentScopeTitle }}</span>
      </div>
    </div>

    <div class="portal-hero-card portal-page__summary">
      <div>
        <div class="portal-kv__label">可见模型</div>
        <div class="portal-kv__value">{{ models.length }}</div>
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
      <div class="portal-empty__title">当前范围下没有可见模型</div>
      <div class="portal-empty__text">请检查模型资产发布状态和绑定授权，或切换到其他账号范围查看。</div>
    </div>

    <div v-else class="portal-card-grid">
      <article v-for="item in models" :key="item.id" class="portal-list-card">
        <div class="portal-list-card__header">
          <div>
            <div class="portal-list-card__title">{{ item.name }}</div>
            <div class="portal-list-card__subtitle">{{ item.vendor }}</div>
          </div>
          <div class="portal-list-card__badge">{{ item.sdk }}</div>
        </div>

        <p class="portal-list-card__intro">{{ buildCapabilityText(item) }}</p>

        <div class="portal-list-card__meta">
          <span>输入 {{ item.pricing?.inputPer1K ?? item.inputTokenPrice }} / 1K</span>
          <span>输出 {{ item.pricing?.outputPer1K ?? item.outputTokenPrice }} / 1K</span>
        </div>

        <div class="portal-list-card__meta">
          <span>标签：{{ item.tags?.join(' / ') || '-' }}</span>
          <span>更新：{{ formatDateDisplay(item.updatedAt) }}</span>
        </div>

        <div class="portal-list-card__actions">
          <a-button @click="showDetail(item.id)">
            <LinkOutlined />
            查看详情
          </a-button>
          <a-button type="text" @click="copyText(buildRequestURL(item))">
            <CopyOutlined />
            复制地址
          </a-button>
        </div>
      </article>
    </div>

    <a-drawer v-model:open="detailVisible" :width="640" title="模型详情" destroy-on-close>
      <template v-if="selectedModel">
        <div class="portal-detail-stack">
          <div class="portal-hero-card">
            <div class="portal-kv__label">模型名称</div>
            <div class="portal-kv__value">{{ selectedModel.name }}</div>
            <p class="portal-page__description">{{ selectedModel.summary }}</p>
          </div>

          <div class="portal-hero-card">
            <div class="portal-kv__label">能力说明</div>
            <div class="portal-kv__value">{{ buildCapabilityText(selectedModel) }}</div>
          </div>

          <div class="portal-hero-card">
            <div class="portal-kv__label">调用地址</div>
            <div class="portal-copy-row">
              <code class="portal-inline-code">{{ buildRequestURL(selectedModel) }}</code>
              <a-button type="text" @click="copyText(buildRequestURL(selectedModel))">
                <CopyOutlined />
              </a-button>
            </div>
          </div>

          <div class="portal-hero-card">
            <div class="portal-kv__label">调用示例</div>
            <pre class="portal-code-block">curl -X POST {{ buildRequestURL(selectedModel) }} \
  -H "x-api-key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"{{ selectedModel.id }}","messages":[{"role":"user","content":"Hello"}]}'</pre>
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
.portal-empty__text {
  color: var(--portal-text-secondary);
  font-size: 13px;
}

.portal-list-card__badge {
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

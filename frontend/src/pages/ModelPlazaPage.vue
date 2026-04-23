<script setup lang="ts">
import { CopyOutlined, LinkOutlined } from '@ant-design/icons-vue';
import { message } from 'ant-design-vue';
import { computed, onMounted, ref, watch } from 'vue';
import { fetchModelDetail, fetchModels } from '../api';
import { useManagedAccountScope } from '../composables/useManagedAccountScope';
import type { ModelInfo } from '../types';
import { copyTextToClipboard } from '../utils/clipboard';
import { formatDateDisplay } from '../utils/time';

const MODEL_TYPE_LABELS: Record<string, string> = {
  text: '文本模型',
  multimodal: '全模态模型',
  image_generation: '图片生成',
  video_generation: '视频生成',
  speech_recognition: '语音识别',
  speech_synthesis: '语音合成',
  embedding: '向量模型',
};

const MODALITY_LABELS: Record<string, string> = {
  text: '文本',
  image: '图片',
  video: '视频',
  audio: '音频',
  embedding: '向量',
};

const FEATURE_FLAG_LABELS: Record<string, string> = {
  reasoning: '深度思考',
  vision: '视觉理解',
  function_calling: 'Function Calling',
  structured_output: '结构化输出',
  web_search: '联网搜索',
  prefix_completion: '前缀续写',
  prompt_cache: 'Cache 缓存',
  batch_inference: '批量推理',
  fine_tuning: '模型调优',
  long_context: '长上下文',
  model_experience: '模型体验',
};

type DetailRow = {
  label: string;
  value: string;
};

const models = ref<ModelInfo[]>([]);
const selectedModel = ref<ModelInfo | null>(null);
const detailVisible = ref(false);
const loading = ref(false);

const {
  loadManagedAccounts,
  activeConsumerName,
} = useManagedAccountScope();

const emptyState = computed(() => !loading.value && models.value.length === 0);

const modelTypeLabel = (model?: ModelInfo | null) =>
  MODEL_TYPE_LABELS[model?.modelType || ''] || model?.modelType || '未分类';

const featureLabels = (model?: ModelInfo | null) =>
  (model?.capabilities?.featureFlags?.length
    ? model.capabilities.featureFlags
    : model?.capabilities?.features || [])
    .map((item) => FEATURE_FLAG_LABELS[item] || item);

const inputModalities = (model?: ModelInfo | null) =>
  (model?.capabilities?.inputModalities?.length
    ? model.capabilities.inputModalities
    : model?.capabilities?.modalities || [])
    .map((item) => MODALITY_LABELS[item] || item);

const outputModalities = (model?: ModelInfo | null) =>
  (model?.capabilities?.outputModalities?.length
    ? model.capabilities.outputModalities
    : model?.capabilities?.modalities || [])
    .map((item) => MODALITY_LABELS[item] || item);

const buildCapabilitySummary = (model: ModelInfo) => {
  const parts = [
    inputModalities(model).length ? `输入 ${inputModalities(model).join(' / ')}` : '',
    outputModalities(model).length ? `输出 ${outputModalities(model).join(' / ')}` : '',
    featureLabels(model).length ? featureLabels(model).slice(0, 3).join(' / ') : '',
  ].filter(Boolean);
  return parts.join(' | ') || model.capability || '-';
};

const buildPriceRows = (model?: ModelInfo | null): DetailRow[] => {
  if (!model?.pricing) {
    return [];
  }
  const pricing = model.pricing;
  switch (model.modelType) {
    case 'text':
    case 'multimodal':
      return [
        typeof pricing.inputCostPerMillionTokens === 'number'
          ? { label: '输入', value: `${pricing.inputCostPerMillionTokens} 元 / 百万 tokens` }
          : null,
        typeof pricing.outputCostPerMillionTokens === 'number'
          ? { label: '输出', value: `${pricing.outputCostPerMillionTokens} 元 / 百万 tokens` }
          : null,
        typeof pricing.cacheCreationInputTokenCostPerMillionTokens === 'number'
          ? { label: '显式缓存创建', value: `${pricing.cacheCreationInputTokenCostPerMillionTokens} 元 / 百万 tokens` }
          : null,
        typeof pricing.cacheReadInputTokenCostPerMillionTokens === 'number'
          ? { label: '显式缓存命中', value: `${pricing.cacheReadInputTokenCostPerMillionTokens} 元 / 百万 tokens` }
          : null,
      ].filter(Boolean) as DetailRow[];
    case 'embedding':
      return typeof pricing.inputCostPerMillionTokens === 'number'
        ? [{ label: '输入', value: `${pricing.inputCostPerMillionTokens} 元 / 百万 tokens` }]
        : [];
    case 'image_generation':
      return typeof pricing.pricePerImage === 'number'
        ? [{ label: '图片生成', value: `${pricing.pricePerImage} 元 / 每张` }]
        : [];
    case 'video_generation':
      return [
        typeof pricing.pricePerSecond720p === 'number'
          ? { label: '视频生成（720P）', value: `${pricing.pricePerSecond720p} 元 / 每秒` }
          : null,
        typeof pricing.pricePerSecond1080p === 'number'
          ? { label: '视频生成（1080P）', value: `${pricing.pricePerSecond1080p} 元 / 每秒` }
          : null,
      ].filter(Boolean) as DetailRow[];
    case 'speech_recognition':
      return typeof pricing.pricePerSecond === 'number'
        ? [{ label: '语音识别', value: `${pricing.pricePerSecond} 元 / 每秒` }]
        : [];
    case 'speech_synthesis':
      return typeof pricing.pricePer10kChars === 'number'
        ? [{ label: '语音合成', value: `${pricing.pricePer10kChars} 元 / 每万字符` }]
        : [];
    default:
      return [
        typeof (pricing.inputCostPerMillionTokens ?? model.inputPricePerMillionTokens) === 'number'
          ? { label: '输入', value: `${pricing.inputCostPerMillionTokens ?? model.inputPricePerMillionTokens} 元 / 百万 tokens` }
          : null,
        typeof (pricing.outputCostPerMillionTokens ?? model.outputPricePerMillionTokens) === 'number'
          ? { label: '输出', value: `${pricing.outputCostPerMillionTokens ?? model.outputPricePerMillionTokens} 元 / 百万 tokens` }
          : null,
      ].filter(Boolean) as DetailRow[];
  }
};

const buildPriceSummary = (model: ModelInfo) => {
  const rows = buildPriceRows(model);
  return rows.length ? rows.map((item) => `${item.label} ${item.value}`).join(' / ') : '-';
};

const buildLimitRows = (model?: ModelInfo | null): DetailRow[] => {
  if (!model?.limits) {
    return [];
  }
  const limits = model.limits;
  const rows: Array<DetailRow | null> = [
    typeof limits.maxInputTokens === 'number' && limits.maxInputTokens > 0
      ? { label: '最大输入长度', value: String(limits.maxInputTokens) }
      : null,
    typeof limits.maxOutputTokens === 'number' && limits.maxOutputTokens > 0
      ? { label: '最大输出长度', value: String(limits.maxOutputTokens) }
      : null,
    typeof limits.maxInputTokensInReasoningMode === 'number' && limits.maxInputTokensInReasoningMode > 0
      ? { label: '最大输入长度（思考模式）', value: String(limits.maxInputTokensInReasoningMode) }
      : null,
    typeof limits.maxOutputTokensInReasoningMode === 'number' && limits.maxOutputTokensInReasoningMode > 0
      ? { label: '最大输出长度（思考模式）', value: String(limits.maxOutputTokensInReasoningMode) }
      : null,
    typeof limits.contextWindowTokens === 'number' && limits.contextWindowTokens > 0
      ? { label: '上下文长度', value: String(limits.contextWindowTokens) }
      : null,
    typeof limits.maxReasoningTokens === 'number' && limits.maxReasoningTokens > 0
      ? { label: '最大思维链长度', value: String(limits.maxReasoningTokens) }
      : null,
    typeof limits.rpm === 'number' && limits.rpm > 0
      ? { label: 'RPM', value: String(limits.rpm) }
      : null,
    typeof limits.tpm === 'number' && limits.tpm > 0
      ? { label: 'TPM', value: String(limits.tpm) }
      : null,
  ];
  return rows.filter(Boolean) as DetailRow[];
};

const buildRequestURL = (model: ModelInfo) => model.requestUrl || model.endpoint || '/v1/chat/completions';

const buildApiExample = (model: ModelInfo) => {
  const url = buildRequestURL(model);
  switch (model.modelType) {
    case 'embedding':
      return `curl -X POST ${url} \\
  -H "x-api-key: $API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"${model.id}","input":"你好，向量化这段文本"}'`;
    case 'image_generation':
      return `curl -X POST ${url} \\
  -H "x-api-key: $API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"${model.id}","prompt":"生成一张未来城市夜景海报"}'`;
    case 'video_generation':
      return `curl -X POST ${url} \\
  -H "x-api-key: $API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"${model.id}","prompt":"生成产品演示短视频","resolution":"1080p"}'`;
    case 'speech_recognition':
      return `curl -X POST ${url} \\
  -H "x-api-key: $API_KEY" \\
  -F "model=${model.id}" \\
  -F "file=@sample.wav"`;
    case 'speech_synthesis':
      return `curl -X POST ${url} \\
  -H "x-api-key: $API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"${model.id}","input":"你好，欢迎使用语音合成能力"}'`;
    default:
      return `curl -X POST ${url} \\
  -H "x-api-key: $API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"${model.id}","messages":[{"role":"user","content":"你好"}]}'`;
  }
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
    await copyTextToClipboard(value);
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
          <div class="portal-model-card__header-tags">
            <span class="portal-pill">{{ modelTypeLabel(item) }}</span>
            <span class="portal-pill portal-pill--soft">{{ item.sdk || 'OpenAI-compatible' }}</span>
          </div>
        </div>

        <div class="portal-record__summary portal-model-card__summary">{{ item.summary || buildCapabilitySummary(item) }}</div>

        <div class="portal-model-card__tags">
          <span
            v-for="tag in featureLabels(item).slice(0, 4)"
            :key="tag"
            class="portal-chip"
          >
            {{ tag }}
          </span>
        </div>

        <div class="portal-data-grid">
          <div class="portal-data-item">
            <div class="portal-data-item__label">能力</div>
            <div class="portal-data-item__value">{{ buildCapabilitySummary(item) }}</div>
          </div>
          <div class="portal-data-item">
            <div class="portal-data-item__label">模型价格</div>
            <div class="portal-data-item__value">{{ buildPriceSummary(item) }}</div>
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

    <a-drawer v-model:open="detailVisible" :width="860" title="模型详情" destroy-on-close>
      <template v-if="selectedModel">
        <div class="portal-stack">
          <section class="portal-section">
            <div class="portal-section__header">
              <div>
                <div class="portal-section__eyebrow">Overview</div>
                <h2 class="portal-section__title">模型介绍</h2>
              </div>
            </div>

            <div class="portal-callout">
              {{ selectedModel.summary || buildCapabilitySummary(selectedModel) }}
            </div>

            <div class="portal-data-grid">
              <div class="portal-data-item">
                <div class="portal-data-item__label">模型名称</div>
                <div class="portal-data-item__value">{{ selectedModel.name }}</div>
              </div>
              <div class="portal-data-item">
                <div class="portal-data-item__label">供应商</div>
                <div class="portal-data-item__value">{{ selectedModel.vendor }}</div>
              </div>
              <div class="portal-data-item">
                <div class="portal-data-item__label">模型类型</div>
                <div class="portal-data-item__value">{{ modelTypeLabel(selectedModel) }}</div>
              </div>
              <div class="portal-data-item">
                <div class="portal-data-item__label">更新时间</div>
                <div class="portal-data-item__value">{{ formatDateDisplay(selectedModel.updatedAt) }}</div>
              </div>
            </div>

            <div v-if="selectedModel.tags?.length" class="portal-model-card__tags">
              <span v-for="tag in selectedModel.tags" :key="tag" class="portal-chip">{{ tag }}</span>
            </div>
          </section>

          <section class="portal-section">
            <div class="portal-section__header">
              <div>
                <div class="portal-section__eyebrow">Capabilities</div>
                <h2 class="portal-section__title">模型能力</h2>
              </div>
            </div>

            <div class="portal-detail-grid">
              <article class="portal-detail-card">
                <div class="portal-detail-card__label">输入模态</div>
                <div class="portal-detail-card__value">{{ inputModalities(selectedModel).join(' / ') || '-' }}</div>
              </article>
              <article class="portal-detail-card">
                <div class="portal-detail-card__label">输出模态</div>
                <div class="portal-detail-card__value">{{ outputModalities(selectedModel).join(' / ') || '-' }}</div>
              </article>
              <article class="portal-detail-card portal-detail-card--wide">
                <div class="portal-detail-card__label">固定能力标签</div>
                <div class="portal-model-card__tags">
                  <span
                    v-for="tag in featureLabels(selectedModel)"
                    :key="tag"
                    class="portal-chip"
                  >
                    {{ tag }}
                  </span>
                  <span v-if="!featureLabels(selectedModel).length" class="portal-detail-card__value">-</span>
                </div>
              </article>
            </div>
          </section>

          <section class="portal-section">
            <div class="portal-section__header">
              <div>
                <div class="portal-section__eyebrow">Pricing</div>
                <h2 class="portal-section__title">模型价格</h2>
              </div>
            </div>

            <div v-if="buildPriceRows(selectedModel).length" class="portal-detail-grid">
              <article
                v-for="row in buildPriceRows(selectedModel)"
                :key="row.label"
                class="portal-detail-card"
              >
                <div class="portal-detail-card__label">{{ row.label }}</div>
                <div class="portal-detail-card__value">{{ row.value }}</div>
              </article>
            </div>
            <div v-else class="portal-empty-note">当前模型没有可展示的价格配置。</div>
          </section>

          <section class="portal-section">
            <div class="portal-section__header">
              <div>
                <div class="portal-section__eyebrow">Limits</div>
                <h2 class="portal-section__title">模型限流与上下文</h2>
              </div>
            </div>

            <div v-if="buildLimitRows(selectedModel).length" class="portal-detail-grid">
              <article
                v-for="row in buildLimitRows(selectedModel)"
                :key="row.label"
                class="portal-detail-card"
              >
                <div class="portal-detail-card__label">{{ row.label }}</div>
                <div class="portal-detail-card__value">{{ row.value }}</div>
              </article>
            </div>
            <div v-else class="portal-empty-note">当前模型没有可展示的限流与上下文配置。</div>
          </section>

          <section class="portal-section">
            <div class="portal-section__header">
              <div>
                <div class="portal-section__eyebrow">API Example</div>
                <h2 class="portal-section__title">API 示例</h2>
              </div>
            </div>

            <div class="portal-copy-row">
              <code class="portal-inline-code">{{ buildRequestURL(selectedModel) }}</code>
              <a-button @click="copyText(buildRequestURL(selectedModel))">
                <CopyOutlined />
              </a-button>
            </div>
            <pre class="portal-code-block">{{ buildApiExample(selectedModel) }}</pre>
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
  min-height: 72px;
}

.portal-model-card__header-tags,
.portal-model-card__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.portal-chip {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--portal-primary) 10%, white);
  color: var(--portal-primary);
  font-size: 12px;
  font-weight: 600;
}

.portal-pill--soft {
  background: var(--portal-surface-soft);
  color: var(--portal-text-soft);
}

.portal-model-card__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 18px;
}

.portal-detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.portal-detail-card {
  padding: 14px 16px;
  border: 1px solid var(--portal-border);
  border-radius: 14px;
  background: var(--portal-surface-soft);
}

.portal-detail-card--wide {
  grid-column: 1 / -1;
}

.portal-detail-card__label {
  margin-bottom: 8px;
  color: var(--portal-text-soft);
  font-size: 12px;
}

.portal-detail-card__value {
  color: var(--portal-text);
  line-height: 1.6;
  word-break: break-word;
}

.portal-empty-note {
  padding: 18px 16px;
  border: 1px dashed var(--portal-border);
  border-radius: 14px;
  color: var(--portal-text-soft);
  background: var(--portal-surface-soft);
}

@media (max-width: 960px) {
  .portal-detail-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>

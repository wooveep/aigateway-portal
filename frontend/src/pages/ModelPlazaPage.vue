<template>
  <div>
    <a-card title="平台可用模型">
      <a-row :gutter="16">
        <a-col v-for="item in models" :key="item.id" :xs="24" :md="12" :xl="8" style="margin-bottom: 16px">
          <a-card :title="item.name" hoverable>
            <p><strong>厂商：</strong>{{ item.vendor }}</p>
            <p><strong>能力：</strong>{{ buildCapabilityText(item) }}</p>
            <p><strong>标签：</strong>{{ item.tags?.length ? item.tags.join(' / ') : '-' }}</p>
            <p><strong>价格：</strong>{{ buildPricingText(item) }}</p>
            <p><strong>更新时间：</strong>{{ formatDateDisplay(item.updatedAt) }}</p>
            <a-button type="link" @click="showDetail(item.id)">查看模型详情</a-button>
          </a-card>
        </a-col>
      </a-row>
    </a-card>

    <a-drawer v-model:open="detailVisible" :width="560" title="模型详情" destroy-on-close>
      <template v-if="selectedModel">
        <a-descriptions bordered :column="1">
          <a-descriptions-item label="模型名称">{{ selectedModel.name }}</a-descriptions-item>
          <a-descriptions-item label="厂商">{{ selectedModel.vendor }}</a-descriptions-item>
          <a-descriptions-item label="能力">{{ buildCapabilityText(selectedModel) }}</a-descriptions-item>
          <a-descriptions-item label="标签">{{ selectedModel.tags?.length ? selectedModel.tags.join(' / ') : '-' }}</a-descriptions-item>
          <a-descriptions-item label="请求类型">
            {{ selectedModel.capabilities?.requestKinds?.length ? selectedModel.capabilities.requestKinds.join(' / ') : '-' }}
          </a-descriptions-item>
          <a-descriptions-item label="输入 Token 单价">{{ buildInputPriceText(selectedModel) }}</a-descriptions-item>
          <a-descriptions-item label="输出 Token 单价">{{ buildOutputPriceText(selectedModel) }}</a-descriptions-item>
          <a-descriptions-item label="RPM 限制">{{ selectedModel.limits?.rpm ?? '-' }}</a-descriptions-item>
          <a-descriptions-item label="TPM 限制">{{ selectedModel.limits?.tpm ?? '-' }}</a-descriptions-item>
          <a-descriptions-item label="上下文限制">{{ selectedModel.limits?.contextWindow ?? '-' }}</a-descriptions-item>
          <a-descriptions-item label="调用路径">{{ selectedModel.endpoint }}</a-descriptions-item>
          <a-descriptions-item label="调用方式">{{ selectedModel.sdk }}</a-descriptions-item>
          <a-descriptions-item label="说明">{{ selectedModel.summary }}</a-descriptions-item>
        </a-descriptions>
        <a-card title="调用示例" class="portal-card">
          <pre class="code-block">curl -X POST {{ buildRequestUrl(selectedModel) }} \
  -H "x-api-key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"{{ selectedModel.id }}","messages":[{"role":"user","content":"Hello"}]}'</pre>
        </a-card>
      </template>
    </a-drawer>
  </div>
</template>

<script setup lang="ts">
import { fetchModelDetail, fetchModels } from '../api';
import type { ModelInfo } from '../types';
import { formatDateDisplay } from '../utils/time';
import { message } from 'ant-design-vue';
import { onMounted, ref } from 'vue';

const models = ref<ModelInfo[]>([]);
const selectedModel = ref<ModelInfo | null>(null);
const detailVisible = ref(false);

const joinText = (items?: string[]) => {
  return items && items.length ? items.join(' / ') : '';
};

const buildCapabilityText = (model: ModelInfo) => {
  const modalities = joinText(model.capabilities?.modalities);
  const features = joinText(model.capabilities?.features);
  const requestKinds = joinText(model.capabilities?.requestKinds);
  return [modalities, features, requestKinds].filter(Boolean).join(' | ') || model.capability || '-';
};

const buildInputPriceText = (model: ModelInfo) => {
  const currency = model.pricing?.currency || 'USD';
  const value = model.pricing?.inputPer1K ?? model.inputTokenPrice ?? 0;
  return `${currency} ${value} / 1K tokens`;
};

const buildOutputPriceText = (model: ModelInfo) => {
  const currency = model.pricing?.currency || 'USD';
  const value = model.pricing?.outputPer1K ?? model.outputTokenPrice ?? 0;
  return `${currency} ${value} / 1K tokens`;
};

const buildPricingText = (model: ModelInfo) => {
  return `输入 ${buildInputPriceText(model)}，输出 ${buildOutputPriceText(model)}`;
};

const buildRequestUrl = (model: ModelInfo) => {
  const endpoint = (model.endpoint || '').trim();
  if (endpoint.startsWith('http://') || endpoint.startsWith('https://')) {
    return endpoint;
  }
  const normalizedPath = endpoint && endpoint !== '-' ? endpoint : '/v1/chat/completions';
  return `https://api.example.com${normalizedPath}`;
};

const loadModels = async () => {
  try {
    models.value = await fetchModels();
  } catch {
    message.error('模型数据加载失败');
  }
};

const showDetail = async (id: string) => {
  try {
    selectedModel.value = await fetchModelDetail(id);
    detailVisible.value = true;
  } catch {
    message.error('加载模型详情失败');
  }
};

onMounted(loadModels);
</script>

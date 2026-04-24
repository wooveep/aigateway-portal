<template>
  <div class="portal-auth">
    <section class="portal-auth__hero">
      <div class="portal-page__eyebrow">AIGateway Portal</div>
      <h1 class="portal-auth__title">登录</h1>
    </section>

    <a-card class="auth-card" :bordered="false">
      <template #title>
        <h2 class="auth-title">登录</h2>
      </template>

      <a-alert
        v-if="ssoMessage"
        class="portal-auth__alert"
        type="success"
        show-icon
        :message="ssoMessage"
      />
      <a-alert
        v-if="ssoError"
        class="portal-auth__alert"
        type="error"
        show-icon
        :message="ssoError"
      />

      <a-form layout="vertical" :model="form" @finish="onSubmit">
        <a-form-item label="用户名" name="username" :rules="[{ required: true, message: '请输入用户名' }]">
          <a-input v-model:value="form.username" placeholder="请输入用户名" />
        </a-form-item>

        <a-form-item label="密码" name="password" :rules="[{ required: true, message: '请输入密码' }]">
          <a-input-password v-model:value="form.password" placeholder="请输入密码" />
        </a-form-item>

        <a-form-item>
          <a-button type="primary" html-type="submit" block :loading="loading">登录</a-button>
        </a-form-item>

        <template v-if="ssoConfig.enabled">
          <a-divider>或</a-divider>
          <a-form-item>
            <a-button block @click="startSSOLogin">{{ ssoConfig.displayName || '企业 SSO 登录' }}</a-button>
          </a-form-item>
        </template>

        <div class="portal-record__meta">
          没有账号？
          <a-button type="link" @click="goRegister">去注册</a-button>
        </div>
      </a-form>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, shallowRef, ref } from 'vue';
import { fetchSSOConfig, login, apiBaseURL } from '../api';
import { authState, refreshAuth } from '../auth';
import { message } from 'ant-design-vue';
import { useRoute, useRouter } from 'vue-router';

const router = useRouter();
const route = useRoute();
const loading = ref(false);
const ssoConfig = shallowRef({
  enabled: false,
  displayName: '企业 SSO 登录',
});
const form = reactive({
  username: '',
  password: '',
});

const redirectTarget = computed(() => typeof route.query.redirect === 'string' ? route.query.redirect : '/billing');
const ssoMessage = computed(() => typeof route.query.ssoMessage === 'string' ? route.query.ssoMessage : '');
const ssoError = computed(() => typeof route.query.ssoError === 'string' ? route.query.ssoError : '');

async function loadSSOConfig() {
  try {
    ssoConfig.value = await fetchSSOConfig();
  } catch {
    ssoConfig.value = {
      enabled: false,
      displayName: '企业 SSO 登录',
    };
  }
}

const onSubmit = async () => {
  loading.value = true;
  try {
    const user = await login({ username: form.username.trim(), password: form.password });
    authState.user = user;
    authState.initialized = true;
    await refreshAuth();
    message.success('登录成功');
    router.push(redirectTarget.value);
  } catch (error: any) {
    message.error(error?.response?.data?.message || '登录失败');
  } finally {
    loading.value = false;
  }
};

const startSSOLogin = () => {
  window.location.href = `${apiBaseURL}/auth/sso/authorize?redirect=${encodeURIComponent(redirectTarget.value)}`;
};

const goRegister = () => {
  router.push('/register');
};

onMounted(loadSSOConfig);
</script>

<style scoped>
.portal-auth__alert {
  margin-bottom: 16px;
}
</style>

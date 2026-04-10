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

        <div class="portal-record__meta">
          没有账号？
          <a-button type="link" @click="goRegister">去注册</a-button>
        </div>
      </a-form>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { login } from '../api';
import { authState, refreshAuth } from '../auth';
import { message } from 'ant-design-vue';
import { reactive, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

const router = useRouter();
const route = useRoute();
const loading = ref(false);
const form = reactive({
  username: '',
  password: '',
});

const onSubmit = async () => {
  loading.value = true;
  try {
    const user = await login({ username: form.username.trim(), password: form.password });
    authState.user = user;
    authState.initialized = true;
    await refreshAuth();
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/billing';
    message.success('登录成功');
    router.push(redirect);
  } catch (error: any) {
    message.error(error?.response?.data?.message || '登录失败');
  } finally {
    loading.value = false;
  }
};

const goRegister = () => {
  router.push('/register');
};
</script>

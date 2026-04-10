<template>
  <div class="portal-auth">
    <section class="portal-auth__hero">
      <div class="portal-page__eyebrow">Create Account</div>
      <h1 class="portal-auth__title">注册</h1>
    </section>

    <a-card class="auth-card" :bordered="false">
      <template #title>
        <h2 class="auth-title">注册</h2>
      </template>

      <a-form layout="vertical" :model="form" @finish="onSubmit">
        <a-form-item
          label="邀请码"
          name="inviteCode"
          :rules="[{ required: true, message: '请输入邀请码' }]"
          extra="请联系管理员获取邀请码。"
        >
          <a-input v-model:value="form.inviteCode" placeholder="请输入邀请码" />
        </a-form-item>

        <a-form-item label="用户名" name="username" :rules="[{ required: true, message: '请输入用户名' }]">
          <a-input v-model:value="form.username" placeholder="请输入用户名" />
        </a-form-item>

        <a-form-item label="显示名称">
          <a-input v-model:value="form.displayName" placeholder="可选，默认与用户名一致" />
        </a-form-item>

        <a-form-item label="邮箱">
          <a-input v-model:value="form.email" placeholder="可选" />
        </a-form-item>

        <a-form-item label="密码" name="password" :rules="[{ required: true, message: '请输入密码' }, { min: 8, message: '密码至少 8 位' }]">
          <a-input-password v-model:value="form.password" placeholder="至少 8 位" />
        </a-form-item>

        <a-form-item>
          <a-button type="primary" html-type="submit" block :loading="loading">提交注册</a-button>
        </a-form-item>

        <div class="portal-record__meta">
          已有账号？
          <a-button type="link" @click="goLogin">去登录</a-button>
        </div>
      </a-form>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { register } from '../api';
import { message } from 'ant-design-vue';
import { reactive, ref } from 'vue';
import { useRouter } from 'vue-router';

const router = useRouter();
const loading = ref(false);
const form = reactive({
  inviteCode: '',
  username: '',
  displayName: '',
  email: '',
  password: '',
});

const onSubmit = async () => {
  loading.value = true;
  try {
    await register({
      inviteCode: form.inviteCode.trim(),
      username: form.username.trim(),
      displayName: form.displayName.trim(),
      email: form.email.trim(),
      password: form.password,
    });
    message.success('注册成功，请联系管理员启用账号后登录');
    router.push('/login');
  } catch (error: any) {
    message.error(error?.response?.data?.message || '注册失败');
  } finally {
    loading.value = false;
  }
};

const goLogin = () => {
  router.push('/login');
};
</script>

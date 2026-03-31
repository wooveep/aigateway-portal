<script setup lang="ts">
import { changePassword } from '../api';
import { authState, clearAuth } from '../auth';
import { message } from 'ant-design-vue';
import { computed, reactive, ref } from 'vue';
import { useRouter } from 'vue-router';

const router = useRouter();
const loading = ref(false);
const form = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
});

const currentUsername = computed(() => authState.user?.consumerName || '-');

const validateConfirmPassword = async (_rule: unknown, value: string) => {
  if (!value) {
    return Promise.reject('请再次输入新密码');
  }
  if (value !== form.newPassword) {
    return Promise.reject('两次输入的新密码不一致');
  }
  return Promise.resolve();
};

const onSubmit = async () => {
  loading.value = true;
  try {
    await changePassword({
      oldPassword: form.oldPassword,
      newPassword: form.newPassword,
    });
    message.success('密码修改成功，请重新登录');
    clearAuth();
    await router.replace('/login');
  } catch (error: any) {
    message.error(error?.response?.data?.message || '修改密码失败');
  } finally {
    loading.value = false;
  }
};

const goBack = () => {
  router.push('/billing');
};
</script>

<template>
  <div class="change-password-page">
    <a-card class="portal-card change-password-card" :bordered="false" title="修改密码">
      <a-alert
        class="change-password-alert"
        message="修改成功后，当前登录会失效，请使用新密码重新登录。"
        type="info"
        show-icon
      />

      <a-form layout="vertical" :model="form" @finish="onSubmit">
        <a-form-item label="当前账号">
          <a-input :value="currentUsername" disabled />
        </a-form-item>

        <a-form-item
          label="当前密码"
          name="oldPassword"
          :rules="[{ required: true, message: '请输入当前密码' }]"
        >
          <a-input-password v-model:value="form.oldPassword" placeholder="请输入当前密码" />
        </a-form-item>

        <a-form-item
          label="新密码"
          name="newPassword"
          :rules="[
            { required: true, message: '请输入新密码' },
            { min: 8, message: '新密码至少 8 位' },
          ]"
        >
          <a-input-password v-model:value="form.newPassword" placeholder="请输入至少 8 位的新密码" />
        </a-form-item>

        <a-form-item
          label="确认新密码"
          name="confirmPassword"
          :dependencies="['newPassword']"
          :rules="[
            { required: true, message: '请再次输入新密码' },
            { validator: validateConfirmPassword },
          ]"
        >
          <a-input-password v-model:value="form.confirmPassword" placeholder="请再次输入新密码" />
        </a-form-item>

        <div class="change-password-actions">
          <a-button @click="goBack">返回</a-button>
          <a-button type="primary" html-type="submit" :loading="loading">确认修改</a-button>
        </div>
      </a-form>
    </a-card>
  </div>
</template>

<style scoped>
.change-password-page {
  display: flex;
  justify-content: center;
}

.change-password-card {
  width: 100%;
  max-width: 640px;
}

.change-password-alert {
  margin-bottom: 24px;
}

.change-password-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
</style>

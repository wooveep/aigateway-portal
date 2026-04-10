<script setup lang="ts">
import { changePassword } from '../../api';
import { message } from 'ant-design-vue';
import { computed, reactive, ref } from 'vue';

const props = withDefaults(defineProps<{
  username?: string;
  submitText?: string;
  showCancel?: boolean;
  cancelText?: string;
}>(), {
  username: '',
  submitText: '确认修改',
  showCancel: true,
  cancelText: '取消',
});

const emit = defineEmits<{
  cancel: [];
  success: [];
}>();

const submitting = ref(false);
const form = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
});

const accountLabel = computed(() => props.username || '-');

const validateConfirmPassword = async (_rule: unknown, value: string) => {
  if (!value) {
    return Promise.reject('请再次输入新密码');
  }
  if (value !== form.newPassword) {
    return Promise.reject('两次输入的新密码不一致');
  }
  return Promise.resolve();
};

const resetForm = () => {
  form.oldPassword = '';
  form.newPassword = '';
  form.confirmPassword = '';
};

const onSubmit = async () => {
  submitting.value = true;
  try {
    await changePassword({
      oldPassword: form.oldPassword,
      newPassword: form.newPassword,
    });
    resetForm();
    message.success('密码修改成功，请重新登录');
    emit('success');
  } catch (error: any) {
    message.error(error?.response?.data?.message || '修改密码失败');
  } finally {
    submitting.value = false;
  }
};
</script>

<template>
  <div class="password-form">
    <div class="password-form__summary">
      <div class="password-form__label">账号</div>
      <div class="password-form__value">{{ accountLabel }}</div>
    </div>

    <a-form layout="vertical" :model="form" @finish="onSubmit">
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
        <a-input-password v-model:value="form.newPassword" placeholder="至少 8 位" />
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

      <div class="password-form__actions">
        <a-button v-if="showCancel" @click="emit('cancel')">{{ cancelText }}</a-button>
        <a-button type="primary" html-type="submit" :loading="submitting">{{ submitText }}</a-button>
      </div>
    </a-form>
  </div>
</template>

<style scoped>
.password-form {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.password-form__summary {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 14px 16px;
  border: 1px solid var(--portal-border);
  border-radius: 14px;
  background: var(--portal-surface-subtle);
}

.password-form__label {
  font-size: 12px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--portal-text-muted);
}

.password-form__value {
  font-size: 15px;
  font-weight: 600;
  color: var(--portal-text-primary);
}

.password-form__actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

@media (max-width: 767px) {
  .password-form__actions {
    flex-direction: column-reverse;
  }
}
</style>

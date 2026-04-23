<script setup lang="ts">
import { authState, clearAuth } from '../auth';
import ChangePasswordForm from '../components/shell/ChangePasswordForm.vue';
import { computed } from 'vue';
import { useRouter } from 'vue-router';

const router = useRouter();

const currentUsername = computed(() => authState.user?.consumerName || '-');

const handleSuccess = async () => {
  clearAuth();
  await router.replace('/login');
};

const goBack = () => {
  router.push('/billing');
};
</script>

<template>
  <section class="portal-page">
    <section class="portal-section portal-page__narrow">
      <ChangePasswordForm
        :username="currentUsername"
        cancel-text="返回"
        @cancel="goBack"
        @success="handleSuccess"
      />
    </section>
  </section>
</template>

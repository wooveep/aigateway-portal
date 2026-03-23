import { reactive } from 'vue';
import { fetchMe } from './api';
import type { AuthUser } from './types';

export const authState = reactive<{
  initialized: boolean;
  user: AuthUser | null;
}>({
  initialized: false,
  user: null,
});

export async function ensureAuthLoaded() {
  if (authState.initialized) {
    return;
  }
  try {
    authState.user = await fetchMe();
  } catch {
    authState.user = null;
  } finally {
    authState.initialized = true;
  }
}

export async function refreshAuth() {
  authState.initialized = false;
  await ensureAuthLoaded();
}

export function clearAuth() {
  authState.user = null;
  authState.initialized = true;
}

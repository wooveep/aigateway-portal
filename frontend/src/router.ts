import { createRouter, createWebHistory } from 'vue-router';
import { authState, ensureAuthLoaded } from './auth';

const publicPaths = new Set(['/login', '/register']);

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/billing' },
    { path: '/login', name: 'login', component: () => import('./pages/LoginPage.vue') },
    { path: '/register', name: 'register', component: () => import('./pages/RegisterPage.vue') },
    { path: '/billing', name: 'billing', component: () => import('./pages/BillingPage.vue') },
    { path: '/accounts', name: 'accounts', component: () => import('./pages/ManagedAccountsPage.vue') },
    { path: '/change-password', name: 'change-password', component: () => import('./pages/ChangePasswordPage.vue') },
    { path: '/models', name: 'models', component: () => import('./pages/ModelPlazaPage.vue') },
    { path: '/agents', name: 'agents', component: () => import('./pages/AgentPlazaPage.vue') },
    { path: '/ai-chat', name: 'ai-chat', component: () => import('./pages/AIChatPage.vue') },
    { path: '/open-platform', name: 'open-platform', component: () => import('./pages/OpenPlatformPage.vue') },
    { path: '/invoices', name: 'invoices', component: () => import('./pages/InvoicePage.vue') },
  ],
});

router.beforeEach(async (to) => {
  await ensureAuthLoaded();
  const isPublic = publicPaths.has(to.path);

  if (!authState.user && !isPublic) {
    return { path: '/login', query: { redirect: to.fullPath } };
  }

  if (authState.user && isPublic) {
    const redirect = typeof to.query.redirect === 'string' ? to.query.redirect : '/billing';
    return { path: redirect };
  }

  if (to.path === '/accounts' && authState.user && !authState.user.isDepartmentAdmin) {
    return { path: '/billing' };
  }

  return true;
});

export default router;

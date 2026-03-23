import { createRouter, createWebHistory } from 'vue-router';
import BillingPage from './pages/BillingPage.vue';
import ModelPlazaPage from './pages/ModelPlazaPage.vue';
import OpenPlatformPage from './pages/OpenPlatformPage.vue';
import InvoicePage from './pages/InvoicePage.vue';
import LoginPage from './pages/LoginPage.vue';
import RegisterPage from './pages/RegisterPage.vue';
import { authState, ensureAuthLoaded } from './auth';

const publicPaths = new Set(['/login', '/register']);

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/billing' },
    { path: '/login', name: 'login', component: LoginPage },
    { path: '/register', name: 'register', component: RegisterPage },
    { path: '/billing', name: 'billing', component: BillingPage },
    { path: '/models', name: 'models', component: ModelPlazaPage },
    { path: '/open-platform', name: 'open-platform', component: OpenPlatformPage },
    { path: '/invoices', name: 'invoices', component: InvoicePage },
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

  return true;
});

export default router;

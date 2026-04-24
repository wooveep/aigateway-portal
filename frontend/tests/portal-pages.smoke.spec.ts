import { expect, request, test, type APIRequestContext, type Page } from '@playwright/test';

const username = process.env.PORTAL_E2E_USERNAME || 'portal-e2e-user';
const password = process.env.PORTAL_E2E_PASSWORD || 'PortalE2E!123';
const expectAdmin = /^(1|true|yes)$/i.test(process.env.PORTAL_E2E_EXPECT_ADMIN || '');
const consoleBaseURL = process.env.CONSOLE_E2E_BASE_URL || 'http://127.0.0.1:8080';
const consoleAdminUsername = process.env.CONSOLE_E2E_ADMIN_USERNAME || 'admin';
const consoleAdminPassword = process.env.CONSOLE_E2E_ADMIN_PASSWORD || 'admin';

type DepartmentNode = {
  departmentId: string;
  name: string;
  children?: DepartmentNode[];
};

function unwrapData<T>(payload: unknown): T {
  if (payload && typeof payload === 'object' && 'data' in payload) {
    return (payload as { data: T }).data;
  }
  return payload as T;
}

function flattenDepartments(nodes: DepartmentNode[]): DepartmentNode[] {
  return nodes.reduce<DepartmentNode[]>((items, node) => {
    items.push(node, ...flattenDepartments(node.children || []));
    return items;
  }, []);
}

async function withConsoleAdminContext<T>(callback: (api: APIRequestContext) => Promise<T>) {
  const api = await request.newContext({
    baseURL: consoleBaseURL,
  });
  try {
    const loginResponse = await api.post('/session/login', {
      data: {
        username: consoleAdminUsername,
        password: consoleAdminPassword,
      },
    });
    expect(loginResponse.ok(), await loginResponse.text()).toBeTruthy();
    return await callback(api);
  } finally {
    await api.dispose();
  }
}

async function ensurePortalTestAccount() {
  await withConsoleAdminContext(async (api) => {
    const accountsResponse = await api.get('/v1/org/accounts');
    expect(accountsResponse.ok(), await accountsResponse.text()).toBeTruthy();
    const accounts = unwrapData<Array<{ consumerName: string; departmentId?: string }>>(await accountsResponse.json());
    const existing = Array.isArray(accounts)
      ? accounts.find((item) => item && item.consumerName === username)
      : null;

    let departmentId = existing?.departmentId || '';
    if (!departmentId) {
      const departmentsResponse = await api.get('/v1/org/departments/tree');
      expect(departmentsResponse.ok(), await departmentsResponse.text()).toBeTruthy();
      const departments = flattenDepartments(unwrapData<DepartmentNode[]>(await departmentsResponse.json()) || []);
      departmentId = departments.find((item) => item.name !== 'ROOT')?.departmentId || departments[0]?.departmentId || '';
    }

    const payload = {
      consumerName: username,
      displayName: 'Portal E2E User',
      email: 'portal-e2e@example.com',
      userLevel: 'normal',
      status: 'active',
      departmentId: departmentId || undefined,
      password,
    };

    const saveResponse = existing
      ? await api.put(`/v1/org/accounts/${encodeURIComponent(username)}`, { data: payload })
      : await api.post('/v1/org/accounts', { data: payload });
    expect(saveResponse.ok(), await saveResponse.text()).toBeTruthy();
  });
}

async function signIn(page: Page) {
  await ensurePortalTestAccount();

  const loginResponse = await page.request.post('/api/auth/login', {
    data: {
      username,
      password,
    },
  });
  expect(loginResponse.ok(), await loginResponse.text()).toBeTruthy();

  const meResponse = await page.request.get('/api/auth/me');
  expect(meResponse.ok(), await meResponse.text()).toBeTruthy();

  await page.goto('/billing');
  await page.waitForURL(/\/billing(?:\?|$)/);
}

async function getCurrentUser(page: Page) {
  const meResponse = await page.request.get('/api/auth/me');
  expect(meResponse.ok(), await meResponse.text()).toBeTruthy();
  return meResponse.json() as Promise<{ isDepartmentAdmin?: boolean }>;
}

async function expectPortalShell(page: Page, route: string, title: string) {
  await page.goto(route);
  await page.waitForLoadState('networkidle');
  await expect(page.locator('main .portal-shell__header-title')).toHaveText(title);
}

async function expectButtonOrEmptyState(page: Page, buttonName: string | RegExp, emptyTitle: string) {
  const actionButton = page.getByRole('button', { name: buttonName }).first();
  const emptyState = page.getByText(emptyTitle, { exact: true });
  if (await actionButton.count()) {
    await expect(actionButton).toBeVisible();
    return;
  }
  await expect(emptyState).toBeVisible();
}

test.describe('portal page smoke', () => {
  test.beforeEach(async ({ page }) => {
    await signIn(page);
  });

  test('loads billing, model plaza and agent plaza', async ({ page }) => {
    await expectPortalShell(page, '/billing', '个人账单');
    await expect(page.getByText('消费记录', { exact: true })).toBeVisible();

    await expectPortalShell(page, '/models', '模型广场');
    await expectButtonOrEmptyState(page, '查看详情', '当前范围下还没有可见模型');

    await expectPortalShell(page, '/agents', '智能体广场');
    await expectButtonOrEmptyState(page, '查看详情', '当前范围下还没有可见智能体');
  });

  test('loads open platform and opens create key dialog', async ({ page }) => {
    await expectPortalShell(page, '/open-platform', '开放平台');
    await expect(page.getByText('API Key 管理', { exact: true })).toBeVisible();
    await page.getByRole('button', { name: '新建 API Key' }).click();
    await expect(page.locator('.ant-modal-title').filter({ hasText: '新建 API Key' })).toBeVisible();
  });

  test('loads ai chat shell and composer controls', async ({ page }) => {
    await expectPortalShell(page, '/ai-chat', 'AI 对话');
    await expect(page.getByRole('button', { name: '新建会话' })).toBeVisible();
    await expect(page.locator('.chat-composer__select .ant-select-selector')).toHaveCount(2);
    await expect(page.getByPlaceholder('输入您的问题或指令...')).toBeVisible();
  });

  test('accounts route respects admin guard', async ({ page }) => {
    const currentUser = await getCurrentUser(page);
    const isDepartmentAdmin = typeof currentUser?.isDepartmentAdmin === 'boolean'
      ? currentUser.isDepartmentAdmin
      : expectAdmin;

    await page.goto('/accounts');
    await page.waitForLoadState('networkidle');

    if (isDepartmentAdmin) {
      await expect(page).toHaveURL(/\/accounts(?:\?|$)/);
      await expect(page.getByText('部门成员列表', { exact: true })).toBeVisible();
      await page.getByRole('button', { name: '新建成员' }).click();
      await expect(page.getByText('新建部门成员', { exact: true })).toBeVisible();
      return;
    }

    await expect(page).toHaveURL(/\/billing(?:\?|$)/);
  });
});

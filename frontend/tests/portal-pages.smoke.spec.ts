import { expect, test, type Page } from '@playwright/test';

const username = process.env.PORTAL_E2E_USERNAME || 'admin';
const password = process.env.PORTAL_E2E_PASSWORD || 'admin';
const expectAdmin = /^(1|true|yes)$/i.test(process.env.PORTAL_E2E_EXPECT_ADMIN || '');

async function signIn(page: Page) {
  await page.goto('/billing');

  if (!page.url().includes('/login')) {
    return;
  }

  await page.getByPlaceholder('请输入用户名').fill(username);
  await page.getByPlaceholder('请输入密码').fill(password);
  await page.getByRole('button', { name: '登录' }).click();
  await page.waitForURL(/\/billing(?:\?|$)/);
}

async function expectPortalShell(page: Page, route: string, title: string) {
  await page.goto(route);
  await page.waitForLoadState('networkidle');
  await expect(page.getByText(title, { exact: true })).toBeVisible();
}

test.describe('portal page smoke', () => {
  test.beforeEach(async ({ page }) => {
    await signIn(page);
  });

  test('loads billing, model plaza and agent plaza', async ({ page }) => {
    await expectPortalShell(page, '/billing', '个人账单');
    await expect(page.getByText('消费记录', { exact: true })).toBeVisible();

    await expectPortalShell(page, '/models', '模型广场');
    await expect(page.getByRole('button', { name: '查看详情' }).first()).toBeVisible();

    await expectPortalShell(page, '/agents', '智能体广场');
    await expect(page.getByRole('button', { name: '查看详情' }).first()).toBeVisible();
  });

  test('loads open platform and opens create key dialog', async ({ page }) => {
    await expectPortalShell(page, '/open-platform', '开放平台');
    await expect(page.getByText('API Key 管理', { exact: true })).toBeVisible();
    await page.getByRole('button', { name: '新建 API Key' }).click();
    await expect(page.getByText('新建 API Key', { exact: true })).toBeVisible();
  });

  test('loads ai chat shell and composer controls', async ({ page }) => {
    await expectPortalShell(page, '/ai-chat', 'AI 对话');
    await expect(page.getByRole('button', { name: '新建会话' })).toBeVisible();
    await expect(page.getByText('选择模型', { exact: true })).toBeVisible();
    await expect(page.getByText('选择 API Key', { exact: true })).toBeVisible();
  });

  test('accounts route respects admin guard', async ({ page }) => {
    await page.goto('/accounts');
    await page.waitForLoadState('networkidle');

    if (expectAdmin) {
      await expect(page).toHaveURL(/\/accounts(?:\?|$)/);
      await expect(page.getByText('部门成员列表', { exact: true })).toBeVisible();
      await page.getByRole('button', { name: '新建成员' }).click();
      await expect(page.getByText('新建部门成员', { exact: true })).toBeVisible();
      return;
    }

    await expect(page).toHaveURL(/\/billing(?:\?|$)/);
  });
});

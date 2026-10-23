import { expect, test, type Browser } from '@playwright/test';

import { storageState, USERS } from './support/accounts';

// The demo directory of docker-compose.dev.yml (deploy/ldap); the seed points
// KazUTB at it with sign-in turned off.
const STUDENT = { login: 'sarsenova.a', password: 'Aigerim-Ldap-2026', email: USERS.student };

// One university, one set of directory settings: the projects must not toggle
// them at the same time.
test.describe.configure({ mode: 'serial' });
test.skip(
  ({ isMobile }) => isMobile,
  'the settings are shared: the desktop project covers the flow',
);

/** Turns directory sign-in off again, so the run leaves the stack as it found it. */
async function turnOff(browser: Browser) {
  const context = await browser.newContext({ storageState: storageState('admin') });
  const request = context.request;
  const current = await request.get('/api/v1/admin/directory');
  if (!current.ok()) return;
  const {
    has_bind_password: _password,
    configured: _configured,
    plain_ldap_allowed: _plain,
    presets: _presets,
    ...settings
  } = (await current.json()) as Record<string, unknown>;
  if (settings.enabled === false) return;
  const csrf = (await context.cookies()).find((c) => c.name === 'qadam_csrf')?.value ?? '';
  await request.put('/api/v1/admin/directory', {
    headers: { 'X-CSRF-Token': csrf, 'If-Match': current.headers().etag ?? '' },
    data: { ...settings, enabled: false },
  });
  await context.close();
}

test.afterAll(async ({ browser }) => {
  await turnOff(browser);
});

test.describe('the administrator', () => {
  test.use({ storageState: storageState('admin') });

  test('checks the directory and a user, then turns sign-in on', async ({ page }) => {
    await page.goto('/app/admin/directory');
    await expect(page.getByRole('heading', { name: 'Вход через каталог', level: 1 })).toBeVisible();

    await page.getByRole('button', { name: 'Проверить подключение' }).click();
    await expect(page.getByText('Подключение работает')).toBeVisible();
    for (const step of ['connect', 'bind', 'search', 'server']) {
      await expect(page.getByTestId(`directory-step-${step}`).first()).toHaveAttribute(
        'data-status',
        'ok',
      );
    }

    await page.getByRole('button', { name: 'Проверить пользователя' }).click();
    const dialog = page.getByRole('dialog');
    await dialog.getByLabel('Логин').fill(STUDENT.login);
    await dialog.getByRole('button', { name: 'Найти' }).click();
    await expect(dialog.getByText('uid=sarsenova.a,ou=Students,dc=kazutb,dc=local')).toBeVisible();
    await expect(dialog.getByText('Вход будет разрешён')).toBeVisible();
    await page.keyboard.press('Escape');

    const toggle = page.getByRole('switch', { name: /Включить вход через каталог/ });
    if (!(await toggle.isChecked())) {
      await toggle.click();
      await page.getByRole('button', { name: 'Сохранить' }).click();
      await expect(page.getByText('Настройки сохранены')).toBeVisible();
    }
    await expect(page.getByTestId('directory-status')).toHaveText('Включён');
  });
});

test.describe('a student', () => {
  test('signs in with the university account', async ({ page }) => {
    await page.goto('/login');
    await expect(page.getByRole('radio', { name: 'Учётная запись КазУТБ' })).toBeChecked();

    await page.getByLabel('Логин').fill(STUDENT.login);
    await page.getByRole('textbox', { name: 'Пароль', exact: true }).fill('Not-The-Password-1');
    await page.getByRole('button', { name: 'Войти', exact: true }).click();
    await expect(page.getByText('Неверный логин или пароль.')).toBeVisible();

    await page.getByRole('textbox', { name: 'Пароль', exact: true }).fill(STUDENT.password);
    await page.getByRole('button', { name: 'Войти', exact: true }).click();
    await page.waitForURL((url) => !url.pathname.startsWith('/login'));
    // The demo account with the same address, linked to the directory.
    const session = await page.request.get('/api/v1/auth/session');
    expect(session.ok()).toBe(true);
    expect(((await session.json()) as { user: { email: string } }).user.email).toBe(STUDENT.email);
  });
});

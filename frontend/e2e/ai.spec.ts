import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';

import { storageState } from './support/accounts';

async function csrfToken(page: Page): Promise<string> {
  const cookies = await page.context().cookies();
  return cookies.find((cookie) => cookie.name === 'qadam_csrf')?.value ?? '';
}

/** Saves the current settings unchanged, as another administrator would: the version moves on. */
async function touchSettingsElsewhere(page: Page) {
  const current = await page.request.get('/api/v1/admin/ai/settings');
  expect(current.ok()).toBe(true);
  const settings = (await current.json()) as Record<string, unknown>;
  const saved = await page.request.put('/api/v1/admin/ai/settings', {
    headers: {
      'X-CSRF-Token': await csrfToken(page),
      'If-Match': current.headers().etag ?? '',
    },
    data: {
      routes: settings.routes,
      local_only: settings.local_only,
      daily_user_tokens: settings.daily_user_tokens,
      daily_tenant_tokens: settings.daily_tenant_tokens,
      mentor_mode: settings.mentor_mode,
    },
  });
  expect(saved.status(), await saved.text()).toBe(200);
}

async function expectNoSeriousViolations(page: Page) {
  const results = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
    .analyze();
  const serious = results.violations.filter(
    (v) => v.impact === 'serious' || v.impact === 'critical',
  );
  expect(
    serious.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`),
  ).toEqual([]);
}

async function expectNoHorizontalScroll(page: Page) {
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
  );
  expect(overflow).toBeLessThanOrEqual(0);
}

test.describe('AI administration', () => {
  test.use({ storageState: storageState('admin') });

  test('the connection check streams the model answer and shows up in the usage report', async ({
    page,
  }) => {
    await page.goto('/app/admin/ai');
    await expect(page.getByRole('heading', { name: 'AI-настройки', level: 1 })).toBeVisible();
    await expectNoHorizontalScroll(page);

    const mentor = page.getByTestId('ai-feature-mentor');
    await mentor.getByRole('button', { name: 'Проверить' }).click();
    const panel = mentor.getByTestId('ai-check');
    await expect(panel.getByText('Подключение работает')).toBeVisible({ timeout: 20_000 });
    await expect(panel.getByTestId('ai-check-text')).not.toBeEmpty();
    await expect(panel).toContainText('mock-mentor');
    await expect(panel).toContainText('connection_check@1');

    await page.getByRole('link', { name: 'Расход AI' }).first().click();
    await expect(page).toHaveURL(/\/app\/admin\/ai\/usage$/);
    await expect(page.getByRole('heading', { name: 'Расход AI', level: 1 })).toBeVisible();
    await expect(
      page.getByRole('region', { name: 'По функциям' }).getByRole('cell', { name: 'AI-ментор' }),
    ).toBeVisible();
    await page.getByRole('radio', { name: '7 дней' }).click();
    await expect(page).toHaveURL(/days=7/);
    await expectNoHorizontalScroll(page);
  });

  // Both tests edit the one settings record of the university: never at the same time.
  test.describe('settings', () => {
    test.describe.configure({ mode: 'serial' });

    test('settings are saved, and a concurrent change is detected instead of overwritten', async ({
      page,
    }, testInfo) => {
      // Both projects share one university's settings; one run is enough.
      test.skip(testInfo.project.name !== 'desktop', 'shared tenant settings');

      await page.goto('/app/admin/ai');
      await page.getByText('Соавтор', { exact: true }).click();
      await expect(page.getByText('Есть несохранённые изменения')).toBeVisible();
      await page.getByRole('button', { name: 'Сохранить' }).click();
      await expect(page.getByText('AI-настройки сохранены')).toBeVisible();
      await page.reload();
      await expect(page.getByRole('radio', { name: 'Соавтор' })).toBeChecked();

      // Another administrator saves meanwhile: this save must not silently win.
      await page.getByText('Наставник', { exact: true }).click();
      await touchSettingsElsewhere(page);
      await page.getByRole('button', { name: 'Сохранить' }).click();
      await expect(page.getByText('Настройки изменил другой администратор')).toBeVisible();
      await page.getByRole('button', { name: 'Загрузить актуальную версию' }).click();
      await expect(page.getByText('Настройки изменил другой администратор')).toBeHidden();

      await page.getByText('Наставник', { exact: true }).click();
      await page.getByRole('button', { name: 'Сохранить' }).click();
      await expect(page.getByText('AI-настройки сохранены')).toBeVisible();
      await expect(page.getByRole('radio', { name: 'Наставник' })).toBeChecked();
    });

    test('a custom route chain is edited and reset to the default', async ({ page }, testInfo) => {
      test.skip(testInfo.project.name !== 'desktop', 'shared tenant settings');

      await page.goto('/app/admin/ai');
      const summary = page.getByTestId('ai-feature-summary');
      await summary.getByRole('button', { name: 'Настроить' }).click();
      await expect(summary.getByText('Своя цепочка')).toBeVisible();
      await summary.getByRole('button', { name: 'Добавить резервный' }).click();
      await summary.getByRole('textbox', { name: 'Модель — Резерв 1' }).fill('mock-summary-small');
      await page.getByRole('button', { name: 'Сохранить' }).click();
      await expect(page.getByText('AI-настройки сохранены')).toBeVisible();

      await page.reload();
      await expect(summary.getByRole('textbox', { name: 'Модель — Резерв 1' })).toHaveValue(
        'mock-summary-small',
      );
      await summary.getByRole('button', { name: 'Вернуть по умолчанию' }).click();
      await page.getByRole('button', { name: 'Сохранить' }).click();
      await expect(page.getByText('AI-настройки сохранены')).toBeVisible();
      await expect(summary.getByText('По умолчанию', { exact: true })).toBeVisible();
    });
  });

  test('the knowledge base indexes a new document, finds it and deletes it', async ({ page }) => {
    const salt = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
    const title = `E2E регламент ${salt}`;
    const sentence = `Хакатон ${salt} проходит в коворкинге КазУТБ четырнадцатого ноября`;

    await page.goto('/app/admin/knowledge');
    await page.getByRole('button', { name: 'Добавить документ' }).first().click();
    const dialog = page.getByRole('dialog', { name: 'Новый документ' });
    await dialog.getByRole('button', { name: 'Добавить' }).click();
    await expect(dialog.getByText('Заполните это поле')).toBeVisible();
    await dialog.getByRole('textbox', { name: 'Название' }).fill(title);
    await dialog
      .getByRole('textbox', { name: 'Текст документа' })
      .fill(`Правила хакатона\n\n${sentence}. Команды регистрируются на платформе Qadam.`);
    await dialog.getByRole('button', { name: 'Добавить' }).click();
    await expect(page.getByText('Документ добавлен и индексируется')).toBeVisible();

    // The worker indexes it in the background; the list polls until it is ready.
    const row = page.getByTestId('knowledge-row').filter({ hasText: title });
    await expect(row.getByText('Готов').filter({ visible: true })).toBeVisible({
      timeout: 30_000,
    });
    await expectNoHorizontalScroll(page);

    await page.getByRole('searchbox', { name: 'Вопрос к базе знаний' }).fill(sentence);
    await expect(page.getByTestId('knowledge-hits').locator('li').first()).toContainText(title);

    await row.getByRole('button', { name: `Удалить «${title}»` }).click();
    await page
      .getByRole('alertdialog')
      .getByRole('button', { name: 'Удалить', exact: true })
      .click();
    await expect(page.getByText('Документ удалён')).toBeVisible();
    await expect(row).toHaveCount(0);
  });

  for (const url of ['/app/admin/ai', '/app/admin/ai/usage', '/app/admin/knowledge']) {
    test(`${url} has no serious accessibility violations`, async ({ page }, testInfo) => {
      test.skip(testInfo.project.name !== 'desktop', 'checked once per theme on desktop');
      for (const theme of ['light', 'dark']) {
        await page.addInitScript((t) => {
          localStorage.setItem('qadam.color-theme', t);
        }, theme);
        await page.goto(url);
        await page.waitForLoadState('networkidle');
        await expectNoSeriousViolations(page);
      }
    });
  }
});

test.describe('AI usage for leadership', () => {
  test.use({ storageState: storageState('rector') });

  test('leadership sees the usage report but not the AI settings', async ({ page }) => {
    await page.goto('/app/admin/ai/usage');
    // The rector reads the interface in Kazakh.
    await expect(
      page.getByRole('heading', { name: /Расход AI|AI шығыны/, level: 1 }),
    ).toBeVisible();
    await page.goto('/app/admin/ai');
    await expect(page).toHaveURL(/\/app$/);
  });
});

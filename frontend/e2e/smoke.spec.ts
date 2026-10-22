import { expect, test } from '@playwright/test';

test('the platform is up: web, API and database answer', async ({ page }) => {
  await page.goto('/');
  await expect(
    page.getByRole('heading', { level: 1, name: 'От идеи до рынка — один шаг' }),
  ).toBeVisible();
  await expect(page.getByTestId('api-version')).not.toBeEmpty();
});

test('the home page loads without Content Security Policy violations', async ({ page }) => {
  const violations: string[] = [];
  page.on('console', (message) => {
    if (message.type() === 'error' && message.text().includes('Content Security Policy')) {
      violations.push(message.text().slice(0, 200));
    }
  });
  await page.goto('/');
  await page.waitForLoadState('networkidle');
  await page.evaluate(() => document.fonts.ready);
  expect(violations).toEqual([]);
});

test('the interface switches between Kazakh, Russian and English', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'Язык' }).click();
  await page.getByRole('menuitemradio', { name: 'Қазақша' }).click();
  await expect(
    page.getByRole('heading', { level: 1, name: 'Идеядан нарыққа — бір қадам' }),
  ).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('lang', 'kk');

  await page.getByRole('button', { name: 'Тіл' }).click();
  await page.getByRole('menuitemradio', { name: 'English' }).click();
  await expect(
    page.getByRole('heading', { level: 1, name: 'From idea to market — one step' }),
  ).toBeVisible();

  await page.reload();
  await expect(
    page.getByRole('heading', { level: 1, name: 'From idea to market — one step' }),
  ).toBeVisible();
});

test('unknown pages show a localised not-found screen', async ({ page }) => {
  await page.goto('/no-such-page');
  await expect(page.getByRole('heading', { name: 'Страница не найдена' })).toBeVisible();
  await page.getByRole('link', { name: 'На главную' }).click();
  await expect(page).toHaveURL(/\/$/);
});

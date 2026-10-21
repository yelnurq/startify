import { expect, test } from '@playwright/test';

test('the platform is up: web, API and database answer', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { level: 1, name: 'Qadam' })).toBeVisible();
  await expect(page.getByTestId('api-version')).not.toBeEmpty();
});

test('the interface switches between Kazakh, Russian and English', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'Язык' }).click();
  await page.getByRole('menuitemradio', { name: 'Қазақша' }).click();
  await expect(page.getByText('Платформа күйі')).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('lang', 'kk');

  await page.getByRole('button', { name: 'Тіл' }).click();
  await page.getByRole('menuitemradio', { name: 'English' }).click();
  await expect(page.getByText('Platform status')).toBeVisible();

  await page.reload();
  await expect(page.getByText('Platform status')).toBeVisible();
});

test('unknown pages show a localised not-found screen', async ({ page }) => {
  await page.goto('/no-such-page');
  await expect(page.getByRole('heading', { name: 'Страница не найдена' })).toBeVisible();
  await page.getByRole('link', { name: 'На главную' }).click();
  await expect(page).toHaveURL(/\/$/);
});

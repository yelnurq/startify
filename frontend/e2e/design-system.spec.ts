import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';

const SCREENS = ['/design', '/design/shell', '/design/workspace', '/design/auth', '/'];

test.describe('design system', () => {
  for (const url of SCREENS) {
    test(`${url} never scrolls horizontally`, async ({ page }) => {
      await page.goto(url);
      await page.waitForLoadState('networkidle');
      const overflow = await page.evaluate(
        () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
      );
      expect(overflow).toBeLessThanOrEqual(0);
    });
  }

  test('/design has no serious accessibility violations', async ({ page }) => {
    await page.goto('/design');
    await page.waitForLoadState('networkidle');
    const results = await new AxeBuilder({ page })
      .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
      .analyze();
    const serious = results.violations.filter(
      (v) => v.impact === 'serious' || v.impact === 'critical',
    );
    expect(
      serious.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`),
    ).toEqual([]);
  });

  test.describe('with a dark system theme', () => {
    test.use({ colorScheme: 'dark' });

    test('the site stays light and offers no theme switch', async ({ page }) => {
      await page.addInitScript(() => {
        localStorage.setItem('qadam.color-theme', 'dark');
      });
      await page.goto('/');
      await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
      await expect(page.getByRole('button', { name: 'Оформление' })).toHaveCount(0);
    });
  });

  test('the command palette opens with the keyboard and navigates', async ({ page }) => {
    await page.goto('/design');
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
    await page.keyboard.press('Control+k');
    const palette = page.getByRole('dialog');
    await expect(palette).toBeVisible();
    await page.keyboard.type('Главная');
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/$/);
  });

  test('the showcase speaks Kazakh', async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('qadam.locale', 'kk');
    });
    await page.goto('/design');
    await expect(page.getByRole('heading', { level: 1 })).toHaveText('Qadam дизайн-жүйесі');
    await expect(page.locator('html')).toHaveAttribute('lang', 'kk');
  });
});

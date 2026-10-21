import { readFileSync } from 'node:fs';
import path from 'node:path';

import { expect, test, type Page } from '@playwright/test';

import { signIn, storageState, uniqueEmail } from './support/accounts';
import { linkTo, mailboxMark, waitForMessage } from './support/mailpit';

async function upload(page: Page, name: string, content: string | Buffer) {
  await page
    .getByTestId('import-dropzone')
    .locator('input[type=file]')
    .setInputFiles({
      name,
      mimeType: 'text/csv',
      buffer: typeof content === 'string' ? Buffer.from(`\uFEFF${content}`, 'utf8') : content,
    });
}

test.use({ storageState: storageState('admin') });

function summaryValue(page: Page, label: string) {
  return page
    .getByTestId('import-summary')
    .locator('div')
    .filter({ has: page.getByText(label, { exact: true }) })
    .locator('dd');
}

test('the administrator imports students, who are invited to set a password', async ({
  page,
  browser,
}) => {
  const first = uniqueEmail('import');
  const second = uniqueEmail('import');
  const since = await mailboxMark();

  await page.goto('/app/admin/import');
  await page.getByText('Студенты', { exact: true }).click();
  await upload(
    page,
    'students.csv',
    [
      'почта,фио,курс',
      `${first},Айым Нұрланқызы,1`,
      `${second},Дамир Омаров,2`,
      'not-an-email,Без почты,1',
    ].join('\n'),
  );

  await expect(summaryValue(page, 'Строк в файле')).toHaveText('3');
  await expect(summaryValue(page, 'С ошибками')).toHaveText('1');
  // Wide screens show a table, phones a list of cards: check the visible one.
  await expect(page.getByText('not-an-email').filter({ visible: true })).toBeVisible();

  await page.getByRole('button', { name: 'Импортировать 2 строки' }).click();
  await expect(page.getByText('Импорт завершён')).toBeVisible();
  await expect(page.getByTestId('import-result')).toContainText('Отправлено 2 приглашения');

  await page.goto('/app/admin/members');
  await page.getByLabel('Поиск участников').fill(first);
  await expect(
    page.getByTestId('member-row').filter({ hasText: first }).getByText('Приглашён'),
  ).toBeVisible();

  // The invited student sets a password from the letter and signs in.
  const letter = await waitForMessage(first, { since });
  const invited = await browser.newPage({ storageState: { cookies: [], origins: [] } });
  await invited.goto(linkTo(letter, '/reset-password'));
  await expect(invited.getByRole('heading', { name: 'Создайте пароль' })).toBeVisible();
  const password = 'Первый пароль студента 2026';
  await invited.getByRole('textbox', { name: 'Новый пароль', exact: true }).fill(password);
  await invited.getByRole('textbox', { name: 'Повторите пароль', exact: true }).fill(password);
  await invited.getByRole('button', { name: 'Сохранить пароль' }).click();
  await expect(invited).toHaveURL(/\/login/);
  await signIn(invited, first, password);
  await expect(invited).toHaveURL(/\/onboarding$/);
  await invited.close();
});

test('the templates import without errors', async ({ page }) => {
  const template = readFileSync(
    path.join(import.meta.dirname, '..', 'public', 'templates', 'structure.csv'),
  );
  await page.goto('/app/admin/import');
  await page.getByText('Структура вуза', { exact: true }).click();
  await upload(page, 'structure.csv', template);

  await expect(summaryValue(page, 'С ошибками')).toHaveText('0');
  await page.getByRole('button', { name: /^Импортировать/ }).click();
  await expect(page.getByText('Импорт завершён')).toBeVisible();

  // The students template matches the structure that was just imported.
  await page.getByRole('button', { name: 'Импортировать ещё файл' }).click();
  await page.getByText('Студенты', { exact: true }).click();
  await upload(
    page,
    'students.csv',
    readFileSync(path.join(import.meta.dirname, '..', 'public', 'templates', 'students.csv')),
  );
  await expect(summaryValue(page, 'Строк в файле')).toHaveText('5');
  await expect(summaryValue(page, 'С ошибками')).toHaveText('0');
});

test('files that are not CSV or XLSX are refused before upload', async ({ page }) => {
  await page.goto('/app/admin/import');
  await page
    .getByTestId('import-dropzone')
    .locator('input[type=file]')
    .setInputFiles({
      name: 'students.pdf',
      mimeType: 'application/pdf',
      buffer: Buffer.from('%PDF-1.7'),
    });
  await expect(page.getByText('Нужен файл CSV или XLSX.')).toBeVisible();
});

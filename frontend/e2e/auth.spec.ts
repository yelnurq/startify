import { expect, test } from '@playwright/test';

import {
  createStudent,
  openUserMenu,
  signIn,
  storageState,
  submitSignIn,
  uniqueEmail,
  USERS,
} from './support/accounts';
import { codeFrom, linkTo, mailboxMark, waitForMessage } from './support/mailpit';

// A seeded account that no other spec signs in with (its inbox stays predictable).
const CODE_USER = 'gulnar@qadam.test';

test('a student signs up, completes onboarding and confirms the e-mail', async ({
  page,
  isMobile,
}) => {
  test.skip(isMobile, 'sign-ups are rate-limited per IP; the desktop project covers the flow');
  const email = uniqueEmail('signup');
  const since = await mailboxMark();

  await page.goto('/register');
  await page.getByLabel('Имя и фамилия').fill('Айым Нұрланқызы');
  await page.getByLabel('Электронная почта').fill(email);
  await page.getByRole('textbox', { name: 'Пароль', exact: true }).fill('Жасыл алма таңертең 2026');
  await page
    .getByRole('checkbox', { name: 'Даю согласие на сбор и обработку моих персональных данных' })
    .check();
  await page.getByRole('button', { name: 'Создать аккаунт' }).click();

  await expect(page).toHaveURL(/\/onboarding$/);
  await page.getByRole('button', { name: 'Далее' }).click();

  // Studies: the seeded university has faculties, so the faculty is required.
  await expect(page.getByRole('heading', { name: 'Где вы учитесь?' })).toBeVisible();
  await page.getByRole('combobox', { name: 'Факультет' }).click();
  await page.getByRole('option').first().click();
  await page
    .getByRole('group', { name: 'Уровень обучения' })
    .getByText('Бакалавриат', { exact: true })
    .click();
  await page.getByRole('group', { name: 'Курс' }).getByText('1', { exact: true }).click();
  await page.getByRole('button', { name: 'Далее' }).click();

  await page.getByRole('button', { name: 'EdTech' }).click();
  await page.getByRole('button', { name: 'Далее' }).click();

  await page.getByText('Пока нет', { exact: true }).click();
  await page.getByRole('button', { name: 'Готово' }).click();

  await expect(page).toHaveURL(/\/app$/);
  // The dashboard greets by the first name.
  await expect(page.getByRole('heading', { level: 1 })).toContainText('Айым!');
  await expect(page.getByText('Подтвердите почту')).toBeVisible();

  const letter = await waitForMessage(email, { since, subject: /подтвердите/ });
  await page.goto(linkTo(letter, '/verify-email'));
  await expect(page.getByText('Почта подтверждена')).toBeVisible();

  // The profile: a headline, a skill and a public page.
  await page.goto('/app/profile');
  await page.getByLabel('Коротко о себе').fill('Студентка 1 курса, люблю EdTech');
  await page.getByPlaceholder('Свой навык, например Python').fill('Figma');
  await page.getByPlaceholder('Свой навык, например Python').press('Enter');
  const isPublic = page.getByRole('switch', { name: 'Публичный профиль' });
  if ((await isPublic.getAttribute('aria-checked')) !== 'true') await isPublic.click();
  await page.getByRole('button', { name: 'Сохранить' }).click();
  await expect(page.getByText('Профиль сохранён')).toBeVisible();

  const url = await page.getByTestId('public-profile-url').textContent();
  await page.goto(new URL(url ?? '').pathname);
  await expect(page.getByRole('heading', { level: 1, name: 'Айым Нұрланқызы' })).toBeVisible();
  await expect(page.getByText('Студентка 1 курса, люблю EdTech')).toBeVisible();
});

test('a member signs in with a password, stays signed in and signs out', async ({ page }) => {
  await signIn(page, USERS.student);
  await expect(page).toHaveURL(/\/app$/);

  await page.reload();
  await expect(page).toHaveURL(/\/app$/);

  await openUserMenu(page);
  await page.getByRole('menuitem', { name: 'Выйти' }).click();
  await expect(page).toHaveURL(/\/login/);

  await page.goto('/app/team');
  await expect(page).toHaveURL(/\/login\?redirect=%2Fapp%2Fteam/);
});

test('a wrong password gets a clear message', async ({ page }) => {
  await submitSignIn(page, USERS.student, 'not-the-password');
  await expect(page.getByText('Неверная почта или пароль.')).toBeVisible();
  await expect(page).toHaveURL(/\/login/);
});

test('signing in returns to the page that asked for it', async ({ page }) => {
  await page.goto('/app/settings/security');
  await expect(page).toHaveURL(/\/login\?redirect=/);
  await page.getByLabel('Электронная почта').fill(USERS.student);
  await page.getByRole('textbox', { name: 'Пароль', exact: true }).fill('Qadam-Demo-2026!');
  await page.getByRole('button', { name: 'Войти', exact: true }).click();
  await expect(page).toHaveURL(/\/app\/settings\/security$/);
  await expect(page.getByText('Это устройство')).toBeVisible();
});

// Codes and recovery letters are rate-limited per address and IP: run these once.
test.describe('e-mailed codes and links', () => {
  test.skip(({ isMobile }) => isMobile, 'covered by the desktop project');

  test('a member signs in with a code from the e-mail', async ({ page }) => {
    const since = await mailboxMark();
    await page.goto('/login');
    await page.getByLabel('Электронная почта').fill(CODE_USER);
    await page.getByRole('button', { name: 'Войти по коду из письма' }).click();
    await expect(page).toHaveURL(/\/login\/code/);

    const letter = await waitForMessage(CODE_USER, { since, subject: /Код для входа/ });
    await page.getByRole('textbox', { name: 'Цифра 1 из 6' }).fill(codeFrom(letter));
    await expect(page).toHaveURL(/\/app$/);
  });

  test('a forgotten password is replaced through the e-mailed link', async ({ page, browser }) => {
    const setup = await browser.newPage();
    const { email } = await createStudent(setup);
    await setup.close();

    const since = await mailboxMark();
    await page.goto('/forgot-password');
    await page.getByLabel('Электронная почта').fill(email);
    await page.getByRole('button', { name: 'Отправить ссылку' }).click();
    await expect(page.getByText('Проверьте почту').first()).toBeVisible();

    const letter = await waitForMessage(email, { since, subject: /восстановление пароля/ });
    await page.goto(linkTo(letter, '/reset-password'));
    const password = 'Новый пароль для Qadam 2026';
    await page.getByRole('textbox', { name: 'Новый пароль', exact: true }).fill(password);
    await page.getByRole('textbox', { name: 'Повторите пароль', exact: true }).fill(password);
    await page.getByRole('button', { name: 'Сохранить пароль' }).click();
    await expect(page).toHaveURL(/\/login/);

    await signIn(page, email, password);
    await expect(page).toHaveURL(/\/app$/);
  });
});

test.describe('as a student', () => {
  test.use({ storageState: storageState('student') });

  test('administration is closed', async ({ page }) => {
    await page.goto('/app');
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Участники' })).toHaveCount(0);

    await page.goto('/app/admin/members');
    await expect(page).toHaveURL(/\/app$/);
  });
});

test.describe('as the innovation manager', () => {
  test.use({ storageState: storageState('manager') });

  test('members are visible but roles cannot be changed', async ({ page }) => {
    await page.goto('/app/admin/members');
    await expect(page.getByRole('heading', { name: 'Участники', level: 1 })).toBeVisible();
    await expect(page.getByTestId('member-row').first()).toBeVisible();
    await expect(page.getByRole('button', { name: /^Действия:/ })).toHaveCount(0);

    await page.goto('/app/admin/import');
    await expect(page).toHaveURL(/\/app$/);
  });
});

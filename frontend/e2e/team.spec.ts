import { expect, test } from '@playwright/test';

import { storageState, USERS } from './support/accounts';
import { linkTo, mailboxMark, waitForMessage } from './support/mailpit';

test('a student finds a teammate on the «Ищу в команду» board', async ({ browser, isMobile }) => {
  // Each project uses its own pair of seeded students, so letters never mix.
  const authorUser = isMobile ? 'timur' : 'nurlan';
  const respondentUser = isMobile ? 'madina' : 'dana';
  const authorContext = await browser.newContext({ storageState: storageState(authorUser) });
  const page = await authorContext.newPage();
  // «E2E …» titles are removed after the run (global-teardown.ts).
  const title = `E2E Ищу дизайнера в агротех-стартап ${String(Date.now())}`;

  // The author publishes a request.
  await page.goto('/app/team');
  await page.getByRole('button', { name: 'Опубликовать запрос' }).first().click();
  const form = page.getByRole('dialog');
  await form.getByLabel('Заголовок').fill(title);
  await form
    .getByLabel('Описание')
    .fill('Делаем сервис, который помогает фермерам продавать урожай напрямую магазинам.');
  await form.getByRole('button', { name: 'Дизайнер' }).click();
  await form.getByPlaceholder('Например: Figma').fill('Figma');
  await form.getByPlaceholder('Например: Figma').press('Enter');
  await form.getByRole('button', { name: 'Опубликовать' }).click();
  await expect(form).toBeHidden();
  await expect(page.getByRole('heading', { name: title })).toBeVisible();

  // Another student finds it and replies.
  const respondentContext = await browser.newContext({
    storageState: storageState(respondentUser),
  });
  const other = await respondentContext.newPage();
  const since = await mailboxMark();
  await other.goto('/app/team');
  await other.getByLabel('Поиск по запросам').fill(title);
  const card = other.getByTestId('team-request').filter({ hasText: title });
  await card.getByRole('button', { name: 'Откликнуться' }).click();
  const reply = other.getByRole('dialog');
  await reply
    .getByLabel('Сообщение автору')
    .fill('Здравствуйте! Делаю интерфейсы в Figma, хочу присоединиться к проекту.');
  await reply.getByRole('button', { name: 'Отправить отклик' }).click();
  await expect(card.getByText('Вы откликнулись')).toBeVisible();
  await respondentContext.close();

  // The author gets a letter with a link to the request and sees the contact.
  const letter = await waitForMessage(USERS[authorUser], { since });
  await page.goto(linkTo(letter, '/team/'));
  await expect(page).toHaveURL(/\/app\/team\/[0-9a-f-]+$/);
  await expect(page.getByRole('link', { name: USERS[respondentUser] })).toBeVisible();

  // The author closes the request once the team is complete.
  await page.getByRole('button', { name: 'Действия с запросом' }).click();
  await page.getByRole('menuitem', { name: 'Закрыть набор' }).click();
  await expect(page.getByText('Набор закрыт').first()).toBeVisible();

  await page.goto('/app/team?tab=mine');
  await expect(
    page.getByTestId('team-request').filter({ hasText: title }).getByText('1 отклик'),
  ).toBeVisible();
  await authorContext.close();
});

test.describe('as a student', () => {
  test.use({ storageState: storageState('student') });

  test('a request needs a title, a description and at least one role', async ({ page }) => {
    await page.goto('/app/team?new=1');
    const form = page.getByRole('dialog');
    await form.getByRole('button', { name: 'Опубликовать' }).click();
    await expect(form.getByText('Минимум 5 символов')).toBeVisible();
    await expect(form.getByText('Минимум 20 символов')).toBeVisible();
    await expect(form.getByText('Выберите хотя бы 1 вариант')).toBeVisible();
  });
});

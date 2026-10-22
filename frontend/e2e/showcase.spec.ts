import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Browser, type Page } from '@playwright/test';

import { createStudent, storageState } from './support/accounts';

async function csrf(page: Page): Promise<string> {
  const cookies = await page.context().cookies();
  return cookies.find((cookie) => cookie.name === 'qadam_csrf')?.value ?? '';
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

/** A student with a fresh project whose first step is answered (the page is prefilled from it). */
async function studentWithProject(browser: Browser, title: string) {
  const context = await browser.newContext();
  const page = await context.newPage();
  await createStudent(page, { name: 'Ерлан Сәрсенов' });
  const tenant = (await (await page.request.get('/api/v1/tenant')).json()) as {
    ip_policy: { version: string } | null;
  };
  const created = await page.request.post('/api/v1/projects', {
    headers: { 'X-CSRF-Token': await csrf(page), 'Idempotency-Key': crypto.randomUUID() },
    data: {
      title,
      tagline: 'Свободная аудитория для самоподготовки за минуту',
      industry: 'edtech',
      ...(tenant.ip_policy ? { ip_policy_version: tenant.ip_policy.version } : {}),
    },
  });
  expect(created.status(), await created.text()).toBe(201);
  const projectId = ((await created.json()) as { id: string }).id;
  const current = await page.request.get(`/api/v1/projects/${projectId}/steps/problem`);
  const answered = await page.request.patch(`/api/v1/projects/${projectId}/steps/problem`, {
    headers: { 'X-CSRF-Token': await csrf(page), 'If-Match': current.headers().etag ?? '' },
    data: {
      values: {
        statement:
          'Первокурсники КазУТБ тратят до 30 минут в день на поиск свободной аудитории для самоподготовки и работы в командах.',
        who: 'Студенты первого и второго курса, около 3 000 человек в главном корпусе.',
        alternatives:
          'Обходят этажи и спрашивают на кафедрах; расписание не показывает, что аудитория свободна.',
      },
    },
  });
  expect(answered.status(), await answered.text()).toBe(200);
  return { context, page, projectId };
}

/** Fills the team's page and sends it to the showcase with the given visibility. */
async function publishPage(page: Page, projectId: string, slug: string, everyone: boolean) {
  await page.goto(`/app/projects/${projectId}/showcase`);
  await expect(page.getByRole('heading', { name: 'Витрина', level: 1 })).toBeVisible();
  await expect(page.getByText('Черновик из инкубатора')).toBeVisible();
  await page.getByLabel('Адрес страницы').fill(slug);
  await expect(page.getByRole('textbox', { name: 'Проблема', exact: true })).not.toHaveValue('');
  await page
    .getByRole('textbox', { name: 'Решение', exact: true })
    .fill(
      'Телеграм-бот показывает свободные аудитории по расписанию и отметкам студентов и бронирует их на час.',
    );
  await page.getByRole('button', { name: 'Пилот', exact: true }).click();
  if (everyone) await page.getByRole('radio', { name: /^Для всех/ }).check();
  await page
    .getByRole('button', { name: everyone ? 'Отправить на модерацию' : 'Опубликовать' })
    .click();
  await expect(
    page.getByText(everyone ? 'Страница отправлена на модерацию' : 'Страница опубликована'),
  ).toBeVisible();
}

/** Takes the test page off the showcase so that demo catalogues stay clean. */
async function takeDown(page: Page, projectId: string) {
  const response = await page.request.post(`/api/v1/projects/${projectId}/showcase/unpublish`, {
    headers: { 'X-CSRF-Token': await csrf(page) },
  });
  expect(response.status(), await response.text()).toBe(200);
}

test.describe('public showcase', () => {
  test('a visitor searches and filters the catalogue, and the address keeps the filters', async ({
    page,
  }) => {
    await page.goto('/showcase');
    await expect(
      page.getByRole('heading', { name: 'Проекты, которые ищут партнёров', level: 1 }),
    ).toBeVisible();
    await expect(page.getByRole('link', { name: 'AgroScan' })).toBeVisible();
    // Pages for members only and pages in review are not public.
    await expect(page.getByRole('link', { name: 'Sunqora' })).toHaveCount(0);
    await expect(page.getByRole('link', { name: 'QalaSignal' })).toHaveCount(0);
    await expectNoHorizontalScroll(page);

    await page.getByRole('searchbox', { name: 'Поиск по витрине' }).fill('AgroScan');
    await expect(page).toHaveURL(/[?&]q=AgroScan/);
    await expect(page.getByRole('link', { name: 'PomaFilm' })).toHaveCount(0);
    await expect(page.getByRole('link', { name: 'AgroScan' })).toBeVisible();

    await page.goto('/showcase?industry=agritech&need=pilot');
    await expect(page.getByRole('link', { name: 'AgroScan' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Kiiz Lab' })).toHaveCount(0);
    await page.getByRole('button', { name: 'Убрать фильтр «Агротех»' }).click();
    await expect(page).not.toHaveURL(/industry=/);
    await expect(page).toHaveURL(/need=pilot/);

    await page.goto('/showcase?q=несуществующийпроект');
    await expect(page.getByRole('heading', { name: 'Ничего не нашлось' })).toBeVisible();
    await page.getByRole('button', { name: 'Сбросить фильтры' }).click();
    await expect(page.getByRole('link', { name: 'AgroScan' })).toBeVisible();
  });

  test('quick view, the national scope and the project page', async ({ page }) => {
    await page.goto('/showcase');
    await page.getByRole('button', { name: 'Быстрый просмотр: AgroScan' }).click();
    const sheet = page.getByRole('dialog', { name: 'AgroScan' });
    await expect(sheet.getByText('Проблема', { exact: true })).toBeVisible();
    await sheet.getByRole('link', { name: 'Открыть страницу проекта' }).click();
    await expect(page).toHaveURL(/\/showcase\/agroscan$/);
    await expect(page.getByRole('heading', { name: 'AgroScan', level: 1 })).toBeVisible();
    await expect(page).toHaveTitle(/^AgroScan — витрина/);
    const anchors = page.getByRole('navigation', { name: 'Разделы страницы' });
    await expect(anchors.getByRole('link', { name: 'Команда' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Что нужно проекту', level: 2 })).toBeVisible();
    await expectNoHorizontalScroll(page);

    await page.goto('/showcase');
    await page.getByRole('radio', { name: 'Вся страна' }).click();
    await expect(page).toHaveURL(/scope=national/);
    await expect(
      page.getByRole('heading', { name: 'Проекты университетов Казахстана', level: 1 }),
    ).toBeVisible();
    await expect(page.getByText('КазУТБ').first()).toBeVisible();
    await page.getByRole('radio', { name: 'Вуз' }).click();
    await expect(page).not.toHaveURL(/scope=/);
  });

  test('a missing page says so', async ({ page }) => {
    await page.goto('/showcase/no-such-project-page');
    await expect(page.getByRole('heading', { name: 'Страница проекта не найдена' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'К витрине' })).toBeVisible();
  });

  test('the contact form checks the fields and keeps the bot trap empty', async ({
    page,
  }, testInfo) => {
    await page.goto('/showcase/agroscan');
    await page.getByRole('button', { name: 'Связаться с командой' }).first().click();
    const dialog = page.getByRole('dialog', { name: 'Связаться с командой' });
    await dialog.getByRole('button', { name: 'Отправить' }).click();
    await expect(dialog.getByText('Заполните это поле').first()).toBeVisible();
    // The trap is off screen, out of the tab order and hidden from assistive technology.
    const trap = dialog.locator('input[name="website"]');
    await expect(trap).not.toBeInViewport();
    await expect(trap).toHaveAttribute('tabindex', '-1');
    await expect(dialog.getByRole('textbox', { name: 'Сайт' })).toHaveCount(0);
    await expect(trap).toHaveValue('');
    await expectNoSeriousViolations(page);
    test.skip(
      testInfo.project.name === 'mobile',
      'requests are rate-limited: sent once, on desktop',
    );

    await dialog.getByLabel('Ваше имя').fill('Айдос Сеитов');
    await dialog.getByLabel('Электронная почта').fill('aidos@agrofund.kz');
    await dialog.getByLabel('Организация').fill('AgroFund');
    await dialog.getByRole('combobox', { name: 'Тема обращения' }).click();
    await page.getByRole('option', { name: 'Пилот' }).click();
    await dialog.getByLabel('Сообщение').fill('Коротко');
    await dialog.getByRole('button', { name: 'Отправить' }).click();
    await expect(dialog.getByText(/Минимум 20/)).toBeVisible();
    await dialog
      .getByLabel('Сообщение')
      .fill('Готовы дать поле в Акмолинской области для пилота следующей весной.');
    await dialog.getByRole('button', { name: 'Отправить' }).click();
    await expect(page.getByRole('dialog', { name: 'Заявка отправлена' })).toBeVisible();
    await expect(page.getByText(/Инновационный офис университета проверит её/)).toBeVisible();
  });

  for (const theme of ['light', 'dark'] as const) {
    test(`catalogue and project page pass axe in the ${theme} theme`, async ({ page }) => {
      await page.addInitScript((t) => {
        localStorage.setItem('qadam.theme', t);
      }, theme);
      for (const url of ['/showcase', '/showcase/agroscan', '/']) {
        await page.goto(url);
        // The app's page, not the first screen the server draws before it.
        await expect(page.locator('#root main h1')).toBeVisible();
        await expect(page.locator('#boot')).toHaveCount(0);
        await page.waitForLoadState('networkidle');
        await expectNoSeriousViolations(page);
        await expectNoHorizontalScroll(page);
      }
    });
  }

  test('the server draws the first screen, and the app takes its place', async ({ page }) => {
    const response = await page.request.get('/showcase/agroscan', {
      headers: { 'Accept-Language': 'ru-RU,ru;q=0.9' },
    });
    const html = await response.text();
    expect(html).toContain('<div id="boot" lang="ru"');
    expect(html).toContain('AgroScan</h1>');
    await page.goto('/showcase/agroscan');
    await expect(page.locator('#boot')).toHaveCount(0);
    await expect(
      page.locator('#root').getByRole('heading', { level: 1, name: 'AgroScan' }),
    ).toBeVisible();
    await expect(page.getByRole('heading', { level: 1 })).toHaveCount(1);
  });

  test('the landing page shows live numbers and the best projects', async ({ page }) => {
    await page.goto('/');
    const numbers = page.getByRole('region', { name: 'Qadam в цифрах' });
    await expect(numbers.getByText('Проектов на витрине')).toBeVisible();
    await expect(numbers.locator('dd')).toHaveCount(4);
    await expect(numbers.locator('dd .sr-only').first()).toHaveText(/^\d[\d\s]*$/);
    await expect(page.getByRole('heading', { name: 'Лучшие проекты витрины' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'AgroScan' })).toBeVisible();
    // Phones keep «Войти» in the header (B-011).
    await expect(page.getByRole('banner').getByRole('link', { name: 'Войти' })).toBeVisible();
    await page.getByRole('link', { name: 'Вся витрина' }).click();
    await expect(page).toHaveURL(/\/showcase$/);
  });
});

test.describe('showcase publishing and moderation', () => {
  test.describe.configure({ mode: 'serial' });

  test('a public page goes through review, a partner writes, the team gets the request', async ({
    browser,
  }, testInfo) => {
    test.skip(testInfo.project.name === 'mobile', 'the full cycle runs once, on desktop');
    test.setTimeout(180_000);
    const salt = Date.now().toString(36);
    const title = `Аудитория ${salt}`;
    const slug = `e2e-auditoria-${salt}`;
    const team = await studentWithProject(browser, title);
    await publishPage(team.page, team.projectId, slug, true);
    await expect(team.page.getByText('На модерации').first()).toBeVisible();
    await expectNoSeriousViolations(team.page);

    // Not public until approved.
    const visitorContext = await browser.newContext();
    const visitor = await visitorContext.newPage();
    await visitor.goto(`/showcase/${slug}`);
    await expect(
      visitor.getByRole('heading', { name: 'Страница проекта не найдена' }),
    ).toBeVisible();

    const managerContext = await browser.newContext({ storageState: storageState('manager') });
    const manager = await managerContext.newPage();
    await manager.goto('/app/admin/showcase');
    await expect(
      manager.getByRole('heading', { name: 'Модерация витрины', level: 1 }),
    ).toBeVisible();
    const card = manager.getByRole('article').filter({ hasText: title });
    await expect(card).toBeVisible();
    await expectNoSeriousViolations(manager);
    await card.getByRole('button', { name: 'Одобрить' }).click();
    await expect(manager.getByText('Страница опубликована')).toBeVisible();

    await visitor.goto(`/showcase?q=${salt}`);
    await expect(visitor.getByRole('link', { name: title })).toBeVisible();
    await visitor.getByRole('link', { name: title }).click();
    await expect(visitor.getByRole('heading', { name: title, level: 1 })).toBeVisible();
    await visitor.getByRole('button', { name: 'Связаться с командой' }).first().click();
    const dialog = visitor.getByRole('dialog', { name: 'Связаться с командой' });
    await dialog.getByLabel('Ваше имя').fill('Дина Оспанова');
    await dialog.getByLabel('Электронная почта').fill('dina@partner.kz');
    await dialog.getByRole('combobox', { name: 'Тема обращения' }).click();
    await visitor.getByRole('option', { name: 'Партнёрство' }).click();
    const message = `Предлагаем пилот в нашем коворкинге, напишите нам (${salt}).`;
    await dialog.getByLabel('Сообщение').fill(message);
    await dialog.getByRole('button', { name: 'Отправить' }).click();
    await expect(visitor.getByRole('dialog', { name: 'Заявка отправлена' })).toBeVisible();

    // The team sees nothing until the innovation office forwards the request.
    await team.page.reload();
    await expect(team.page.getByText('Пока никто не написал')).toBeVisible();

    await manager.goto('/app/admin/contacts');
    const request = manager.getByRole('article').filter({ hasText: message });
    await expect(request).toBeVisible();
    await expectNoSeriousViolations(manager);
    await request.getByRole('button', { name: 'Передать команде' }).click();
    await expect(manager.getByText('Заявка передана команде')).toBeVisible();

    await team.page.reload();
    await expect(team.page.getByText(message)).toBeVisible();
    await expect(team.page.getByRole('link', { name: 'Ответить на почту' })).toHaveAttribute(
      'href',
      'mailto:dina@partner.kz',
    );

    await takeDown(team.page, team.projectId);
    await team.context.close();
    await visitorContext.close();
    await managerContext.close();
  });

  test('a rejected page shows the comment; a university page is for members only', async ({
    browser,
  }, testInfo) => {
    test.skip(testInfo.project.name === 'mobile', 'the full cycle runs once, on desktop');
    test.setTimeout(180_000);
    const salt = Date.now().toString(36);
    const title = `Бронь ${salt}`;
    const team = await studentWithProject(browser, title);
    await publishPage(team.page, team.projectId, `e2e-bron-${salt}`, true);

    const managerContext = await browser.newContext({ storageState: storageState('manager') });
    const manager = await managerContext.newPage();
    await manager.goto('/app/admin/showcase');
    const card = manager.getByRole('article').filter({ hasText: title });
    await card.getByRole('button', { name: 'Отклонить' }).click();
    const reject = manager.getByRole('dialog', { name: `Отклонить страницу «${title}»?` });
    await reject.getByRole('button', { name: 'Отклонить' }).click();
    await expect(reject.getByText('Заполните это поле')).toBeVisible();
    await reject
      .getByLabel('Комментарий для команды')
      .fill('Добавьте результаты опроса студентов в раздел «Трекшн».');
    await reject.getByRole('button', { name: 'Отклонить' }).click();
    await expect(manager.getByText('Страница отклонена')).toBeVisible();

    await team.page.reload();
    await expect(team.page.getByText('Отклонена модератором')).toBeVisible();
    await expect(team.page.getByText(/Добавьте результаты опроса студентов/)).toBeVisible();

    // Members of the university only: published at once, hidden from visitors.
    await team.page.getByRole('radio', { name: /^Только университет/ }).check();
    await team.page.getByRole('button', { name: 'Опубликовать' }).click();
    await expect(team.page.getByText('Страница опубликована')).toBeVisible();
    await team.page.goto(`/showcase/e2e-bron-${salt}`);
    await expect(team.page.getByRole('heading', { name: title, level: 1 })).toBeVisible();
    const visitorContext = await browser.newContext();
    const visitor = await visitorContext.newPage();
    await visitor.goto(`/showcase/e2e-bron-${salt}`);
    await expect(
      visitor.getByRole('heading', { name: 'Страница проекта не найдена' }),
    ).toBeVisible();

    await takeDown(team.page, team.projectId);
    await team.context.close();
    await visitorContext.close();
    await managerContext.close();
  });
});

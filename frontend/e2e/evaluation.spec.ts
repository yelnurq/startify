import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';

import { createStudent, storageState } from './support/accounts';

async function csrf(page: Page): Promise<string> {
  const cookies = await page.context().cookies();
  return cookies.find((cookie) => cookie.name === 'qadam_csrf')?.value ?? '';
}

/** A fresh project of the signed-in student, accepting the IP policy. */
async function createProject(page: Page, title: string): Promise<string> {
  const tenant = (await (await page.request.get('/api/v1/tenant')).json()) as {
    ip_policy: { version: string } | null;
  };
  const response = await page.request.post('/api/v1/projects', {
    headers: { 'X-CSRF-Token': await csrf(page), 'Idempotency-Key': crypto.randomUUID() },
    data: {
      title,
      industry: 'edtech',
      ...(tenant.ip_policy ? { ip_policy_version: tenant.ip_policy.version } : {}),
    },
  });
  expect(response.status(), await response.text()).toBe(201);
  return ((await response.json()) as { id: string }).id;
}

/** Answers the first step through the API so the project can be evaluated. */
async function answerProblem(page: Page, projectId: string) {
  const current = await page.request.get(`/api/v1/projects/${projectId}/steps/problem`);
  const response = await page.request.patch(`/api/v1/projects/${projectId}/steps/problem`, {
    headers: { 'X-CSRF-Token': await csrf(page), 'If-Match': current.headers().etag ?? '' },
    data: {
      values: {
        statement:
          'Первокурсники КазУТБ тратят до 30 минут в день на поиск свободной аудитории для самоподготовки и работы в командах.',
        who: 'Студенты первого и второго курса, около 3 000 человек в главном корпусе.',
        alternatives:
          'Обходят этажи и спрашивают на кафедрах; расписание не показывает, что аудитория свободна.',
        evidence: 'Опросили 40 студентов: 31 из 40 ищут аудиторию хотя бы раз в неделю.',
      },
    },
  });
  expect(response.status(), await response.text()).toBe(200);
}

async function projectIdOf(page: Page, title: string): Promise<string> {
  const list = (await (
    await page.request.get(`/api/v1/projects?q=${encodeURIComponent(title)}`)
  ).json()) as { items: { id: string; title: string }[] };
  const project = list.items.find((p) => p.title === title);
  expect(project, `project ${title}`).toBeDefined();
  return project?.id ?? '';
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

const gauge = (page: Page) => page.getByRole('img', { name: /^Итоговый балл \d+ из 100/ });

async function total(page: Page): Promise<number> {
  const label = (await gauge(page).getAttribute('aria-label')) ?? '';
  return Number(/(\d+) из 100/.exec(label)?.[1] ?? Number.NaN);
}

test.describe('AI evaluation', () => {
  test.use({ storageState: storageState('nurlan') });

  test('the report shows the score, criteria, red flags, experiments and history', async ({
    page,
  }, testInfo) => {
    const projectId = await projectIdOf(page, 'AgroScan');
    await page.goto(`/app/projects/${projectId}/evaluation`);
    await expect(gauge(page)).toBeVisible();
    await expect(page.getByText(/к прошлой оценке/).first()).toBeVisible();
    await expect(page.getByRole('img', { name: 'Баллы по критериям, из 10' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Red flags' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Эксперименты на 7 дней' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Изменено человеком' })).toBeVisible();
    await expect(
      page.getByText('Оценка сгенерирована AI и носит рекомендательный характер'),
    ).toBeVisible();

    // The ticks of experiments are personal and survive a reload.
    const first = page.getByRole('checkbox').first();
    await first.click();
    await expect(first).toBeChecked();
    await page.reload();
    await expect(page.getByRole('checkbox').first()).toBeChecked();
    await page.getByRole('checkbox').first().click();

    if (testInfo.project.name === 'desktop') {
      const history = page.getByRole('complementary', { name: 'История оценок' });
      await expect(history.getByRole('link')).toHaveCount(2);
      await history.getByRole('link').nth(1).click();
      await expect(page).toHaveURL(/\/evaluations\//);
      await expect(page.getByText('Первая оценка проекта')).toBeVisible();

      const download = page.waitForEvent('download');
      await page.getByRole('button', { name: 'Скачать PDF' }).click();
      expect((await download).suggestedFilename()).toMatch(/\.pdf$/);
    }

    // The project card in the list shows the latest score.
    await page.goto('/app/projects');
    await expect(page.getByText(/AI-оценка: \d+ из 100/).first()).toBeAttached();
  });

  test('a team asks for an evaluation and the supervisor corrects a score', async ({
    browser,
  }, testInfo) => {
    test.skip(testInfo.project.name === 'mobile', 'the full cycle runs once, on desktop');
    test.setTimeout(120_000);
    const studentContext = await browser.newContext();
    const student = await studentContext.newPage();
    await createStudent(student, { name: 'Әсем Төлеген' });
    const projectId = await createProject(student, 'Свободная аудитория — оценка');

    await student.goto(`/app/projects/${projectId}/evaluation`);
    await expect(student.getByRole('heading', { name: 'Проект ещё не оценивался' })).toBeVisible();
    // An empty project cannot be evaluated yet.
    await student.getByRole('button', { name: 'Получить AI-оценку' }).click();
    await expect(student.getByText(/ответьте хотя бы на первый шаг/)).toBeVisible();

    await answerProblem(student, projectId);
    await student.getByRole('button', { name: 'Получить AI-оценку' }).click();
    await expect(student).toHaveURL(/\/evaluations\//);
    await expect(gauge(student)).toBeVisible({ timeout: 60_000 });
    await expect(student.getByText('Первая оценка проекта')).toBeVisible();
    await expect(student.getByRole('button', { name: 'Скорректировать балл' })).toHaveCount(0);

    const people = (await (
      await student.request.get('/api/v1/people?q=Гүлнар&role=supervisor')
    ).json()) as { items: { id: string }[] };
    const added = await student.request.post(`/api/v1/projects/${projectId}/members`, {
      headers: { 'X-CSRF-Token': await csrf(student) },
      data: { user_id: people.items[0]?.id ?? '', role: 'supervisor' },
    });
    expect(added.status(), await added.text()).toBe(201);

    const supervisorContext = await browser.newContext({
      storageState: storageState('supervisor'),
    });
    const supervisor = await supervisorContext.newPage();
    await supervisor.goto(`/app/projects/${projectId}/evaluation`);
    await expect(gauge(supervisor)).toBeVisible();
    const before = await total(supervisor);
    await supervisor.getByRole('button', { name: 'Скорректировать балл' }).click();
    const dialog = supervisor.getByRole('dialog', { name: 'Скорректировать балл' });
    await dialog.getByRole('button', { name: 'Увеличить балл' }).click();
    await dialog.getByRole('button', { name: 'Увеличить балл' }).click();
    await dialog.getByLabel('Комментарий').fill('Коротко');
    await dialog.getByRole('button', { name: 'Сохранить' }).click();
    await expect(dialog.getByText(/Минимум 10/)).toBeVisible();
    await dialog
      .getByLabel('Комментарий')
      .fill('Проблема подтверждена опросом 40 студентов — это сильнее, чем решил AI.');
    await dialog.getByRole('button', { name: 'Сохранить' }).click();
    await expect(dialog).toBeHidden();
    await expect(supervisor.getByRole('heading', { name: 'Изменено человеком' })).toBeVisible();
    await expect(supervisor.getByText(/Изменено экспертом · AI:/)).toBeVisible();
    await expect(supervisor.getByText(/AI поставил \d+/)).toBeVisible();
    await expect.poll(() => total(supervisor)).toBeGreaterThan(before);

    await student.reload();
    await expect(student.getByText(/Проблема подтверждена опросом 40 студентов/)).toBeVisible();
    await studentContext.close();
    await supervisorContext.close();
  });
});

test.describe('evaluation rubric', () => {
  test.use({ storageState: storageState('admin') });
  test.describe.configure({ mode: 'serial' });

  test('an administrator changes the weights and restores the defaults', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name === 'mobile', 'the rubric is shared: edited once, on desktop');
    await page.goto('/app/admin/rubric');
    await expect(page.getByRole('heading', { name: 'Рубрика оценки', level: 1 })).toBeVisible();
    const problem = page.getByLabel('Проблема и её значимость', { exact: true });
    const team = page.getByLabel('Команда', { exact: true });
    const save = page.getByRole('button', { name: 'Сохранить' });

    await problem.fill('20');
    await expect(page.getByText('Сумма: 105 из 100')).toBeVisible();
    await expect(page.getByText(/Сумма весов должна быть 100/)).toBeVisible();
    await expect(save).toBeDisabled();
    await team.fill('5');
    await expect(page.getByText('Сумма: 100 из 100')).toBeVisible();
    await save.click();
    await expect(page.getByText('Рубрика сохранена')).toBeVisible();
    await expect(page.getByText('Веса изменены вузом', { exact: true })).toBeVisible();

    await page.getByRole('button', { name: 'Вернуть веса по умолчанию' }).click();
    await expect(problem).toHaveValue('15');
    await save.click();
    await expect(page.getByText('Рубрика сохранена').last()).toBeVisible();
    await expect(page.getByText('Веса по умолчанию', { exact: true })).toBeVisible();
  });
});

test.describe('evaluation accessibility', () => {
  test('the report and the rubric are accessible and fit a phone', async ({ browser }) => {
    const nurlanContext = await browser.newContext({ storageState: storageState('nurlan') });
    const nurlan = await nurlanContext.newPage();
    const adminContext = await browser.newContext({ storageState: storageState('admin') });
    const admin = await adminContext.newPage();
    const projectId = await projectIdOf(nurlan, 'AgroScan');
    for (const page of [nurlan, admin]) {
      await page.emulateMedia({ reducedMotion: 'reduce' });
    }
    await nurlan.goto(`/app/projects/${projectId}/evaluation`);
    await expect(gauge(nurlan)).toBeVisible();
    await expectNoSeriousViolations(nurlan);
    await expectNoHorizontalScroll(nurlan);

    await admin.goto('/app/admin/rubric');
    await expect(admin.getByRole('heading', { name: 'Рубрика оценки', level: 1 })).toBeVisible();
    await expectNoSeriousViolations(admin);
    await expectNoHorizontalScroll(admin);
    await nurlanContext.close();
    await adminContext.close();
  });
});

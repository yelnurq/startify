import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Browser, type Page } from '@playwright/test';

import { createStudent, storageState } from './support/accounts';

interface TemplateField {
  key: string;
  type: string;
  required: boolean;
  min_length?: number;
  max_length?: number;
  min_items?: number;
  options?: { value: string }[];
  columns?: TemplateField[];
}
interface Template {
  stages: { steps: { key: string; fields: TemplateField[] }[] }[];
}

async function csrf(page: Page): Promise<string> {
  const cookies = await page.context().cookies();
  return cookies.find((cookie) => cookie.name === 'qadam_csrf')?.value ?? '';
}

/** Creates a project through the API, accepting the university's IP policy. */
async function createProject(page: Page, title: string): Promise<string> {
  const tenant = (await (await page.request.get('/api/v1/tenant')).json()) as {
    ip_policy: { version: string } | null;
  };
  const response = await page.request.post('/api/v1/projects', {
    headers: { 'X-CSRF-Token': await csrf(page), 'Idempotency-Key': crypto.randomUUID() },
    data: {
      title,
      industry: 'agritech',
      ...(tenant.ip_policy ? { ip_policy_version: tenant.ip_policy.version } : {}),
    },
  });
  expect(response.status(), await response.text()).toBe(201);
  return ((await response.json()) as { id: string }).id;
}

async function myId(page: Page): Promise<string> {
  const session = (await (await page.request.get('/api/v1/auth/session')).json()) as {
    user: { id: string };
  };
  return session.user.id;
}

async function addMember(page: Page, projectId: string, userId: string, role = 'member') {
  const response = await page.request.post(`/api/v1/projects/${projectId}/members`, {
    headers: { 'X-CSRF-Token': await csrf(page) },
    data: { user_id: userId, role },
  });
  expect(response.status(), await response.text()).toBe(201);
}

/** A value that meets the minimum of a field, for filling steps quickly. */
function minimal(field: TemplateField, seed: string): unknown {
  const text = (min = 1, max = 4000) => {
    const base = `${seed}: ответ команды, проверенный на интервью с пользователями. `;
    let out = base;
    while (out.length < min + 5) out += base;
    return out.slice(0, Math.min(max, Math.max(min + 5, base.length))).trim();
  };
  switch (field.type) {
    case 'number':
      return 150000;
    case 'select':
      return field.options?.[0]?.value ?? '';
    case 'url':
      return 'https://example.com/pitch';
    case 'list':
      return Array.from(
        { length: Math.max(field.min_items ?? 1, 1) },
        (_, i) => text(1, field.max_length ?? 300).slice(0, 40) + ` ${String(i + 1)}`,
      );
    case 'table':
      return Array.from({ length: Math.max(field.min_items ?? 1, 1) }, (_, i) =>
        Object.fromEntries(
          (field.columns ?? []).map((c) => [c.key, minimal(c, `${seed} ${String(i + 1)}`)]),
        ),
      );
    default:
      return text(field.min_length ?? 1, field.max_length ?? (field.type === 'text' ? 200 : 4000));
  }
}

/** Saves the minimum of a step and completes it, as a hurried team would. */
async function completeStep(
  page: Page,
  projectId: string,
  step: { key: string; fields: TemplateField[] },
) {
  const current = await page.request.get(`/api/v1/projects/${projectId}/steps/${step.key}`);
  expect(current.ok(), await current.text()).toBe(true);
  const values = Object.fromEntries(
    step.fields.filter((f) => f.required).map((f) => [f.key, minimal(f, step.key)]),
  );
  const saved = await page.request.patch(`/api/v1/projects/${projectId}/steps/${step.key}`, {
    headers: { 'X-CSRF-Token': await csrf(page), 'If-Match': current.headers().etag ?? '' },
    data: { values },
  });
  expect(saved.status(), await saved.text()).toBe(200);
  const completed = await page.request.post(
    `/api/v1/projects/${projectId}/steps/${step.key}/complete`,
    { headers: { 'X-CSRF-Token': await csrf(page) } },
  );
  expect(completed.status(), await completed.text()).toBe(200);
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

const saved = (page: Page) =>
  page
    .getByRole('status')
    .filter({ hasText: /Сохранено/ })
    .first();

test.describe('incubator', () => {
  test('a student goes from the idea to the pitch', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name === 'mobile', 'the full path runs once, on desktop');
    test.setTimeout(180_000);
    await createStudent(page, { name: 'Ерлан Сапаров' });

    await page.goto('/app/projects');
    await expect(page.getByRole('heading', { name: 'Проекты', level: 1 })).toBeVisible();
    await page.getByRole('button', { name: 'Создать первый проект' }).click();
    const dialog = page.getByRole('dialog', { name: 'Новый проект' });
    await dialog.getByLabel('Название').fill('ЗерноСкан');
    await dialog.getByLabel('Слоган').fill('Болезни пшеницы по фото листа');
    await dialog.getByRole('combobox', { name: 'Отрасль' }).click();
    await page.getByRole('option', { name: 'Агротех' }).click();
    await dialog.getByRole('button', { name: 'Создать проект' }).click();
    await expect(dialog.getByText('Чтобы создать проект, примите политику')).toBeVisible();
    await dialog.getByRole('checkbox', { name: /согласен/ }).check();
    await dialog.getByRole('button', { name: 'Создать проект' }).click();

    await expect(page).toHaveURL(/\/app\/projects\/[^/]+\/steps\/problem$/);
    const projectId = /projects\/([^/]+)\//.exec(page.url())?.[1] ?? '';
    await expect(
      page.getByRole('heading', { name: 'Проблема и для кого она', level: 1 }),
    ).toBeVisible();

    // Autosave of the first answers.
    await page
      .getByRole('textbox', { name: 'Формулировка проблемы', exact: true })
      .fill(
        'Фермеры Костанайской области замечают ржавчину пшеницы слишком поздно: обработка уже не помогает, и они теряют до пятой части урожая.',
      );
    await page
      .getByRole('textbox', { name: 'Кто сталкивается с проблемой', exact: true })
      .fill('Фермерские хозяйства Северного Казахстана с полями от 200 до 2000 гектаров.');
    await page
      .getByRole('textbox', { name: 'Как её решают сейчас', exact: true })
      .fill('Агроном объезжает поля раз в неделю, а болезнь распространяется за три-четыре дня.');
    await expect(saved(page)).toBeVisible({ timeout: 15_000 });

    // The mentor proposes a wording; it goes into the field.
    const mentor = page.getByRole('complementary', { name: 'AI-ментор' });
    await mentor.getByLabel('Спросите ментора…').fill('Помоги сформулировать цену проблемы');
    await mentor.getByLabel('Спросите ментора…').press('Enter');
    const apply = mentor.getByRole('button', { name: 'Применить в поле' }).last();
    await expect(apply).toBeVisible({ timeout: 30_000 });
    await apply.click();
    await expect(page.getByText(/Текст добавлен в поле/)).toBeVisible();
    await expect(mentor.getByRole('button', { name: 'Применено' }).last()).toBeVisible();

    // The AI review and completion.
    await page.getByRole('button', { name: 'Проверить с AI' }).click();
    await expect(page.getByText(/Шаг выглядит готовым|Есть что усилить/)).toBeVisible({
      timeout: 30_000,
    });
    await expect(page.getByText(/Рекомендация AI, а не оценка/)).toBeVisible();
    await page.getByRole('button', { name: 'Завершить шаг' }).click();
    await expect(page.getByText('Шаг завершён').first()).toBeVisible();

    // The rest of the steps, quickly through the API.
    const template = (await (
      await page.request.get('/api/v1/incubator/template')
    ).json()) as Template;
    for (const step of template.stages.flatMap((stage) => stage.steps).slice(1)) {
      await completeStep(page, projectId, step);
    }

    // Step 14: the pitch and a one-pager built from the answers.
    await page.goto(`/app/projects/${projectId}/steps/pitch`);
    await expect(page.getByRole('heading', { name: 'Питч и материалы', level: 1 })).toBeVisible();
    await page.getByRole('button', { name: 'Собрать PDF' }).first().click();
    const ready = page.getByRole('link', { name: 'Скачать' }).first();
    await expect(ready).toBeVisible({ timeout: 90_000 });
    const download = page.waitForEvent('download');
    await ready.click();
    expect((await download).suggestedFilename()).toMatch(/\.pdf$/);

    await page.goto(`/app/projects/${projectId}`);
    await expect(page.getByText('Все 14 шагов пройдены')).toBeVisible();
    await expect(
      page.getByRole('progressbar', { name: 'Прогресс проекта' }).first(),
    ).toHaveAttribute('aria-valuenow', '100');
  });

  test('simultaneous edits of one field end in a clear choice', async ({ browser }, testInfo) => {
    test.skip(testInfo.project.name === 'mobile', 'two members on desktop');
    const { owner, member, projectId } = await twoMembers(browser, 'Конфликт');

    await member.goto(`/app/projects/${projectId}/steps/problem`);
    await expect(
      member.getByRole('textbox', { name: 'Формулировка проблемы', exact: true }),
    ).toBeVisible();
    await owner.goto(`/app/projects/${projectId}/steps/problem`);
    await owner
      .getByRole('textbox', { name: 'Формулировка проблемы', exact: true })
      .fill('Версия владельца: студенты не находят свободные аудитории.');
    await expect(saved(owner)).toBeVisible({ timeout: 15_000 });

    await member
      .getByRole('textbox', { name: 'Формулировка проблемы', exact: true })
      .fill('Версия участника: аудитории пустуют, но их не видно в расписании.');
    const conflict = member.getByRole('dialog', { name: 'Шаг изменили одновременно с вами' });
    await expect(conflict).toBeVisible({ timeout: 15_000 });
    await expect(conflict.getByText('Формулировка проблемы')).toBeVisible();
    await conflict.getByRole('radio', { name: 'Моя' }).click();
    await conflict.getByRole('button', { name: 'Сохранить выбор' }).click();
    await expect(saved(member)).toBeVisible({ timeout: 15_000 });

    await owner.reload();
    await expect(
      owner.getByRole('textbox', { name: 'Формулировка проблемы', exact: true }),
    ).toHaveValue(/Версия участника/);
  });

  test('history shows who changed a field and restores an earlier value', async ({
    browser,
  }, testInfo) => {
    test.skip(testInfo.project.name === 'mobile', 'two members on desktop');
    const { owner, member, projectId } = await twoMembers(browser, 'История');
    for (const [page, text] of [
      [owner, 'Первая формулировка владельца проекта.'],
      [member, 'Вторая формулировка участника команды.'],
    ] as const) {
      const current = await page.request.get(`/api/v1/projects/${projectId}/steps/problem`);
      const response = await page.request.patch(`/api/v1/projects/${projectId}/steps/problem`, {
        headers: { 'X-CSRF-Token': await csrf(page), 'If-Match': current.headers().etag ?? '' },
        data: { values: { statement: text } },
      });
      expect(response.status(), await response.text()).toBe(200);
    }

    await owner.goto(`/app/projects/${projectId}/steps/problem`);
    await owner.getByRole('button', { name: 'История поля «Формулировка проблемы»' }).click();
    const history = owner.getByRole('dialog', { name: 'История изменений' });
    await expect(history.getByRole('listitem')).toHaveCount(2);
    await history.getByRole('button', { name: 'Восстановить' }).click();
    await expect(owner.getByText('Значение восстановлено')).toBeVisible();
    await expect(
      owner.getByRole('textbox', { name: 'Формулировка проблемы', exact: true }),
    ).toHaveValue('Первая формулировка владельца проекта.');
  });

  test('the supervisor comments with a mention and approves a step', async ({
    browser,
  }, testInfo) => {
    test.skip(testInfo.project.name === 'mobile', 'the supervisor works on desktop');
    const studentContext = await browser.newContext();
    const student = await studentContext.newPage();
    await createStudent(student, { name: 'Айгүл Нұрланова' });
    const projectId = await createProject(student, 'Кампус без очередей');
    const people = (await (
      await student.request.get('/api/v1/people?q=Гүлнар&role=supervisor')
    ).json()) as { items: { id: string }[] };
    await addMember(student, projectId, people.items[0]?.id ?? '', 'supervisor');

    const supervisorContext = await browser.newContext({
      storageState: storageState('supervisor'),
    });
    const supervisor = await supervisorContext.newPage();
    await supervisor.goto(`/app/projects/${projectId}/steps/problem`);
    await expect(supervisor.getByText(/Вы смотрите проект как руководитель/)).toBeVisible();
    await expect(
      supervisor.getByRole('textbox', { name: 'Формулировка проблемы', exact: true }),
    ).toBeDisabled();

    await supervisor
      .getByRole('button', { name: 'Комментировать «Формулировка проблемы»' })
      .click();
    const sheet = supervisor.getByRole('dialog', { name: 'Комментарии' });
    const box = sheet.getByLabel('Новый комментарий');
    await box.fill('Уточните, сколько студентов это касается, @Айг');
    await sheet.getByRole('button', { name: 'Айгүл Нұрланова' }).click();
    await expect(box).toHaveValue(/@Айгүл Нұрланова /);
    await sheet.getByRole('button', { name: 'Отправить' }).click();
    await expect(sheet.getByText(/сколько студентов это касается/)).toBeVisible();
    await supervisor.keyboard.press('Escape');

    await supervisor.getByRole('button', { name: 'Одобрить' }).click();
    const approve = supervisor.getByRole('dialog', { name: 'Одобрить шаг' });
    await approve
      .getByLabel('Комментарий для команды')
      .fill('Проблема описана честно, двигайтесь дальше.');
    await approve.getByRole('button', { name: 'Одобрить' }).click();
    await expect(supervisor.getByText('Шаг одобрен').first()).toBeVisible();

    await student.goto(`/app/projects/${projectId}/steps/problem`);
    await expect(student.getByText(/Проблема описана честно/)).toBeVisible();
    await expect(
      student.getByRole('button', { name: /«Формулировка проблемы»: 1 открытый комментарий/ }),
    ).toBeVisible();
    await studentContext.close();
    await supervisorContext.close();
  });

  test('screens are accessible and fit a phone', async ({ page }) => {
    await createStudent(page, { name: 'Мадияр Ахметов' });
    const projectId = await createProject(page, 'Доступный проект');
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await page.goto('/app/projects');
    await expect(page.getByRole('link', { name: /Доступный проект/ })).toBeVisible();
    await expectNoSeriousViolations(page);
    await expectNoHorizontalScroll(page);

    await page.goto(`/app/projects/${projectId}/steps/audience`);
    await expect(
      page.getByRole('heading', { name: 'Целевая аудитория и персоны', level: 1 }),
    ).toBeVisible();
    await expectNoSeriousViolations(page);
    await expectNoHorizontalScroll(page);

    await page.goto(`/app/projects/${projectId}`);
    await expect(page.getByRole('heading', { name: 'Доступный проект', level: 1 })).toBeVisible();
    await expectNoSeriousViolations(page);
    await expectNoHorizontalScroll(page);
  });
});

/** An owner and a member of a fresh project, each in their own browser context. */
async function twoMembers(browser: Browser, title: string) {
  const ownerContext = await browser.newContext();
  const owner = await ownerContext.newPage();
  await createStudent(owner, { name: 'Владелец Проекта' });
  const memberContext = await browser.newContext();
  const member = await memberContext.newPage();
  await createStudent(member, { name: 'Участник Команды' });
  const projectId = await createProject(owner, title);
  await addMember(owner, projectId, await myId(member));
  return { owner, member, projectId };
}

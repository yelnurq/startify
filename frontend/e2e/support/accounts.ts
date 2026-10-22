import path from 'node:path';

import { expect, type Page } from '@playwright/test';

// Seeded demo accounts (backend seed); all share one password.
export const DEMO_PASSWORD = 'Qadam-Demo-2026!';
export const USERS = {
  admin: 'admin@qadam.test',
  manager: 'innovation@qadam.test',
  student: 'aigerim@qadam.test',
  nurlan: 'nurlan@qadam.test',
  dana: 'dana@qadam.test',
  timur: 'timur@qadam.test',
  madina: 'madina@qadam.test',
  rector: 'rector@qadam.test',
  supervisor: 'gulnar@qadam.test',
} as const;

export type SeededUser = keyof typeof USERS;

/** Saved cookies of a seeded account, written by `auth.setup.ts` once per run. */
export function storageState(user: SeededUser): string {
  return path.join(import.meta.dirname, '..', '..', 'test-results', '.auth', `${user}.json`);
}

/** A fresh address, unique per test, project and run. */
export function uniqueEmail(prefix: string): string {
  const salt = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 7)}`;
  return `e2e.${prefix}.${salt}@qadam.test`;
}

/** Fills in and submits the sign-in form. */
export async function submitSignIn(page: Page, email: string, password = DEMO_PASSWORD) {
  await page.goto('/login');
  await page.getByLabel('Электронная почта').fill(email);
  await page.getByRole('textbox', { name: 'Пароль', exact: true }).fill(password);
  await page.getByRole('button', { name: 'Войти', exact: true }).click();
}

/** Signs in and waits until the app has taken the member past the sign-in page. */
export async function signIn(page: Page, email: string, password = DEMO_PASSWORD) {
  await submitSignIn(page, email, password);
  await page.waitForURL((url) => !url.pathname.startsWith('/login'));
}

async function csrfToken(page: Page): Promise<string> {
  const cookies = await page.context().cookies();
  return cookies.find((cookie) => cookie.name === 'qadam_csrf')?.value ?? '';
}

/**
 * Registers a student through the API (sharing cookies with the page) and,
 * unless told otherwise, completes onboarding — for tests about later screens.
 */
export async function createStudent(
  page: Page,
  { name = 'Тест Студентов', onboard = true }: { name?: string; onboard?: boolean } = {},
): Promise<{ email: string; password: string; name: string }> {
  const email = uniqueEmail('student');
  const password = `Жасыл алма ${String(Date.now())}`;
  const tenant = await page.request.get('/api/v1/tenant');
  expect(tenant.ok()).toBe(true);
  const { consent } = (await tenant.json()) as { consent: { version: string } };
  const registered = await page.request.post('/api/v1/auth/register', {
    headers: { 'Idempotency-Key': crypto.randomUUID() },
    data: { email, password, full_name: name, locale: 'ru', consent_version: consent.version },
  });
  expect(registered.status(), await registered.text()).toBeLessThan(300);
  if (onboard) {
    const faculties = await page.request.get('/api/v1/faculties');
    const { items } = (await faculties.json()) as { items: { id: string }[] };
    const onboarded = await page.request.put('/api/v1/me/onboarding', {
      headers: { 'X-CSRF-Token': await csrfToken(page) },
      data: {
        locale: 'ru',
        faculty_id: items[0]?.id ?? null,
        study_level: 'bachelor',
        course: 2,
        interests: ['EdTech'],
        skills: ['Figma'],
        has_idea: false,
      },
    });
    expect(onboarded.status(), await onboarded.text()).toBeLessThan(300);
  }
  return { email, password, name };
}

/** Opens the user menu, which lives in the sidebar sheet below the `lg` breakpoint. */
export async function openUserMenu(page: Page) {
  if ((page.viewportSize()?.width ?? 1440) < 1024) {
    await page.getByRole('button', { name: 'Открыть меню' }).click();
  }
  await page.getByRole('button', { name: 'Меню пользователя' }).click();
}

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from '@tanstack/react-router';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';

import type { Session, Tenant } from './api';
import { RegisterForm } from './register-page';

const server = setupServer();

beforeAll(() => {
  server.listen({ onUnhandledFrame: 'error' });
});
afterEach(() => {
  server.resetHandlers();
});
afterAll(() => {
  server.close();
});

const tenant: Tenant = {
  id: '0190a000-0000-7000-8000-000000000002',
  slug: 'kazutb',
  name: { ru: 'Казахский университет технологии и бизнеса' },
  short_name: { ru: 'КазУТБ' },
  default_locale: 'kk',
  primary_color: '#00A3C4',
  logo_url: null,
  cover_url: null,
  self_signup: true,
  email_domains: ['kazutb.edu.kz'],
  sso_providers: [],
  directory_login: { enabled: false, label: null, username_hint: null },
  consent: { version: '2026-09', text: { ru: 'Согласие на обработку персональных данных.' } },
  ip_policy: null,
};

async function renderForm() {
  const root = createRootRoute({ component: () => <Outlet /> });
  const routeTree = root.addChildren([
    createRoute({
      getParentRoute: () => root,
      path: '/',
      component: () => <RegisterForm tenant={tenant} />,
    }),
    createRoute({
      getParentRoute: () => root,
      path: '/onboarding',
      component: () => <p>onboarding</p>,
    }),
  ]);
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  });
  const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  await screen.findByRole('button', { name: 'Создать аккаунт' });
  return userEvent.setup();
}

async function fillValid(user: ReturnType<typeof userEvent.setup>, email: string) {
  await user.type(screen.getByLabelText(/Имя и фамилия/), 'Айгерім Сапарова');
  await user.type(screen.getByLabelText(/Электронная почта/), email);
  await user.type(screen.getByLabelText(/^Пароль/), 'жасыл алма 2026 Qadam');
  await user.click(screen.getByRole('checkbox'));
}

describe('RegisterForm', () => {
  it('explains every invalid field in the interface language', async () => {
    const user = await renderForm();
    await user.click(screen.getByRole('button', { name: 'Создать аккаунт' }));

    expect(await screen.findByText('Минимум 2 символа')).toBeInTheDocument();
    expect(screen.getByText('Введите корректный адрес почты')).toBeInTheDocument();
    expect(screen.getByText('Минимум 10 символов')).toBeInTheDocument();
    expect(
      screen.getByText('Нужно согласие на обработку персональных данных.'),
    ).toBeInTheDocument();
  });

  it('accepts only the university e-mail domains', async () => {
    const user = await renderForm();
    await fillValid(user, 'aigerim@gmail.com');
    await user.click(screen.getByRole('button', { name: 'Создать аккаунт' }));

    expect(
      await screen.findByText('Для регистрации используйте университетскую почту.'),
    ).toBeInTheDocument();
  });

  it('sends the accepted consent version and shows a taken e-mail next to the field', async () => {
    let body: Record<string, unknown> | undefined;
    let idempotencyKey: string | null = null;
    server.use(
      http.post('*/api/v1/auth/register', async ({ request }) => {
        body = (await request.json()) as Record<string, unknown>;
        idempotencyKey = request.headers.get('Idempotency-Key');
        return HttpResponse.json(
          { type: 'about:blank', title: 'Conflict', status: 409, code: 'auth.email_taken' },
          { status: 409, headers: { 'Content-Type': 'application/problem+json' } },
        );
      }),
    );
    const user = await renderForm();
    await fillValid(user, 'aigerim.saparova@kazutb.edu.kz');
    await user.click(screen.getByRole('button', { name: 'Создать аккаунт' }));

    expect(
      await screen.findByText('Этот адрес уже зарегистрирован. Войдите или восстановите пароль.'),
    ).toBeInTheDocument();
    expect(body).toMatchObject({
      email: 'aigerim.saparova@kazutb.edu.kz',
      full_name: 'Айгерім Сапарова',
      consent_version: '2026-09',
      locale: 'ru',
    });
    expect(idempotencyKey).toBeTruthy();
  });

  it('starts no session for a new account: the link from the letter signs in', async () => {
    let resent: unknown;
    server.use(
      http.post('*/api/v1/auth/register', () =>
        HttpResponse.json({ verification_sent: true }, { status: 202 }),
      ),
      http.post('*/api/v1/auth/email/resend', async ({ request }) => {
        resent = await request.json();
        return new HttpResponse(null, { status: 202 });
      }),
    );
    const user = await renderForm();
    await fillValid(user, 'aigerim.saparova@kazutb.edu.kz');
    await user.click(screen.getByRole('button', { name: 'Создать аккаунт' }));

    expect(await screen.findByText('Проверьте почту')).toBeInTheDocument();
    expect(screen.getByText(/aigerim\.saparova@kazutb\.edu\.kz/)).toBeInTheDocument();
    expect(screen.queryByText('onboarding')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Отправить письмо ещё раз' }));
    expect(await screen.findByRole('button', { name: 'Письмо отправлено' })).toBeDisabled();
    expect(resent).toEqual({ email: 'aigerim.saparova@kazutb.edu.kz' });
  });

  it('signs in an account with a verified e-mail that joins with its password', async () => {
    const session: Session = {
      user: {
        id: '0190a000-0000-7000-8000-000000000010',
        email: 'aigerim.saparova@kazutb.edu.kz',
        email_verified: true,
        full_name: 'Айгерім Сапарова',
        locale: 'ru',
        avatar_url: null,
        is_super_admin: false,
        created_at: '2026-10-01T09:00:00Z',
      },
      membership: {
        tenant_id: tenant.id,
        roles: ['student'],
        status: 'active',
        faculty: null,
        department: null,
        group: null,
        study_level: null,
        course: null,
        onboarded: false,
      },
      tenant: {
        id: tenant.id,
        slug: tenant.slug,
        name: tenant.name,
        short_name: tenant.short_name,
      },
    };
    server.use(
      http.post('*/api/v1/auth/register', () => HttpResponse.json(session, { status: 201 })),
    );
    const user = await renderForm();
    await fillValid(user, 'aigerim.saparova@kazutb.edu.kz');
    await user.click(screen.getByRole('button', { name: 'Создать аккаунт' }));

    expect(await screen.findByText('onboarding')).toBeInTheDocument();
  });
});

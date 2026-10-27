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

import { TooltipProvider } from '@/design-system';

import type { Tenant } from './api';
import { LoginPage } from './login-page';

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
  email_domains: [],
  sso_providers: [],
  directory_login: { enabled: false, label: null, username_hint: null },
  consent: { version: '2026-09', text: { ru: 'Согласие.' } },
  ip_policy: null,
};

async function renderPage() {
  const root = createRootRoute({ component: () => <Outlet /> });
  const stub = (path: string) =>
    createRoute({ getParentRoute: () => root, path, component: () => <p>{path}</p> });
  const routeTree = root.addChildren([
    createRoute({ getParentRoute: () => root, path: '/', component: () => <LoginPage /> }),
    stub('/forgot-password'),
    stub('/register'),
    stub('/app'),
  ]);
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  });
  const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <TooltipProvider>
        <RouterProvider router={router} />
      </TooltipProvider>
    </QueryClientProvider>,
  );
  await screen.findByRole('button', { name: 'Войти' });
  return userEvent.setup();
}

describe('LoginPage', () => {
  it('offers to send the verification letter again when the address is not verified', async () => {
    let resent: unknown;
    server.use(
      http.get('*/api/v1/tenant', () => HttpResponse.json(tenant)),
      http.post('*/api/v1/auth/login', () =>
        HttpResponse.json(
          { type: 'about:blank', title: 'Forbidden', status: 403, code: 'auth.email_unverified' },
          { status: 403, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
      http.post('*/api/v1/auth/email/resend', async ({ request }) => {
        resent = await request.json();
        return new HttpResponse(null, { status: 202 });
      }),
    );
    const user = await renderPage();
    await user.type(screen.getByLabelText('Электронная почта'), 'aru@kazutb.edu.kz');
    await user.type(screen.getByLabelText('Пароль'), 'Жасыл алма 2026');
    await user.click(screen.getByRole('button', { name: 'Войти' }));

    expect(await screen.findByText('Подтвердите почту')).toBeInTheDocument();
    expect(screen.getByText(/aru@kazutb\.edu\.kz ещё не подтверждён/)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Отправить письмо ещё раз' }));
    expect(await screen.findByRole('button', { name: 'Письмо отправлено' })).toBeDisabled();
    expect(resent).toEqual({ email: 'aru@kazutb.edu.kz' });
  });

  it('offers the university directory first when the university turned it on', async () => {
    let sent: unknown;
    server.use(
      http.get('*/api/v1/tenant', () =>
        HttpResponse.json({
          ...tenant,
          directory_login: {
            enabled: true,
            label: { ru: 'Учётная запись КазУТБ' },
            username_hint: { ru: 'Логин как в Outlook: ivanov.a' },
          },
        }),
      ),
      http.post('*/api/v1/auth/directory/login', async ({ request }) => {
        sent = await request.json();
        return HttpResponse.json(
          {
            type: 'about:blank',
            title: 'Unauthorized',
            status: 401,
            code: 'auth.invalid_credentials',
          },
          { status: 401, headers: { 'Content-Type': 'application/problem+json' } },
        );
      }),
    );
    const user = await renderPage();
    expect(await screen.findByRole('radio', { name: 'Учётная запись КазУТБ' })).toBeChecked();
    expect(screen.getByText('Логин как в Outlook: ivanov.a')).toBeInTheDocument();
    await user.type(screen.getByLabelText('Логин'), 'ivanov.a');
    await user.type(screen.getByLabelText('Пароль'), 'Kazutb-Directory-1');
    await user.click(screen.getByRole('button', { name: 'Войти' }));

    expect(await screen.findByText('Неверный логин или пароль.')).toBeInTheDocument();
    expect(sent).toEqual({ username: 'ivanov.a', password: 'Kazutb-Directory-1' });

    // E-mail sign-in stays one click away, with the code and SSO.
    await user.click(screen.getByRole('radio', { name: 'Почта' }));
    expect(screen.getByLabelText('Электронная почта')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Войти по коду из письма' })).toBeInTheDocument();
    expect(screen.queryByText('Неверный логин или пароль.')).not.toBeInTheDocument();
  });
});

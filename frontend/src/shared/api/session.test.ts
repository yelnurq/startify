import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';

import { api, ApiError, setUnauthenticatedHandler, unwrap } from './client';

const server = setupServer();

beforeAll(() => {
  server.listen({ onUnhandledFrame: 'error' });
});
beforeEach(() => {
  // A refresh in a previous test must not count as "another tab just refreshed".
  localStorage.clear();
});
afterEach(() => {
  server.resetHandlers();
  setCsrfCookie(undefined);
  setUnauthenticatedHandler(() => undefined);
});
afterAll(() => {
  server.close();
});

function setCsrfCookie(value: string | undefined) {
  document.cookie =
    value === undefined
      ? 'qadam_csrf=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/'
      : `qadam_csrf=${value}; path=/`;
}

function problem(status: number, code: string) {
  return HttpResponse.json(
    { type: 'about:blank', title: code, status, code },
    { status, headers: { 'Content-Type': 'application/problem+json' } },
  );
}

const session = {
  user: {
    id: '0190a000-0000-7000-8000-000000000001',
    email: 'student@kazutb.edu.kz',
    email_verified: true,
    full_name: 'Айгерім Сапарова',
    locale: 'ru',
    avatar_url: null,
    is_super_admin: false,
    created_at: '2026-10-01T10:00:00Z',
  },
  membership: {
    tenant_id: '0190a000-0000-7000-8000-000000000002',
    roles: ['student'],
    status: 'active',
    faculty: null,
    department: null,
    group: null,
    study_level: null,
    course: null,
    onboarded: true,
  },
  tenant: {
    id: '0190a000-0000-7000-8000-000000000002',
    slug: 'kazutb',
    name: { ru: 'КазУТБ' },
    short_name: { ru: 'КазУТБ' },
  },
};

describe('CSRF protection', () => {
  it('sends the token from the cookie with state-changing requests only', async () => {
    setCsrfCookie('csrf-token-1');
    const seen: Record<string, string | null> = {};
    server.use(
      http.get('*/api/v1/auth/session', ({ request }) => {
        seen.get = request.headers.get('X-CSRF-Token');
        return HttpResponse.json(session);
      }),
      http.post('*/api/v1/auth/logout', ({ request }) => {
        seen.post = request.headers.get('X-CSRF-Token');
        return new HttpResponse(null, { status: 204 });
      }),
    );
    await unwrap(api.GET('/auth/session'));
    await unwrap(api.POST('/auth/logout'));
    expect(seen.get).toBeNull();
    expect(seen.post).toBe('csrf-token-1');
  });
});

describe('session refresh', () => {
  it('refreshes once for concurrent 401s and retries every request', async () => {
    setCsrfCookie('csrf-token-1');
    let refreshed = false;
    let refreshCalls = 0;
    let refreshCsrf: string | null = null;
    server.use(
      http.get('*/api/v1/auth/session', () =>
        refreshed ? HttpResponse.json(session) : problem(401, 'auth.unauthenticated'),
      ),
      http.post('*/api/v1/auth/refresh', async ({ request }) => {
        refreshCalls += 1;
        refreshCsrf = request.headers.get('X-CSRF-Token');
        await new Promise((resolve) => setTimeout(resolve, 20));
        refreshed = true;
        return new HttpResponse(null, { status: 204 });
      }),
    );

    const results = await Promise.all([
      unwrap(api.GET('/auth/session')),
      unwrap(api.GET('/auth/session')),
      unwrap(api.GET('/auth/session')),
    ]);

    expect(results.map((r) => r.user.email)).toEqual([
      'student@kazutb.edu.kz',
      'student@kazutb.edu.kz',
      'student@kazutb.edu.kz',
    ]);
    expect(refreshCalls).toBe(1);
    expect(refreshCsrf).toBe('csrf-token-1');
  });

  it('reports a lost session when the refresh is rejected', async () => {
    setCsrfCookie('csrf-token-1');
    const lost = vi.fn();
    setUnauthenticatedHandler(lost);
    let sessionCalls = 0;
    server.use(
      http.get('*/api/v1/auth/session', () => {
        sessionCalls += 1;
        return problem(401, 'auth.unauthenticated');
      }),
      http.post('*/api/v1/auth/refresh', () => problem(401, 'auth.session_revoked')),
    );

    const error = await unwrap(api.GET('/auth/session')).catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).status).toBe(401);
    expect(sessionCalls).toBe(1);
    expect(lost).toHaveBeenCalledTimes(1);
  });

  it('never refreshes for the sign-in endpoints themselves', async () => {
    setCsrfCookie('csrf-token-1');
    const lost = vi.fn();
    setUnauthenticatedHandler(lost);
    const refresh = vi.fn(() => new HttpResponse(null, { status: 204 }));
    server.use(
      http.post('*/api/v1/auth/login', () => problem(401, 'auth.invalid_credentials')),
      http.post('*/api/v1/auth/refresh', refresh),
    );

    const error = await unwrap(
      api.POST('/auth/login', { body: { email: 'student@kazutb.edu.kz', password: 'wrong' } }),
    ).catch((e: unknown) => e);

    expect((error as ApiError).code).toBe('auth.invalid_credentials');
    expect(refresh).not.toHaveBeenCalled();
    expect(lost).not.toHaveBeenCalled();
  });

  it('does not try to refresh when the browser never had a session', async () => {
    const lost = vi.fn();
    setUnauthenticatedHandler(lost);
    const refresh = vi.fn(() => new HttpResponse(null, { status: 204 }));
    server.use(
      http.get('*/api/v1/me', () => problem(401, 'auth.unauthenticated')),
      http.post('*/api/v1/auth/refresh', refresh),
    );

    await expect(unwrap(api.GET('/me'))).rejects.toBeInstanceOf(ApiError);
    expect(refresh).not.toHaveBeenCalled();
    expect(lost).toHaveBeenCalledTimes(1);
  });
});

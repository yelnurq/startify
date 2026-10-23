import { QueryClient } from '@tanstack/react-query';
import { isRedirect, type ParsedLocation } from '@tanstack/react-router';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';

import { sessionQuery, type Session } from './api';
import { redirectIfSignedIn, requireOnboarded, requireSession } from './guards';

const server = setupServer();
beforeAll(() => {
  server.listen({ onUnhandledFrame: 'error' });
});
afterEach(() => {
  server.resetHandlers();
  document.cookie = 'qadam_csrf=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/';
});
afterAll(() => {
  server.close();
});

function session(onboarded: boolean): Session {
  return {
    user: {
      id: 'u1',
      email: 'student@kazutb.edu.kz',
      email_verified: true,
      full_name: 'Айгерім',
      locale: 'ru',
      avatar_url: null,
      is_super_admin: false,
      created_at: '2026-10-01T10:00:00Z',
    },
    membership: {
      tenant_id: 't',
      roles: ['student'],
      status: 'active',
      faculty: null,
      department: null,
      group: null,
      study_level: null,
      course: null,
      onboarded,
    },
    tenant: { id: 't', slug: 'kazutb', name: { ru: 'КазУТБ' }, short_name: { ru: 'КазУТБ' } },
  };
}

const at = { href: '/app/projects/p1?tab=x' } as ParsedLocation;

async function redirectOf(promise: Promise<unknown>) {
  const error: unknown = await promise.then(
    () => undefined,
    (e: unknown) => e,
  );
  if (!isRedirect(error)) throw new Error('expected a redirect');
  return error.options;
}

describe('route guards', () => {
  it('check a remembered "signed out" again: the person may have signed in in another tab', async () => {
    const client = new QueryClient();
    client.setQueryData(sessionQuery.queryKey, null);
    // The other tab signed in: the shared cookie is there now.
    document.cookie = 'qadam_csrf=token; path=/';
    server.use(http.get('*/api/v1/auth/session', () => HttpResponse.json(session(true))));
    await expect(requireSession(client, at)).resolves.toMatchObject({ user: { id: 'u1' } });
  });

  it('send visitors to sign-in without asking the server', async () => {
    const client = new QueryClient();
    client.setQueryData(sessionQuery.queryKey, null);
    const options = await redirectOf(requireSession(client, at));
    expect(options).toMatchObject({ to: '/login', search: { redirect: '/app/projects/p1?tab=x' } });
  });

  it('keep the page a member was going to through onboarding', async () => {
    const client = new QueryClient();
    client.setQueryData(sessionQuery.queryKey, session(false));
    const options = await redirectOf(requireOnboarded(client, at));
    expect(options).toMatchObject({
      to: '/onboarding',
      search: { redirect: '/app/projects/p1?tab=x' },
    });
  });

  it('send a signed-in member away from sign-in, also after signing in elsewhere', async () => {
    const client = new QueryClient();
    client.setQueryData(sessionQuery.queryKey, null);
    document.cookie = 'qadam_csrf=token; path=/';
    server.use(http.get('*/api/v1/auth/session', () => HttpResponse.json(session(true))));
    const options = await redirectOf(redirectIfSignedIn(client, '/app/team'));
    expect(options).toMatchObject({ href: '/app/team' });
  });
});

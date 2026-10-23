import { QueryClient } from '@tanstack/react-query';
import { describe, expect, it, vi } from 'vitest';

import { sessionQuery, type Session } from '@/features/auth/api';

import { forgetUserData, watchSessionUser } from './session-watch';

function session(id: string): Session {
  return {
    user: {
      id,
      email: `${id}@kazutb.edu.kz`,
      email_verified: true,
      full_name: id,
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
      onboarded: true,
    },
    tenant: { id: 't', slug: 'kazutb', name: { ru: 'КазУТБ' }, short_name: { ru: 'КазУТБ' } },
  };
}

function cachedKeys(client: QueryClient): string[] {
  return client
    .getQueryCache()
    .getAll()
    .map((query) => String(query.queryKey[0]))
    .sort();
}

function withUserData(client: QueryClient) {
  client.setQueryData(['projects', 'list', 'mine', ''], { pages: [{ items: ['first'] }] });
  client.setQueryData(['me'], { name: 'first' });
  client.setQueryData(['tenant'], { slug: 'kazutb' });
}

describe('watchSessionUser', () => {
  it('keeps the cache when the first session becomes known', () => {
    const client = new QueryClient();
    const seen: (string | null)[] = [];
    const stop = watchSessionUser(client, (user) => seen.push(user));
    withUserData(client);
    client.setQueryData(sessionQuery.queryKey, session('a'));
    expect(cachedKeys(client)).toEqual(['me', 'projects', 'session', 'tenant']);
    expect(seen).toEqual(['a']);
    stop();
  });

  it('forgets the previous user when another one signs in', () => {
    const client = new QueryClient();
    client.setQueryData(sessionQuery.queryKey, session('a'));
    const stop = watchSessionUser(client);
    withUserData(client);
    client.setQueryData(sessionQuery.queryKey, session('b'));
    // Only what belongs to nobody stays: the new session and the university.
    expect(cachedKeys(client)).toEqual(['session', 'tenant']);
    expect(client.getQueryData(sessionQuery.queryKey)).toEqual(session('b'));
    stop();
  });

  it('forgets the user whose session was lost, and nothing on a later sign-in', () => {
    const client = new QueryClient();
    const seen: (string | null)[] = [];
    client.setQueryData(sessionQuery.queryKey, session('a'));
    const stop = watchSessionUser(client, (user) => seen.push(user));
    withUserData(client);
    client.setQueryData(sessionQuery.queryKey, null);
    expect(cachedKeys(client)).toEqual(['session', 'tenant']);

    // A visitor's page (the public showcase) is not personal data.
    client.setQueryData(['showcase', 'list'], { items: [] });
    client.setQueryData(sessionQuery.queryKey, session('b'));
    expect(cachedKeys(client)).toEqual(['session', 'showcase', 'tenant']);
    expect(seen).toEqual(['a', null, 'b']);
    stop();
  });

  it('ignores refreshes of the same user', () => {
    const client = new QueryClient();
    const onUser = vi.fn();
    client.setQueryData(sessionQuery.queryKey, session('a'));
    const stop = watchSessionUser(client, onUser);
    withUserData(client);
    client.setQueryData(sessionQuery.queryKey, session('a'));
    expect(cachedKeys(client)).toEqual(['me', 'projects', 'session', 'tenant']);
    expect(onUser).toHaveBeenCalledTimes(1);
    stop();
  });

  it('also survives a full sign-out that clears the cache first', () => {
    const client = new QueryClient();
    const seen: (string | null)[] = [];
    client.setQueryData(sessionQuery.queryKey, session('a'));
    const stop = watchSessionUser(client, (user) => seen.push(user));
    client.clear();
    client.setQueryData(sessionQuery.queryKey, null);
    expect(seen).toEqual(['a', null]);
    stop();
  });
});

describe('forgetUserData', () => {
  it('drops personal queries and pending mutations', () => {
    const client = new QueryClient();
    withUserData(client);
    client.setQueryData(sessionQuery.queryKey, session('a'));
    client.getMutationCache().build(client, { mutationKey: ['draft'] });
    forgetUserData(client);
    expect(cachedKeys(client)).toEqual(['session', 'tenant']);
    expect(client.getMutationCache().getAll()).toHaveLength(0);
  });
});

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import type { ReactNode } from 'react';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';

import { useCreateProject, type ProjectCreate } from '@/features/projects/api';

import { ApiError, NetworkError } from './client';
import { SubmissionKey } from './idempotency';

function problem(status: number, code: string) {
  return new ApiError({ type: 'about:blank', title: code, status, code });
}

function counter() {
  let n = 0;
  return () => `key-${String(++n)}`;
}

describe('SubmissionKey', () => {
  it('keeps the key while the same payload is retried after a lost answer', () => {
    const key = new SubmissionKey(counter());
    expect(key.for({ title: 'A' })).toBe('key-1');
    key.settle(new NetworkError(new Error('offline')));
    expect(key.for({ title: 'A' })).toBe('key-1');
    key.settle(problem(503, 'internal'));
    expect(key.for({ title: 'A' })).toBe('key-1');
    key.settle(problem(409, 'idempotency.in_progress'));
    expect(key.for({ title: 'A' })).toBe('key-1');
  });

  it('starts a new submission after success, a refusal or a changed payload', () => {
    const key = new SubmissionKey(counter());
    expect(key.for({ title: 'A' })).toBe('key-1');
    key.settle(null);
    expect(key.for({ title: 'A' })).toBe('key-2');
    key.settle(problem(422, 'validation.failed'));
    expect(key.for({ title: 'A' })).toBe('key-3');
    key.settle(new NetworkError(new Error('offline')));
    // The person edited the form after the failure: a different request.
    expect(key.for({ title: 'B' })).toBe('key-4');
  });
});

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

describe('creating mutations', () => {
  it('send one key per submission of a form', async () => {
    const keys: (string | null)[] = [];
    let fail = true;
    server.use(
      http.post('*/api/v1/projects', ({ request }) => {
        keys.push(request.headers.get('Idempotency-Key'));
        if (fail) return HttpResponse.error();
        return HttpResponse.json({ id: 'p1' }, { status: 201 });
      }),
    );
    const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useCreateProject(), { wrapper });
    const body: ProjectCreate = { title: 'AgroScan', tagline: '', industry: 'agritech' };

    await act(() => result.current.mutateAsync(body).catch(() => undefined));
    fail = false;
    await act(() => result.current.mutateAsync(body));
    await act(() => result.current.mutateAsync(body));

    expect(keys).toHaveLength(3);
    // The retry after the lost answer reuses the key; the next project is new.
    expect(keys[1]).toBe(keys[0]);
    expect(keys[2]).not.toBe(keys[1]);
  });
});

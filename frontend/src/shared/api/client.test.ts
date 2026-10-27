import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';

import { api, ApiError, NetworkError, unwrap } from './client';

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

const meta = {
  product: 'Qadam',
  version: '0.1.0',
  commit: 'abc1234',
  built_at: 'unknown',
  locales: ['kk', 'ru', 'en'],
  default_locale: 'kk',
  server_time: '2026-10-21T10:00:00Z',
};

describe('unwrap', () => {
  it('returns data of successful responses', async () => {
    server.use(http.get('*/api/v1/meta', () => HttpResponse.json(meta)));
    await expect(unwrap(api.GET('/meta'))).resolves.toMatchObject({ version: '0.1.0' });
  });

  it('throws ApiError carrying the problem code', async () => {
    server.use(
      http.get('*/api/v1/meta', () =>
        HttpResponse.json(
          {
            type: 'https://qadam.kz/problems/route.not_found',
            title: 'Not Found',
            status: 404,
            code: 'route.not_found',
          },
          { status: 404, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );
    const error = await unwrap(api.GET('/meta')).catch((e: unknown) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).code).toBe('route.not_found');
    expect((error as ApiError).status).toBe(404);
  });

  it('maps non-problem failures to a generic code', async () => {
    server.use(http.get('*/api/v1/meta', () => new HttpResponse('bad gateway', { status: 502 })));
    const error = await unwrap(api.GET('/meta')).catch((e: unknown) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).code).toBe('internal');
  });

  it('throws NetworkError when the server is unreachable', async () => {
    server.use(http.get('*/api/v1/meta', () => HttpResponse.error()));
    await expect(unwrap(api.GET('/meta'))).rejects.toBeInstanceOf(NetworkError);
  });

  it('sends the interface language', async () => {
    let language: string | null = null;
    server.use(
      http.get('*/api/v1/meta', ({ request }) => {
        language = request.headers.get('Accept-Language');
        return HttpResponse.json(meta);
      }),
    );
    await unwrap(api.GET('/meta'));
    expect(language).not.toBeNull();
  });
});

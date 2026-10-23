import { QueryClient } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';

import { stepQuery } from './api';
import { prefetchStepPage } from './prefetch';

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

describe('prefetchStepPage', () => {
  it('asks for the template, project, steps and step at the same time', async () => {
    const seen: string[] = [];
    let release: () => void = () => undefined;
    const allArrived = new Promise<void>((resolve) => {
      release = resolve;
    });
    // Every answer waits until all four requests have arrived: one after another would hang.
    const gate = async (name: string, body: Record<string, unknown>) => {
      seen.push(name);
      if (seen.length === 4) release();
      await Promise.race([allArrived, new Promise((resolve) => setTimeout(resolve, 2_000))]);
      return HttpResponse.json(body, { headers: { ETag: '"3"' } });
    };
    server.use(
      http.get('*/api/v1/incubator/template', () => gate('template', { stages: [] })),
      http.get('*/api/v1/projects/p1', () => gate('project', { id: 'p1' })),
      http.get('*/api/v1/projects/p1/steps', () => gate('steps', { items: [] })),
      http.get('*/api/v1/projects/p1/steps/problem', () =>
        gate('step', { key: 'problem', version: 3, values: {} }),
      ),
    );
    const client = new QueryClient();
    const started = Date.now();
    await prefetchStepPage(client, 'p1', 'problem');
    expect(Date.now() - started).toBeLessThan(1_500);
    expect([...seen].sort()).toEqual(['project', 'step', 'steps', 'template']);
    expect(client.getQueryData(stepQuery('p1', 'problem').queryKey)?.etag).toBe('"3"');
  });

  it('leaves failures to the page', async () => {
    server.use(http.get('*/api/v1/*', () => HttpResponse.error()));
    await expect(prefetchStepPage(new QueryClient(), 'p1', 'problem')).resolves.toBeDefined();
  });
});

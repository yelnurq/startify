import { QueryClient, type Query } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';

import { materialsQuery } from '@/features/incubator/api';
import { finishedSince } from '@/shared/api/polling';

import {
  evaluationQuery,
  evaluationsQuery,
  isRunning,
  POLL_MS,
  reportUrl,
  type Evaluation,
  type EvaluationSummary,
} from './api';

function stateOf<T>(data: T | undefined): Query<T, Error, T, string[]> {
  return { state: { data } } as unknown as Query<T, Error, T, string[]>;
}

describe('evaluation polling', () => {
  it('knows which statuses are still running', () => {
    expect(isRunning('pending')).toBe(true);
    expect(isRunning('running')).toBe(true);
    expect(isRunning('ready')).toBe(false);
    expect(isRunning('failed')).toBe(false);
    expect(isRunning(undefined)).toBe(false);
  });

  it('polls one evaluation only while it runs', () => {
    const interval = evaluationQuery('p', 'e').refetchInterval;
    if (typeof interval !== 'function') throw new Error('expected a function');
    const at = (status: Evaluation['status'] | undefined) =>
      interval(stateOf(status ? ({ status } as Evaluation) : undefined));
    expect(at('pending')).toBe(POLL_MS);
    expect(at('running')).toBe(POLL_MS);
    expect(at('ready')).toBe(false);
    expect(at(undefined)).toBe(false);
  });

  it('refreshes the history while any evaluation runs', () => {
    const interval = evaluationsQuery('p').refetchInterval;
    if (typeof interval !== 'function') throw new Error('expected a function');
    const list = (...statuses: EvaluationSummary['status'][]) =>
      interval(stateOf(statuses.map((status) => ({ status }) as EvaluationSummary)));
    expect(list('ready', 'running')).toBe(POLL_MS);
    expect(list('ready', 'failed')).toBe(false);
  });

  it('builds the report address in a language', () => {
    expect(reportUrl('p1', 'e1', 'kk')).toBe('/api/v1/projects/p1/evaluations/e1/report?locale=kk');
  });
});

describe('finishedSince', () => {
  const running = (item: { id: string; status: string }) => item.status === 'running';
  it('notices an item that finished between two refreshes', () => {
    expect(finishedSince(undefined, [{ id: 'a', status: 'ready' }], running)).toBe(false);
    expect(
      finishedSince([{ id: 'a', status: 'running' }], [{ id: 'a', status: 'running' }], running),
    ).toBe(false);
    expect(
      finishedSince([{ id: 'a', status: 'running' }], [{ id: 'a', status: 'failed' }], running),
    ).toBe(true);
    // A new evaluation that is already done was never seen running here.
    expect(finishedSince([], [{ id: 'b', status: 'ready' }], running)).toBe(false);
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

function summary(status: EvaluationSummary['status']): EvaluationSummary {
  return {
    id: 'e1',
    status,
    total: status === 'ready' ? 72 : null,
    level: null,
    overridden: false,
    created_at: '2026-10-23T10:00:00Z',
    completed_at: null,
  };
}

/** A client with the project's card and lists cached and fresh. */
function clientWithScores() {
  const client = new QueryClient({ defaultOptions: { queries: { staleTime: Infinity } } });
  client.setQueryData(['projects', 'p1', 'card'], { data: {}, etag: '"1"' });
  client.setQueryData(['projects', 'list', 'mine', ''], { pages: [] });
  return client;
}

function invalidated(client: QueryClient, key: readonly unknown[]): boolean {
  return client.getQueryState(key)?.isInvalidated ?? false;
}

describe('results of background work', () => {
  it('refresh the project’s score once a running evaluation finishes', async () => {
    let status: EvaluationSummary['status'] = 'running';
    server.use(
      http.get('*/api/v1/projects/p1/evaluations', () =>
        HttpResponse.json({ items: [summary(status)] }),
      ),
    );
    const client = clientWithScores();
    await client.query(evaluationsQuery('p1'));
    expect(invalidated(client, ['projects', 'p1', 'card'])).toBe(false);

    status = 'ready';
    await client.query({ ...evaluationsQuery('p1'), staleTime: 0 });
    expect(invalidated(client, ['projects', 'p1', 'card'])).toBe(true);
    expect(invalidated(client, ['projects', 'list', 'mine', ''])).toBe(true);
  });

  it('refresh the project once a material is generated', async () => {
    let status = 'pending';
    server.use(
      http.get('*/api/v1/projects/p1/materials', () =>
        HttpResponse.json({
          items: [
            {
              id: 'm1',
              kind: 'one_pager',
              format: 'pdf',
              locale: 'ru',
              status,
              error: null,
              download_url: null,
              created_at: '2026-10-23T10:00:00Z',
              updated_at: '2026-10-23T10:00:00Z',
            },
          ],
        }),
      ),
    );
    const client = clientWithScores();
    await client.query(materialsQuery('p1'));
    expect(invalidated(client, ['projects', 'p1', 'card'])).toBe(false);
    status = 'ready';
    await client.query({ ...materialsQuery('p1'), staleTime: 0 });
    expect(invalidated(client, ['projects', 'p1', 'card'])).toBe(true);
  });
});

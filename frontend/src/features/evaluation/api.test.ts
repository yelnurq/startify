import type { Query } from '@tanstack/react-query';
import { describe, expect, it } from 'vitest';

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

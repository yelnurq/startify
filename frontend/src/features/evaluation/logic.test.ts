import { act, renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import {
  bySeverity,
  DEFAULT_WEIGHTS,
  delta,
  experimentPlan,
  levelTone,
  useExperimentChecks,
  weightSum,
  weightsProblem,
} from './logic';

describe('evaluation logic', () => {
  it('colours levels from neutral to success', () => {
    expect(levelTone('idea')).toBe('neutral');
    expect(levelTone('validated_problem')).toBe('info');
    expect(levelTone('mvp_ready')).toBe('primary');
    expect(levelTone('investment_ready')).toBe('success');
    expect(levelTone(null)).toBe('neutral');
  });

  it('computes deltas only against an earlier score', () => {
    expect(delta(72, 64)).toEqual({ value: 8, direction: 'up' });
    expect(delta(60, 64)).toEqual({ value: -4, direction: 'down' });
    expect(delta(5, 5)).toEqual({ value: 0, direction: 'same' });
    expect(delta(5, null)).toBeNull();
    expect(delta(undefined, 5)).toBeNull();
  });

  it('orders red flags by severity, stably', () => {
    const flags = [
      { title: 'a', severity: 'low' as const },
      { title: 'b', severity: 'high' as const },
      { title: 'c', severity: 'medium' as const },
      { title: 'd', severity: 'high' as const },
    ];
    expect(bySeverity(flags).map((f) => f.title)).toEqual(['b', 'd', 'c', 'a']);
  });

  it('groups experiments by day and keeps their positions', () => {
    const e = (title: string, day: number) => ({ title, how: '', metric: '', success: '', day });
    const plan = experimentPlan([e('x', 4), e('y', 1), e('z', 4), e('w', 9)]);
    expect(plan.map((d) => d.day)).toEqual([1, 4, 7]);
    expect(plan[1]?.items.map((i) => i.index)).toEqual([0, 2]);
  });

  it('accepts only whole weights adding up to 100', () => {
    const keys = Object.keys(DEFAULT_WEIGHTS);
    expect(weightSum(DEFAULT_WEIGHTS)).toBe(100);
    expect(weightsProblem(DEFAULT_WEIGHTS, keys)).toBeNull();
    expect(weightsProblem({ ...DEFAULT_WEIGHTS, problem: 20 }, keys)).toBe('sum');
    expect(weightsProblem({ ...DEFAULT_WEIGHTS, problem: 1.5 }, keys)).toBe('invalid');
    expect(weightsProblem({ ...DEFAULT_WEIGHTS, team: -10, problem: 35 }, keys)).toBe('invalid');
    const { impact: _impact, ...missing } = DEFAULT_WEIGHTS;
    expect(weightsProblem(missing, keys)).toBe('invalid');
  });
});

describe('experiment checks', () => {
  afterEach(() => {
    localStorage.clear();
  });

  it('remembers ticks per evaluation', () => {
    const first = renderHook(() => useExperimentChecks('e1'));
    act(() => {
      first.result.current.toggle(2);
    });
    expect(first.result.current.done.has(2)).toBe(true);
    const again = renderHook(() => useExperimentChecks('e1'));
    expect([...again.result.current.done]).toEqual([2]);
    const other = renderHook(() => useExperimentChecks('e2'));
    expect(other.result.current.done.size).toBe(0);
    act(() => {
      again.result.current.toggle(2);
    });
    expect(again.result.current.done.size).toBe(0);
  });
});

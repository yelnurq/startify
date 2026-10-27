import { useCallback, useState } from 'react';

import type { Tone } from '@/design-system';

import type { Project } from '@/features/projects/api';

import type { Experiment, ReadinessLevel, RedFlagSeverity } from './api';

export const LEVELS = [
  'idea',
  'validated_problem',
  'mvp_ready',
  'investment_ready',
] as const satisfies readonly ReadinessLevel[];

/** Colour of a readiness level: it grows from neutral to success. */
export function levelTone(level: ReadinessLevel | null | undefined): Tone {
  switch (level) {
    case 'validated_problem':
      return 'info';
    case 'mvp_ready':
      return 'primary';
    case 'investment_ready':
      return 'success';
    default:
      return 'neutral';
  }
}

export interface Delta {
  value: number;
  direction: 'up' | 'down' | 'same';
}

/** Change against an earlier score, or null without one. */
export function delta(
  current: number | null | undefined,
  previous: number | null | undefined,
): Delta | null {
  if (current === null || current === undefined || previous === null || previous === undefined) {
    return null;
  }
  const value = current - previous;
  return { value, direction: value > 0 ? 'up' : value < 0 ? 'down' : 'same' };
}

export const SEVERITIES = ['high', 'medium', 'low'] as const satisfies readonly RedFlagSeverity[];

export function severityTone(severity: RedFlagSeverity): 'danger' | 'warning' | 'info' {
  return severity === 'high' ? 'danger' : severity === 'medium' ? 'warning' : 'info';
}

/** Red flags, the most serious first (stable within a severity). */
export function bySeverity<T extends { severity: RedFlagSeverity }>(flags: T[]): T[] {
  return [...flags].sort((a, b) => SEVERITIES.indexOf(a.severity) - SEVERITIES.indexOf(b.severity));
}

/** Experiments in the order of the week, keeping their position for the checklist. */
export function experimentPlan(
  experiments: Experiment[],
): { day: number; items: { index: number; experiment: Experiment }[] }[] {
  const days = new Map<number, { index: number; experiment: Experiment }[]>();
  experiments.forEach((experiment, index) => {
    const day = Math.min(7, Math.max(1, experiment.day));
    days.set(day, [...(days.get(day) ?? []), { index, experiment }]);
  });
  return [...days.entries()].sort(([a], [b]) => a - b).map(([day, items]) => ({ day, items }));
}

/** Weights of the platform's default rubric. */
export const DEFAULT_WEIGHTS: Record<string, number> = {
  problem: 15,
  market: 15,
  solution: 15,
  business_model: 15,
  team: 10,
  validation: 10,
  feasibility: 10,
  impact: 10,
};

export function weightSum(weights: Record<string, number>): number {
  return Object.values(weights).reduce((sum, w) => sum + (Number.isFinite(w) ? w : 0), 0);
}

/** What keeps weights from being saved: a weight out of range or a sum other than 100. */
export function weightsProblem(
  weights: Record<string, number>,
  keys: readonly string[],
): 'invalid' | 'sum' | null {
  for (const key of keys) {
    const w = weights[key];
    if (w === undefined || !Number.isInteger(w) || w < 0 || w > 100) return 'invalid';
  }
  return weightSum(weights) === 100 ? null : 'sum';
}

function checksKey(evaluationId: string): string {
  return `qadam.evaluation.${evaluationId}.experiments`;
}

function readChecks(evaluationId: string): number[] {
  try {
    const raw: unknown = JSON.parse(localStorage.getItem(checksKey(evaluationId)) ?? '[]');
    return Array.isArray(raw) ? raw.filter((v): v is number => typeof v === 'number') : [];
  } catch {
    return [];
  }
}

/** The member's own ticks on the experiments of an evaluation, kept in this browser. */
export function useExperimentChecks(evaluationId: string) {
  const [done, setDone] = useState(() => new Set(readChecks(evaluationId)));
  const toggle = useCallback(
    (index: number) => {
      setDone((current) => {
        const next = new Set(current);
        if (next.has(index)) next.delete(index);
        else next.add(index);
        try {
          localStorage.setItem(checksKey(evaluationId), JSON.stringify([...next]));
        } catch {
          // Not persisted (private mode); the ticks last until the page closes.
        }
        return next;
      });
    },
    [evaluationId],
  );
  return { done, toggle };
}

/** Who may ask for an evaluation: the team, the supervisor, innovation managers. */
export function canRequestEvaluation(
  project: Pick<Project, 'access' | 'archived'>,
  manages: boolean,
): boolean {
  if (project.archived) return false;
  return project.access !== 'manager' || manages;
}

/** Limits of an expert correction, checked again by the server. */
export const OVERRIDE_LIMITS = { comment: [10, 1000], score: [0, 10] } as const;

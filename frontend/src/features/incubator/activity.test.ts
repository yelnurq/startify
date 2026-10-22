import { describe, expect, it } from 'vitest';

import type { Activity } from '@/features/projects/api';

import { groupActivity } from './activity';

const actor = (id: string) => ({ id, full_name: id, avatar_url: null, faculty: null });
const edit = (who: string, step: string, field: string, at: string): Activity => ({
  kind: 'edit',
  actor: actor(who),
  step_key: step,
  field_key: field,
  at,
});

describe('groupActivity', () => {
  it('folds a member’s consecutive edits of one step', () => {
    const groups = groupActivity([
      edit('a', 'unit', 'price', '2026-10-22T10:10:00Z'),
      edit('a', 'unit', 'cac', '2026-10-22T10:09:00Z'),
      edit('a', 'unit', 'price', '2026-10-22T10:08:00Z'),
      edit('b', 'unit', 'cac', '2026-10-22T10:07:00Z'),
      {
        kind: 'comment',
        actor: actor('a'),
        step_key: 'unit',
        field_key: null,
        at: '2026-10-22T10:06:00Z',
      },
      edit('a', 'unit', 'unit', '2026-10-22T10:05:00Z'),
      edit('a', 'unit', 'cac', '2026-10-22T09:30:00Z'),
    ]);
    expect(groups.map((g) => [g.kind, g.actor?.id, g.fields])).toEqual([
      ['edit', 'a', ['price', 'cac']],
      ['edit', 'b', ['cac']],
      ['comment', 'a', []],
      ['edit', 'a', ['unit']],
      ['edit', 'a', ['cac']],
    ]);
    expect(groups[0]?.at).toBe('2026-10-22T10:10:00Z');
  });
});

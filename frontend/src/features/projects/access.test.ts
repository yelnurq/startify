import { describe, expect, it } from 'vitest';

import { canApprove, canComment } from './api';

describe('who acts in a project', () => {
  const open = { archived: false } as const;

  it('lets the team, supervisors and innovation managers comment', () => {
    expect(canComment({ ...open, access: 'owner' }, false)).toBe(true);
    expect(canComment({ ...open, access: 'member' }, false)).toBe(true);
    expect(canComment({ ...open, access: 'supervisor' }, false)).toBe(true);
    expect(canComment({ ...open, access: 'manager' }, true)).toBe(true);
  });

  it('keeps leadership and administrators to reading, and archives closed', () => {
    expect(canComment({ ...open, access: 'manager' }, false)).toBe(false);
    expect(canComment({ archived: true, access: 'owner' }, false)).toBe(false);
  });

  it('never lets the team approve its own steps', () => {
    expect(canApprove({ ...open, access: 'owner' }, true)).toBe(false);
    expect(canApprove({ ...open, access: 'supervisor' }, false)).toBe(true);
    expect(canApprove({ ...open, access: 'manager' }, true)).toBe(true);
  });
});

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError, NetworkError } from '@/shared/api/client';

import type { Step } from './api';
import { StepEditor, type EditorBackend, type SaveSource, type Tagged } from './step-editor';
import type { StepValues } from './values';

function step(values: StepValues, version: number): Tagged {
  const data: Step = {
    key: 'problem',
    status: 'in_progress',
    version,
    values,
    missing: [],
    check: null,
    completed_at: null,
    approved_by: null,
    approval_comment: null,
    updated_at: null,
    updated_by: null,
  };
  return { data, etag: `"${String(version)}"` };
}

/** A fake server holding one step. */
class Server implements EditorBackend {
  values: StepValues;
  version: number;
  calls: { values: StepValues; source: SaveSource; etag: string }[] = [];
  offline = false;

  constructor(values: StepValues, version = 1) {
    this.values = values;
    this.version = version;
  }

  save = (body: { values: StepValues; source: SaveSource }, etag: string) => {
    this.calls.push({ ...body, etag });
    if (this.offline) return Promise.reject(new NetworkError(new Error('down')));
    if (etag !== `"${String(this.version)}"`) {
      return Promise.reject(
        new ApiError({ type: 'about:blank', title: 'x', status: 412, code: 'precondition.failed' }),
      );
    }
    const cleaned = Object.fromEntries(
      Object.entries(body.values).map(([k, v]) => [k, typeof v === 'string' ? v.trim() : v]),
    );
    this.values = { ...this.values, ...cleaned };
    this.version++;
    return Promise.resolve(step(this.values, this.version));
  };

  load = () => Promise.resolve(step(this.values, this.version));

  /** Another member saves. */
  edit(values: StepValues) {
    this.values = { ...this.values, ...values };
    this.version++;
  }
}

beforeEach(() => {
  vi.useFakeTimers();
});
afterEach(() => {
  vi.useRealTimers();
});

describe('StepEditor', () => {
  it('saves only changed fields after a pause, with the version they were made on', async () => {
    const server = new Server({ statement: 'old', who: 'students' });
    const editor = new StepEditor(step(server.values, 1), server, 1000);
    editor.set('statement', 'new');
    editor.set('statement', 'newer ');
    expect(editor.getSnapshot().status).toBe('dirty');
    await vi.advanceTimersByTimeAsync(999);
    expect(server.calls).toHaveLength(0);
    await vi.advanceTimersByTimeAsync(1);
    expect(server.calls).toEqual([
      { values: { statement: 'newer ' }, source: 'edit', etag: '"1"' },
    ]);
    const snap = editor.getSnapshot();
    expect(snap.status).toBe('saved');
    // The trailing space the server trimmed does not count as an unsaved edit.
    expect(snap.dirty).toEqual([]);
    expect(snap.values.statement).toBe('newer ');
    expect(editor.version).toBe(2);
  });

  it('merges others’ edits of other fields and saves again', async () => {
    const server = new Server({ statement: 'a', who: 'b' });
    const editor = new StepEditor(step(server.values, 1), server, 500);
    server.edit({ who: 'theirs' });
    editor.set('statement', 'mine');
    await vi.advanceTimersByTimeAsync(500);
    expect(server.calls.map((c) => c.etag)).toEqual(['"1"', '"2"']);
    expect(server.values).toEqual({ statement: 'mine', who: 'theirs' });
    expect(editor.getSnapshot().values).toEqual({ statement: 'mine', who: 'theirs' });
    expect(editor.getSnapshot().status).toBe('saved');
  });

  it('asks when both changed the same field and saves the choice', async () => {
    const server = new Server({ statement: 'a' });
    const editor = new StepEditor(step(server.values, 1), server, 500);
    server.edit({ statement: 'theirs' });
    editor.set('statement', 'mine');
    await vi.advanceTimersByTimeAsync(500);
    const snap = editor.getSnapshot();
    expect(snap.status).toBe('conflict');
    expect(snap.conflict?.fields).toEqual(['statement']);
    expect(snap.conflict?.theirs.statement).toBe('theirs');

    await editor.resolve({ statement: 'mine' });
    expect(server.values.statement).toBe('mine');
    expect(editor.getSnapshot().status).toBe('saved');
    expect(editor.getSnapshot().conflict).toBeUndefined();
  });

  it('keeps edits offline and retries', async () => {
    const server = new Server({ statement: 'a' });
    const editor = new StepEditor(step(server.values, 1), server, 500);
    server.offline = true;
    editor.set('statement', 'b');
    await vi.advanceTimersByTimeAsync(500);
    expect(editor.getSnapshot().status).toBe('offline');
    server.offline = false;
    await vi.advanceTimersByTimeAsync(5_000);
    expect(server.values.statement).toBe('b');
    expect(editor.getSnapshot().status).toBe('saved');
  });

  it('applies a mentor suggestion after flushing pending edits', async () => {
    const server = new Server({ statement: 'a', who: 'x' });
    const editor = new StepEditor(step(server.values, 1), server, 500);
    editor.set('who', 'typed');
    await editor.apply('statement', 'from mentor');
    expect(server.calls.map((c) => [Object.keys(c.values), c.source])).toEqual([
      [['who'], 'edit'],
      [['statement'], 'mentor'],
    ]);
  });

  it('refreshes quietly only without unsaved edits', async () => {
    const server = new Server({ statement: 'a' });
    const editor = new StepEditor(step(server.values, 1), server, 500);
    server.edit({ statement: 'theirs' });
    await editor.refresh();
    expect(editor.getSnapshot().values.statement).toBe('theirs');

    server.edit({ statement: 'again' });
    editor.set('statement', 'typing');
    await editor.refresh();
    expect(editor.getSnapshot().values.statement).toBe('typing');
  });

  it('reports validation errors without retrying', async () => {
    const server = new Server({ statement: 'a' });
    server.save = () =>
      Promise.reject(
        new ApiError({
          type: 'about:blank',
          title: 'x',
          status: 422,
          code: 'validation.failed',
          errors: [{ field: 'fields.statement', code: 'maxLength', params: { count: 10 } }],
        }),
      );
    const editor = new StepEditor(step(server.values, 1), server, 500);
    editor.set('statement', 'very long');
    await vi.advanceTimersByTimeAsync(500);
    expect(editor.getSnapshot().status).toBe('error');
    expect(editor.getSnapshot().fieldErrors[0]?.field).toBe('fields.statement');
  });
});

describe('StepEditor lifecycle', () => {
  it('keeps saving after an unmount and remount of its effects', async () => {
    vi.useFakeTimers();
    const server = new Server({ statement: 'a' });
    const editor = new StepEditor(step(server.values, 1), server, 500);
    editor.dispose();
    editor.resume();
    editor.set('statement', 'b');
    await vi.advanceTimersByTimeAsync(500);
    expect(server.values.statement).toBe('b');
    vi.useRealTimers();
  });
});

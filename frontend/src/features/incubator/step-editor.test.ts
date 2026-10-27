import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError, NetworkError } from '@/shared/api/client';

import type { Step } from './api';
import {
  abandonEditors,
  discardDraftsExcept,
  hasUnsavedEdits,
  sessionDraftStore,
  type DraftStore,
  type StepDraft,
} from './drafts';
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

let stores = 0;

/** A draft store in memory, unique per test (the owner registry is global). */
function memoryStore(draft?: StepDraft): DraftStore & { draft: StepDraft | undefined } {
  const store = {
    id: `test-draft-${String(++stores)}`,
    draft,
    read: () => store.draft,
    write: (next: StepDraft) => {
      store.draft = next;
    },
    clear: () => {
      store.draft = undefined;
    },
  };
  return store;
}

describe('StepEditor drafts', () => {
  afterEach(() => {
    abandonEditors();
  });

  it('keeps unsaved edits in the draft until the server has them', async () => {
    const server = new Server({ statement: 'a', who: 'x' });
    const drafts = memoryStore();
    const editor = new StepEditor(step(server.values, 1), server, 500, undefined, drafts);
    editor.resume();
    server.offline = true;
    editor.set('statement', 'b');
    expect(drafts.draft).toEqual({ values: { statement: 'b' }, base: { statement: 'a' } });
    expect(hasUnsavedEdits()).toBe(true);

    await vi.advanceTimersByTimeAsync(500);
    expect(editor.getSnapshot().status).toBe('offline');
    expect(drafts.draft?.values).toEqual({ statement: 'b' });

    server.offline = false;
    await vi.advanceTimersByTimeAsync(5_000);
    expect(server.values.statement).toBe('b');
    expect(drafts.draft).toBeUndefined();
    expect(hasUnsavedEdits()).toBe(false);
  });

  it('brings the draft back over the newest version and saves it', async () => {
    const server = new Server({ statement: 'a', who: 'x' });
    // Someone else changed another field since the draft was written.
    server.edit({ who: 'theirs' });
    const drafts = memoryStore({ values: { statement: 'mine' }, base: { statement: 'a' } });
    const editor = new StepEditor(step(server.values, 2), server, 500, undefined, drafts);
    const snap = editor.getSnapshot();
    expect(snap.values).toEqual({ statement: 'mine', who: 'theirs' });
    expect(snap.status).toBe('dirty');
    expect(snap.restored).toBe(true);

    editor.resume();
    await vi.advanceTimersByTimeAsync(500);
    expect(server.values).toEqual({ statement: 'mine', who: 'theirs' });
    expect(drafts.draft).toBeUndefined();
  });

  it('asks when the draft and the server changed the same field', () => {
    const server = new Server({ statement: 'theirs' }, 2);
    const drafts = memoryStore({ values: { statement: 'mine' }, base: { statement: 'a' } });
    const editor = new StepEditor(step(server.values, 2), server, 500, undefined, drafts);
    const snap = editor.getSnapshot();
    expect(snap.status).toBe('conflict');
    expect(snap.conflict?.fields).toEqual(['statement']);
    expect(snap.conflict?.mine.statement).toBe('mine');
    expect(snap.conflict?.theirs.statement).toBe('theirs');
  });

  it('forgets a draft the server already has', () => {
    const server = new Server({ statement: 'saved' }, 2);
    const drafts = memoryStore({ values: { statement: 'saved' }, base: { statement: 'a' } });
    const editor = new StepEditor(step(server.values, 2), server, 500, undefined, drafts);
    expect(editor.getSnapshot().restored).toBe(false);
    editor.resume();
    expect(drafts.draft).toBeUndefined();
  });

  it('keeps retrying after the form is gone while offline', async () => {
    const server = new Server({ statement: 'a' });
    const drafts = memoryStore();
    const editor = new StepEditor(step(server.values, 1), server, 500, undefined, drafts);
    editor.resume();
    server.offline = true;
    editor.set('statement', 'b');
    // The person moves to another step at once.
    editor.dispose();
    await vi.advanceTimersByTimeAsync(0);
    expect(server.calls).toHaveLength(1);
    expect(hasUnsavedEdits()).toBe(true);

    await vi.advanceTimersByTimeAsync(5_000);
    expect(server.calls).toHaveLength(2);
    server.offline = false;
    await vi.advanceTimersByTimeAsync(5_000);
    expect(server.values.statement).toBe('b');
    expect(drafts.draft).toBeUndefined();
    expect(hasUnsavedEdits()).toBe(false);
  });

  it('leaves a rejected save in the draft for the next visit', async () => {
    const server = new Server({ statement: 'a' });
    server.save = () =>
      Promise.reject(
        new ApiError({ type: 'about:blank', title: 'x', status: 422, code: 'validation.failed' }),
      );
    const drafts = memoryStore();
    const editor = new StepEditor(step(server.values, 1), server, 500, undefined, drafts);
    editor.resume();
    editor.set('statement', 'too long');
    editor.dispose();
    await vi.advanceTimersByTimeAsync(10_000);
    expect(drafts.draft?.values).toEqual({ statement: 'too long' });
    // Nothing is retrying, so leaving the page does not ask.
    expect(hasUnsavedEdits()).toBe(false);
  });

  it('gives way to a new editor of the same step', async () => {
    const server = new Server({ statement: 'a' });
    const drafts = memoryStore();
    const first = new StepEditor(step(server.values, 1), server, 500, undefined, drafts);
    first.resume();
    server.offline = true;
    first.set('statement', 'b');
    first.dispose();
    await vi.advanceTimersByTimeAsync(0);

    // The step is opened again: the new editor takes over the draft.
    const second = new StepEditor(step(server.values, 1), server, 500, undefined, drafts);
    second.resume();
    expect(second.getSnapshot().values.statement).toBe('b');
    const calls = server.calls.length;
    await vi.advanceTimersByTimeAsync(5_000);
    // Only the new editor retries.
    expect(server.calls.length).toBe(calls + 1);
  });

  it('stops for good when the session ends', async () => {
    const server = new Server({ statement: 'a' });
    const drafts = memoryStore();
    const editor = new StepEditor(step(server.values, 1), server, 500, undefined, drafts);
    editor.resume();
    editor.set('statement', 'b');
    abandonEditors();
    await vi.advanceTimersByTimeAsync(5_000);
    expect(server.calls).toHaveLength(0);
    // The draft stays for the same person signing in again.
    expect(drafts.draft?.values).toEqual({ statement: 'b' });
  });

  it('asks before the page is left with unsaved edits', () => {
    const server = new Server({ statement: 'a' });
    const editor = new StepEditor(step(server.values, 1), server, 500, undefined, memoryStore());
    editor.resume();
    const calm = new Event('beforeunload', { cancelable: true });
    window.dispatchEvent(calm);
    expect(calm.defaultPrevented).toBe(false);

    editor.set('statement', 'b');
    const leaving = new Event('beforeunload', { cancelable: true });
    window.dispatchEvent(leaving);
    expect(leaving.defaultPrevented).toBe(true);
  });
});

describe('sessionDraftStore', () => {
  it('keeps drafts per person and step in the tab', () => {
    sessionStorage.clear();
    const mine = sessionDraftStore('u1', 'p1', 'problem');
    mine.write({ values: { statement: 'b' }, base: { statement: 'a' } });
    expect(sessionDraftStore('u1', 'p1', 'problem').read()).toEqual({
      values: { statement: 'b' },
      base: { statement: 'a' },
    });
    expect(sessionDraftStore('u2', 'p1', 'problem').read()).toBeUndefined();
    mine.clear();
    expect(mine.read()).toBeUndefined();
  });

  it('ignores what is not a draft', () => {
    sessionStorage.clear();
    const store = sessionDraftStore('u1', 'p1', 'problem');
    sessionStorage.setItem(store.id, '{"values": 1}');
    expect(store.read()).toBeUndefined();
    sessionStorage.setItem(store.id, 'not json');
    expect(store.read()).toBeUndefined();
  });

  it('removes the drafts of other people', () => {
    sessionStorage.clear();
    const draft = { values: { statement: 'b' }, base: {} };
    sessionDraftStore('u1', 'p1', 'problem').write(draft);
    sessionDraftStore('u2', 'p1', 'problem').write(draft);
    sessionStorage.setItem('qadam.other', 'kept');
    discardDraftsExcept('u2');
    expect(sessionDraftStore('u1', 'p1', 'problem').read()).toBeUndefined();
    expect(sessionDraftStore('u2', 'p1', 'problem').read()).toEqual(draft);
    expect(sessionStorage.getItem('qadam.other')).toBe('kept');
    discardDraftsExcept(null);
    expect(sessionDraftStore('u2', 'p1', 'problem').read()).toBeUndefined();
  });
});

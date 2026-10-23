import { ApiError, NetworkError, type FieldError } from '@/shared/api/client';

import type { Step } from './api';
import {
  claimDraft,
  releaseDraft,
  type DraftStore,
  type PendingEditor,
  type StepDraft,
} from './drafts';
import { mergeStep, resolveConflicts, type Resolution } from './merge';
import { changedKeys, pick, type FieldValue, type StepValues } from './values';

export type SaveStatus = 'idle' | 'dirty' | 'saving' | 'saved' | 'offline' | 'conflict' | 'error';
export type SaveSource = 'edit' | 'mentor';

export interface Tagged {
  data: Step;
  etag: string | undefined;
}

export interface Conflict {
  fields: string[];
  mine: StepValues;
  theirs: StepValues;
  merged: StepValues;
  latest: Tagged;
}

export interface EditorSnapshot {
  values: StepValues;
  step: Step;
  status: SaveStatus;
  /** Fields with edits the server does not have yet. */
  dirty: string[];
  savedAt: number | undefined;
  conflict: Conflict | undefined;
  error: unknown;
  /** Field errors of the last rejected save, by path (`fields.<key>…`). */
  fieldErrors: FieldError[];
  /** Unsaved edits of an earlier visit were brought back. */
  restored: boolean;
}

export interface EditorBackend {
  save: (body: { values: StepValues; source: SaveSource }, etag: string) => Promise<Tagged>;
  load: () => Promise<Tagged>;
}

/** ETag of a step: from the response header, or built from the version. */
function tagOf(tagged: Tagged): string {
  return tagged.etag ?? `"${String(tagged.data.version)}"`;
}

/**
 * Edits of one step with autosave. Only changed fields are sent, with the
 * version they were made on (If-Match). When the server has a newer version,
 * the edits are merged into it field by field; overlapping changes wait for
 * the person to choose. Local values are never replaced by the server's
 * while they differ from what the server had.
 *
 * Unsaved edits are also kept in a draft store until the server has them:
 * after a reload, a lost connection or leaving the step they come back and
 * are merged into the server's version the same way. A step left with
 * unsaved edits keeps saving them in the background until it is opened again.
 */
export class StepEditor implements PendingEditor {
  private values: StepValues;
  private base: StepValues;
  private step: Step;
  private etag: string;
  private status: SaveStatus = 'idle';
  private savedAt: number | undefined;
  private conflict: Conflict | undefined;
  private error: unknown;
  private fieldErrors: FieldError[] = [];
  private snapshot: EditorSnapshot;
  private listeners = new Set<() => void>();
  private timer: ReturnType<typeof setTimeout> | undefined;
  private inflight: Promise<void> | undefined;
  private again = false;
  private source: SaveSource = 'edit';
  /** Unmounted: no form any more, pending edits are still being saved. */
  private disposed = false;
  /** Given up for good: a new editor of the step took over, or the session ended. */
  private abandoned = false;
  /** Writes the step's draft (from `resume` until its work is done). */
  private owned = false;
  private restored = false;
  private readonly backend: EditorBackend;
  private readonly delay: number;
  private readonly onSaved: ((step: Step) => void) | undefined;
  private readonly drafts: DraftStore | undefined;
  private readonly ownerKey: string | symbol;

  constructor(
    initial: Tagged,
    backend: EditorBackend,
    delay = 1200,
    onSaved?: (step: Step) => void,
    drafts?: DraftStore,
  ) {
    this.backend = backend;
    this.delay = delay;
    this.onSaved = onSaved;
    this.drafts = drafts;
    this.ownerKey = drafts?.id ?? Symbol('step-editor');
    this.values = { ...initial.data.values };
    this.base = { ...initial.data.values };
    this.step = initial.data;
    this.etag = tagOf(initial);
    const draft = drafts?.read();
    if (draft) this.restore(draft, initial);
    this.snapshot = this.build();
  }

  /**
   * Brings back the edits of an earlier visit, merged into the server's
   * version as if a save had found a newer one.
   */
  private restore(draft: StepDraft, initial: Tagged) {
    const server = this.base;
    const base = { ...server, ...draft.base };
    const mine = { ...server, ...draft.values };
    const { merged, conflicts } = mergeStep(base, mine, server);
    if (conflicts.length) {
      this.base = base;
      this.values = mine;
      this.conflict = { fields: conflicts, mine, theirs: server, merged, latest: initial };
      this.status = 'conflict';
      this.restored = true;
      return;
    }
    this.values = merged;
    if (changedKeys(this.base, this.values).length) {
      this.status = 'dirty';
      this.restored = true;
    }
  }

  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };

  getSnapshot = () => this.snapshot;

  private build(): EditorSnapshot {
    return {
      values: this.values,
      step: this.step,
      status: this.status,
      dirty: changedKeys(this.base, this.values),
      savedAt: this.savedAt,
      conflict: this.conflict,
      error: this.error,
      fieldErrors: this.fieldErrors,
      restored: this.restored,
    };
  }

  private emit() {
    this.snapshot = this.build();
    this.persist();
    for (const listener of this.listeners) listener();
  }

  /** The draft holds exactly the fields the server does not have yet. */
  private persist() {
    if (!this.drafts || !this.owned) return;
    const keys = changedKeys(this.base, this.values);
    if (keys.length === 0) {
      this.drafts.clear();
      return;
    }
    this.drafts.write({ values: pick(this.values, keys), base: pick(this.base, keys) });
  }

  /** An edit typed by the person; saved after a pause. */
  set(key: string, value: FieldValue) {
    this.values = { ...this.values, [key]: value };
    if (this.status !== 'saving' && this.status !== 'conflict') {
      this.status = changedKeys(this.base, this.values).length ? 'dirty' : this.status;
    }
    this.emit();
    this.schedule(this.delay);
  }

  /** Writes a mentor suggestion into a field and saves it at once, marked as such. */
  async apply(key: string, value: FieldValue) {
    await this.flush();
    if (this.conflict) return;
    this.values = { ...this.values, [key]: value };
    this.source = 'mentor';
    this.emit();
    try {
      await this.save();
    } finally {
      this.source = 'edit';
    }
  }

  private schedule(ms: number) {
    // An unmounted editor still finishes its saves (retries while offline).
    if (this.abandoned) return;
    clearTimeout(this.timer);
    this.timer = setTimeout(() => {
      this.timer = undefined;
      void this.save();
    }, ms);
  }

  /** Saves pending edits now and waits until the server has them. */
  async flush() {
    clearTimeout(this.timer);
    this.timer = undefined;
    while (this.inflight) await this.inflight;
    if (changedKeys(this.base, this.values).length && !this.conflict) await this.save();
  }

  save(): Promise<void> {
    if (this.abandoned) return Promise.resolve();
    if (this.inflight) {
      this.again = true;
      return this.inflight;
    }
    const keys = changedKeys(this.base, this.values);
    if (this.conflict || keys.length === 0) {
      this.settle();
      return Promise.resolve();
    }
    clearTimeout(this.timer);
    this.timer = undefined;
    this.status = 'saving';
    this.emit();
    this.inflight = this.send(keys).finally(() => {
      this.inflight = undefined;
      if (this.again) {
        this.again = false;
        void this.save();
      } else {
        this.settle();
      }
    });
    return this.inflight;
  }

  private async send(keys: string[]) {
    try {
      const saved = await this.backend.save(
        { values: pick(this.values, keys), source: this.source },
        this.etag,
      );
      this.accept(saved);
      this.savedAt = Date.now();
      this.error = undefined;
      this.fieldErrors = [];
      this.status = changedKeys(this.base, this.values).length ? 'dirty' : 'saved';
      this.emit();
      this.onSaved?.(saved.data);
      if (this.status === 'dirty') this.schedule(this.delay);
    } catch (err) {
      if (err instanceof ApiError && err.status === 412) {
        await this.merge();
        return;
      }
      if (err instanceof NetworkError) {
        this.status = 'offline';
        this.emit();
        this.schedule(5_000);
        return;
      }
      this.status = 'error';
      this.error = err;
      this.fieldErrors = err instanceof ApiError ? err.fieldErrors : [];
      this.emit();
    }
  }

  /** The server's version becomes the base; local edits stay. */
  private accept(tagged: Tagged) {
    this.base = { ...tagged.data.values };
    this.step = tagged.data;
    this.etag = tagOf(tagged);
  }

  /** Someone saved meanwhile: merge by field, then save again or ask. */
  private async merge() {
    let latest: Tagged;
    try {
      latest = await this.backend.load();
    } catch (err) {
      this.status = err instanceof NetworkError ? 'offline' : 'error';
      this.error = err;
      this.emit();
      if (err instanceof NetworkError) this.schedule(5_000);
      return;
    }
    const mine = this.values;
    const { merged, conflicts } = mergeStep(this.base, mine, latest.data.values);
    if (conflicts.length) {
      this.conflict = { fields: conflicts, mine, theirs: latest.data.values, merged, latest };
      this.status = 'conflict';
      this.emit();
      return;
    }
    this.accept(latest);
    this.values = merged;
    this.emit();
    this.again = true;
  }

  /** The person's choice for every conflicting field; then save. */
  resolve(choices: Record<string, Resolution>) {
    const conflict = this.conflict;
    if (!conflict) return Promise.resolve();
    this.values = resolveConflicts(conflict.merged, conflict.theirs, choices);
    this.accept(conflict.latest);
    this.conflict = undefined;
    this.status = changedKeys(this.base, this.values).length ? 'dirty' : 'saved';
    this.emit();
    return this.save();
  }

  /**
   * A newer version exists (seen through presence). Without own unsaved edits
   * it is loaded quietly; with them the merge happens on the next save.
   */
  async refresh() {
    if (this.busy()) return;
    try {
      const latest = await this.backend.load();
      // The person may have typed or saved while the step was loading.
      if (this.busy()) return;
      this.accept(latest);
      this.values = { ...latest.data.values };
      this.emit();
    } catch {
      // The next presence report tries again.
    }
  }

  private busy(): boolean {
    return (
      this.inflight !== undefined ||
      this.conflict !== undefined ||
      changedKeys(this.base, this.values).length > 0
    );
  }

  /** A step returned by another action (check, complete, approve, restore). */
  replace(tagged: Tagged) {
    const { merged } = mergeStep(this.base, this.values, tagged.data.values);
    this.accept(tagged);
    this.values = merged;
    this.emit();
  }

  get version() {
    return this.step.version;
  }

  get currentEtag() {
    return this.etag;
  }

  /**
   * Arms the editor once its form is on screen (again after `dispose`: React
   * may unmount and remount effects). It owns the step's draft from now on,
   * and restored edits are saved.
   */
  resume() {
    if (this.abandoned) return;
    this.disposed = false;
    this.owned = true;
    claimDraft(this.ownerKey, this);
    this.persist();
    if (!this.conflict && changedKeys(this.base, this.values).length) this.schedule(this.delay);
  }

  /**
   * The form is gone: unsaved edits are sent once more and retried in the
   * background while offline. Whatever does not reach the server stays in
   * the draft for the next visit.
   */
  dispose() {
    clearTimeout(this.timer);
    this.timer = undefined;
    if (!this.disposed && !this.conflict && changedKeys(this.base, this.values).length) {
      void this.save();
    }
    this.disposed = true;
    this.settle();
  }

  /** Unsaved, saving, offline or conflicting edits the server does not have. */
  hasUnsaved(): boolean {
    return !this.abandoned && this.busy();
  }

  abandon() {
    this.abandoned = true;
    this.disposed = true;
    this.owned = false;
    clearTimeout(this.timer);
    this.timer = undefined;
  }

  /** An unmounted editor with nothing left to do lets go of the draft. */
  private settle() {
    if (!this.disposed || this.inflight !== undefined || this.timer !== undefined) return;
    this.owned = false;
    releaseDraft(this.ownerKey, this);
  }
}

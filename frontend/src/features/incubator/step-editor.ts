import { ApiError, NetworkError, type FieldError } from '@/shared/api/client';

import type { Step } from './api';
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
 */
export class StepEditor {
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
  private disposed = false;
  private readonly backend: EditorBackend;
  private readonly delay: number;
  private readonly onSaved: ((step: Step) => void) | undefined;

  constructor(
    initial: Tagged,
    backend: EditorBackend,
    delay = 1200,
    onSaved?: (step: Step) => void,
  ) {
    this.backend = backend;
    this.delay = delay;
    this.onSaved = onSaved;
    this.values = { ...initial.data.values };
    this.base = { ...initial.data.values };
    this.step = initial.data;
    this.etag = tagOf(initial);
    this.snapshot = this.build();
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
    };
  }

  private emit() {
    this.snapshot = this.build();
    for (const listener of this.listeners) listener();
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
    if (this.disposed) return;
    clearTimeout(this.timer);
    this.timer = setTimeout(() => {
      void this.save();
    }, ms);
  }

  /** Saves pending edits now and waits until the server has them. */
  async flush() {
    clearTimeout(this.timer);
    while (this.inflight) await this.inflight;
    if (changedKeys(this.base, this.values).length && !this.conflict) await this.save();
  }

  save(): Promise<void> {
    if (this.inflight) {
      this.again = true;
      return this.inflight;
    }
    if (this.conflict) return Promise.resolve();
    const keys = changedKeys(this.base, this.values);
    if (keys.length === 0) return Promise.resolve();
    clearTimeout(this.timer);
    this.status = 'saving';
    this.emit();
    this.inflight = this.send(keys).finally(() => {
      this.inflight = undefined;
      if (this.again) {
        this.again = false;
        void this.save();
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

  /** Arms the editor again after `dispose` (React may unmount and remount effects). */
  resume() {
    this.disposed = false;
  }

  /** Stops timers; unsaved edits are sent one last time. */
  dispose() {
    clearTimeout(this.timer);
    if (!this.disposed && !this.conflict && changedKeys(this.base, this.values).length) {
      void this.save();
    }
    this.disposed = true;
  }
}

import type { StepValues } from './values';

/** Unsaved answers of a step, kept until the server has them. */
export interface StepDraft {
  /** The person's values of the changed fields. */
  values: StepValues;
  /** What the server had in those fields when the person changed them. */
  base: StepValues;
}

/** Where the editor of one step keeps its unsaved answers. */
export interface DraftStore {
  /** Identifies the step (and the person) the draft belongs to. */
  readonly id: string;
  read: () => StepDraft | undefined;
  write: (draft: StepDraft) => void;
  clear: () => void;
}

const PREFIX = 'qadam.draft.';

function session(): Storage | undefined {
  try {
    return window.sessionStorage;
  } catch {
    return undefined;
  }
}

function isValues(value: unknown): value is StepValues {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isDraft(value: unknown): value is StepDraft {
  return isValues(value) && isValues(value.values) && isValues(value.base);
}

/**
 * Drafts in sessionStorage: they survive a reload, a lost connection and
 * leaving the step, and disappear with the tab. Keyed by the person, so that
 * another user of the computer never gets them.
 */
export function sessionDraftStore(userId: string, projectId: string, stepKey: string): DraftStore {
  const id = `${PREFIX}${userId}:${projectId}:${stepKey}`;
  return {
    id,
    read: () => {
      try {
        const raw = session()?.getItem(id);
        if (!raw) return undefined;
        const parsed: unknown = JSON.parse(raw);
        return isDraft(parsed) ? parsed : undefined;
      } catch {
        return undefined;
      }
    },
    write: (draft) => {
      try {
        session()?.setItem(id, JSON.stringify(draft));
      } catch {
        // Full or disabled storage: the edits still go to the server.
      }
    },
    clear: () => {
      try {
        session()?.removeItem(id);
      } catch {
        // Nothing to clear.
      }
    },
  };
}

/** Removes the drafts of everyone but `userId` (another person signed in here). */
export function discardDraftsExcept(userId: string | null): void {
  const storage = session();
  if (!storage) return;
  const own = userId === null ? undefined : `${PREFIX}${userId}:`;
  try {
    for (let i = storage.length - 1; i >= 0; i--) {
      const key = storage.key(i);
      if (key?.startsWith(PREFIX) && !(own && key.startsWith(own))) storage.removeItem(key);
    }
  } catch {
    // Storage went away meanwhile.
  }
}

/** An editor that may still have answers the server does not have. */
export interface PendingEditor {
  hasUnsaved: () => boolean;
  /** Stops for good: no more saves, no more draft writes. */
  abandon: () => void;
}

const owners = new Map<string | symbol, PendingEditor>();
let guarding = false;

/** Leaving the page (closing the tab, reloading) asks first while edits are not saved. */
function guardUnload() {
  if (guarding || typeof window === 'undefined') return;
  guarding = true;
  window.addEventListener('beforeunload', (event) => {
    if (hasUnsavedEdits()) event.preventDefault();
  });
}

/**
 * The editor becomes the one writing this step's draft; an earlier editor
 * of the same step still saving in the background gives way to it.
 */
export function claimDraft(key: string | symbol, editor: PendingEditor): void {
  const previous = owners.get(key);
  if (previous && previous !== editor) previous.abandon();
  owners.set(key, editor);
  guardUnload();
}

export function releaseDraft(key: string | symbol, editor: PendingEditor): void {
  if (owners.get(key) === editor) owners.delete(key);
}

/** True while an open or a background editor has unsaved, saving or offline edits. */
export function hasUnsavedEdits(): boolean {
  for (const editor of owners.values()) if (editor.hasUnsaved()) return true;
  return false;
}

/** Stops every editor (the session ended); their drafts stay for the same person. */
export function abandonEditors(): void {
  for (const editor of owners.values()) editor.abandon();
  owners.clear();
}

import { sameValue, type StepValues } from './values';

export interface MergeResult {
  /** The values to keep editing: others' changes taken, own changes kept. */
  merged: StepValues;
  /** Fields both sides changed differently since the common version. */
  conflicts: string[];
}

/**
 * Three-way merge of a step by field. `base` is the version both sides
 * started from, `mine` the local edits, `theirs` the version on the server.
 */
export function mergeStep(base: StepValues, mine: StepValues, theirs: StepValues): MergeResult {
  const keys = new Set([...Object.keys(base), ...Object.keys(mine), ...Object.keys(theirs)]);
  const merged: StepValues = {};
  const conflicts: string[] = [];
  for (const key of [...keys].sort()) {
    const b = base[key];
    const m = mine[key];
    const t = theirs[key];
    if (sameValue(m, b) || sameValue(m, t)) {
      merged[key] = t;
    } else if (sameValue(t, b)) {
      merged[key] = m;
    } else {
      // Keep the local value until the person decides.
      merged[key] = m;
      conflicts.push(key);
    }
  }
  return { merged, conflicts };
}

export type Resolution = 'mine' | 'theirs';

/** Applies the person's choice for every conflicting field. */
export function resolveConflicts(
  merged: StepValues,
  theirs: StepValues,
  choices: Record<string, Resolution>,
): StepValues {
  const out: StepValues = { ...merged };
  for (const [key, choice] of Object.entries(choices)) {
    if (choice === 'theirs') out[key] = theirs[key];
  }
  return out;
}

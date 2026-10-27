import type { Activity } from '@/features/projects/api';

/** An activity entry; consecutive edits of one step by one member are folded. */
export interface ActivityGroup extends Omit<Activity, 'field_key'> {
  fields: string[];
}

// Edits further apart than this read as separate sessions.
const SESSION_MS = 15 * 60 * 1000;

/** Folds runs of edits by the same member in the same step (newest first). */
export function groupActivity(items: Activity[]): ActivityGroup[] {
  const out: ActivityGroup[] = [];
  for (const item of items) {
    const last = out.at(-1);
    const sameRun =
      last !== undefined &&
      item.kind === 'edit' &&
      last.kind === 'edit' &&
      last.step_key === item.step_key &&
      last.actor?.id === item.actor?.id &&
      Date.parse(last.at) - Date.parse(item.at) < SESSION_MS;
    if (sameRun) {
      if (item.field_key && !last.fields.includes(item.field_key)) last.fields.push(item.field_key);
      continue;
    }
    const { field_key: field, ...rest } = item;
    out.push({ ...rest, fields: field ? [field] : [] });
  }
  return out;
}

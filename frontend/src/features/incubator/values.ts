import type { components } from '@/shared/api/schema.gen';

export type TemplateField = components['schemas']['TemplateField'];
export type StepValues = components['schemas']['StepValues'];

/** A table row: column key → text or number. */
export type Row = Record<string, string | number | null>;

/** Any answer a field can hold. */
export type FieldValue = string | number | null | string[] | Row[] | undefined;

/** Reads a field's value from loosely typed step values. */
export function readValue(values: StepValues, key: string): FieldValue {
  return values[key] as FieldValue;
}

export function textValue(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

export function numberValue(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null;
}

export function listValue(value: unknown): string[] {
  return Array.isArray(value) ? value.map((item) => (typeof item === 'string' ? item : '')) : [];
}

export function rowsValue(value: unknown): Row[] {
  if (!Array.isArray(value)) return [];
  return value.map((row) =>
    row !== null && typeof row === 'object' && !Array.isArray(row) ? (row as Row) : {},
  );
}

/** Blank answers are equal whatever their shape: a field never touched equals a cleared one. */
export function isEmptyValue(value: unknown): boolean {
  if (value === undefined || value === null) return true;
  if (typeof value === 'string') return value.trim() === '';
  if (Array.isArray(value)) return value.length === 0;
  if (typeof value === 'object') return Object.keys(value).length === 0;
  return false;
}

/**
 * JSON with sorted object keys, so equal rows compare equal whatever the key
 * order. Texts compare as the server stores them: trimmed, with LF line ends.
 */
function stable(value: unknown): string {
  if (typeof value === 'string') return JSON.stringify(value.replace(/\r\n?/g, '\n').trim());
  if (Array.isArray(value)) return `[${value.map(stable).join(',')}]`;
  if (value !== null && typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>)
      .filter(([, v]) => v !== undefined)
      .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0));
    return `{${entries.map(([k, v]) => `${JSON.stringify(k)}:${stable(v)}`).join(',')}}`;
  }
  return JSON.stringify(value ?? null);
}

export function sameValue(a: unknown, b: unknown): boolean {
  if (isEmptyValue(a) && isEmptyValue(b)) return true;
  return stable(a) === stable(b);
}

/** Keys whose value differs between two versions of a step. */
export function changedKeys(from: StepValues, to: StepValues): string[] {
  const keys = new Set([...Object.keys(from), ...Object.keys(to)]);
  return [...keys].filter((key) => !sameValue(from[key], to[key])).sort();
}

export function pick(values: StepValues, keys: readonly string[]): StepValues {
  const out: StepValues = {};
  for (const key of keys) out[key] = values[key] ?? null;
  return out;
}

/** Plain text of an answer, for diffs and previews. */
export function valueToText(value: unknown, field?: TemplateField): string {
  if (value === undefined || value === null) return '';
  if (typeof value === 'string') return value;
  if (typeof value === 'number') return String(value);
  if (Array.isArray(value)) {
    return value
      .map((item) => {
        if (typeof item === 'string') return item;
        if (item && typeof item === 'object') {
          const row = item as Row;
          const columns = field?.columns?.map((c) => c.key) ?? Object.keys(row);
          return columns
            .map((key) => row[key])
            .filter((v) => v !== undefined && v !== null && v !== '')
            .join(' · ');
        }
        return '';
      })
      .join('\n');
  }
  return '';
}

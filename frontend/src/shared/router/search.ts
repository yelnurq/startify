/** Helpers for validating URL search params: unknown input, typed output. */

export function optionalString(value: unknown, max = 512): string | undefined {
  return typeof value === 'string' && value.length > 0 && value.length <= max ? value : undefined;
}

export function optionalFlag(value: unknown): boolean | undefined {
  if (value === true || value === 'true' || value === '1' || value === 1) return true;
  return undefined;
}

/** Drops undefined keys so that the typed search stays optional for links. */
export function compact<T extends Record<string, unknown>>(input: T): Partial<T> {
  const out: Partial<T> = {};
  for (const [key, value] of Object.entries(input)) {
    if (value !== undefined) (out as Record<string, unknown>)[key] = value;
  }
  return out;
}

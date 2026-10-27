/**
 * True when an item that was still in progress before a refresh is done
 * after it: the moment to refresh what shows its result elsewhere.
 */
export function finishedSince<T extends { id: string }>(
  before: readonly T[] | undefined,
  after: readonly T[],
  inProgress: (item: T) => boolean,
): boolean {
  if (!before) return false;
  const running = new Set(before.filter(inProgress).map((item) => item.id));
  return after.some((item) => running.has(item.id) && !inProgress(item));
}

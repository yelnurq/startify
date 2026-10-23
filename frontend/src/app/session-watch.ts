import { hashKey, type QueryClient, type QueryKey } from '@tanstack/react-query';

import { sessionQuery, type Session } from '@/features/auth/api';

/** Queries that hold nothing of the signed-in person: they survive a change of user. */
const SHARED_ROOTS = new Set<unknown>(['session', 'tenant']);

function personal(queryKey: QueryKey): boolean {
  return !SHARED_ROOTS.has(queryKey[0]);
}

/**
 * Forgets every cached answer and pending mutation of the signed-in user, so
 * that the next person at this computer sees nothing of theirs (computer
 * classes, shared laptops).
 */
export function forgetUserData(queryClient: QueryClient): void {
  void queryClient.cancelQueries({ predicate: (query) => personal(query.queryKey) });
  queryClient.removeQueries({ predicate: (query) => personal(query.queryKey) });
  queryClient.getMutationCache().clear();
}

function userOf(session: Session | null | undefined): string | null | undefined {
  if (session === undefined) return undefined;
  return session?.user.id ?? null;
}

/**
 * Watches who is signed in. When the user becomes someone else or nobody
 * (sign-in as another person, a lost session), the previous user's data is
 * forgotten. `onUser` hears every resolved change, the first one included.
 * Returns the unsubscribe function.
 */
export function watchSessionUser(
  queryClient: QueryClient,
  onUser?: (user: string | null) => void,
): () => void {
  const hash = hashKey(sessionQuery.queryKey);
  let current = userOf(queryClient.getQueryData(sessionQuery.queryKey));
  if (current !== undefined) onUser?.(current);
  return queryClient.getQueryCache().subscribe((event) => {
    if (event.query.queryHash !== hash) return;
    if (event.type !== 'added' && event.type !== 'updated') return;
    const next = userOf(event.query.state.data as Session | null | undefined);
    if (next === undefined || next === current) return;
    const previous = current;
    current = next;
    if (typeof previous === 'string') forgetUserData(queryClient);
    onUser?.(next);
  });
}

import type { QueryClient } from '@tanstack/react-query';
import { redirect, type ParsedLocation } from '@tanstack/react-router';

import { hasRole, landingAfterSignIn, sessionQuery, type Role, type Session } from './api';

/**
 * The session the guards decide on. A remembered "signed in" is trusted (a
 * lost session shows on the next API call); a remembered "signed out" is
 * checked again, because the person may have signed in in another tab since.
 * Without the session cookie that check sends no request.
 */
function currentSession(queryClient: QueryClient): Promise<Session | null> {
  const signedOut = queryClient.getQueryData(sessionQuery.queryKey) === null;
  return queryClient.query({ ...sessionQuery, staleTime: signedOut ? 0 : 'static' });
}

/** Signed-in users only; visitors go to sign-in and come back afterwards. */
export async function requireSession(
  queryClient: QueryClient,
  location: ParsedLocation,
): Promise<Session> {
  const session = await currentSession(queryClient);
  if (!session) throw redirect({ to: '/login', search: { redirect: location.href } });
  return session;
}

/** Signed-in users who finished onboarding; others onboard first and come back. */
export async function requireOnboarded(
  queryClient: QueryClient,
  location: ParsedLocation,
): Promise<Session> {
  const session = await requireSession(queryClient, location);
  if (!session.membership.onboarded) {
    throw redirect({ to: '/onboarding', search: { redirect: location.href } });
  }
  return session;
}

/** Members holding at least one of the roles; others return to their home page. */
export function requireRole(session: Session, ...roles: Role[]): void {
  if (!hasRole(session, ...roles)) throw redirect({ to: '/app' });
}

/** Sign-in and sign-up pages are pointless for a signed-in user. */
export async function redirectIfSignedIn(queryClient: QueryClient, target?: string): Promise<void> {
  const session = await currentSession(queryClient);
  if (session) throw redirect({ href: landingAfterSignIn(session, target) });
}

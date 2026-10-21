import type { QueryClient } from '@tanstack/react-query';
import { redirect, type ParsedLocation } from '@tanstack/react-router';

import { hasRole, landingAfterSignIn, sessionQuery, type Role, type Session } from './api';

/** Signed-in users only; visitors go to sign-in and come back afterwards. */
export async function requireSession(
  queryClient: QueryClient,
  location: ParsedLocation,
): Promise<Session> {
  const session = await queryClient.query({ ...sessionQuery, staleTime: 'static' });
  if (!session) throw redirect({ to: '/login', search: { redirect: location.href } });
  return session;
}

/** Signed-in users who finished onboarding. */
export async function requireOnboarded(
  queryClient: QueryClient,
  location: ParsedLocation,
): Promise<Session> {
  const session = await requireSession(queryClient, location);
  if (!session.membership.onboarded) throw redirect({ to: '/onboarding' });
  return session;
}

/** Members holding at least one of the roles; others return to their home page. */
export function requireRole(session: Session, ...roles: Role[]): void {
  if (!hasRole(session, ...roles)) throw redirect({ to: '/app' });
}

/** Sign-in and sign-up pages are pointless for a signed-in user. */
export async function redirectIfSignedIn(queryClient: QueryClient, target?: string): Promise<void> {
  const session = await queryClient.query({ ...sessionQuery, staleTime: 'static' });
  if (session) throw redirect({ href: landingAfterSignIn(session, target) });
}

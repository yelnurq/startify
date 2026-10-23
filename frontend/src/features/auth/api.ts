import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query';

import { api, readCookie, unwrap } from '@/shared/api/client';
import { isUnauthenticated } from '@/shared/api/errors';
import type { components } from '@/shared/api/schema.gen';

export type Session = components['schemas']['Session'];
export type VerificationSent = components['schemas']['VerificationSent'];
export type Tenant = components['schemas']['Tenant'];
export type Role = components['schemas']['Role'];
export type Locale = components['schemas']['Locale'];

export const tenantQuery = queryOptions({
  queryKey: ['tenant'],
  queryFn: () => unwrap(api.GET('/tenant')),
  staleTime: 10 * 60_000,
});

/** The signed-in session, or null for visitors. */
export const sessionQuery = queryOptions({
  queryKey: ['session'],
  queryFn: async (): Promise<Session | null> => {
    // The CSRF cookie lives as long as the session: without it there is none to
    // ask about (and visitors of public pages get no 401 in the console).
    if (readCookie('qadam_csrf') === undefined) return null;
    try {
      return await unwrap(api.GET('/auth/session'));
    } catch (error) {
      if (isUnauthenticated(error)) return null;
      throw error;
    }
  },
  staleTime: 5 * 60_000,
});

export function useStoreSession() {
  const queryClient = useQueryClient();
  return (session: Session) => {
    queryClient.setQueryData(sessionQuery.queryKey, session);
  };
}

/**
 * A registration signs in only an existing account with a verified e-mail;
 * a new one waits until the link from the letter is opened.
 */
export function isSession(result: Session | VerificationSent): result is Session {
  return 'user' in result;
}

export function useSignIn() {
  const store = useStoreSession();
  return useMutation({
    mutationFn: (body: components['schemas']['LoginRequest']) =>
      unwrap(api.POST('/auth/login', { body })),
    onSuccess: store,
  });
}

export function useRegister() {
  const store = useStoreSession();
  return useMutation({
    mutationFn: (body: components['schemas']['RegisterRequest']) =>
      unwrap(
        api.POST('/auth/register', {
          body,
          headers: { 'Idempotency-Key': crypto.randomUUID() },
        }),
      ),
    onSuccess: (result) => {
      if (isSession(result)) store(result);
    },
  });
}

export function useRequestLoginCode() {
  return useMutation({
    mutationFn: (email: string) => unwrap(api.POST('/auth/otp/request', { body: { email } })),
  });
}

export function useVerifyLoginCode() {
  const store = useStoreSession();
  return useMutation({
    mutationFn: (body: components['schemas']['LoginCodeRequest']) =>
      unwrap(api.POST('/auth/otp/verify', { body })),
    onSuccess: store,
  });
}

export function useForgotPassword() {
  return useMutation({
    mutationFn: (email: string) => unwrap(api.POST('/auth/password/forgot', { body: { email } })),
  });
}

export function useResetPassword() {
  return useMutation({
    mutationFn: (body: components['schemas']['PasswordResetRequest']) =>
      unwrap(api.POST('/auth/password/reset', { body })),
  });
}

/**
 * Sends the verification letter again. The address is always given: before
 * the e-mail is verified there is no session to tell whose it is.
 */
export function useResendVerification() {
  return useMutation({
    mutationFn: (email: string) => unwrap(api.POST('/auth/email/resend', { body: { email } })),
  });
}

/** Opens the link from the letter: confirms the address and signs in. */
export function verifyEmail(token: string): Promise<Session> {
  return unwrap(api.POST('/auth/email/verify', { body: { token } }));
}

/** Signs out and forgets every cached response of the previous user. */
export function useSignOut() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => unwrap(api.POST('/auth/logout')),
    onSettled: () => {
      queryClient.clear();
      queryClient.setQueryData(sessionQuery.queryKey, null);
    },
  });
}

export function hasRole(session: Session | null | undefined, ...roles: Role[]): boolean {
  return Boolean(session && roles.some((role) => session.membership.roles.includes(role)));
}

const PROBE_ORIGIN = 'https://qadam.invalid';

/** Control characters and backslashes: browsers drop or reinterpret them in addresses. */
function hasUnsafeCharacter(value: string): boolean {
  for (const ch of value) {
    const code = ch.codePointAt(0) ?? 0;
    if (ch === '\\' || code < 0x20 || (code >= 0x7f && code <= 0x9f)) return true;
  }
  return false;
}

/**
 * A same-site path to return to after sign-in; anything else is ignored. Only
 * a path that starts with a single slash passes: browsers drop tabs and line
 * breaks ("/\t/evil.example" becomes "//evil.example") and read "\" as "/".
 */
export function safeRedirect(target: unknown): string | undefined {
  if (typeof target !== 'string' || hasUnsafeCharacter(target)) return undefined;
  if (!target.startsWith('/') || target.startsWith('//')) return undefined;
  let url: URL;
  try {
    url = new URL(target, PROBE_ORIGIN);
  } catch {
    return undefined;
  }
  if (url.origin !== PROBE_ORIGIN || url.pathname.startsWith('//')) return undefined;
  return target;
}

/** Where a freshly signed-in user should land. */
export function landingAfterSignIn(session: Session, redirect?: string): string {
  if (!session.membership.onboarded) return '/onboarding';
  return safeRedirect(redirect) ?? '/app';
}

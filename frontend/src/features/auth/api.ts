import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query';

import { api, unwrap } from '@/shared/api/client';
import { isUnauthenticated } from '@/shared/api/errors';
import type { components } from '@/shared/api/schema.gen';

export type Session = components['schemas']['Session'];
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
    try {
      return await unwrap(api.GET('/auth/session'));
    } catch (error) {
      if (isUnauthenticated(error)) return null;
      throw error;
    }
  },
  staleTime: 5 * 60_000,
});

function useStoreSession() {
  const queryClient = useQueryClient();
  return (session: Session) => {
    queryClient.setQueryData(sessionQuery.queryKey, session);
  };
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
    onSuccess: store,
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

export function useResendVerification() {
  return useMutation({
    mutationFn: () => unwrap(api.POST('/auth/email/resend')),
  });
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

/** A same-site path to return to after sign-in; anything else is ignored. */
export function safeRedirect(target: unknown): string | undefined {
  if (typeof target !== 'string') return undefined;
  if (!target.startsWith('/') || target.startsWith('//') || target.startsWith('/\\'))
    return undefined;
  return target;
}

/** Where a freshly signed-in user should land. */
export function landingAfterSignIn(session: Session, redirect?: string): string {
  if (!session.membership.onboarded) return '/onboarding';
  return safeRedirect(redirect) ?? '/app';
}

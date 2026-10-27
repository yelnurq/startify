import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query';

import { api, unwrap } from '@/shared/api/client';
import type { components } from '@/shared/api/schema.gen';

export type SessionInfo = components['schemas']['SessionInfo'];

export const sessionsQuery = queryOptions({
  queryKey: ['auth', 'sessions'],
  queryFn: async () => (await unwrap(api.GET('/auth/sessions'))).items,
});

export function useRevokeSession() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (sessionId: string) =>
      unwrap(api.DELETE('/auth/sessions/{sessionId}', { params: { path: { sessionId } } })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: sessionsQuery.queryKey }),
  });
}

/** Ends every session except this one. */
export function useRevokeOtherSessions() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => unwrap(api.DELETE('/auth/sessions')),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: sessionsQuery.queryKey }),
  });
}

export function useChangePassword() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: components['schemas']['PasswordChangeRequest']) =>
      unwrap(api.PUT('/auth/password', { body })),
    // Other sessions are revoked by the server.
    onSuccess: () => queryClient.invalidateQueries({ queryKey: sessionsQuery.queryKey }),
  });
}

export interface AgentInfo {
  browser: string | null;
  os: string | null;
  mobile: boolean;
}

const BROWSERS: [RegExp, string][] = [
  [/Edg\/(\d+)/, 'Edge'],
  [/OPR\/(\d+)/, 'Opera'],
  [/YaBrowser\/(\d+)/, 'Yandex Browser'],
  [/Firefox\/(\d+)/, 'Firefox'],
  [/Chrome\/(\d+)/, 'Chrome'],
  [/Version\/(\d+).*Safari/, 'Safari'],
];

const SYSTEMS: [RegExp, string][] = [
  [/Windows NT/, 'Windows'],
  [/Android/, 'Android'],
  [/iPhone|iPad|iPod/, 'iOS'],
  [/Mac OS X/, 'macOS'],
  [/CrOS/, 'ChromeOS'],
  [/Linux/, 'Linux'],
];

/** A short, human description of a User-Agent string. */
export function describeAgent(userAgent: string | null): AgentInfo {
  if (!userAgent) return { browser: null, os: null, mobile: false };
  let browser: string | null = null;
  for (const [pattern, name] of BROWSERS) {
    const match = pattern.exec(userAgent);
    if (match) {
      browser = match[1] ? `${name} ${match[1]}` : name;
      break;
    }
  }
  const os = SYSTEMS.find(([pattern]) => pattern.test(userAgent))?.[1] ?? null;
  return { browser, os, mobile: /Mobi|Android|iPhone|iPad/.test(userAgent) };
}

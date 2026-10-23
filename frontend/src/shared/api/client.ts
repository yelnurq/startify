import createClient from 'openapi-fetch';

import { i18n } from '@/shared/i18n';

import type { components, paths } from './schema.gen';

export type Problem = components['schemas']['Problem'];
export type FieldError = components['schemas']['FieldError'];

/** A failed API call, carrying the RFC 9457 problem returned by the server. */
export class ApiError extends Error {
  readonly problem: Problem;
  /** Seconds from the Retry-After header of 429/503 responses. */
  readonly retryAfter: number | undefined;

  constructor(problem: Problem, retryAfter?: number) {
    super(problem.detail ?? problem.title);
    this.name = 'ApiError';
    this.problem = problem;
    this.retryAfter = retryAfter;
  }

  get code(): string {
    return this.problem.code;
  }

  get status(): number {
    return this.problem.status;
  }

  get fieldErrors(): FieldError[] {
    return this.problem.errors ?? [];
  }
}

/** Raised when the server could not be reached at all. */
export class NetworkError extends Error {
  constructor(cause: unknown) {
    super('network request failed', { cause });
    this.name = 'NetworkError';
  }
}

const API_PREFIX = '/api/v1';
const CSRF_COOKIE = 'qadam_csrf';
const UNSAFE_METHODS = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);
const REFRESHED_AT_KEY = 'qadam.auth.refreshedAt';

// Public auth endpoints answer 401 for wrong credentials, not for an expired
// session: refreshing and retrying them would be wrong.
const NO_REFRESH = new Set([
  '/auth/login',
  '/auth/directory/login',
  '/auth/register',
  '/auth/otp/request',
  '/auth/otp/verify',
  '/auth/refresh',
  '/auth/logout',
  '/auth/password/forgot',
  '/auth/password/reset',
  '/auth/email/verify',
]);

export function readCookie(name: string): string | undefined {
  const prefix = `${name}=`;
  for (const part of document.cookie.split('; ')) {
    if (part.startsWith(prefix)) return decodeURIComponent(part.slice(prefix.length));
  }
  return undefined;
}

function withCsrf(request: Request): Request {
  if (!UNSAFE_METHODS.has(request.method)) return request;
  const token = readCookie(CSRF_COOKIE);
  if (token) request.headers.set('X-CSRF-Token', token);
  return request;
}

function apiPath(request: Request): string {
  const { pathname } = new URL(request.url);
  return pathname.startsWith(API_PREFIX) ? pathname.slice(API_PREFIX.length) : pathname;
}

type UnauthenticatedHandler = () => void;
let onUnauthenticated: UnauthenticatedHandler = () => undefined;

/** The app decides what a lost session means (clear state, go to sign-in). */
export function setUnauthenticatedHandler(handler: UnauthenticatedHandler): void {
  onUnauthenticated = handler;
}

let inflightRefresh: Promise<boolean> | null = null;

async function performRefresh(): Promise<boolean> {
  // Another tab may have just rotated the tokens: cookies are shared, so the
  // retry will already carry fresh ones.
  try {
    const at = Number(localStorage.getItem(REFRESHED_AT_KEY) ?? 0);
    if (Date.now() - at < 5_000) return true;
  } catch {
    // Storage unavailable: refresh unconditionally.
  }
  const request = withCsrf(
    new Request(`${globalThis.location.origin}${API_PREFIX}/auth/refresh`, {
      method: 'POST',
      credentials: 'same-origin',
    }),
  );
  const response = await globalThis.fetch(request);
  if (response.ok) {
    try {
      localStorage.setItem(REFRESHED_AT_KEY, String(Date.now()));
    } catch {
      // Not persisted; harmless.
    }
  }
  return response.ok;
}

/**
 * Rotates the session tokens once, however many requests ask for it at the
 * same time; across tabs the Web Locks API serialises the refresh.
 */
export function refreshSession(): Promise<boolean> {
  inflightRefresh ??= (async () => {
    try {
      if (typeof navigator !== 'undefined' && 'locks' in navigator) {
        return await navigator.locks.request('qadam-auth-refresh', performRefresh);
      }
      return await performRefresh();
    } catch {
      return false;
    } finally {
      inflightRefresh = null;
    }
  })();
  return inflightRefresh;
}

/** fetch with the CSRF header and a single refresh-and-retry on an expired session. */
async function fetchWithSession(request: Request): Promise<Response> {
  const retry = request.clone();
  const response = await globalThis.fetch(withCsrf(request));
  if (response.status !== 401 || NO_REFRESH.has(apiPath(request))) return response;

  // Without the CSRF cookie there was never a session in this browser (or it
  // was signed out): nothing to refresh.
  const refreshed = readCookie(CSRF_COOKIE) !== undefined && (await refreshSession());
  if (!refreshed) {
    onUnauthenticated();
    return response;
  }
  const second = await globalThis.fetch(withCsrf(retry));
  if (second.status === 401) onUnauthenticated();
  return second;
}

export const api = createClient<paths>({
  // Absolute base URL: the Fetch API outside browsers (tests, SSR) rejects relative ones.
  baseUrl: `${globalThis.location.origin}${API_PREFIX}`,
  // Resolved at call time so that instrumentation installed later (MSW) applies.
  fetch: (request) => fetchWithSession(request),
});

api.use({
  onRequest({ request }) {
    request.headers.set('Accept-Language', i18n.language);
    return request;
  },
});

/**
 * Raw fetch of an API path with the same session handling as `api`, for
 * responses openapi-fetch cannot represent (event streams).
 */
export function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const request = new Request(`${globalThis.location.origin}${API_PREFIX}${path}`, {
    credentials: 'same-origin',
    ...init,
  });
  request.headers.set('Accept-Language', i18n.language);
  return fetchWithSession(request);
}

function isProblem(value: unknown): value is Problem {
  return (
    typeof value === 'object' &&
    value !== null &&
    'code' in value &&
    'status' in value &&
    typeof (value as { code: unknown }).code === 'string'
  );
}

interface Outcome<T> {
  data?: T;
  error?: unknown;
  response: Response;
}

function retryAfter(response: Response): number | undefined {
  const value = Number(response.headers.get('Retry-After'));
  return Number.isFinite(value) && value > 0 ? value : undefined;
}

/**
 * Resolves an openapi-fetch call to its data, throwing ApiError for problem
 * responses and NetworkError when the request never reached the server.
 */
export async function unwrap<T>(call: Promise<Outcome<T>>): Promise<T> {
  let outcome: Outcome<T>;
  try {
    outcome = await call;
  } catch (err) {
    throw new NetworkError(err);
  }
  const { data, error, response } = outcome;
  if (!response.ok || error !== undefined) {
    if (isProblem(error)) throw new ApiError(error, retryAfter(response));
    throw new ApiError(
      {
        type: 'about:blank',
        title: response.statusText || 'Error',
        status: response.status,
        code: response.status >= 500 ? 'internal' : 'request.failed',
      },
      retryAfter(response),
    );
  }
  return data as T;
}

/** Like `unwrap`, also returning the ETag of the response for a later If-Match. */
export async function unwrapTagged<T>(
  call: Promise<Outcome<T>>,
): Promise<{ data: T; etag: string | undefined }> {
  let response: Response | undefined;
  const data = await unwrap(
    call.then((outcome) => {
      response = outcome.response;
      return outcome;
    }),
  );
  return { data, etag: response?.headers.get('ETag') ?? undefined };
}

/** Throws the ApiError of a failed raw response (see `apiFetch`). */
export async function throwProblem(response: Response): Promise<never> {
  const error: unknown = await response.json().catch(() => undefined);
  await unwrap(Promise.resolve({ error: error ?? null, response }));
  throw new ApiError({
    type: 'about:blank',
    title: 'Error',
    status: response.status,
    code: 'internal',
  });
}

import createClient from 'openapi-fetch';

import { i18n } from '@/shared/i18n';

import type { components, paths } from './schema.gen';

export type Problem = components['schemas']['Problem'];

/** A failed API call, carrying the RFC 9457 problem returned by the server. */
export class ApiError extends Error {
  readonly problem: Problem;

  constructor(problem: Problem) {
    super(problem.detail ?? problem.title);
    this.name = 'ApiError';
    this.problem = problem;
  }

  get code(): string {
    return this.problem.code;
  }

  get status(): number {
    return this.problem.status;
  }
}

/** Raised when the server could not be reached at all. */
export class NetworkError extends Error {
  constructor(cause: unknown) {
    super('network request failed', { cause });
    this.name = 'NetworkError';
  }
}

export const api = createClient<paths>({
  // Absolute base URL: the Fetch API outside browsers (tests, SSR) rejects relative ones.
  baseUrl: `${globalThis.location.origin}/api/v1`,
  // Resolve fetch at call time so that instrumentation installed later (MSW) applies.
  fetch: (request) => globalThis.fetch(request),
});

api.use({
  onRequest({ request }) {
    request.headers.set('Accept-Language', i18n.language);
    return request;
  },
});

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
    if (isProblem(error)) throw new ApiError(error);
    throw new ApiError({
      type: 'about:blank',
      title: response.statusText || 'Error',
      status: response.status,
      code: response.status >= 500 ? 'internal' : 'request.failed',
    });
  }
  return data as T;
}

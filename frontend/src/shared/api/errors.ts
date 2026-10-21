import { i18n } from '@/shared/i18n';

import { ApiError, NetworkError } from './client';

/**
 * Localised, user-facing message for any error thrown by the API layer.
 * Problem codes map to `errors.<code>` keys; unknown codes fall back to a
 * generic message rather than leaking server wording.
 */
export function errorMessage(error: unknown): string {
  const t = i18n.t.bind(i18n);
  if (error instanceof NetworkError) return t('common:errors.network');
  if (error instanceof ApiError) {
    const key = `common:errors.${error.code}`;
    const params = {
      ...formatParams(error.problem.params),
      // Placeholders must always be filled, or the raw {{…}} would show.
      retry_after_seconds: error.retryAfter ?? error.problem.params?.retry_after_seconds ?? 60,
    };
    // Codes come from the server at runtime, so the key cannot be checked statically.
    const message = t(key as 'common:errors.internal', { ...params, defaultValue: '' });
    if (message !== '') return message;
    if (error.status === 429) return t('common:errors.rate_limited');
    if (error.status === 403) return t('common:errors.rbac.forbidden');
  }
  return t('common:errors.internal');
}

/** Lists (e.g. unknown column names) read as "a, b, c" in messages. */
function formatParams(params: Record<string, unknown> | undefined): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [name, value] of Object.entries(params ?? {})) {
    out[name] = Array.isArray(value) ? value.join(', ') : value;
  }
  return out;
}

/**
 * Message for a bare problem code passed in the address, e.g. `/login?error=auth.oidc_failed`
 * after an unsuccessful SSO sign-in. Unknown codes read as a failed SSO sign-in.
 */
export function codeMessage(code: string): string {
  const t = i18n.t.bind(i18n);
  const message = /^[a-z_]+\.[a-z_]+$/.test(code)
    ? t(`common:errors.${code}` as 'common:errors.internal', { defaultValue: '' })
    : '';
  return message || t('common:errors.auth.oidc_failed');
}

/** True for errors that mean "not signed in any more". */
export function isUnauthenticated(error: unknown): boolean {
  return error instanceof ApiError && error.status === 401;
}

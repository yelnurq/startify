import type { TFunction } from 'i18next';

import { ApiError, NetworkError } from './client';

/**
 * Localised, user-facing message for any error thrown by the API layer.
 * Problem codes map to `errors.<code>` keys; unknown codes fall back to a
 * generic message rather than leaking server wording.
 */
export function errorMessage(t: TFunction, error: unknown): string {
  if (error instanceof NetworkError) return t('errors.network');
  if (error instanceof ApiError) {
    const key = `errors.${error.code}`;
    // Codes come from the server at runtime, so the key cannot be checked statically.
    const message = t(key as 'errors.internal', { ...error.problem.params, defaultValue: '' });
    if (message !== '') return message;
  }
  return t('errors.internal');
}

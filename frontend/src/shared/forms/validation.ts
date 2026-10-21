import type { FieldValues, Path, UseFormSetError } from 'react-hook-form';
import { z } from 'zod';

import { ApiError, type FieldError } from '@/shared/api/client';
import { i18n } from '@/shared/i18n';

let installed = false;

/**
 * Makes zod speak the interface language. Messages are computed at validation
 * time, so a language switch applies to the next validation.
 */
export function installZodLocalization(): void {
  if (installed) return;
  installed = true;
  z.config({
    customError: (issue) => {
      const t = i18n.t.bind(i18n);
      switch (issue.code) {
        case 'too_small': {
          const minimum = Number(issue.minimum);
          if (issue.origin === 'string') {
            return minimum <= 1
              ? t('common:validation.required')
              : t('common:validation.minLength', { count: minimum });
          }
          if (issue.origin === 'array' || issue.origin === 'set') {
            return t('common:validation.minItems', { count: minimum });
          }
          return t('common:validation.min', { value: minimum });
        }
        case 'too_big': {
          const maximum = Number(issue.maximum);
          if (issue.origin === 'string')
            return t('common:validation.maxLength', { count: maximum });
          if (issue.origin === 'array' || issue.origin === 'set') {
            return t('common:validation.maxItems', { count: maximum });
          }
          return t('common:validation.max', { value: maximum });
        }
        case 'invalid_format':
          if (issue.format === 'email') return t('common:validation.email');
          if (issue.format === 'url') return t('common:validation.url');
          return t('common:validation.format');
        case 'invalid_type':
          return t('common:validation.required');
        case 'custom':
          return undefined;
        default:
          return t('common:validation.invalid');
      }
    },
  });
}

/**
 * Puts the field errors of a 422 response on the form. Returns true when the
 * error was about fields, so the caller does not need a general message.
 */
export function applyFieldErrors<T extends FieldValues>(
  error: unknown,
  setError: UseFormSetError<T>,
  fieldMap: Partial<Record<string, Path<T>>> = {},
): boolean {
  if (!(error instanceof ApiError) || error.fieldErrors.length === 0) return false;
  for (const fe of error.fieldErrors) {
    const field = fieldMap[fe.field] ?? (fe.field as Path<T>);
    setError(field, { type: 'server', message: fieldErrorMessage(fe) });
  }
  return true;
}

/** Localised text of a server-side field error (`common:validation.<code>`). */
export function fieldErrorMessage(fe: FieldError): string {
  // Validation codes come from the server at runtime.
  return i18n.t(`common:validation.${fe.code}` as 'common:validation.invalid', {
    ...fe.params,
    defaultValue: i18n.t('common:validation.invalid'),
  });
}

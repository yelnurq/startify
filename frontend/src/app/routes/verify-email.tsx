import { createFileRoute } from '@tanstack/react-router';

import { VerifyEmailRoute } from '@/features/auth/recovery-pages';
import { loadNamespaces } from '@/shared/i18n';
import { compact, optionalString } from '@/shared/router/search';

export const Route = createFileRoute('/verify-email')({
  staticData: { pageTitle: 'verifyEmail' },
  validateSearch: (search: Record<string, unknown>): { token?: string } =>
    compact({ token: optionalString(search.token, 128) }),
  loader: () => loadNamespaces('auth'),
  component: VerifyEmailRoute,
});

import { createFileRoute } from '@tanstack/react-router';

import { ResetPasswordRoute } from '@/features/auth/recovery-pages';
import { loadNamespaces } from '@/shared/i18n';
import { compact, optionalFlag, optionalString } from '@/shared/router/search';

export const Route = createFileRoute('/reset-password')({
  staticData: { pageTitle: 'resetPassword' },
  validateSearch: (search: Record<string, unknown>): { token?: string; invite?: boolean } =>
    compact({ token: optionalString(search.token, 128), invite: optionalFlag(search.invite) }),
  loader: () => loadNamespaces('auth'),
  component: ResetPasswordRoute,
});

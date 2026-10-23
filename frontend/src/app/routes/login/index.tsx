import { createFileRoute } from '@tanstack/react-router';

import { tenantQuery } from '@/features/auth/api';
import { redirectIfSignedIn } from '@/features/auth/guards';
import { LoginRoute } from '@/features/auth/login-page';
import { loadNamespaces } from '@/shared/i18n';
import { compact, optionalString } from '@/shared/router/search';

export const Route = createFileRoute('/login/')({
  staticData: { pageTitle: 'login' },
  validateSearch: (search: Record<string, unknown>): { redirect?: string; error?: string } =>
    compact({ redirect: optionalString(search.redirect), error: optionalString(search.error, 64) }),
  beforeLoad: ({ context, search }) => redirectIfSignedIn(context.queryClient, search.redirect),
  loader: async ({ context }) => {
    await Promise.all([
      loadNamespaces('auth'),
      context.queryClient.query(tenantQuery).catch(() => undefined),
    ]);
  },
  component: LoginRoute,
});

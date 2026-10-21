import { createFileRoute } from '@tanstack/react-router';

import { redirectIfSignedIn } from '@/features/auth/guards';
import { LoginCodeRoute } from '@/features/auth/login-code-page';
import { loadNamespaces } from '@/shared/i18n';
import { compact, optionalFlag, optionalString } from '@/shared/router/search';

export const Route = createFileRoute('/login/code')({
  validateSearch: (
    search: Record<string, unknown>,
  ): { email?: string; sent?: boolean; redirect?: string } =>
    compact({
      email: optionalString(search.email, 254),
      sent: optionalFlag(search.sent),
      redirect: optionalString(search.redirect),
    }),
  beforeLoad: ({ context, search }) => redirectIfSignedIn(context.queryClient, search.redirect),
  loader: () => loadNamespaces('auth'),
  component: LoginCodeRoute,
});

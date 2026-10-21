import { createFileRoute } from '@tanstack/react-router';

import { tenantQuery } from '@/features/auth/api';
import { redirectIfSignedIn } from '@/features/auth/guards';
import { RegisterPage } from '@/features/auth/register-page';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/register')({
  beforeLoad: ({ context }) => redirectIfSignedIn(context.queryClient),
  loader: async ({ context }) => {
    await Promise.all([
      loadNamespaces('auth'),
      context.queryClient.query(tenantQuery).catch(() => undefined),
    ]);
  },
  component: RegisterPage,
});

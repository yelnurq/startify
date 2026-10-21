import { createFileRoute } from '@tanstack/react-router';

import { requireOnboarded } from '@/features/auth/guards';
import { AppRoute } from '@/features/shell/app-layout';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/app')({
  beforeLoad: async ({ context, location }) => ({
    session: await requireOnboarded(context.queryClient, location),
  }),
  loader: () => loadNamespaces('app'),
  component: AppRoute,
});

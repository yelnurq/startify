import { createFileRoute, redirect } from '@tanstack/react-router';

import { requireSession } from '@/features/auth/guards';
import { OnboardingRoute } from '@/features/onboarding/onboarding-page';
import { facultiesQuery } from '@/features/structure/api';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/onboarding')({
  beforeLoad: async ({ context, location }) => {
    const session = await requireSession(context.queryClient, location);
    if (session.membership.onboarded) throw redirect({ to: '/app' });
    return { session };
  },
  loader: async ({ context }) => {
    await Promise.all([
      loadNamespaces('onboarding'),
      context.queryClient.query(facultiesQuery).catch(() => undefined),
    ]);
  },
  component: OnboardingRoute,
});

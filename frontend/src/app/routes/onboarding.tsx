import { createFileRoute, redirect } from '@tanstack/react-router';

import { landingAfterSignIn } from '@/features/auth/api';
import { requireSession } from '@/features/auth/guards';
import { OnboardingRoute, type OnboardingSearch } from '@/features/onboarding/onboarding-page';
import { facultiesQuery } from '@/features/structure/api';
import { loadNamespaces } from '@/shared/i18n';
import { compact, optionalString } from '@/shared/router/search';

export const Route = createFileRoute('/onboarding')({
  staticData: { pageTitle: 'onboarding' },
  // The page the member was going to before onboarding, opened afterwards.
  validateSearch: (search: Record<string, unknown>): OnboardingSearch =>
    compact({ redirect: optionalString(search.redirect) }),
  beforeLoad: async ({ context, location, search }) => {
    const session = await requireSession(context.queryClient, location);
    if (session.membership.onboarded) {
      throw redirect({ href: landingAfterSignIn(session, search.redirect) });
    }
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

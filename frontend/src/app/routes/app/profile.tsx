import { createFileRoute } from '@tanstack/react-router';

import { meQuery } from '@/features/profile/api';
import { ProfilePage } from '@/features/profile/profile-page';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/app/profile')({
  staticData: { titleKey: 'profile' },
  loader: async ({ context }) => {
    await Promise.all([
      loadNamespaces('onboarding'),
      context.queryClient.query(meQuery).catch(() => undefined),
    ]);
  },
  component: ProfilePage,
});

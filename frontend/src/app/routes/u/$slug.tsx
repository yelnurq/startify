import { createFileRoute } from '@tanstack/react-router';

import { PublicProfileRoute } from '@/features/profile/public-profile-page';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/u/$slug')({
  staticData: { pageTitle: 'member' },
  loader: () => loadNamespaces('app'),
  component: PublicProfileRoute,
});

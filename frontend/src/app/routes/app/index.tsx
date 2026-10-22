import { createFileRoute } from '@tanstack/react-router';

import { DashboardRoute } from '@/features/dashboard/dashboard-page';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/app/')({
  staticData: { titleKey: 'home' },
  loader: () => loadNamespaces('team', 'auth', 'incubator'),
  component: DashboardRoute,
});

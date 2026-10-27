import { createFileRoute } from '@tanstack/react-router';

import { TeamRequestRoute } from '@/features/team/request-page';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/app/team/$requestId')({
  staticData: { titleKey: 'team' },
  loader: () => loadNamespaces('team'),
  component: TeamRequestRoute,
});

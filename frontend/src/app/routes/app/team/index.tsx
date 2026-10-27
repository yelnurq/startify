import { createFileRoute } from '@tanstack/react-router';

import { isTeamRole } from '@/features/team/api';
import { TeamRoute, type TeamSearch } from '@/features/team/team-board-page';
import { loadNamespaces } from '@/shared/i18n';
import { compact, optionalFlag } from '@/shared/router/search';

export const Route = createFileRoute('/app/team/')({
  staticData: { titleKey: 'team' },
  validateSearch: (search: Record<string, unknown>): TeamSearch =>
    compact({
      tab: search.tab === 'mine' ? ('mine' as const) : undefined,
      role: isTeamRole(search.role) ? search.role : undefined,
      new: optionalFlag(search.new),
    }),
  loader: () => loadNamespaces('team'),
  component: TeamRoute,
});

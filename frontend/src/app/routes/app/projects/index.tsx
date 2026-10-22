import { createFileRoute } from '@tanstack/react-router';

import { ProjectsRoute, type ProjectsSearch } from '@/features/projects/projects-page';
import { loadNamespaces } from '@/shared/i18n';
import { compact, optionalFlag } from '@/shared/router/search';

export const Route = createFileRoute('/app/projects/')({
  staticData: { titleKey: 'projects' },
  validateSearch: (search: Record<string, unknown>): ProjectsSearch =>
    compact({
      scope: search.scope === 'all' ? ('all' as const) : undefined,
      new: optionalFlag(search.new),
    }),
  loader: () => loadNamespaces('incubator'),
  component: ProjectsRoute,
});

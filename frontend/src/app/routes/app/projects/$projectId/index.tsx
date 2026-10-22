import { createFileRoute } from '@tanstack/react-router';

import { ProjectOverviewRoute } from '@/features/incubator/workspace';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/app/projects/$projectId/')({
  staticData: { titleKey: 'projects', bleed: true },
  loader: () => loadNamespaces('incubator', 'ai'),
  component: ProjectOverviewRoute,
});

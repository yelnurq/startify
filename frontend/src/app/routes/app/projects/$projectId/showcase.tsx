import { createFileRoute } from '@tanstack/react-router';

import { ProjectShowcaseRoute } from '@/features/incubator/workspace';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/app/projects/$projectId/showcase')({
  staticData: { titleKey: 'projects', bleed: true },
  loader: () => loadNamespaces('incubator', 'evaluation', 'showcase'),
  component: ProjectShowcaseRoute,
});

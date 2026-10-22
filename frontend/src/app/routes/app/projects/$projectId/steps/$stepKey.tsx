import { createFileRoute } from '@tanstack/react-router';

import { ProjectStepRoute } from '@/features/incubator/workspace';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/app/projects/$projectId/steps/$stepKey')({
  staticData: { titleKey: 'projects', bleed: true },
  loader: () => loadNamespaces('incubator', 'ai', 'evaluation', 'showcase'),
  component: ProjectStepRoute,
});

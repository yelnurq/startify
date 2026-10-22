import { createFileRoute } from '@tanstack/react-router';

import { ProjectEvaluationRoute } from '@/features/incubator/workspace';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/app/projects/$projectId/evaluation')({
  staticData: { titleKey: 'projects', bleed: true },
  loader: () => loadNamespaces('incubator', 'ai', 'evaluation', 'showcase'),
  component: ProjectEvaluationRoute,
});

import { createFileRoute } from '@tanstack/react-router';

import { ProjectEvaluationHistoryRoute } from '@/features/incubator/workspace';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/app/projects/$projectId/evaluations/$evaluationId')({
  staticData: { titleKey: 'projects', bleed: true },
  loader: () => loadNamespaces('incubator', 'ai', 'evaluation'),
  component: ProjectEvaluationHistoryRoute,
});

import { createFileRoute } from '@tanstack/react-router';

import { ProjectEvaluationHistoryRoute } from '@/features/incubator/workspace';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/app/projects/$projectId/evaluations/$evaluationId')({
  staticData: { titleKey: 'projects', bleed: true },
  loader: ({ context, params }) =>
    Promise.all([
      loadNamespaces('incubator', 'ai', 'evaluation', 'showcase'),
      // The workspace's data comes with the route (its own chunk keeps the entry small).
      import('@/features/incubator/prefetch').then(({ prefetchWorkspace }) =>
        prefetchWorkspace(context.queryClient, params.projectId),
      ),
    ]),
  component: ProjectEvaluationHistoryRoute,
});

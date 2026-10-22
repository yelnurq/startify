import { createFileRoute } from '@tanstack/react-router';

import { showcaseProjectQuery } from '@/features/showcase/api';
import { ProjectPage } from '@/features/showcase/project-page';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/showcase/$slug')({
  loader: async ({ context, params }) => {
    // The page renders with its data at once; a missing page is shown by the component.
    await Promise.all([
      loadNamespaces('showcase'),
      context.queryClient.query(showcaseProjectQuery(params.slug)).catch(() => undefined),
    ]);
  },
  component: ProjectPage,
});

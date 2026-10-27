import { createFileRoute } from '@tanstack/react-router';

import { WorkspaceDemo } from '@/features/design-system/demos/workspace-demo';

export const Route = createFileRoute('/design/workspace')({
  staticData: { pageTitle: 'design' },
  component: WorkspaceDemo,
});

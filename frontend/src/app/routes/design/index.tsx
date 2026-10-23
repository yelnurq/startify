import { createFileRoute } from '@tanstack/react-router';

import { DesignPage } from '@/features/design-system/design-page';

export const Route = createFileRoute('/design/')({
  staticData: { pageTitle: 'design' },
  component: DesignPage,
});

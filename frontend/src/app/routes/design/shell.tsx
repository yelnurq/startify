import { createFileRoute } from '@tanstack/react-router';

import { ShellDemo } from '@/features/design-system/demos/shell-demo';

export const Route = createFileRoute('/design/shell')({
  staticData: { pageTitle: 'design' },
  component: ShellDemo,
});

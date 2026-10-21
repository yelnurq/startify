import { createFileRoute } from '@tanstack/react-router';

import { AuthDemo } from '@/features/design-system/demos/auth-demo';

export const Route = createFileRoute('/design/auth')({
  component: AuthDemo,
});

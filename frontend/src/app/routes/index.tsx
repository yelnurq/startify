import { createFileRoute } from '@tanstack/react-router';

import { StatusPage } from '@/features/system/status-page';

export const Route = createFileRoute('/')({
  component: StatusPage,
});

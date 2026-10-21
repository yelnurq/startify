import { createFileRoute } from '@tanstack/react-router';

import { LandingPage } from '@/features/landing/landing-page';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/')({
  loader: () => loadNamespaces('landing'),
  component: LandingPage,
});

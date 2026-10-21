import { createFileRoute } from '@tanstack/react-router';

import { SecurityPage } from '@/features/settings/security-page';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/app/settings/security')({
  staticData: { titleKey: 'security' },
  loader: () => loadNamespaces('auth'),
  component: SecurityPage,
});

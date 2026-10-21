import { createFileRoute } from '@tanstack/react-router';

import { ImportPage } from '@/features/admin/import-page';
import { requireRole } from '@/features/auth/guards';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/app/admin/import')({
  staticData: { titleKey: 'import' },
  beforeLoad: ({ context }) => {
    requireRole(context.session, 'tenant_admin');
  },
  loader: () => loadNamespaces('admin'),
  component: ImportPage,
});

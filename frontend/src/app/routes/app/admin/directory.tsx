import { createFileRoute } from '@tanstack/react-router';

import { requireRole } from '@/features/auth/guards';
import { DirectoryPage } from '@/features/directory/directory-page';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/app/admin/directory')({
  staticData: { titleKey: 'directory' },
  beforeLoad: ({ context }) => {
    requireRole(context.session, 'tenant_admin');
  },
  loader: () => loadNamespaces('directory', 'auth'),
  component: DirectoryPage,
});

import { createFileRoute } from '@tanstack/react-router';

import { MembersRoute } from '@/features/admin/members-page';
import { requireRole } from '@/features/auth/guards';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/app/admin/members')({
  staticData: { titleKey: 'members' },
  beforeLoad: ({ context }) => {
    requireRole(context.session, 'tenant_admin', 'innovation_manager');
  },
  loader: () => loadNamespaces('admin'),
  component: MembersRoute,
});

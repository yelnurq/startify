import { createFileRoute } from '@tanstack/react-router';

import { requireRole } from '@/features/auth/guards';
import { ContactsPage } from '@/features/showcase/contacts-page';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/app/admin/contacts')({
  staticData: { titleKey: 'contacts' },
  beforeLoad: ({ context }) => {
    requireRole(context.session, 'tenant_admin', 'innovation_manager');
  },
  loader: () => loadNamespaces('showcase'),
  component: ContactsPage,
});

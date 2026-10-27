import { createFileRoute } from '@tanstack/react-router';

import { requireRole } from '@/features/auth/guards';
import { RubricPage } from '@/features/evaluation/rubric-page';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/app/admin/rubric')({
  staticData: { titleKey: 'rubric' },
  beforeLoad: ({ context }) => {
    requireRole(context.session, 'tenant_admin', 'innovation_manager');
  },
  loader: () => loadNamespaces('evaluation'),
  component: RubricPage,
});

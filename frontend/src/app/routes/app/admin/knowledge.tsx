import { createFileRoute } from '@tanstack/react-router';

import { KnowledgePage } from '@/features/ai/knowledge-page';
import { requireRole } from '@/features/auth/guards';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/app/admin/knowledge')({
  staticData: { titleKey: 'knowledge' },
  beforeLoad: ({ context }) => {
    requireRole(context.session, 'tenant_admin', 'innovation_manager');
  },
  loader: () => loadNamespaces('ai'),
  component: KnowledgePage,
});

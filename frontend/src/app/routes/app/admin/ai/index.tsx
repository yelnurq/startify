import { createFileRoute } from '@tanstack/react-router';

import { AiSettingsPage } from '@/features/ai/settings-page';
import { requireRole } from '@/features/auth/guards';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/app/admin/ai/')({
  staticData: { titleKey: 'ai' },
  beforeLoad: ({ context }) => {
    requireRole(context.session, 'tenant_admin');
  },
  loader: () => loadNamespaces('ai'),
  component: AiSettingsPage,
});

import { createFileRoute } from '@tanstack/react-router';

import { isUsagePeriod } from '@/features/ai/api';
import { AiUsageRoute, type UsageSearch } from '@/features/ai/usage-page';
import { requireRole } from '@/features/auth/guards';
import { loadNamespaces } from '@/shared/i18n';
import { compact } from '@/shared/router/search';

export const Route = createFileRoute('/app/admin/ai/usage')({
  staticData: { titleKey: 'aiUsage' },
  validateSearch: (search: Record<string, unknown>): UsageSearch =>
    compact({ days: isUsagePeriod(search.days) && search.days !== 30 ? search.days : undefined }),
  beforeLoad: ({ context }) => {
    requireRole(context.session, 'tenant_admin', 'leadership');
  },
  loader: () => loadNamespaces('ai'),
  component: AiUsageRoute,
});

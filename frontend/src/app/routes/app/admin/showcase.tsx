import { createFileRoute } from '@tanstack/react-router';

import { requireRole } from '@/features/auth/guards';
import { ModerationPage } from '@/features/showcase/moderation-page';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/app/admin/showcase')({
  staticData: { titleKey: 'showcaseReview' },
  beforeLoad: ({ context }) => {
    requireRole(context.session, 'tenant_admin', 'innovation_manager');
  },
  loader: () => loadNamespaces('showcase'),
  component: ModerationPage,
});

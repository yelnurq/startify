import { createFileRoute } from '@tanstack/react-router';

import { ForgotPasswordPage } from '@/features/auth/recovery-pages';
import { loadNamespaces } from '@/shared/i18n';

export const Route = createFileRoute('/forgot-password')({
  staticData: { pageTitle: 'forgotPassword' },
  loader: () => loadNamespaces('auth'),
  component: ForgotPasswordPage,
});

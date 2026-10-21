import { Link } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';

import { PublicLayout } from '@/app/layouts/public-layout';
import { Button, EmptyState, Logo } from '@/design-system';

export function NotFoundPage() {
  const { t } = useTranslation();
  return (
    <PublicLayout
      logo={
        <Link to="/" className="rounded-lg">
          <Logo />
        </Link>
      }
    >
      <div className="mx-auto flex min-h-[calc(100dvh-4rem)] max-w-page items-center justify-center px-4">
        <EmptyState
          illustration="error"
          title={t('notFound.title')}
          description={t('notFound.description')}
          action={
            <Button asChild>
              <Link to="/">{t('notFound.back')}</Link>
            </Button>
          }
        />
      </div>
    </PublicLayout>
  );
}

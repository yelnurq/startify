import { Link } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';

export function NotFoundPage() {
  const { t } = useTranslation();
  return (
    <main className="mx-auto flex min-h-dvh max-w-md flex-col items-center justify-center gap-4 px-6 text-center">
      <h1 className="text-3xl font-semibold">{t('notFound.title')}</h1>
      <p className="text-slate-600">{t('notFound.description')}</p>
      <Link to="/" className="underline underline-offset-4">
        {t('notFound.back')}
      </Link>
    </main>
  );
}
